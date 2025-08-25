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

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

const (
	entityTypeEnterpriseName = "enterprise_id"
	entityTypeOrgName        = "org_id"
	entityTypeRepoName       = "repo_id"
)

func (s *sqlDatabase) ListAllPools(_ context.Context) ([]params.Pool, error) {
	var pools []Pool

	q := s.conn.
		Preload("Tags").
		Preload("Organization").
		Preload("Organization.Endpoint").
		Preload("Repository").
		Preload("Repository.Endpoint").
		Preload("Enterprise").
		Preload("Enterprise.Endpoint").
		Omit("extra_specs").
		Find(&pools)
	if q.Error != nil {
		return nil, fmt.Errorf("error fetching all pools: %w", q.Error)
	}

	ret := make([]params.Pool, len(pools))
	var err error
	for idx, val := range pools {
		ret[idx], err = s.sqlToCommonPool(val)
		if err != nil {
			return nil, fmt.Errorf("error converting pool: %w", err)
		}
	}
	return ret, nil
}

func (s *sqlDatabase) GetPoolByID(_ context.Context, poolID string) (params.Pool, error) {
	preloadList := []string{
		"Tags",
		"Instances",
		"Enterprise",
		"Enterprise.Endpoint",
		"Organization",
		"Organization.Endpoint",
		"Repository",
		"Repository.Endpoint",
	}
	pool, err := s.getPoolByID(s.conn, poolID, preloadList...)
	if err != nil {
		return params.Pool{}, fmt.Errorf("error fetching pool by ID: %w", err)
	}
	return s.sqlToCommonPool(pool)
}

func (s *sqlDatabase) DeletePoolByID(_ context.Context, poolID string) (err error) {
	pool, err := s.getPoolByID(s.conn, poolID)
	if err != nil {
		return fmt.Errorf("error fetching pool by ID: %w", err)
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.PoolEntityType, common.DeleteOperation, params.Pool{ID: poolID})
		}
	}()

	if q := s.conn.Unscoped().Delete(&pool); q.Error != nil {
		return fmt.Errorf("error removing pool: %w", q.Error)
	}

	return nil
}

func (s *sqlDatabase) getEntityPool(tx *gorm.DB, entityType params.ForgeEntityType, entityID, poolID string, preload ...string) (Pool, error) {
	if entityID == "" {
		return Pool{}, fmt.Errorf("error missing entity id: %w", runnerErrors.ErrBadRequest)
	}

	u, err := uuid.Parse(poolID)
	if err != nil {
		return Pool{}, fmt.Errorf("error parsing id: %w", runnerErrors.ErrBadRequest)
	}

	var fieldName string
	var entityField string
	switch entityType {
	case params.ForgeEntityTypeRepository:
		fieldName = entityTypeRepoName
		entityField = repositoryFieldName
	case params.ForgeEntityTypeOrganization:
		fieldName = entityTypeOrgName
		entityField = organizationFieldName
	case params.ForgeEntityTypeEnterprise:
		fieldName = entityTypeEnterpriseName
		entityField = enterpriseFieldName
	default:
		return Pool{}, fmt.Errorf("invalid entityType: %v", entityType)
	}

	q := tx
	q = q.Preload(entityField)
	if len(preload) > 0 {
		for _, item := range preload {
			q = q.Preload(item)
		}
	}

	var pool Pool
	condition := fmt.Sprintf("id = ? and %s = ?", fieldName)
	err = q.Model(&Pool{}).
		Where(condition, u, entityID).
		First(&pool).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Pool{}, fmt.Errorf("error finding pool: %w", runnerErrors.ErrNotFound)
		}
		return Pool{}, fmt.Errorf("error fetching pool: %w", err)
	}

	return pool, nil
}

func (s *sqlDatabase) listEntityPools(tx *gorm.DB, entityType params.ForgeEntityType, entityID string, preload ...string) ([]Pool, error) {
	if _, err := uuid.Parse(entityID); err != nil {
		return nil, fmt.Errorf("error parsing id: %w", runnerErrors.ErrBadRequest)
	}

	if err := s.hasGithubEntity(tx, entityType, entityID); err != nil {
		return nil, fmt.Errorf("error checking entity existence: %w", err)
	}

	var preloadEntity string
	var fieldName string
	switch entityType {
	case params.ForgeEntityTypeRepository:
		fieldName = entityTypeRepoName
		preloadEntity = "Repository"
	case params.ForgeEntityTypeOrganization:
		fieldName = entityTypeOrgName
		preloadEntity = "Organization"
	case params.ForgeEntityTypeEnterprise:
		fieldName = entityTypeEnterpriseName
		preloadEntity = "Enterprise"
	default:
		return nil, fmt.Errorf("invalid entityType: %v", entityType)
	}

	q := tx
	q = q.Preload(preloadEntity)
	if len(preload) > 0 {
		for _, item := range preload {
			q = q.Preload(item)
		}
	}

	var pools []Pool
	condition := fmt.Sprintf("%s = ?", fieldName)
	err := q.Model(&Pool{}).
		Where(condition, entityID).
		Omit("extra_specs").
		Find(&pools).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []Pool{}, nil
		}
		return nil, fmt.Errorf("error fetching pool: %w", err)
	}

	return pools, nil
}

