package sql

import (
	"context"

	runnerErrors "github.com/cloudbase/garm/errors"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *sqlDatabase) CreateEnterprise(ctx context.Context, name, credentialsName, webhookSecret string) (params.Enterprise, error) {
	if webhookSecret == "" {
		return params.Enterprise{}, errors.New("creating enterprise: missing secret")
	}
	secret, err := util.Aes256EncodeString(webhookSecret, s.cfg.Passphrase)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "encoding secret")
	}
	newEnterprise := Enterprise{
		Name:            name,
		WebhookSecret:   secret,
		CredentialsName: credentialsName,
	}

	q := s.conn.Create(&newEnterprise)
	if q.Error != nil {
		return params.Enterprise{}, errors.Wrap(q.Error, "creating enterprise")
	}

	param, err := s.sqlToCommonEnterprise(newEnterprise)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "creating enterprise")
	}

	return param, nil
}

func (s *sqlDatabase) GetEnterprise(ctx context.Context, name string) (params.Enterprise, error) {
	enterprise, err := s.getEnterprise(ctx, name)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
	}

	param, err := s.sqlToCommonEnterprise(enterprise)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
	}
	return param, nil
}

func (s *sqlDatabase) GetEnterpriseByID(ctx context.Context, enterpriseID string) (params.Enterprise, error) {
	enterprise, err := s.getEnterpriseByID(ctx, enterpriseID, "Pools")
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
	}

	param, err := s.sqlToCommonEnterprise(enterprise)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
	}
	return param, nil
}

func (s *sqlDatabase) ListEnterprises(ctx context.Context) ([]params.Enterprise, error) {
	var enterprises []Enterprise
	q := s.conn.Find(&enterprises)
	if q.Error != nil {
		return []params.Enterprise{}, errors.Wrap(q.Error, "fetching enterprises")
	}

	ret := make([]params.Enterprise, len(enterprises))
	for idx, val := range enterprises {
		var err error
		ret[idx], err = s.sqlToCommonEnterprise(val)
		if err != nil {
			return nil, errors.Wrap(err, "fetching enterprises")
		}
	}

	return ret, nil
}

func (s *sqlDatabase) DeleteEnterprise(ctx context.Context, enterpriseID string) error {
	enterprise, err := s.getEnterpriseByID(ctx, enterpriseID)
	if err != nil {
		return errors.Wrap(err, "fetching enterprise")
	}

	q := s.conn.Unscoped().Delete(&enterprise)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return errors.Wrap(q.Error, "deleting enterprise")
	}

	return nil
}

func (s *sqlDatabase) UpdateEnterprise(ctx context.Context, enterpriseID string, param params.UpdateRepositoryParams) (params.Enterprise, error) {
	enterprise, err := s.getEnterpriseByID(ctx, enterpriseID)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
	}

	if param.CredentialsName != "" {
		enterprise.CredentialsName = param.CredentialsName
	}

	if param.WebhookSecret != "" {
		secret, err := util.Aes256EncodeString(param.WebhookSecret, s.cfg.Passphrase)
		if err != nil {
			return params.Enterprise{}, errors.Wrap(err, "encoding secret")
		}
		enterprise.WebhookSecret = secret
	}

	q := s.conn.Save(&enterprise)
	if q.Error != nil {
		return params.Enterprise{}, errors.Wrap(q.Error, "saving enterprise")
	}

	newParams, err := s.sqlToCommonEnterprise(enterprise)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "updating enterprise")
	}
	return newParams, nil
}

func (s *sqlDatabase) CreateEnterprisePool(ctx context.Context, enterpriseID string, param params.CreatePoolParams) (params.Pool, error) {
	if len(param.Tags) == 0 {
		return params.Pool{}, runnerErrors.NewBadRequestError("no tags specified")
	}

	enterprise, err := s.getEnterpriseByID(ctx, enterpriseID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching enterprise")
	}

	newPool := Pool{
		ProviderName:           param.ProviderName,
		MaxRunners:             param.MaxRunners,
		MinIdleRunners:         param.MinIdleRunners,
		RunnerPrefix:           param.GetRunnerPrefix(),
		Image:                  param.Image,
		Flavor:                 param.Flavor,
		OSType:                 param.OSType,
		OSArch:                 param.OSArch,
		EnterpriseID:           enterprise.ID,
		Enabled:                param.Enabled,
		RunnerBootstrapTimeout: param.RunnerBootstrapTimeout,
	}

	if len(param.ExtraSpecs) > 0 {
		newPool.ExtraSpecs = datatypes.JSON(param.ExtraSpecs)
	}

	_, err = s.getEnterprisePoolByUniqueFields(ctx, enterpriseID, newPool.ProviderName, newPool.Image, newPool.Flavor)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return params.Pool{}, errors.Wrap(err, "creating pool")
		}
	} else {
		return params.Pool{}, runnerErrors.NewConflictError("pool with the same image and flavor already exists on this provider")
	}

	tags := []Tag{}
	for _, val := range param.Tags {
		t, err := s.getOrCreateTag(val)
		if err != nil {
			return params.Pool{}, errors.Wrap(err, "fetching tag")
		}
		tags = append(tags, t)
	}

	q := s.conn.Create(&newPool)
	if q.Error != nil {
		return params.Pool{}, errors.Wrap(q.Error, "adding pool")
	}

	for _, tt := range tags {
		if err := s.conn.Model(&newPool).Association("Tags").Append(&tt); err != nil {
			return params.Pool{}, errors.Wrap(err, "saving tag")
		}
	}

	pool, err := s.getPoolByID(ctx, newPool.ID.String(), "Tags", "Instances", "Enterprise", "Organization", "Repository")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}

	return s.sqlToCommonPool(pool), nil
}

