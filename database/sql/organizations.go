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

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) CreateOrganization(ctx context.Context, name, credentialsName, webhookSecret string, poolBalancerType params.PoolBalancerType) (params.Organization, error) {
	if webhookSecret == "" {
		return params.Organization{}, errors.New("creating org: missing secret")
	}
	secret, err := util.Seal([]byte(webhookSecret), []byte(s.cfg.Passphrase))
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "encoding secret")
	}
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
		newOrg.CredentialsID = &creds.ID

		q := tx.Create(&newOrg)
		if q.Error != nil {
			return errors.Wrap(q.Error, "creating org")
		}

		newOrg.Credentials = creds

		return nil
	})
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "creating org")
	}

	param, err := s.sqlToCommonOrganization(newOrg, true)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "creating org")
	}
	param.WebhookSecret = webhookSecret

	return param, nil
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
	q := s.conn.Preload("Credentials").Find(&orgs)
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

func (s *sqlDatabase) DeleteOrganization(ctx context.Context, orgID string) error {
	org, err := s.getOrgByID(ctx, s.conn, orgID)
	if err != nil {
		return errors.Wrap(err, "fetching org")
	}

	q := s.conn.Unscoped().Delete(&org)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return errors.Wrap(q.Error, "deleting org")
	}

	return nil
}

func (s *sqlDatabase) UpdateOrganization(ctx context.Context, orgID string, param params.UpdateEntityParams) (params.Organization, error) {
	var org Organization
	var creds GithubCredentials
	err := s.conn.Transaction(func(tx *gorm.DB) error {
		var err error
		org, err = s.getOrgByID(ctx, tx, orgID, "Credentials", "Endpoint")
		if err != nil {
			return errors.Wrap(err, "fetching org")
		}

		if param.CredentialsName != "" {
			org.CredentialsName = param.CredentialsName
			creds, err = s.getGithubCredentialsByName(ctx, tx, param.CredentialsName, false)
			if err != nil {
				return errors.Wrap(err, "fetching credentials")
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

		if creds.ID != 0 {
			org.Credentials = creds
		}

		return nil
	})
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "saving org")
	}

	newParams, err := s.sqlToCommonOrganization(org, true)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "saving org")
	}
	return newParams, nil
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
