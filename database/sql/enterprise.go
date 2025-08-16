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
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
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
		return params.Enterprise{}, fmt.Errorf("enterprises are not supported on this forge type: %w", runnerErrors.ErrBadRequest)
	}

	secret, err := util.Seal([]byte(webhookSecret), []byte(s.cfg.Passphrase))
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error encoding secret: %w", err)
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
			return fmt.Errorf("error creating enterprise: %w", q.Error)
		}

		newEnterprise, err = s.getEnterpriseByID(ctx, tx, newEnterprise.ID.String(), "Pools", "Credentials", "Endpoint", "Credentials.Endpoint")
		if err != nil {
			return fmt.Errorf("error creating enterprise: %w", err)
		}
		return nil
	})
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error creating enterprise: %w", err)
	}

	ret, err := s.GetEnterpriseByID(ctx, newEnterprise.ID.String())
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error creating enterprise: %w", err)
	}

	return ret, nil
}

func (s *sqlDatabase) GetEnterprise(ctx context.Context, name, endpointName string) (params.Enterprise, error) {
	enterprise, err := s.getEnterprise(ctx, name, endpointName)
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error fetching enterprise: %w", err)
	}

	param, err := s.sqlToCommonEnterprise(enterprise, true)
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error fetching enterprise: %w", err)
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
		return params.Enterprise{}, fmt.Errorf("error fetching enterprise: %w", err)
	}

	param, err := s.sqlToCommonEnterprise(enterprise, true)
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error fetching enterprise: %w", err)
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
		return []params.Enterprise{}, fmt.Errorf("error fetching enterprises: %w", q.Error)
	}

	ret := make([]params.Enterprise, len(enterprises))
	for idx, val := range enterprises {
		var err error
		ret[idx], err = s.sqlToCommonEnterprise(val, true)
		if err != nil {
			return nil, fmt.Errorf("error fetching enterprises: %w", err)
		}
	}

	return ret, nil
}

func (s *sqlDatabase) DeleteEnterprise(ctx context.Context, enterpriseID string) error {
	enterprise, err := s.getEnterpriseByID(ctx, s.conn, enterpriseID, "Endpoint", "Credentials", "Credentials.Endpoint")
	if err != nil {
		return fmt.Errorf("error fetching enterprise: %w", err)
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
		return fmt.Errorf("error deleting enterprise: %w", q.Error)
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
			return fmt.Errorf("error fetching enterprise: %w", err)
		}

		if enterprise.EndpointName == nil {
			return fmt.Errorf("error enterprise has no endpoint: %w", runnerErrors.ErrUnprocessable)
		}

		if param.CredentialsName != "" {
			creds, err = s.getGithubCredentialsByName(ctx, tx, param.CredentialsName, false)
			if err != nil {
				return fmt.Errorf("error fetching credentials: %w", err)
			}
			if creds.EndpointName == nil {
				return fmt.Errorf("error credentials have no endpoint: %w", runnerErrors.ErrUnprocessable)
			}

			if *creds.EndpointName != *enterprise.EndpointName {
				return fmt.Errorf("error endpoint mismatch: %w", runnerErrors.ErrBadRequest)
			}
			enterprise.CredentialsID = &creds.ID
		}
		if param.WebhookSecret != "" {
			secret, err := util.Seal([]byte(param.WebhookSecret), []byte(s.cfg.Passphrase))
			if err != nil {
				return fmt.Errorf("error encoding secret: %w", err)
			}
			enterprise.WebhookSecret = secret
		}

		if param.PoolBalancerType != "" {
			enterprise.PoolBalancerType = param.PoolBalancerType
		}

		q := tx.Save(&enterprise)
		if q.Error != nil {
			return fmt.Errorf("error saving enterprise: %w", q.Error)
		}

		return nil
	})
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error updating enterprise: %w", err)
	}

	enterprise, err = s.getEnterpriseByID(ctx, s.conn, enterpriseID, "Endpoint", "Credentials", "Credentials.Endpoint")
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error updating enterprise: %w", err)
	}
	newParams, err = s.sqlToCommonEnterprise(enterprise, true)
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error updating enterprise: %w", err)
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
		return Enterprise{}, fmt.Errorf("error fetching enterprise from database: %w", q.Error)
	}
	return enterprise, nil
}

func (s *sqlDatabase) getEnterpriseByID(_ context.Context, tx *gorm.DB, id string, preload ...string) (Enterprise, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return Enterprise{}, fmt.Errorf("error parsing id: %w", runnerErrors.ErrBadRequest)
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
		return Enterprise{}, fmt.Errorf("error fetching enterprise from database: %w", q.Error)
	}
	return enterprise, nil
}
