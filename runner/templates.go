// Copyright 2025 Cloudbase Solutions SRL
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
package runner

import (
	"context"
	"errors"
	"fmt"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/internal/templates"
	"github.com/cloudbase/garm/params"
)

func (r *Runner) CreateTemplate(ctx context.Context, param params.CreateTemplateParams) (params.Template, error) {
	if !auth.IsAdmin(ctx) {
		return params.Template{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.Template{}, runnerErrors.NewBadRequestError("invalid create params %q", err)
	}

	template, err := r.store.CreateTemplate(ctx, param)
	if err != nil {
		return params.Template{}, fmt.Errorf("failed to create template: %w", err)
	}
	return template, nil
}

func (r *Runner) GetTemplate(ctx context.Context, id uint) (params.Template, error) {
	if !auth.IsAdmin(ctx) {
		return params.Template{}, runnerErrors.ErrUnauthorized
	}
	template, err := r.store.GetTemplate(ctx, id)
	if err != nil {
		return params.Template{}, fmt.Errorf("failed to get template: %w", err)
	}
	return template, nil
}

func (r *Runner) GetTemplateByName(ctx context.Context, templateName string) (params.Template, error) {
	if !auth.IsAdmin(ctx) {
		return params.Template{}, runnerErrors.ErrUnauthorized
	}
	template, err := r.store.GetTemplateByName(ctx, templateName)
	if err != nil {
		return params.Template{}, fmt.Errorf("failed to get template: %w", err)
	}
	return template, nil
}

func (r *Runner) RestoreTemplate(ctx context.Context, param params.RestoreTemplateRequest) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	// Determine which templates to restore
	var templatesConfig []struct {
		OS    commonParams.OSType
		Forge params.EndpointType
	}

	if param.RestoreAll {
		// Restore all system templates
		for _, os := range []commonParams.OSType{commonParams.Linux, commonParams.Windows} {
			for _, forge := range []params.EndpointType{params.GiteaEndpointType, params.GithubEndpointType} {
				templatesConfig = append(templatesConfig, struct {
					OS    commonParams.OSType
					Forge params.EndpointType
				}{OS: os, Forge: forge})
			}
		}
	} else {
		// Restore specific template
		templatesConfig = append(templatesConfig, struct {
			OS    commonParams.OSType
			Forge params.EndpointType
		}{OS: param.OSType, Forge: param.Forge})
	}

	// Process each template
	for _, cfg := range templatesConfig {
		// Get the template content from internal/templates
		templateContent, err := templates.GetTemplateContent(cfg.OS, cfg.Forge)
		if err != nil {
			return fmt.Errorf("failed to get template content for %s/%s: %w", cfg.Forge, cfg.OS, err)
		}

		// Find existing system template for this OS/Forge combination
		existingTemplates, err := r.ListTemplates(ctx, &cfg.OS, &cfg.Forge, nil)
		if err != nil {
			return fmt.Errorf("failed to list templates for %s/%s: %w", cfg.Forge, cfg.OS, err)
		}

		var systemTemplate *params.Template
		for _, tpl := range existingTemplates {
			if tpl.Owner == params.SystemUser || tpl.Owner == "" {
				systemTemplate = &tpl
				break
			}
		}

		// Generate template name
		templateName := fmt.Sprintf("%s_%s", cfg.Forge, cfg.OS)
		description := fmt.Sprintf("Default template for %s runners on %s", cfg.Forge, cfg.OS)

		if systemTemplate != nil {
			// Update existing system template
			updateParams := params.UpdateTemplateParams{
				Data: templateContent,
			}
			// Only update name if it was changed by user (different from expected system name)
			if systemTemplate.Name != templateName {
				updateParams.Name = &templateName
			}
			_, err := r.UpdateTemplate(ctx, systemTemplate.ID, updateParams)
			if err != nil {
				return fmt.Errorf("failed to update system template %d for %s/%s: %w", systemTemplate.ID, cfg.Forge, cfg.OS, err)
			}
		} else {
			// Create new system template
			createParams := params.CreateTemplateParams{
				Name:        templateName,
				Description: description,
				Data:        templateContent,
				OSType:      cfg.OS,
				ForgeType:   cfg.Forge,
				IsSystem:    true,
			}
			_, err := r.CreateTemplate(ctx, createParams)
			if err != nil {
				return fmt.Errorf("failed to create system template for %s/%s: %w", cfg.Forge, cfg.OS, err)
			}
		}
	}

	return nil
}

func (r *Runner) ListTemplates(ctx context.Context, osType *commonParams.OSType, forgeType *params.EndpointType, partialName *string) ([]params.Template, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	templates, err := r.store.ListTemplates(ctx, osType, forgeType, partialName)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}
	return templates, nil
}

func (r *Runner) UpdateTemplate(ctx context.Context, id uint, param params.UpdateTemplateParams) (params.Template, error) {
	if !auth.IsAdmin(ctx) {
		return params.Template{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.Template{}, runnerErrors.NewBadRequestError("invalid update params: %q", err)
	}

	template, err := r.store.UpdateTemplate(ctx, id, param)
	if err != nil {
		return params.Template{}, fmt.Errorf("failed to update template: %w", err)
	}
	return template, nil
}

func (r *Runner) DeleteTemplate(ctx context.Context, id uint) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	if err := r.store.DeleteTemplate(ctx, id); err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return fmt.Errorf("failed to delete template: %w", err)
		}
	}
	return nil
}
