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
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) CreateOrganization(ctx context.Context, name, credentialsName, webhookSecret string, poolBalancerType params.PoolBalancerType) (org params.Organization, err error) {
	if webhookSecret == "" {
		return params.Organization{}, errors.New("creating org: missing secret")
	}
	secret, err := util.Seal([]byte(webhookSecret), []byte(s.cfg.Passphrase))
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "encoding secret")
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.OrganizationEntityType, common.CreateOperation, org)
		}
	}()
	newOrg := Organization{
		Name:             name,
		WebhookSecret:    secret,
		CredentialsName:  credentialsName,
		PoolBalancerType: poolBalancerType,
	}

	err = s.conn.Transaction(func(tx *gorm.DB) error {
		creds, err := s.getGithubCredentialsByName(ctx, tx, credentialsName, false)
		if err != nil {
			return errors.Wrap(err, "creating org")
		}
		if creds.EndpointName == nil {
			return errors.Wrap(runnerErrors.ErrUnprocessable, "credentials have no endpoint")
		}
		newOrg.CredentialsID = &creds.ID
		newOrg.CredentialsName = creds.Name
		newOrg.EndpointName = creds.EndpointName

		q := tx.Create(&newOrg)
		if q.Error != nil {
			return errors.Wrap(q.Error, "creating org")
		}

		newOrg.Credentials = creds
		newOrg.Endpoint = creds.Endpoint

		return nil
	})
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "creating org")
	}

	org, err = s.sqlToCommonOrganization(newOrg, true)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "creating org")
	}
	org.WebhookSecret = webhookSecret

	return org, nil
}

func (s *sqlDatabase) GetOrganization(ctx context.Context, name string) (params.Organization, error) {
	org, err := s.getOrg(ctx, name)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "fetching org")
	}

	param, err := s.sqlToCommonOrganization(org, true)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "fetching org")
	}

	return param, nil
}

func (s *sqlDatabase) ListOrganizations(_ context.Context) ([]params.Organization, error) {
	var orgs []Organization
	q := s.conn.
		Preload("Credentials").
		Preload("Credentials.Endpoint").
		Preload("Endpoint").
		Find(&orgs)
	if q.Error != nil {
		return []params.Organization{}, errors.Wrap(q.Error, "fetching org from database")
	}

	ret := make([]params.Organization, len(orgs))
	for idx, val := range orgs {
		var err error
		ret[idx], err = s.sqlToCommonOrganization(val, true)
		if err != nil {
			return nil, errors.Wrap(err, "fetching org")
		}
	}

	return ret, nil
}

func (s *sqlDatabase) DeleteOrganization(ctx context.Context, orgID string) (err error) {
	org, err := s.getOrgByID(ctx, s.conn, orgID, "Endpoint", "Credentials", "Credentials.Endpoint")
	if err != nil {
		return errors.Wrap(err, "fetching org")
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
		return errors.Wrap(q.Error, "deleting org")
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
			return errors.Wrap(err, "fetching org")
		}
		if org.EndpointName == nil {
			return errors.Wrap(runnerErrors.ErrUnprocessable, "org has no endpoint")
		}

		if param.CredentialsName != "" {
			org.CredentialsName = param.CredentialsName
			creds, err = s.getGithubCredentialsByName(ctx, tx, param.CredentialsName, false)
			if err != nil {
				return errors.Wrap(err, "fetching credentials")
			}
			if creds.EndpointName == nil {
				return errors.Wrap(runnerErrors.ErrUnprocessable, "credentials have no endpoint")
			}

			if *creds.EndpointName != *org.EndpointName {
				return errors.Wrap(runnerErrors.ErrBadRequest, "endpoint mismatch")
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

		q := tx.Save(&org)
		if q.Error != nil {
			return errors.Wrap(q.Error, "saving org")
		}

		return nil
	})
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "saving org")
	}

	org, err = s.getOrgByID(ctx, s.conn, orgID, "Endpoint", "Credentials", "Credentials.Endpoint")
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "updating enterprise")
	}
	paramOrg, err = s.sqlToCommonOrganization(org, true)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "saving org")
	}
	return paramOrg, nil
}

func (s *sqlDatabase) GetOrganizationByID(ctx context.Context, orgID string) (params.Organization, error) {
	org, err := s.getOrgByID(ctx, s.conn, orgID, "Pools", "Credentials", "Endpoint")
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "fetching org")
	}

	param, err := s.sqlToCommonOrganization(org, true)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "fetching org")
	}
	return param, nil
}

func (s *sqlDatabase) getOrgByID(_ context.Context, db *gorm.DB, id string, preload ...string) (Organization, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return Organization{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
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
		return Organization{}, errors.Wrap(q.Error, "fetching org from database")
	}
	return org, nil
}

func (s *sqlDatabase) getOrg(_ context.Context, name string) (Organization, error) {
	var org Organization

	q := s.conn.Where("name = ? COLLATE NOCASE", name).
		Preload("Credentials").
		Preload("Credentials.Endpoint").
		Preload("Endpoint").
		First(&org)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Organization{}, runnerErrors.ErrNotFound
		}
		return Organization{}, errors.Wrap(q.Error, "fetching org from database")
	}
	return org, nil
}
