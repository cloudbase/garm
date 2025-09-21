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
	"errors"
	"fmt"

	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) ListTemplates(ctx context.Context, osType *commonParams.OSType, forgeType *params.EndpointType, partialName *string) ([]params.Template, error) {
	var templates []Template
	q := s.conn.Model(&Template{}).Omit("data").Preload("User")
	if !auth.IsAdmin(ctx) {
		userID, err := getUIDFromContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("error listing templates: %w", err)
		}
		q = q.Where("user_id = ? or user_id IS NULL", userID)
	}

	if osType != nil {
		q = q.Where("os_type = ?", *osType)
	}

	if partialName != nil {
		q = q.Where("name like ? COLLATE NOCASE", fmt.Sprintf("%%%s%%", *partialName))
	}

	if forgeType != nil {
		q = q.Where("forge_type = ?", *forgeType)
	}

	q = q.Find(&templates)
	if q.Error != nil {
		return nil, fmt.Errorf("failed to get templates: %w", q.Error)
	}

	ret := make([]params.Template, len(templates))
	for idx, tpl := range templates {
		retTpl, err := s.sqlToParamTemplate(tpl)
		if err != nil {
			return nil, fmt.Errorf("failed to convert template: %w", err)
		}
		ret[idx] = retTpl
	}
	return ret, nil
}

func (s *sqlDatabase) getTemplate(ctx context.Context, tx *gorm.DB, id uint, preload ...string) (Template, error) {
	var template Template
	q := tx.Model(&Template{}).Where("id = ?", id)

	if len(preload) > 0 {
		for _, item := range preload {
			q = q.Preload(item)
		}
	}

	if !auth.IsAdmin(ctx) {
		userID, err := getUIDFromContext(ctx)
		if err != nil {
			return Template{}, fmt.Errorf("error listing templates: %w", err)
		}
		q = q.Where("user_id = ? or user_id IS NULL", userID)
	}

	q = q.First(&template)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return Template{}, runnerErrors.ErrNotFound
		}
		return Template{}, fmt.Errorf("failed to get template: %w", q.Error)
	}
	return template, nil
}

func (s *sqlDatabase) GetTemplate(ctx context.Context, id uint) (params.Template, error) {
	template, err := s.getTemplate(ctx, s.conn, id, "User")
	if err != nil {
		return params.Template{}, fmt.Errorf("failed to get template: %w", err)
	}

	ret, err := s.sqlToParamTemplate(template)
	if err != nil {
		return params.Template{}, fmt.Errorf("failed to convert template: %w", err)
	}
	return ret, nil
}

func (s *sqlDatabase) GetTemplateByName(ctx context.Context, name string) (params.Template, error) {
	userID, err := getUIDFromContext(ctx)
	if err != nil {
		return params.Template{}, fmt.Errorf("failed to get template: %w", err)
	}
	var template Template
	q := s.conn.Model(&Template{}).
		Where("name = ?", name).
		Where("user_id = ? or user_id IS NULL", userID).
		Preload("ScaleSets").
		Preload("Pools").
		Preload("User")

	q = q.First(&template)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return params.Template{}, runnerErrors.ErrNotFound
		}
		return params.Template{}, fmt.Errorf("failed to get template: %w", q.Error)
	}

	ret, err := s.sqlToParamTemplate(template)
	if err != nil {
		return params.Template{}, fmt.Errorf("failed to convert template: %w", err)
	}
	return ret, nil
}

func (s *sqlDatabase) createSystemTemplate(ctx context.Context, param params.CreateTemplateParams) (template params.Template, err error) {
	if !auth.IsAdmin(ctx) {
		return params.Template{}, runnerErrors.ErrUnauthorized
	}
	defer func() {
		if err == nil {
			s.sendNotify(common.TemplateEntityType, common.CreateOperation, template)
		}
	}()
	sealed, err := s.marshalAndSeal(param.Data)
	if err != nil {
		return params.Template{}, fmt.Errorf("failed to seal data: %w", err)
	}
	tpl := Template{
		UserID:      nil,
		Name:        param.Name,
		Description: param.Description,
		OSType:      param.OSType,
		Data:        sealed,
		ForgeType:   param.ForgeType,
	}

	if err := s.conn.Create(&tpl).Error; err != nil {
		return params.Template{}, fmt.Errorf("error creating template: %w", err)
	}

	template, err = s.sqlToParamTemplate(tpl)
	if err != nil {
		return params.Template{}, fmt.Errorf("failed to convert template: %w", err)
	}

	return template, nil
}