func (s *sqlDatabase) findPoolByTags(id string, poolType params.ForgeEntityType, tags []string) ([]params.Pool, error) {
	if len(tags) == 0 {
		return nil, runnerErrors.NewBadRequestError("missing tags")
	}
	u, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("error parsing id: %w", runnerErrors.ErrBadRequest)
	}

	var fieldName string
	switch poolType {
	case params.ForgeEntityTypeRepository:
		fieldName = entityTypeRepoName
	case params.ForgeEntityTypeOrganization:
		fieldName = entityTypeOrgName
	case params.ForgeEntityTypeEnterprise:
		fieldName = entityTypeEnterpriseName
	default:
		return nil, fmt.Errorf("invalid poolType: %v", poolType)
	}

	var pools []Pool
	where := fmt.Sprintf("tags.name COLLATE NOCASE in ? and %s = ? and enabled = true", fieldName)
	q := s.conn.Joins("JOIN pool_tags on pool_tags.pool_id=pools.id").
		Joins("JOIN tags on tags.id=pool_tags.tag_id").
		Group("pools.id").
		Preload("Tags").
		Having("count(1) = ?", len(tags)).
		Where(where, tags, u).
		Order("priority desc").
		Find(&pools)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return nil, runnerErrors.ErrNotFound
		}
		return nil, fmt.Errorf("error fetching pool: %w", q.Error)
	}

	if len(pools) == 0 {
		return nil, runnerErrors.ErrNotFound
	}

	ret := make([]params.Pool, len(pools))
	for idx, val := range pools {
		ret[idx], err = s.sqlToCommonPool(val)
		if err != nil {
			return nil, fmt.Errorf("error converting pool: %w", err)
		}
	}

	return ret, nil
}

func (s *sqlDatabase) FindPoolsMatchingAllTags(_ context.Context, entityType params.ForgeEntityType, entityID string, tags []string) ([]params.Pool, error) {
	if len(tags) == 0 {
		return nil, runnerErrors.NewBadRequestError("missing tags")
	}

	pools, err := s.findPoolByTags(entityID, entityType, tags)
	if err != nil {
		if errors.Is(err, runnerErrors.ErrNotFound) {
			return []params.Pool{}, nil
		}
		return nil, fmt.Errorf("error fetching pools: %w", err)
	}

	return pools, nil
}

