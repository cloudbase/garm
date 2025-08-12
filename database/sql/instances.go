// Copyright 2022 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package sql

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) CreateInstance(_ context.Context, poolID string, param params.CreateInstanceParams) (instance params.Instance, err error) {
	pool, err := s.getPoolByID(s.conn, poolID)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching pool")
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.InstanceEntityType, common.CreateOperation, instance)
		}
	}()

	var labels datatypes.JSON
	if len(param.AditionalLabels) > 0 {
		labels, err = json.Marshal(param.AditionalLabels)
		if err != nil {
			return params.Instance{}, errors.Wrap(err, "marshalling labels")
		}
	}

	var secret []byte
	if len(param.JitConfiguration) > 0 {
		secret, err = s.marshalAndSeal(param.JitConfiguration)
		if err != nil {
			return params.Instance{}, errors.Wrap(err, "marshalling jit config")
		}
	}

	newInstance := Instance{
		Pool:              pool,
		Name:              param.Name,
		Status:            param.Status,
		RunnerStatus:      param.RunnerStatus,
		OSType:            param.OSType,
		OSArch:            param.OSArch,
		CallbackURL:       param.CallbackURL,
		MetadataURL:       param.MetadataURL,
		GitHubRunnerGroup: param.GitHubRunnerGroup,
		JitConfiguration:  secret,
		AditionalLabels:   labels,
		AgentID:           param.AgentID,
	}
	q := s.conn.Create(&newInstance)
	if q.Error != nil {
		return params.Instance{}, errors.Wrap(q.Error, "creating instance")
	}

	return s.sqlToParamsInstance(newInstance)
}

func (s *sqlDatabase) getPoolInstanceByName(poolID string, instanceName string) (Instance, error) {
	pool, err := s.getPoolByID(s.conn, poolID)
	if err != nil {
		return Instance{}, errors.Wrap(err, "fetching pool")
	}

	var instance Instance
	q := s.conn.Model(&Instance{}).
		Preload(clause.Associations).
		Where("name = ? and pool_id = ?", instanceName, pool.ID).
		First(&instance)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Instance{}, errors.Wrap(runnerErrors.ErrNotFound, "fetching pool instance by name")
		}
		return Instance{}, errors.Wrap(q.Error, "fetching pool instance by name")
	}

	instance.Pool = pool
	return instance, nil
}

func (s *sqlDatabase) getInstanceByName(_ context.Context, instanceName string, preload ...string) (Instance, error) {
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
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Instance{}, errors.Wrap(runnerErrors.ErrNotFound, "fetching instance by name")
		}
		return Instance{}, errors.Wrap(q.Error, "fetching instance by name")
	}
	return instance, nil
}

func (s *sqlDatabase) GetPoolInstanceByName(_ context.Context, poolID string, instanceName string) (params.Instance, error) {
	instance, err := s.getPoolInstanceByName(poolID, instanceName)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching instance")
	}

	return s.sqlToParamsInstance(instance)
}

func (s *sqlDatabase) GetInstanceByName(ctx context.Context, instanceName string) (params.Instance, error) {
	instance, err := s.getInstanceByName(ctx, instanceName, "StatusMessages", "Pool", "ScaleSet")
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching instance")
	}

	return s.sqlToParamsInstance(instance)
}

func (s *sqlDatabase) DeleteInstance(_ context.Context, poolID string, instanceName string) (err error) {
	instance, err := s.getPoolInstanceByName(poolID, instanceName)
	if err != nil {
		if errors.Is(err, runnerErrors.ErrNotFound) {
			return nil
		}
		return errors.Wrap(err, "deleting instance")
	}

	defer func() {
		if err == nil {
			var providerID string
			if instance.ProviderID != nil {
				providerID = *instance.ProviderID
			}
			instanceNotif := params.Instance{
				ID:         instance.ID.String(),
				Name:       instance.Name,
				ProviderID: providerID,
				AgentID:    instance.AgentID,
			}
			switch {
			case instance.PoolID != nil:
				instanceNotif.PoolID = instance.PoolID.String()
			case instance.ScaleSetFkID != nil:
				instanceNotif.ScaleSetID = *instance.ScaleSetFkID
			}

			if notifyErr := s.sendNotify(common.InstanceEntityType, common.DeleteOperation, instanceNotif); notifyErr != nil {
				slog.With(slog.Any("error", notifyErr)).Error("failed to send notify")
			}
		}
	}()

	if q := s.conn.Unscoped().Delete(&instance); q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		return errors.Wrap(q.Error, "deleting instance")
	}
	return nil
}

