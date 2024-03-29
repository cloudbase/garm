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

func (s *sqlDatabase) CreateOrganization(_ context.Context, name, credentialsName, webhookSecret string, poolBalancerType params.PoolBalancerType) (params.Organization, error) {
	if webhookSecret == "" {
		return params.Organization{}, errors.New("creating org: missing secret")
	}
	secret, err := util.Seal([]byte(webhookSecret), []byte(s.cfg.Passphrase))
	if err != nil {
		return params.Organization{}, fmt.Errorf("failed to encrypt string")
	}
	newOrg := Organization{
		Name:             name,
		WebhookSecret:    secret,
		CredentialsName:  credentialsName,
		PoolBalancerType: poolBalancerType,
	}

	q := s.conn.Create(&newOrg)
	if q.Error != nil {
		return params.Organization{}, errors.Wrap(q.Error, "creating org")
	}

	param, err := s.sqlToCommonOrganization(newOrg)
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

	param, err := s.sqlToCommonOrganization(org)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "fetching org")
	}

	return param, nil
}

func (s *sqlDatabase) ListOrganizations(_ context.Context) ([]params.Organization, error) {
	var orgs []Organization
	q := s.conn.Find(&orgs)
	if q.Error != nil {
		return []params.Organization{}, errors.Wrap(q.Error, "fetching org from database")
	}

	ret := make([]params.Organization, len(orgs))
	for idx, val := range orgs {
		var err error
		ret[idx], err = s.sqlToCommonOrganization(val)
		if err != nil {
			return nil, errors.Wrap(err, "fetching org")
		}
	}

	return ret, nil
}

func (s *sqlDatabase) DeleteOrganization(ctx context.Context, orgID string) error {
	org, err := s.getOrgByID(ctx, orgID)
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
	org, err := s.getOrgByID(ctx, orgID)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "fetching org")
	}

	if param.CredentialsName != "" {
		org.CredentialsName = param.CredentialsName
	}

	if param.WebhookSecret != "" {
		secret, err := util.Seal([]byte(param.WebhookSecret), []byte(s.cfg.Passphrase))
		if err != nil {
			return params.Organization{}, fmt.Errorf("saving org: failed to encrypt string: %w", err)
		}
		org.WebhookSecret = secret
	}

	if param.PoolBalancerType != "" {
		org.PoolBalancerType = param.PoolBalancerType
	}

	q := s.conn.Save(&org)
	if q.Error != nil {
		return params.Organization{}, errors.Wrap(q.Error, "saving org")
	}

	newParams, err := s.sqlToCommonOrganization(org)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "saving org")
	}
	return newParams, nil
}

func (s *sqlDatabase) GetOrganizationByID(ctx context.Context, orgID string) (params.Organization, error) {
	org, err := s.getOrgByID(ctx, orgID, "Pools")
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "fetching org")
	}

	param, err := s.sqlToCommonOrganization(org)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "fetching enterprise")
	}
	return param, nil
}

func (s *sqlDatabase) getOrgByID(_ context.Context, id string, preload ...string) (Organization, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return Organization{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}
	var org Organization

	q := s.conn
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

	q := s.conn.Where("name = ? COLLATE NOCASE", name)
	q = q.First(&org)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Organization{}, runnerErrors.ErrNotFound
		}
		return Organization{}, errors.Wrap(q.Error, "fetching org from database")
	}
	return org, nil
}
