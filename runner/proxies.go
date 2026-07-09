// Copyright 2026 Cloudbase Solutions SRL
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
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

func (r *Runner) CreateProxy(ctx context.Context, param params.CreateProxyParams) (params.Proxy, error) {
	if !auth.IsAdmin(ctx) {
		return params.Proxy{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.Proxy{}, fmt.Errorf("failed to validate create params: %w", err)
	}

	proxy, err := r.store.CreateProxy(ctx, param)
	if err != nil {
		return params.Proxy{}, fmt.Errorf("failed to create proxy: %w", err)
	}
	return proxy, nil
}

func (r *Runner) GetProxy(ctx context.Context, id uint) (params.Proxy, error) {
	if !auth.IsAdmin(ctx) {
		return params.Proxy{}, runnerErrors.ErrUnauthorized
	}

	proxy, err := r.store.GetProxy(ctx, id)
	if err != nil {
		return params.Proxy{}, fmt.Errorf("failed to get proxy: %w", err)
	}
	return proxy, nil
}

func (r *Runner) GetProxyByName(ctx context.Context, name string) (params.Proxy, error) {
	if !auth.IsAdmin(ctx) {
		return params.Proxy{}, runnerErrors.ErrUnauthorized
	}

	proxy, err := r.store.GetProxyByName(ctx, name)
	if err != nil {
		return params.Proxy{}, fmt.Errorf("failed to get proxy: %w", err)
	}
	return proxy, nil
}

func (r *Runner) ListProxies(ctx context.Context) ([]params.Proxy, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	proxies, err := r.store.ListProxies(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list proxies: %w", err)
	}
	return proxies, nil
}

func (r *Runner) UpdateProxy(ctx context.Context, id uint, param params.UpdateProxyParams) (params.Proxy, error) {
	if !auth.IsAdmin(ctx) {
		return params.Proxy{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.Proxy{}, fmt.Errorf("failed to validate update params: %w", err)
	}

	proxy, err := r.store.UpdateProxy(ctx, id, param)
	if err != nil {
		return params.Proxy{}, fmt.Errorf("failed to update proxy: %w", err)
	}
	return proxy, nil
}

func (r *Runner) DeleteProxy(ctx context.Context, id uint) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	if err := r.store.DeleteProxy(ctx, id); err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return fmt.Errorf("failed to delete proxy: %w", err)
		}
	}
	return nil
}
