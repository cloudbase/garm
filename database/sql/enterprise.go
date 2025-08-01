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

func (s *sqlDatabase) CreateEnterprise(ctx context.Context, name string, credentials params.ForgeCredentials, webhookSecret string, poolBalancerType params.PoolBalancerType) (paramEnt params.Enterprise, err error) {
	if webhookSecret == "" {
		return params.Enterprise{}, errors.New("creating enterprise: missing secret")
	}
	if credentials.ForgeType != params.GithubEndpointType {
		return params.Enterprise{}, errors.Wrap(runnerErrors.ErrBadRequest, "enterprises are not supported on this forge type")
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
		PoolBalancerType: poolBalancerType,
	}
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		newEnterprise.CredentialsID = &credentials.ID
		newEnterprise.EndpointName = &credentials.Endpoint.Name

		q := tx.Create(&newEnterprise)
		if q.Error != nil {
			return errors.Wrap(q.Error, "creating enterprise")
		}

		newEnterprise, err = s.getEnterpriseByID(ctx, tx, newEnterprise.ID.String(), "Pools", "Credentials", "Endpoint", "Credentials.Endpoint")
		if err != nil {
			return errors.Wrap(err, "creating enterprise")
		}
		return nil
	})
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "creating enterprise")
	}

	ret, err := s.GetEnterpriseByID(ctx, newEnterprise.ID.String())
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "creating enterprise")
	}

	return ret, nil
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
	preloadList := []string{
		"Pools",
		"Credentials",
		"Endpoint",
		"Credentials.Endpoint",
		"Events",
	}
	enterprise, err := s.getEnterpriseByID(ctx, s.conn, enterpriseID, preloadList...)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
	}

	param, err := s.sqlToCommonEnterprise(enterprise, true)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
	}
	return param, nil
}

func (s *sqlDatabase) ListEnterprises(_ context.Context, filter params.EnterpriseFilter) ([]params.Enterprise, error) {
	var enterprises []Enterprise
	q := s.conn.
		Preload("Credentials").
		Preload("Credentials.Endpoint").
		Preload("Endpoint")
	if filter.Name != "" {
		q = q.Where("name = ?", filter.Name)
	}
	if filter.Endpoint != "" {
		q = q.Where("endpoint_name = ?", filter.Endpoint)
	}
	q = q.Find(&enterprises)
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
