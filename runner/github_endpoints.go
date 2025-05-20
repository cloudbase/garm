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

package runner

import (
	"context"

	"github.com/pkg/errors"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

func (r *Runner) CreateGithubEndpoint(ctx context.Context, param params.CreateGithubEndpointParams) (params.ForgeEndpoint, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeEndpoint{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "failed to validate github endpoint params")
	}

	ep, err := r.store.CreateGithubEndpoint(ctx, param)
	if err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "failed to create github endpoint")
	}

	return ep, nil
}

func (r *Runner) GetGithubEndpoint(ctx context.Context, name string) (params.ForgeEndpoint, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeEndpoint{}, runnerErrors.ErrUnauthorized
	}
	endpoint, err := r.store.GetGithubEndpoint(ctx, name)
	if err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "failed to get github endpoint")
	}

	return endpoint, nil
}

func (r *Runner) DeleteGithubEndpoint(ctx context.Context, name string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	err := r.store.DeleteGithubEndpoint(ctx, name)
	if err != nil {
		return errors.Wrap(err, "failed to delete github endpoint")
	}

	return nil
}

func (r *Runner) UpdateGithubEndpoint(ctx context.Context, name string, param params.UpdateGithubEndpointParams) (params.ForgeEndpoint, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeEndpoint{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "failed to validate github endpoint params")
	}

	newEp, err := r.store.UpdateGithubEndpoint(ctx, name, param)
	if err != nil {
		return params.ForgeEndpoint{}, errors.Wrap(err, "failed to update github endpoint")
	}
	return newEp, nil
}

func (r *Runner) ListGithubEndpoints(ctx context.Context) ([]params.ForgeEndpoint, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	endpoints, err := r.store.ListGithubEndpoints(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list github endpoints")
	}

	return endpoints, nil
}
