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

func (s *sqlDatabase) CreateOrganization(ctx context.Context, name string, credentials params.ForgeCredentials, webhookSecret string, poolBalancerType params.PoolBalancerType, agentMode bool) (param params.Organization, err error) {
	if webhookSecret == "" {
		return params.Organization{}, errors.New("creating org: missing secret")
	}
	secret, err := util.Seal([]byte(webhookSecret), []byte(s.cfg.Passphrase))
	if err != nil {
		return params.Organization{}, fmt.Errorf("error encoding secret: %w", err)
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.OrganizationEntityType, common.CreateOperation, param)
		}
	}()
	newOrg := Organization{
		Name:             name,
		WebhookSecret:    secret,
		PoolBalancerType: poolBalancerType,
		AgentMode:        agentMode,
	}

	err = s.conn.Transaction(func(tx *gorm.DB) error {
		switch credentials.ForgeType {
		case params.GithubEndpointType:
			newOrg.CredentialsID = &credentials.ID
		case params.GiteaEndpointType:
			newOrg.GiteaCredentialsID = &credentials.ID
		default:
			return fmt.Errorf("unsupported credentials type: %w", runnerErrors.ErrBadRequest)
		}

		newOrg.EndpointName = &credentials.Endpoint.Name
		q := tx.Create(&newOrg)
		if q.Error != nil {
			return fmt.Errorf("error creating org: %w", q.Error)
		}
		return nil
	})
	if err != nil {
		return params.Organization{}, fmt.Errorf("error creating org: %w", err)
	}

	ret, err := s.GetOrganizationByID(ctx, newOrg.ID.String())
	if err != nil {
		return params.Organization{}, fmt.Errorf("error creating org: %w", err)
	}

	return ret, nil
}

func (s *sqlDatabase) GetOrganization(ctx context.Context, name, endpointName string) (params.Organization, error) {
	org, err := s.getOrg(ctx, name, endpointName)
	if err != nil {
		return params.Organization{}, fmt.Errorf("error fetching org: %w", err)
	}

	param, err := s.sqlToCommonOrganization(org, true)
	if err != nil {
		return params.Organization{}, fmt.Errorf("error fetching org: %w", err)
	}

	return param, nil
}

func (s *sqlDatabase) ListOrganizations(_ context.Context, filter params.OrganizationFilter) ([]params.Organization, error) {
	var orgs []Organization
	q := s.conn.
		Preload("Credentials").
		Preload("GiteaCredentials").
		Preload("Credentials.Endpoint").
		Preload("GiteaCredentials.Endpoint").
		Preload("Endpoint")

	if filter.Name != "" {
		q = q.Where("name = ?", filter.Name)
	}

	if filter.Endpoint != "" {
		q = q.Where("endpoint_name = ?", filter.Endpoint)
	}
	q = q.Find(&orgs)
	if q.Error != nil {
		return []params.Organization{}, fmt.Errorf("error fetching org from database: %w", q.Error)
	}

	ret := make([]params.Organization, len(orgs))
	for idx, val := range orgs {
		var err error
		ret[idx], err = s.sqlToCommonOrganization(val, true)
		if err != nil {
			return nil, fmt.Errorf("error fetching org: %w", err)
		}
	}

	return ret, nil
}

func (s *sqlDatabase) DeleteOrganization(ctx context.Context, orgID string) (err error) {
	org, err := s.getOrgByID(ctx, s.conn, orgID, "Endpoint", "Credentials", "Credentials.Endpoint", "GiteaCredentials", "GiteaCredentials.Endpoint")
	if err != nil {
		return fmt.Errorf("error fetching org: %w", err)
	}

	defer func(org Organization) {
		if err == nil {
			asParam, innerErr := s.sqlToCommonOrganization(org, true)
			if innerErr == nil {
				s.sendNotify(common.OrganizationEntityType, common.DeleteOperation, asParam)
			} else {
				slog.With(slog.Any("error", innerErr)).ErrorContext(ctx, "error sending delete notification", "org", orgID)
			}
		}
	}(org)

	q := s.conn.Unscoped().Delete(&org)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("error deleting org: %w", q.Error)
	}

	return nil
}