func (s *sqlDatabase) CreateTemplate(ctx context.Context, param params.CreateTemplateParams) (template params.Template, err error) {
	userID, err := getUIDFromContext(ctx)
	if err != nil {
		return params.Template{}, fmt.Errorf("error creating template: %w", err)
	}
	defer func() {
		if err == nil {
			s.sendNotify(common.TemplateEntityType, common.CreateOperation, template)
		}
	}()

	sealed, err := s.marshalAndSeal(param.Data)
	if err != nil {
		return params.Template{}, fmt.Errorf("failed to seal data: %w", err)
	}
	tpl := Template{
		UserID:      &userID,
		Name:        param.Name,
		Description: param.Description,
		OSType:      param.OSType,
		Data:        sealed,
		ForgeType:   param.ForgeType,
	}
	if err := param.Validate(); err != nil {
		return params.Template{}, fmt.Errorf("failed to validate create params: %w", err)
	}

	if err := s.conn.Create(&tpl).Error; err != nil {
		return params.Template{}, fmt.Errorf("error creating template: %w", err)
	}

	return s.GetTemplate(ctx, tpl.ID)
}

func (s *sqlDatabase) UpdateTemplate(ctx context.Context, id uint, param params.UpdateTemplateParams) (template params.Template, err error) {
	var hasChange bool
	defer func() {
		if err == nil && hasChange {
			s.sendNotify(common.TemplateEntityType, common.UpdateOperation, template)
		}
	}()
	var tpl Template
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		tpl, err = s.getTemplate(ctx, tx, id)
		if err != nil {
			return fmt.Errorf("failed to get template: %w", err)
		}
		if !auth.IsAdmin(ctx) {
			if tpl.UserID == nil {
				return runnerErrors.NewBadRequestError("cannot edit system templates")
			}
		}
		if param.Description != nil {
			hasChange = true
			tpl.Description = *param.Description
		}

		if param.Name != nil {
			hasChange = true
			tpl.Name = *param.Name
		}
		if len(param.Data) > 0 {
			hasChange = true
			data, err := s.marshalAndSeal(param.Data)
			if err != nil {
				return fmt.Errorf("failed to seal data: %w", err)
			}
			tpl.Data = data
		}

		if !hasChange {
			return nil
		}

		if q := tx.Save(&tpl); q.Error != nil {
			return fmt.Errorf("failed to save template: %w", q.Error)
		}

		template, err = s.sqlToParamTemplate(tpl)
		if err != nil {
			return fmt.Errorf("failed to convert template: %w", err)
		}
		return nil
	})
	if err != nil {
		return params.Template{}, fmt.Errorf("failed to update template: %w", err)
	}
	return template, nil
}

func (s *sqlDatabase) DeleteTemplate(ctx context.Context, id uint) (err error) {
	var template params.Template

	defer func() {
		if err == nil {
			s.sendNotify(common.TemplateEntityType, common.DeleteOperation, template)
		}
	}()
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		tpl, err := s.getTemplate(ctx, tx, id, "Pools", "ScaleSets")
		if err != nil {
			return fmt.Errorf("failed to get template: %w", err)
		}
		if !auth.IsAdmin(ctx) {
			if tpl.UserID == nil {
				return runnerErrors.NewBadRequestError("cannot delete system templates")
			}
		}

		if len(tpl.Pools) > 0 || len(tpl.ScaleSets) > 0 {
			return runnerErrors.NewBadRequestError("cannot delete template while in use by pools or scale sets")
		}
		template, err = s.sqlToParamTemplate(tpl)
		if err != nil {
			return fmt.Errorf("failed to convert template: %w", err)
		}

		if q := tx.Unscoped().Delete(&tpl); q.Error != nil {
			return fmt.Errorf("failed to delete template: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}
	return nil
}