func (s *sqlDatabase) CreateEntityPool(ctx context.Context, entity params.ForgeEntity, param params.CreatePoolParams) (pool params.Pool, err error) {
	if len(param.Tags) == 0 {
		return params.Pool{}, runnerErrors.NewBadRequestError("no tags specified")
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.PoolEntityType, common.CreateOperation, pool)
		}
	}()

	newPool := Pool{
		ProviderName:           param.ProviderName,
		MaxRunners:             param.MaxRunners,
		MinIdleRunners:         param.MinIdleRunners,
		RunnerPrefix:           param.GetRunnerPrefix(),
		Image:                  param.Image,
		Flavor:                 param.Flavor,
		OSType:                 param.OSType,
		OSArch:                 param.OSArch,
		Enabled:                param.Enabled,
		RunnerBootstrapTimeout: param.RunnerBootstrapTimeout,
		GitHubRunnerGroup:      param.GitHubRunnerGroup,
		Priority:               param.Priority,
	}
	if len(param.ExtraSpecs) > 0 {
		newPool.ExtraSpecs = datatypes.JSON(param.ExtraSpecs)
	}

	entityID, err := uuid.Parse(entity.ID)
	if err != nil {
		return params.Pool{}, fmt.Errorf("error parsing id: %w", runnerErrors.ErrBadRequest)
	}

	switch entity.EntityType {
	case params.ForgeEntityTypeRepository:
		newPool.RepoID = &entityID
	case params.ForgeEntityTypeOrganization:
		newPool.OrgID = &entityID
	case params.ForgeEntityTypeEnterprise:
		newPool.EnterpriseID = &entityID
	}
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		if err := s.hasGithubEntity(tx, entity.EntityType, entity.ID); err != nil {
			return fmt.Errorf("error checking entity existence: %w", err)
		}

		tags := []Tag{}
		for _, val := range param.Tags {
			t, err := s.getOrCreateTag(tx, val)
			if err != nil {
				return fmt.Errorf("error creating tag: %w", err)
			}
			tags = append(tags, t)
		}

		q := tx.Create(&newPool)
		if q.Error != nil {
			return fmt.Errorf("error creating pool: %w", q.Error)
		}

		for i := range tags {
			if err := tx.Model(&newPool).Association("Tags").Append(&tags[i]); err != nil {
				return fmt.Errorf("error associating tags: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return params.Pool{}, err
	}

	return s.GetPoolByID(ctx, newPool.ID.String())
}

func (s *sqlDatabase) GetEntityPool(_ context.Context, entity params.ForgeEntity, poolID string) (params.Pool, error) {
	pool, err := s.getEntityPool(s.conn, entity.EntityType, entity.ID, poolID, "Tags", "Instances")
	if err != nil {
		return params.Pool{}, fmt.Errorf("fetching pool: %w", err)
	}
	return s.sqlToCommonPool(pool)
}

func (s *sqlDatabase) DeleteEntityPool(_ context.Context, entity params.ForgeEntity, poolID string) (err error) {
	entityID, err := uuid.Parse(entity.ID)
	if err != nil {
		return fmt.Errorf("error parsing id: %w", runnerErrors.ErrBadRequest)
	}

	defer func() {
		if err == nil {
			pool := params.Pool{
				ID: poolID,
			}
			s.sendNotify(common.PoolEntityType, common.DeleteOperation, pool)
		}
	}()

	poolUUID, err := uuid.Parse(poolID)
	if err != nil {
		return fmt.Errorf("error parsing pool id: %w", runnerErrors.ErrBadRequest)
	}
	var fieldName string
	switch entity.EntityType {
	case params.ForgeEntityTypeRepository:
		fieldName = entityTypeRepoName
	case params.ForgeEntityTypeOrganization:
		fieldName = entityTypeOrgName
	case params.ForgeEntityTypeEnterprise:
		fieldName = entityTypeEnterpriseName
	default:
		return fmt.Errorf("invalid entityType: %v", entity.EntityType)
	}
	condition := fmt.Sprintf("id = ? and %s = ?", fieldName)
	if err := s.conn.Unscoped().Where(condition, poolUUID, entityID).Delete(&Pool{}).Error; err != nil {
		return fmt.Errorf("error removing pool: %w", err)
	}
	return nil
}

func (s *sqlDatabase) UpdateEntityPool(ctx context.Context, entity params.ForgeEntity, poolID string, param params.UpdatePoolParams) (updatedPool params.Pool, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.PoolEntityType, common.UpdateOperation, updatedPool)
		}
	}()
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		pool, err := s.getEntityPool(tx, entity.EntityType, entity.ID, poolID, "Tags", "Instances")
		if err != nil {
			return fmt.Errorf("error fetching pool: %w", err)
		}

		updatedPool, err = s.updatePool(tx, pool, param)
		if err != nil {
			return fmt.Errorf("error updating pool: %w", err)
		}
		return nil
	})
	if err != nil {
		return params.Pool{}, err
	}

	updatedPool, err = s.GetPoolByID(ctx, poolID)
	if err != nil {
		return params.Pool{}, err
	}
	return updatedPool, nil
}

func (s *sqlDatabase) ListEntityPools(_ context.Context, entity params.ForgeEntity) ([]params.Pool, error) {
	pools, err := s.listEntityPools(s.conn, entity.EntityType, entity.ID, "Tags")
	if err != nil {
		return nil, fmt.Errorf("error fetching pools: %w", err)
	}

	ret := make([]params.Pool, len(pools))
	for idx, pool := range pools {
		ret[idx], err = s.sqlToCommonPool(pool)
		if err != nil {
			return nil, fmt.Errorf("error fetching pool: %w", err)
		}
	}

	return ret, nil
}

func (s *sqlDatabase) ListEntityInstances(_ context.Context, entity params.ForgeEntity) ([]params.Instance, error) {
	pools, err := s.listEntityPools(s.conn, entity.EntityType, entity.ID, "Instances", "Instances.Job")
	if err != nil {
		return nil, fmt.Errorf("error fetching entity: %w", err)
	}
	ret := []params.Instance{}
	for _, pool := range pools {
		instances := pool.Instances
		pool.Instances = nil
		for _, instance := range instances {
			instance.Pool = pool
			paramsInstance, err := s.sqlToParamsInstance(instance)
			if err != nil {
				return nil, fmt.Errorf("error fetching instance: %w", err)
			}
			ret = append(ret, paramsInstance)
		}
	}
	return ret, nil
}
