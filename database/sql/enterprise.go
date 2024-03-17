package sql

import (
	"context"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) CreateEnterprise(_ context.Context, name, credentialsName, webhookSecret string, poolBalancerType params.PoolBalancerType) (params.Enterprise, error) {
	if webhookSecret == "" {
		return params.Enterprise{}, errors.New("creating enterprise: missing secret")
	}
	secret, err := util.Seal([]byte(webhookSecret), []byte(s.cfg.Passphrase))
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "encoding secret")
	}
	newEnterprise := Enterprise{
		Name:             name,
		WebhookSecret:    secret,
		CredentialsName:  credentialsName,
		PoolBalancerType: poolBalancerType,
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

func (s *sqlDatabase) ListEnterprises(_ context.Context) ([]params.Enterprise, error) {
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

func (s *sqlDatabase) UpdateEnterprise(ctx context.Context, enterpriseID string, param params.UpdateEntityParams) (params.Enterprise, error) {
	enterprise, err := s.getEnterpriseByID(ctx, enterpriseID)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
	}

	if param.CredentialsName != "" {
		enterprise.CredentialsName = param.CredentialsName
	}

	if param.WebhookSecret != "" {
		secret, err := util.Seal([]byte(param.WebhookSecret), []byte(s.cfg.Passphrase))
		if err != nil {
			return params.Enterprise{}, errors.Wrap(err, "encoding secret")
		}
		enterprise.WebhookSecret = secret
	}

	if param.PoolBalancerType != "" {
		enterprise.PoolBalancerType = param.PoolBalancerType
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
		EnterpriseID:           &enterprise.ID,
		Enabled:                param.Enabled,
		RunnerBootstrapTimeout: param.RunnerBootstrapTimeout,
		GitHubRunnerGroup:      param.GitHubRunnerGroup,
		Priority:               param.Priority,
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

	for i := range tags {
		if err := s.conn.Model(&newPool).Association("Tags").Append(&tags[i]); err != nil {
			return params.Pool{}, errors.Wrap(err, "saving tag")
		}
	}

	pool, err := s.getPoolByID(ctx, newPool.ID.String(), "Tags", "Instances", "Enterprise", "Organization", "Repository")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}

	return s.sqlToCommonPool(pool)
}

func (s *sqlDatabase) GetEnterprisePool(ctx context.Context, enterpriseID, poolID string) (params.Pool, error) {
	pool, err := s.getEntityPool(ctx, params.GithubEntityTypeEnterprise, enterpriseID, poolID, "Tags", "Instances")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return s.sqlToCommonPool(pool)
}

func (s *sqlDatabase) DeleteEnterprisePool(ctx context.Context, enterpriseID, poolID string) error {
	pool, err := s.getEntityPool(ctx, params.GithubEntityTypeEnterprise, enterpriseID, poolID)
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
	pool, err := s.getEntityPool(ctx, params.GithubEntityTypeEnterprise, enterpriseID, poolID, "Tags", "Instances", "Enterprise", "Organization", "Repository")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}

	return s.updatePool(pool, param)
}

func (s *sqlDatabase) FindEnterprisePoolByTags(_ context.Context, enterpriseID string, tags []string) (params.Pool, error) {
	pool, err := s.findPoolByTags(enterpriseID, params.GithubEntityTypeEnterprise, tags)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool[0], nil
}

func (s *sqlDatabase) ListEnterprisePools(ctx context.Context, enterpriseID string) ([]params.Pool, error) {
	pools, err := s.listEntityPools(ctx, params.GithubEntityTypeEnterprise, enterpriseID, "Tags", "Instances", "Enterprise")
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}

	ret := make([]params.Pool, len(pools))
	for idx, pool := range pools {
		ret[idx], err = s.sqlToCommonPool(pool)
		if err != nil {
			return nil, errors.Wrap(err, "fetching pools")
		}
	}

	return ret, nil
}

func (s *sqlDatabase) ListEnterpriseInstances(ctx context.Context, enterpriseID string) ([]params.Instance, error) {
	pools, err := s.listEntityPools(ctx, params.GithubEntityTypeEnterprise, enterpriseID, "Instances", "Tags", "Instances.Job")
	if err != nil {
		return nil, errors.Wrap(err, "fetching enterprise")
	}
	ret := []params.Instance{}
	for _, pool := range pools {
		for _, instance := range pool.Instances {
			paramsInstance, err := s.sqlToParamsInstance(instance)
			if err != nil {
				return nil, errors.Wrap(err, "fetching instance")
			}
			ret = append(ret, paramsInstance)
		}
	}
	return ret, nil
}

func (s *sqlDatabase) getEnterprise(_ context.Context, name string) (Enterprise, error) {
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

func (s *sqlDatabase) getEnterpriseByID(_ context.Context, id string, preload ...string) (Enterprise, error) {
	u, err := uuid.Parse(id)
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
