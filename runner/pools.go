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

package runner

import (
	"context"
	"errors"
	"fmt"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

func (r *Runner) ListAllPools(ctx context.Context) ([]params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return []params.Pool{}, runnerErrors.ErrUnauthorized
	}

	pools, err := r.store.ListAllPools(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching pools: %w", err)
	}
	return pools, nil
}

func (r *Runner) GetPoolByID(ctx context.Context, poolID string) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	pool, err := r.store.GetPoolByID(ctx, poolID)
	if err != nil {
		return params.Pool{}, fmt.Errorf("error fetching pool: %w", err)
	}
	return pool, nil
}

func (r *Runner) DeletePoolByID(ctx context.Context, poolID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	pool, err := r.store.GetPoolByID(ctx, poolID)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return fmt.Errorf("error fetching pool: %w", err)
		}
		return nil
	}

	if len(pool.Instances) > 0 {
		return runnerErrors.NewBadRequestError("pool has runners")
	}

	if err := r.store.DeletePoolByID(ctx, poolID); err != nil {
		return fmt.Errorf("error deleting pool: %w", err)
	}
	return nil
}

func (r *Runner) UpdatePoolByID(ctx context.Context, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	pool, err := r.store.GetPoolByID(ctx, poolID)
	if err != nil {
		return params.Pool{}, fmt.Errorf("error fetching pool: %w", err)
	}

	maxRunners := pool.MaxRunners
	minIdleRunners := pool.MinIdleRunners

	if param.MaxRunners != nil {
		maxRunners = *param.MaxRunners
	}
	if param.MinIdleRunners != nil {
		minIdleRunners = *param.MinIdleRunners
	}

	if param.RunnerBootstrapTimeout != nil && *param.RunnerBootstrapTimeout == 0 {
		return params.Pool{}, runnerErrors.NewBadRequestError("runner_bootstrap_timeout cannot be 0")
	}

	if minIdleRunners > maxRunners {
		return params.Pool{}, runnerErrors.NewBadRequestError("min_idle_runners cannot be larger than max_runners")
	}

	entity, err := pool.GetEntity()
	if err != nil {
		return params.Pool{}, fmt.Errorf("error getting entity: %w", err)
	}

	newPool, err := r.store.UpdateEntityPool(ctx, entity, poolID, param)
	if err != nil {
		return params.Pool{}, fmt.Errorf("error updating pool: %w", err)
	}
	return newPool, nil
}

func (r *Runner) ListAllJobs(ctx context.Context) ([]params.Job, error) {
	if !auth.IsAdmin(ctx) {
		return []params.Job{}, runnerErrors.ErrUnauthorized
	}

	jobs, err := r.store.ListAllJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching jobs: %w", err)
	}
	return jobs, nil
}