func (s *sqlDatabase) UpdateOrganization(ctx context.Context, orgID string, param params.UpdateEntityParams) (paramOrg params.Organization, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.OrganizationEntityType, common.UpdateOperation, paramOrg)
		}
	}()
	var org Organization
	var creds GithubCredentials
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		var err error
		org, err = s.getOrgByID(ctx, tx, orgID)
		if err != nil {
			return fmt.Errorf("error fetching org: %w", err)
		}
		if org.EndpointName == nil {
			return fmt.Errorf("error org has no endpoint: %w", runnerErrors.ErrUnprocessable)
		}

		if param.CredentialsName != "" {
			creds, err = s.getGithubCredentialsByName(ctx, tx, param.CredentialsName, false)
			if err != nil {
				return fmt.Errorf("error fetching credentials: %w", err)
			}
			if creds.EndpointName == nil {
				return fmt.Errorf("error credentials have no endpoint: %w", runnerErrors.ErrUnprocessable)
			}

			if *creds.EndpointName != *org.EndpointName {
				return fmt.Errorf("error endpoint mismatch: %w", runnerErrors.ErrBadRequest)
			}
			org.CredentialsID = &creds.ID
		}

		if param.WebhookSecret != "" {
			secret, err := util.Seal([]byte(param.WebhookSecret), []byte(s.cfg.Passphrase))
			if err != nil {
				return fmt.Errorf("saving org: failed to encrypt string: %w", err)
			}
			org.WebhookSecret = secret
		}

		if param.PoolBalancerType != "" {
			org.PoolBalancerType = param.PoolBalancerType
		}

		if param.AgentMode != nil {
			org.AgentMode = *param.AgentMode
		}

		q := tx.Save(&org)
		if q.Error != nil {
			return fmt.Errorf("error saving org: %w", q.Error)
		}

		return nil
	})
	if err != nil {
		return params.Organization{}, fmt.Errorf("error saving org: %w", err)
	}

	org, err = s.getOrgByID(ctx, s.conn, orgID, "Endpoint", "Credentials", "Credentials.Endpoint", "GiteaCredentials", "GiteaCredentials.Endpoint")
	if err != nil {
		return params.Organization{}, fmt.Errorf("error updating enterprise: %w", err)
	}
	paramOrg, err = s.sqlToCommonOrganization(org, true)
	if err != nil {
		return params.Organization{}, fmt.Errorf("error saving org: %w", err)
	}
	return paramOrg, nil
}

func (s *sqlDatabase) GetOrganizationByID(ctx context.Context, orgID string) (params.Organization, error) {
	preloadList := []string{
		"Pools",
		"Credentials",
		"Endpoint",
		"Credentials.Endpoint",
		"GiteaCredentials",
		"GiteaCredentials.Endpoint",
		"Events",
	}
	org, err := s.getOrgByID(ctx, s.conn, orgID, preloadList...)
	if err != nil {
		return params.Organization{}, fmt.Errorf("error fetching org: %w", err)
	}

	param, err := s.sqlToCommonOrganization(org, true)
	if err != nil {
		return params.Organization{}, fmt.Errorf("error fetching org: %w", err)
	}
	return param, nil
}

func (s *sqlDatabase) getOrgByID(_ context.Context, db *gorm.DB, id string, preload ...string) (Organization, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return Organization{}, fmt.Errorf("error parsing id: %w", runnerErrors.ErrBadRequest)
	}
	var org Organization

	q := db
	if len(preload) > 0 {
		for _, field := range preload {
			q = q.Preload(field)
		}
	}
	q = q.Where("id = ?", u).First(&org)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Organization{}, runnerErrors.ErrNotFound
		}
		return Organization{}, fmt.Errorf("error fetching org from database: %w", q.Error)
	}
	return org, nil
}

func (s *sqlDatabase) getOrg(_ context.Context, name, endpointName string) (Organization, error) {
	var org Organization

	q := s.conn.Where("name = ? COLLATE NOCASE and endpoint_name = ? COLLATE NOCASE", name, endpointName).
		Preload("Credentials").
		Preload("GiteaCredentials").
		Preload("Credentials.Endpoint").
		Preload("GiteaCredentials.Endpoint").
		Preload("Endpoint").
		First(&org)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Organization{}, runnerErrors.ErrNotFound
		}
		return Organization{}, fmt.Errorf("error fetching org from database: %w", q.Error)
	}
	return org, nil
}
