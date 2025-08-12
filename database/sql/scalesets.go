// Copyright 2025 Cloudbase Solutions SRL
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
	"gorm.io/datatypes"
	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) ListAllScaleSets(_ context.Context) ([]params.ScaleSet, error) {
	var scaleSets []ScaleSet

	q := s.conn.Model(&ScaleSet{}).
		Preload("Organization").
		Preload("Organization.Endpoint").
		Preload("Repository").
		Preload("Repository.Endpoint").
		Preload("Enterprise").
		Preload("Enterprise.Endpoint").
		Omit("extra_specs").
		Omit("status_messages").
		Find(&scaleSets)
	if q.Error != nil {
		return nil, errors.Wrap(q.Error, "fetching all scale sets")
	}

	ret := make([]params.ScaleSet, len(scaleSets))
	var err error
	for idx, val := range scaleSets {
		ret[idx], err = s.sqlToCommonScaleSet(val)
		if err != nil {
			return nil, errors.Wrap(err, "converting scale sets")
		}
	}
	return ret, nil
}

func (s *sqlDatabase) CreateEntityScaleSet(_ context.Context, entity params.ForgeEntity, param params.CreateScaleSetParams) (scaleSet params.ScaleSet, err error) {
	if err := param.Validate(); err != nil {
		return params.ScaleSet{}, fmt.Errorf("failed to validate create params: %w", err)
	}

	defer func() {
		if err == nil {
			s.sendNotify(common.ScaleSetEntityType, common.CreateOperation, scaleSet)
		}
	}()

	newScaleSet := ScaleSet{
		Name:                   param.Name,
		ScaleSetID:             param.ScaleSetID,
		DisableUpdate:          param.DisableUpdate,
		ProviderName:           param.ProviderName,
		RunnerPrefix:           param.GetRunnerPrefix(),
		MaxRunners:             param.MaxRunners,
		MinIdleRunners:         param.MinIdleRunners,
		RunnerBootstrapTimeout: param.RunnerBootstrapTimeout,
		Image:                  param.Image,
		Flavor:                 param.Flavor,
		OSType:                 param.OSType,
		OSArch:                 param.OSArch,
		Enabled:                param.Enabled,
		GitHubRunnerGroup:      param.GitHubRunnerGroup,
		State:                  params.ScaleSetPendingCreate,
	}

	if len(param.ExtraSpecs) > 0 {
		newScaleSet.ExtraSpecs = datatypes.JSON(param.ExtraSpecs)
	}

	entityID, err := uuid.Parse(entity.ID)
	if err != nil {
		return params.ScaleSet{}, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}

	switch entity.EntityType {
	case params.ForgeEntityTypeRepository:
		newScaleSet.RepoID = &entityID
	case params.ForgeEntityTypeOrganization:
		newScaleSet.OrgID = &entityID
	case params.ForgeEntityTypeEnterprise:
		newScaleSet.EnterpriseID = &entityID
	}
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		if err := s.hasGithubEntity(tx, entity.EntityType, entity.ID); err != nil {
			return errors.Wrap(err, "checking entity existence")
		}

		q := tx.Create(&newScaleSet)
		if q.Error != nil {
			return errors.Wrap(q.Error, "creating scale set")
		}

		return nil
	})
	if err != nil {
		return params.ScaleSet{}, err
	}

	dbScaleSet, err := s.getScaleSetByID(s.conn, newScaleSet.ID, "Instances", "Enterprise", "Organization", "Repository")
	if err != nil {
		return params.ScaleSet{}, errors.Wrap(err, "fetching scale set")
	}

	return s.sqlToCommonScaleSet(dbScaleSet)
}

func (s *sqlDatabase) listEntityScaleSets(tx *gorm.DB, entityType params.ForgeEntityType, entityID string, preload ...string) ([]ScaleSet, error) {
	if _, err := uuid.Parse(entityID); err != nil {
		return nil, errors.Wrap(runnerErrors.ErrBadRequest, "parsing id")
	}

	if err := s.hasGithubEntity(tx, entityType, entityID); err != nil {
		return nil, errors.Wrap(err, "checking entity existence")
	}

	var preloadEntity string
	var fieldName string
	switch entityType {
	case params.ForgeEntityTypeRepository:
		fieldName = entityTypeRepoName
		preloadEntity = repositoryFieldName
	case params.ForgeEntityTypeOrganization:
		fieldName = entityTypeOrgName
		preloadEntity = organizationFieldName
	case params.ForgeEntityTypeEnterprise:
		fieldName = entityTypeEnterpriseName
		preloadEntity = enterpriseFieldName
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

	var scaleSets []ScaleSet
	condition := fmt.Sprintf("%s = ?", fieldName)
	err := q.Model(&ScaleSet{}).
		Where(condition, entityID).
		Omit("extra_specs").
		Omit("status_messages").
		Find(&scaleSets).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []ScaleSet{}, nil
		}
		return nil, errors.Wrap(err, "fetching scale sets")
	}

	return scaleSets, nil
}

