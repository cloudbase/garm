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
	"fmt"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/params"
)

func (r *Runner) ListCredentials(ctx context.Context) ([]params.ForgeCredentials, error) {
	// Get the credentials from the store. The cache is always updated after the database successfully
	// commits the transaction that created/updated the credentials.
	// If we create a set of credentials then immediately after we call ListCredentials,
	// there is a posibillity that not all creds will be in the cache.
	creds, err := r.store.ListGithubCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching github credentials: %w", err)
	}

	// If we do have cache, update the rate limit for each credential. The rate limits are queried
	// every 30 seconds and set in cache.
	credsCache := cache.GetAllGithubCredentialsAsMap()
	for idx, cred := range creds {
		inCache, ok := credsCache[cred.ID]
		if ok {
			creds[idx].RateLimit = inCache.RateLimit
		}
	}
	return creds, nil
}

func (r *Runner) CreateGithubCredentials(ctx context.Context, param params.CreateGithubCredentialsParams) (params.ForgeCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeCredentials{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.ForgeCredentials{}, fmt.Errorf("failed to validate github credentials params: %w", err)
	}

	creds, err := r.store.CreateGithubCredentials(ctx, param)
	if err != nil {
		return params.ForgeCredentials{}, fmt.Errorf("failed to create github credentials: %w", err)
	}

	return creds, nil
}

func (r *Runner) GetGithubCredentials(ctx context.Context, id uint) (params.ForgeCredentials, error) {
	creds, err := r.store.GetGithubCredentials(ctx, id, true)
	if err != nil {
		return params.ForgeCredentials{}, fmt.Errorf("failed to get github credentials: %w", err)
	}

	cached, ok := cache.GetGithubCredentials((creds.ID))
	if ok {
		creds.RateLimit = cached.RateLimit
	}

	return creds, nil
}

func (r *Runner) DeleteGithubCredentials(ctx context.Context, id uint) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	if err := r.store.DeleteGithubCredentials(ctx, id); err != nil {
		return fmt.Errorf("failed to delete github credentials: %w", err)
	}

	return nil
}

func (r *Runner) UpdateGithubCredentials(ctx context.Context, id uint, param params.UpdateGithubCredentialsParams) (params.ForgeCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeCredentials{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.ForgeCredentials{}, fmt.Errorf("failed to validate github credentials params: %w", err)
	}

	newCreds, err := r.store.UpdateGithubCredentials(ctx, id, param)
	if err != nil {
		return params.ForgeCredentials{}, fmt.Errorf("failed to update github credentials: %w", err)
	}

	return newCreds, nil
}
