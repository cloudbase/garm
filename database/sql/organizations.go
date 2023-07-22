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

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *sqlDatabase) CreateOrganization(ctx context.Context, name, credentialsName, webhookSecret string) (params.Organization, error) {
	if webhookSecret == "" {
		return params.Organization{}, errors.New("creating org: missing secret")
	}
	secret, err := util.Aes256EncodeString(webhookSecret, s.cfg.Passphrase)
	if err != nil {
		return params.Organization{}, fmt.Errorf("failed to encrypt string")
	}
	newOrg := Organization{
		Name:            name,
		WebhookSecret:   secret,
		CredentialsName: credentialsName,
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

func (s *sqlDatabase) ListOrganizations(ctx context.Context) ([]params.Organization, error) {
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
		secret, err := util.Aes256EncodeString(param.WebhookSecret, s.cfg.Passphrase)
		if err != nil {
			return params.Organization{}, fmt.Errorf("saving org: failed to encrypt string: %w", err)
		}
		org.WebhookSecret = secret
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

func (s *sqlDatabase) CreateOrganizationPool(ctx context.Context, orgId string, param params.CreatePoolParams) (params.Pool, error) {
	if len(param.Tags) == 0 {
		return params.Pool{}, runnerErrors.NewBadRequestError("no tags specified")
	}

	org, err := s.getOrgByID(ctx, orgId)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching org")
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
		OrgID:                  &org.ID,
		Enabled:                param.Enabled,
		RunnerBootstrapTimeout: param.RunnerBootstrapTimeout,
	}

	if len(param.ExtraSpecs) > 0 {
		newPool.ExtraSpecs = datatypes.JSON(param.ExtraSpecs)
	}

	_, err = s.getOrgPoolByUniqueFields(ctx, orgId, newPool.ProviderName, newPool.Image, newPool.Flavor)
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

func (s *sqlDatabase) ListOrgPools(ctx context.Context, orgID string) ([]params.Pool, error) {
	pools, err := s.listEntityPools(ctx, params.OrganizationPool, orgID, "Tags", "Instances")
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}

	ret := make([]params.Pool, len(pools))
	for idx, pool := range pools {
		ret[idx] = s.sqlToCommonPool(pool)
	}

	return ret, nil
}

func (s *sqlDatabase) GetOrganizationPool(ctx context.Context, orgID, poolID string) (params.Pool, error) {
	pool, err := s.getEntityPool(ctx, params.OrganizationPool, orgID, poolID, "Tags", "Instances")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return s.sqlToCommonPool(pool), nil
}

func (s *sqlDatabase) DeleteOrganizationPool(ctx context.Context, orgID, poolID string) error {
	pool, err := s.getEntityPool(ctx, params.OrganizationPool, orgID, poolID)
	if err != nil {
		return errors.Wrap(err, "looking up org pool")
	}
	q := s.conn.Unscoped().Delete(&pool)
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return errors.Wrap(q.Error, "deleting pool")
	}
	return nil
}

func (s *sqlDatabase) FindOrganizationPoolByTags(ctx context.Context, orgID string, tags []string) (params.Pool, error) {
	pool, err := s.findPoolByTags(orgID, params.OrganizationPool, tags)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool[0], nil
}

func (s *sqlDatabase) ListOrgInstances(ctx context.Context, orgID string) ([]params.Instance, error) {
	pools, err := s.listEntityPools(ctx, params.OrganizationPool, orgID, "Tags", "Instances")
	if err != nil {
		return nil, errors.Wrap(err, "fetching org")
	}
	ret := []params.Instance{}
	for _, pool := range pools {
		for _, instance := range pool.Instances {
			ret = append(ret, s.sqlToParamsInstance(instance))
		}
	}
	return ret, nil
}

func (s *sqlDatabase) UpdateOrganizationPool(ctx context.Context, orgID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	pool, err := s.getEntityPool(ctx, params.OrganizationPool, orgID, poolID, "Tags", "Instances", "Enterprise", "Organization", "Repository")
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}

	return s.updatePool(pool, param)
}

func (s *sqlDatabase) getPoolByID(ctx context.Context, poolID string, preload ...string) (Pool, error) {
	u, err := uuid.Parse(poolID)
	if err != nil {
		return Pool{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}
	var pool Pool
	q := s.conn.Model(&Pool{})
	if len(preload) > 0 {
		for _, item := range preload {
			q = q.Preload(item)
		}
	}

	q = q.Where("id = ?", u).First(&pool)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Pool{}, runnerErrors.ErrNotFound
		}
		return Pool{}, errors.Wrap(q.Error, "fetching org from database")
	}
	return pool, nil
}

func (s *sqlDatabase) getOrgByID(ctx context.Context, id string, preload ...string) (Organization, error) {
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

func (s *sqlDatabase) getOrg(ctx context.Context, name string) (Organization, error) {
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

func (s *sqlDatabase) getOrgPoolByUniqueFields(ctx context.Context, orgID string, provider, image, flavor string) (Pool, error) {
	org, err := s.getOrgByID(ctx, orgID)
	if err != nil {
		return Pool{}, errors.Wrap(err, "fetching org")
	}

	q := s.conn
	var pool []Pool
	err = q.Model(&org).Association("Pools").Find(&pool, "provider_name = ? and image = ? and flavor = ?", provider, image, flavor)
	if err != nil {
		return Pool{}, errors.Wrap(err, "fetching pool")
	}
	if len(pool) == 0 {
		return Pool{}, runnerErrors.ErrNotFound
	}

	return pool[0], nil
}
