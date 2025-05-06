// Copyright 2024 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.

package sql

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) CreateEnterprise(ctx context.Context, name, credentialsName, webhookSecret string, poolBalancerType params.PoolBalancerType) (paramEnt params.Enterprise, err error) {
	if webhookSecret == "" {
		return params.Enterprise{}, errors.New("creating enterprise: missing secret")
	}
	secret, err := util.Seal([]byte(webhookSecret), []byte(s.cfg.Passphrase))
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "encoding secret")
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.EnterpriseEntityType, common.CreateOperation, paramEnt)
		}
	}()
	newEnterprise := Enterprise{
		Name:             name,
		WebhookSecret:    secret,
		CredentialsName:  credentialsName,
		PoolBalancerType: poolBalancerType,
	}
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		creds, err := s.getGithubCredentialsByName(ctx, tx, credentialsName, false)
		if err != nil {
			return errors.Wrap(err, "creating enterprise")
		}
		if creds.EndpointName == nil {
			return errors.Wrap(runnerErrors.ErrUnprocessable, "credentials have no endpoint")
		}
		newEnterprise.CredentialsID = &creds.ID
		newEnterprise.CredentialsName = creds.Name
		newEnterprise.EndpointName = creds.EndpointName

		q := tx.Create(&newEnterprise)
		if q.Error != nil {
			return errors.Wrap(q.Error, "creating enterprise")
		}

		newEnterprise.Credentials = creds
		newEnterprise.Endpoint = creds.Endpoint

		return nil
	})
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "creating enterprise")
	}

	paramEnt, err = s.sqlToCommonEnterprise(newEnterprise, true)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "creating enterprise")
	}

	return paramEnt, nil
}

func (s *sqlDatabase) GetEnterprise(ctx context.Context, name, endpointName string) (params.Enterprise, error) {
	enterprise, err := s.getEnterprise(ctx, name, endpointName)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
	}

	param, err := s.sqlToCommonEnterprise(enterprise, true)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
	}
	return param, nil
}

func (s *sqlDatabase) GetEnterpriseByID(ctx context.Context, enterpriseID string) (params.Enterprise, error) {
	enterprise, err := s.getEnterpriseByID(ctx, s.conn, enterpriseID, "Pools", "Credentials", "Endpoint", "Credentials.Endpoint")
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
	}

	param, err := s.sqlToCommonEnterprise(enterprise, true)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
	}
	return param, nil
}

func (s *sqlDatabase) ListEnterprises(_ context.Context) ([]params.Enterprise, error) {
	var enterprises []Enterprise
	q := s.conn.
		Preload("Credentials").
		Preload("Credentials.Endpoint").
		Preload("Endpoint").
		Find(&enterprises)
	if q.Error != nil {
		return []params.Enterprise{}, errors.Wrap(q.Error, "fetching enterprises")
	}

	ret := make([]params.Enterprise, len(enterprises))
	for idx, val := range enterprises {
		var err error
		ret[idx], err = s.sqlToCommonEnterprise(val, true)
		if err != nil {
			return nil, errors.Wrap(err, "fetching enterprises")
		}
	}

	return ret, nil
}

func (s *sqlDatabase) DeleteEnterprise(ctx context.Context, enterpriseID string) error {
	enterprise, err := s.getEnterpriseByID(ctx, s.conn, enterpriseID, "Endpoint", "Credentials", "Credentials.Endpoint")
	if err != nil {
		return errors.Wrap(err, "fetching enterprise")
	}

	defer func(ent Enterprise) {
		if err == nil {
			asParams, innerErr := s.sqlToCommonEnterprise(ent, true)
			if innerErr == nil {
				s.sendNotify(common.EnterpriseEntityType, common.DeleteOperation, asParams)
			} else {
				slog.With(slog.Any("error", innerErr)).ErrorContext(ctx, "error sending delete notification", "enterprise", enterpriseID)
			}
		}
	}(enterprise)

	q := s.conn.Unscoped().Delete(&enterprise)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return errors.Wrap(q.Error, "deleting enterprise")
	}

	return nil
}

func (s *sqlDatabase) UpdateEnterprise(ctx context.Context, enterpriseID string, param params.UpdateEntityParams) (newParams params.Enterprise, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.EnterpriseEntityType, common.UpdateOperation, newParams)
		}
	}()
	var enterprise Enterprise
	var creds GithubCredentials
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		var err error
		enterprise, err = s.getEnterpriseByID(ctx, tx, enterpriseID)
		if err != nil {
			return errors.Wrap(err, "fetching enterprise")
		}

		if enterprise.EndpointName == nil {
			return errors.Wrap(runnerErrors.ErrUnprocessable, "enterprise has no endpoint")
		}

		if param.CredentialsName != "" {
			creds, err = s.getGithubCredentialsByName(ctx, tx, param.CredentialsName, false)
			if err != nil {
				return errors.Wrap(err, "fetching credentials")
			}
			if creds.EndpointName == nil {
				return errors.Wrap(runnerErrors.ErrUnprocessable, "credentials have no endpoint")
			}

			if *creds.EndpointName != *enterprise.EndpointName {
				return errors.Wrap(runnerErrors.ErrBadRequest, "endpoint mismatch")
			}
			enterprise.CredentialsID = &creds.ID
		}
		if param.WebhookSecret != "" {
			secret, err := util.Seal([]byte(param.WebhookSecret), []byte(s.cfg.Passphrase))
			if err != nil {
				return errors.Wrap(err, "encoding secret")
			}
			enterprise.WebhookSecret = secret
		}

		if param.PoolBalancerType != "" {
			enterprise.PoolBalancerType = param.PoolBalancerType
		}

		q := tx.Save(&enterprise)
		if q.Error != nil {
			return errors.Wrap(q.Error, "saving enterprise")
		}

		return nil
	})
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "updating enterprise")
	}

	enterprise, err = s.getEnterpriseByID(ctx, s.conn, enterpriseID, "Endpoint", "Credentials", "Credentials.Endpoint")
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "updating enterprise")
	}
	newParams, err = s.sqlToCommonEnterprise(enterprise, true)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "updating enterprise")
	}
	return newParams, nil
}

func (s *sqlDatabase) getEnterprise(_ context.Context, name, endpointName string) (Enterprise, error) {
	var enterprise Enterprise

	q := s.conn.Where("name = ? COLLATE NOCASE and endpoint_name = ? COLLATE NOCASE", name, endpointName).
		Preload("Credentials").
		Preload("Credentials.Endpoint").
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

func (s *sqlDatabase) getEnterpriseByID(_ context.Context, tx *gorm.DB, id string, preload ...string) (Enterprise, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return Enterprise{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}
	var enterprise Enterprise

	q := tx
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