func (s *sqlDatabase) GetEnterprisePool(ctx context.Context, enterpriseID, poolID string) (params.Pool, error) {
	pool, err := s.getEntityPool(ctx, params.EnterprisePool, enterpriseID, poolID, "Tags", "Instances")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return s.sqlToCommonPool(pool), nil
}

func (s *sqlDatabase) DeleteEnterprisePool(ctx context.Context, enterpriseID, poolID string) error {
	pool, err := s.getEntityPool(ctx, params.EnterprisePool, enterpriseID, poolID)
	if err != nil {
		return errors.Wrap(err, "looking up enterprise pool")
	}
	q := s.conn.Unscoped().Delete(&pool)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return errors.Wrap(q.Error, "deleting pool")
	}
	return nil
}

func (s *sqlDatabase) UpdateEnterprisePool(ctx context.Context, enterpriseID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	pool, err := s.getEntityPool(ctx, params.EnterprisePool, enterpriseID, poolID, "Tags", "Instances", "Enterprise", "Organization", "Repository")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}

	return s.updatePool(pool, param)
}

func (s *sqlDatabase) FindEnterprisePoolByTags(ctx context.Context, enterpriseID string, tags []string) (params.Pool, error) {
	pool, err := s.findPoolByTags(enterpriseID, "enterprise_id", tags)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool, nil
}

func (s *sqlDatabase) ListEnterprisePools(ctx context.Context, enterpriseID string) ([]params.Pool, error) {
	pools, err := s.getEnterprisePools(ctx, enterpriseID, "Tags", "Enterprise")
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}

	ret := make([]params.Pool, len(pools))
	for idx, pool := range pools {
		ret[idx] = s.sqlToCommonPool(pool)
	}

	return ret, nil
}

func (s *sqlDatabase) ListEnterpriseInstances(ctx context.Context, enterpriseID string) ([]params.Instance, error) {
	pools, err := s.getEnterprisePools(ctx, enterpriseID, "Instances")
	if err != nil {
		return nil, errors.Wrap(err, "fetching enterprise")
	}
	ret := []params.Instance{}
	for _, pool := range pools {
		for _, instance := range pool.Instances {
			ret = append(ret, s.sqlToParamsInstance(instance))
		}
	}
	return ret, nil
}

func (s *sqlDatabase) getEnterprise(ctx context.Context, name string) (Enterprise, error) {
	var enterprise Enterprise

	q := s.conn.Where("name = ? COLLATE NOCASE", name)
	q = q.First(&enterprise)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Enterprise{}, runnerErrors.ErrNotFound
		}
		return Enterprise{}, errors.Wrap(q.Error, "fetching enterprise from database")
	}
	return enterprise, nil
}

func (s *sqlDatabase) getEnterpriseByID(ctx context.Context, id string, preload ...string) (Enterprise, error) {
	u, err := uuid.FromString(id)
	if err != nil {
		return Enterprise{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}
	var enterprise Enterprise

	q := s.conn
	if len(preload) > 0 {
		for _, field := range preload {
			q = q.Preload(field)
		}
	}
	q = q.Where("id = ?", u).First(&enterprise)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Enterprise{}, runnerErrors.ErrNotFound
		}
		return Enterprise{}, errors.Wrap(q.Error, "fetching enterprise from database")
	}
	return enterprise, nil
}

func (s *sqlDatabase) getEnterprisePoolByUniqueFields(ctx context.Context, enterpriseID string, provider, image, flavor string) (Pool, error) {
	enterprise, err := s.getEnterpriseByID(ctx, enterpriseID)
	if err != nil {
		return Pool{}, errors.Wrap(err, "fetching enterprise")
	}

	q := s.conn
	var pool []Pool
	err = q.Model(&enterprise).Association("Pools").Find(&pool, "provider_name = ? and image = ? and flavor = ?", provider, image, flavor)
	if err != nil {
		return Pool{}, errors.Wrap(err, "fetching pool")
	}
	if len(pool) == 0 {
		return Pool{}, runnerErrors.ErrNotFound
	}

	return pool[0], nil
}

func (s *sqlDatabase) getEnterprisePools(ctx context.Context, enterpriseID string, preload ...string) ([]Pool, error) {
	_, err := s.getEnterpriseByID(ctx, enterpriseID)
	if err != nil {
		return nil, errors.Wrap(err, "fetching enterprise")
	}

	q := s.conn
	if len(preload) > 0 {
		for _, item := range preload {
			q = q.Preload(item)
		}
	}

	var pools []Pool
	err = q.Model(&Pool{}).Where("enterprise_id = ?", enterpriseID).
		Omit("extra_specs").
		Find(&pools).Error

	if err != nil {
		return nil, errors.Wrap(err, "fetching pool")
	}

	return pools, nil
}
