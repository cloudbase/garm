package sql

import (
	"context"

	"github.com/google/uuid"
	"github.com/pkg/errors"
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
	enterprise, err := s.getEnterpriseByID(ctx, enterpriseID, "Pools", "Credentials", "Endpoint")
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
	q := s.conn.Preload("Credentials").Find(&enterprises)
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
	enterprise, err := s.getEnterpriseByID(ctx, enterpriseID, "Credentials", "Endpoint")
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

func (s *sqlDatabase) getEnterprise(_ context.Context, name string) (Enterprise, error) {
	var enterprise Enterprise

	q := s.conn.Where("name = ? COLLATE NOCASE", name).
		Preload("Credentials").
		Preload("Endpoint").
		First(&enterprise)
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