func (s *sqlDatabase) ListEntityScaleSets(_ context.Context, entity params.ForgeEntity) ([]params.ScaleSet, error) {
	scaleSets, err := s.listEntityScaleSets(s.conn, entity.EntityType, entity.ID)
	if err != nil {
		return nil, errors.Wrap(err, "fetching scale sets")
	}

	ret := make([]params.ScaleSet, len(scaleSets))
	for idx, set := range scaleSets {
		ret[idx], err = s.sqlToCommonScaleSet(set)
		if err != nil {
			return nil, errors.Wrap(err, "conbverting scale set")
		}
	}

	return ret, nil
}

func (s *sqlDatabase) UpdateEntityScaleSet(_ context.Context, entity params.ForgeEntity, scaleSetID uint, param params.UpdateScaleSetParams, callback func(old, newSet params.ScaleSet) error) (updatedScaleSet params.ScaleSet, err error) {
	defer func() {
		if err == nil {
			s.sendNotify(common.ScaleSetEntityType, common.UpdateOperation, updatedScaleSet)
		}
	}()
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		scaleSet, err := s.getEntityScaleSet(tx, entity.EntityType, entity.ID, scaleSetID, "Instances")
		if err != nil {
			return errors.Wrap(err, "fetching scale set")
		}

		old, err := s.sqlToCommonScaleSet(scaleSet)
		if err != nil {
			return errors.Wrap(err, "converting scale set")
		}

		updatedScaleSet, err = s.updateScaleSet(tx, scaleSet, param)
		if err != nil {
			return errors.Wrap(err, "updating scale set")
		}

		if callback != nil {
			if err := callback(old, updatedScaleSet); err != nil {
				return errors.Wrap(err, "executing update callback")
			}
		}
		return nil
	})
	if err != nil {
		return params.ScaleSet{}, err
	}
	return updatedScaleSet, nil
}

func (s *sqlDatabase) getEntityScaleSet(tx *gorm.DB, entityType params.ForgeEntityType, entityID string, scaleSetID uint, preload ...string) (ScaleSet, error) {
	if entityID == "" {
		return ScaleSet{}, errors.Wrap(runnerErrors.ErrBadRequest, "missing entity id")
	}

	if scaleSetID == 0 {
		return ScaleSet{}, errors.Wrap(runnerErrors.ErrBadRequest, "missing scaleset id")
	}

	var fieldName string
	var entityField string
	switch entityType {
	case params.ForgeEntityTypeRepository:
		fieldName = entityTypeRepoName
		entityField = "Repository"
	case params.ForgeEntityTypeOrganization:
		fieldName = entityTypeOrgName
		entityField = "Organization"
	case params.ForgeEntityTypeEnterprise:
		fieldName = entityTypeEnterpriseName
		entityField = "Enterprise"
	default:
		return ScaleSet{}, fmt.Errorf("invalid entityType: %v", entityType)
	}

	q := tx
	q = q.Preload(entityField)
	if len(preload) > 0 {
		for _, item := range preload {
			q = q.Preload(item)
		}
	}

	var scaleSet ScaleSet
	condition := fmt.Sprintf("id = ? and %s = ?", fieldName)
	err := q.Model(&ScaleSet{}).
		Where(condition, scaleSetID, entityID).
		First(&scaleSet).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ScaleSet{}, errors.Wrap(runnerErrors.ErrNotFound, "finding scale set")
		}
		return ScaleSet{}, errors.Wrap(err, "fetching scale set")
	}

	return scaleSet, nil
}