func (s *sqlDatabase) DeleteInstanceByName(ctx context.Context, instanceName string) error {
	instance, err := s.getInstanceByName(ctx, instanceName, "Pool", "ScaleSet")
	if err != nil {
		if errors.Is(err, runnerErrors.ErrNotFound) {
			return nil
		}
		return errors.Wrap(err, "deleting instance")
	}

	defer func() {
		if err == nil {
			var providerID string
			if instance.ProviderID != nil {
				providerID = *instance.ProviderID
			}
			payload := params.Instance{
				ID:         instance.ID.String(),
				Name:       instance.Name,
				ProviderID: providerID,
				AgentID:    instance.AgentID,
			}
			if instance.PoolID != nil {
				payload.PoolID = instance.PoolID.String()
			}
			if instance.ScaleSetFkID != nil {
				payload.ScaleSetID = *instance.ScaleSetFkID
			}
			if notifyErr := s.sendNotify(common.InstanceEntityType, common.DeleteOperation, payload); notifyErr != nil {
				slog.With(slog.Any("error", notifyErr)).Error("failed to send notify")
			}
		}
	}()

	if q := s.conn.Unscoped().Delete(&instance); q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		return errors.Wrap(q.Error, "deleting instance")
	}
	return nil
}

func (s *sqlDatabase) AddInstanceEvent(ctx context.Context, instanceName string, event params.EventType, eventLevel params.EventLevel, statusMessage string) error {
	instance, err := s.getInstanceByName(ctx, instanceName)
	if err != nil {
		return errors.Wrap(err, "updating instance")
	}

	msg := InstanceStatusUpdate{
		Message:    statusMessage,
		EventType:  event,
		EventLevel: eventLevel,
	}

	if err := s.conn.Model(&instance).Association("StatusMessages").Append(&msg); err != nil {
		return errors.Wrap(err, "adding status message")
	}
	return nil
}

func (s *sqlDatabase) UpdateInstance(ctx context.Context, instanceName string, param params.UpdateInstanceParams) (params.Instance, error) {
	instance, err := s.getInstanceByName(ctx, instanceName, "Pool", "ScaleSet")
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "updating instance")
	}

	if param.AgentID != 0 {
		instance.AgentID = param.AgentID
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
	if param.CreateAttempt != 0 {
		instance.CreateAttempt = param.CreateAttempt
	}

	if param.TokenFetched != nil {
		instance.TokenFetched = *param.TokenFetched
	}

	if param.JitConfiguration != nil {
		secret, err := s.marshalAndSeal(param.JitConfiguration)
		if err != nil {
			return params.Instance{}, errors.Wrap(err, "marshalling jit config")
		}
		instance.JitConfiguration = secret
	}

	instance.ProviderFault = param.ProviderFault

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
	inst, err := s.sqlToParamsInstance(instance)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "converting instance")
	}
	s.sendNotify(common.InstanceEntityType, common.UpdateOperation, inst)
	return inst, nil
}

func (s *sqlDatabase) ListPoolInstances(_ context.Context, poolID string) ([]params.Instance, error) {
	u, err := uuid.Parse(poolID)
	if err != nil {
		return nil, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}

	var instances []Instance
	query := s.conn.
		Preload("Pool").
		Preload("Job").
		Where("pool_id = ?", u)

	if err := query.Find(&instances); err.Error != nil {
		return nil, errors.Wrap(err.Error, "fetching instances")
	}

	ret := make([]params.Instance, len(instances))
	for idx, inst := range instances {
		ret[idx], err = s.sqlToParamsInstance(inst)
		if err != nil {
			return nil, errors.Wrap(err, "converting instance")
		}
	}
	return ret, nil
}

func (s *sqlDatabase) ListAllInstances(_ context.Context) ([]params.Instance, error) {
	var instances []Instance

	q := s.conn.
		Preload("Pool").
		Preload("ScaleSet").
		Preload("Job").
		Find(&instances)
	if q.Error != nil {
		return nil, errors.Wrap(q.Error, "fetching instances")
	}
	ret := make([]params.Instance, len(instances))
	var err error
	for idx, instance := range instances {
		ret[idx], err = s.sqlToParamsInstance(instance)
		if err != nil {
			return nil, errors.Wrap(err, "converting instance")
		}
	}
	return ret, nil
}

func (s *sqlDatabase) PoolInstanceCount(_ context.Context, poolID string) (int64, error) {
	pool, err := s.getPoolByID(s.conn, poolID)
	if err != nil {
		return 0, errors.Wrap(err, "fetching pool")
	}

	var cnt int64
	q := s.conn.Model(&Instance{}).Where("pool_id = ?", pool.ID).Count(&cnt)
	if q.Error != nil {
		return 0, errors.Wrap(q.Error, "fetching instance count")
	}
	return cnt, nil
}
