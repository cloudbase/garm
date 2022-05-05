package sql

import (
	"context"
	runnerErrors "garm/errors"
	"garm/params"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *sqlDatabase) CreateInstance(ctx context.Context, poolID string, param params.CreateInstanceParams) (params.Instance, error) {
	pool, err := s.getPoolByID(ctx, param.Pool)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching pool")
	}
	newInstance := Instance{
		Pool:         pool,
		Name:         param.Name,
		Status:       param.Status,
		RunnerStatus: param.RunnerStatus,
		OSType:       param.OSType,
		OSArch:       param.OSArch,
		CallbackURL:  param.CallbackURL,
	}
	q := s.conn.Create(&newInstance)
	if q.Error != nil {
		return params.Instance{}, errors.Wrap(q.Error, "creating repository")
	}

	return s.sqlToParamsInstance(newInstance), nil
}

func (s *sqlDatabase) getInstanceByID(ctx context.Context, instanceID string) (Instance, error) {
	u, err := uuid.FromString(instanceID)
	if err != nil {
		return Instance{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}
	var instance Instance
	q := s.conn.Model(&Instance{}).
		Preload(clause.Associations).
		Where("id = ?", u).
		First(&instance)
	if q.Error != nil {
		return Instance{}, errors.Wrap(q.Error, "fetching instance")
	}
	return instance, nil
}

func (s *sqlDatabase) getPoolInstanceByName(ctx context.Context, poolID string, instanceName string) (Instance, error) {
	pool, err := s.getPoolByID(ctx, poolID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Instance{}, errors.Wrap(runnerErrors.ErrNotFound, "fetching instance")
		}
		return Instance{}, errors.Wrap(err, "fetching pool")
	}

	var instance Instance
	q := s.conn.Model(&Instance{}).
		Preload(clause.Associations).
		Where("name = ? and pool_id = ?", instanceName, pool.ID).
		First(&instance)
	if q.Error != nil {
		return Instance{}, errors.Wrap(q.Error, "fetching instance")
	}
	return instance, nil
}

func (s *sqlDatabase) getInstanceByName(ctx context.Context, instanceName string, preload ...string) (Instance, error) {
	var instance Instance

	q := s.conn

	if len(preload) > 0 {
		for _, item := range preload {
			q = q.Preload(item)
		}
	}

	q = q.Model(&Instance{}).
		Preload(clause.Associations).
		Where("name = ?", instanceName).
		First(&instance)
	if q.Error != nil {
		return Instance{}, errors.Wrap(q.Error, "fetching instance")
	}
	return instance, nil
}

func (s *sqlDatabase) GetPoolInstanceByName(ctx context.Context, poolID string, instanceName string) (params.Instance, error) {
	instance, err := s.getPoolInstanceByName(ctx, poolID, instanceName)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching instance")
	}
	return s.sqlToParamsInstance(instance), nil
}

func (s *sqlDatabase) GetInstanceByName(ctx context.Context, instanceName string) (params.Instance, error) {
	instance, err := s.getInstanceByName(ctx, instanceName, "StatusMessages")
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching instance")
	}
	return s.sqlToParamsInstance(instance), nil
}

func (s *sqlDatabase) DeleteInstance(ctx context.Context, poolID string, instanceName string) error {
	instance, err := s.getPoolInstanceByName(ctx, poolID, instanceName)
	if err != nil {
		return errors.Wrap(err, "deleting instance")
	}
	if q := s.conn.Unscoped().Delete(&instance); q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		return errors.Wrap(q.Error, "deleting instance")
	}
	return nil
}

func (s *sqlDatabase) AddInstanceStatusMessage(ctx context.Context, instanceID string, statusMessage string) error {
	instance, err := s.getInstanceByID(ctx, instanceID)
	if err != nil {
		return errors.Wrap(err, "updating instance")
	}

	msg := InstanceStatusUpdate{
		Message: statusMessage,
	}

	if err := s.conn.Model(&instance).Association("StatusMessages").Append(&msg); err != nil {
		return errors.Wrap(err, "adding status message")
	}
	return nil
}

func (s *sqlDatabase) UpdateInstance(ctx context.Context, instanceID string, param params.UpdateInstanceParams) (params.Instance, error) {
	instance, err := s.getInstanceByID(ctx, instanceID)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "updating instance")
	}

	if param.ProviderID != "" {
		instance.ProviderID = &param.ProviderID
	}

	if param.OSName != "" {
		instance.OSName = param.OSName
	}

	if param.OSVersion != "" {
		instance.OSVersion = param.OSVersion
	}

	if string(param.RunnerStatus) != "" {
		instance.RunnerStatus = param.RunnerStatus
	}

	if string(param.Status) != "" {
		instance.Status = param.Status
	}

	q := s.conn.Save(&instance)
	if q.Error != nil {
		return params.Instance{}, errors.Wrap(q.Error, "updating instance")
	}

	if len(param.Addresses) > 0 {
		addrs := []Address{}
		for _, addr := range param.Addresses {
			addrs = append(addrs, Address{
				Address: addr.Address,
				Type:    string(addr.Type),
			})
		}
		if err := s.conn.Model(&instance).Association("Addresses").Replace(addrs); err != nil {
			return params.Instance{}, errors.Wrap(err, "updating addresses")
		}
	}
	return s.sqlToParamsInstance(instance), nil
}

func (s *sqlDatabase) ListPoolInstances(ctx context.Context, poolID string) ([]params.Instance, error) {
	pool, err := s.getPoolByID(ctx, poolID, "Tags", "Instances")
	if err != nil {
		return nil, errors.Wrap(err, "fetching pool")
	}

	ret := make([]params.Instance, len(pool.Instances))
	for idx, inst := range pool.Instances {
		ret[idx] = s.sqlToParamsInstance(inst)
	}
	return ret, nil
}

func (s *sqlDatabase) ListAllInstances(ctx context.Context) ([]params.Instance, error) {
	var instances []Instance

	q := s.conn.Model(&Instance{}).Find(&instances)
	if q.Error != nil {
		return nil, errors.Wrap(q.Error, "fetching instances")
	}
	ret := make([]params.Instance, len(instances))
	for idx, instance := range instances {
		ret[idx] = s.sqlToParamsInstance(instance)
	}
	return ret, nil
}