func (s *sqlDatabase) updateScaleSet(tx *gorm.DB, scaleSet ScaleSet, param params.UpdateScaleSetParams) (params.ScaleSet, error) {
	if param.Enabled != nil && scaleSet.Enabled != *param.Enabled {
		scaleSet.Enabled = *param.Enabled
	}

	if param.State != nil && *param.State != scaleSet.State {
		scaleSet.State = *param.State
	}

	if param.ExtendedState != nil && *param.ExtendedState != scaleSet.ExtendedState {
		scaleSet.ExtendedState = *param.ExtendedState
	}

	if param.Name != "" {
		scaleSet.Name = param.Name
	}

	if param.GitHubRunnerGroup != nil && *param.GitHubRunnerGroup != "" {
		scaleSet.GitHubRunnerGroup = *param.GitHubRunnerGroup
	}

	if param.Flavor != "" {
		scaleSet.Flavor = param.Flavor
	}

	if param.Image != "" {
		scaleSet.Image = param.Image
	}

	if param.Prefix != "" {
		scaleSet.RunnerPrefix = param.Prefix
	}

	if param.MaxRunners != nil {
		scaleSet.MaxRunners = *param.MaxRunners
	}

	if param.MinIdleRunners != nil {
		scaleSet.MinIdleRunners = *param.MinIdleRunners
	}

	if param.OSArch != "" {
		scaleSet.OSArch = param.OSArch
	}

	if param.OSType != "" {
		scaleSet.OSType = param.OSType
	}

	if param.ExtraSpecs != nil {
		scaleSet.ExtraSpecs = datatypes.JSON(param.ExtraSpecs)
	}

	if param.RunnerBootstrapTimeout != nil && *param.RunnerBootstrapTimeout > 0 {
		scaleSet.RunnerBootstrapTimeout = *param.RunnerBootstrapTimeout
	}

	if param.GitHubRunnerGroup != nil {
		scaleSet.GitHubRunnerGroup = *param.GitHubRunnerGroup
	}

	if q := tx.Save(&scaleSet); q.Error != nil {
		return params.ScaleSet{}, errors.Wrap(q.Error, "saving database entry")
	}

	return s.sqlToCommonScaleSet(scaleSet)
}

func (s *sqlDatabase) GetScaleSetByID(_ context.Context, scaleSet uint) (params.ScaleSet, error) {
	set, err := s.getScaleSetByID(s.conn, scaleSet, "Instances", "Enterprise", "Organization", "Repository")
	if err != nil {
		return params.ScaleSet{}, errors.Wrap(err, "fetching scale set by ID")
	}
	return s.sqlToCommonScaleSet(set)
}

func (s *sqlDatabase) DeleteScaleSetByID(_ context.Context, scaleSetID uint) (err error) {
	var scaleSet params.ScaleSet
	defer func() {
		if err == nil && scaleSet.ID != 0 {
			s.sendNotify(common.ScaleSetEntityType, common.DeleteOperation, scaleSet)
		}
	}()
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		dbSet, err := s.getScaleSetByID(tx, scaleSetID, "Instances", "Enterprise", "Organization", "Repository")
		if err != nil {
			return errors.Wrap(err, "fetching scale set")
		}

		if len(dbSet.Instances) > 0 {
			return runnerErrors.NewBadRequestError("cannot delete scaleset with runners")
		}
		scaleSet, err = s.sqlToCommonScaleSet(dbSet)
		if err != nil {
			return errors.Wrap(err, "converting scale set")
		}

		if q := tx.Unscoped().Delete(&dbSet); q.Error != nil {
			return errors.Wrap(q.Error, "deleting scale set")
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "removing scale set")
	}
	return nil
}

func (s *sqlDatabase) SetScaleSetLastMessageID(_ context.Context, scaleSetID uint, lastMessageID int64) (err error) {
	var scaleSet params.ScaleSet
	defer func() {
		if err == nil && scaleSet.ID != 0 {
			s.sendNotify(common.ScaleSetEntityType, common.UpdateOperation, scaleSet)
		}
	}()
	if err := s.conn.Transaction(func(tx *gorm.DB) error {
		dbSet, err := s.getScaleSetByID(tx, scaleSetID, "Instances", "Enterprise", "Organization", "Repository")
		if err != nil {
			return errors.Wrap(err, "fetching scale set")
		}
		dbSet.LastMessageID = lastMessageID
		if err := tx.Save(&dbSet).Error; err != nil {
			return errors.Wrap(err, "saving database entry")
		}
		scaleSet, err = s.sqlToCommonScaleSet(dbSet)
		if err != nil {
			return errors.Wrap(err, "converting scale set")
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "setting last message ID")
	}
	return nil
}

func (s *sqlDatabase) SetScaleSetDesiredRunnerCount(_ context.Context, scaleSetID uint, desiredRunnerCount int) (err error) {
	var scaleSet params.ScaleSet
	defer func() {
		if err == nil && scaleSet.ID != 0 {
			s.sendNotify(common.ScaleSetEntityType, common.UpdateOperation, scaleSet)
		}
	}()
	if err := s.conn.Transaction(func(tx *gorm.DB) error {
		dbSet, err := s.getScaleSetByID(tx, scaleSetID, "Instances", "Enterprise", "Organization", "Repository")
		if err != nil {
			return errors.Wrap(err, "fetching scale set")
		}
		dbSet.DesiredRunnerCount = desiredRunnerCount
		if err := tx.Save(&dbSet).Error; err != nil {
			return errors.Wrap(err, "saving database entry")
		}
		scaleSet, err = s.sqlToCommonScaleSet(dbSet)
		if err != nil {
			return errors.Wrap(err, "converting scale set")
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "setting desired runner count")
	}
	return nil
}
