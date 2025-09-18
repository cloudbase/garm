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
