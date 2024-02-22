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
	"fmt"

	"github.com/pkg/errors"

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
		return nil, errors.Wrap(err, "fetching pools")
	}
	return pools, nil
}

func (r *Runner) GetPoolByID(ctx context.Context, poolID string) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	pool, err := r.store.GetPoolByID(ctx, poolID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
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
			return errors.Wrap(err, "fetching pool")
		}
		return nil
	}

	if len(pool.Instances) > 0 {
		return runnerErrors.NewBadRequestError("pool has runners")
	}

	if err := r.store.DeletePoolByID(ctx, poolID); err != nil {
		return errors.Wrap(err, "deleting pool")
	}
	return nil
}

func (r *Runner) UpdatePoolByID(ctx context.Context, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	pool, err := r.store.GetPoolByID(ctx, poolID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
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

	if param.Tags != nil && len(param.Tags) > 0 {
		newTags, err := r.processTags(string(pool.OSArch), pool.OSType, param.Tags)
		if err != nil {
			return params.Pool{}, errors.Wrap(err, "processing tags")
		}
		param.Tags = newTags
	}

	var newPool params.Pool

	switch {
	case pool.RepoID != "":
		newPool, err = r.store.UpdateRepositoryPool(ctx, pool.RepoID, poolID, param)
	case pool.OrgID != "":
		newPool, err = r.store.UpdateOrganizationPool(ctx, pool.OrgID, poolID, param)
	case pool.EnterpriseID != "":
		newPool, err = r.store.UpdateEnterprisePool(ctx, pool.EnterpriseID, poolID, param)
	default:
		return params.Pool{}, fmt.Errorf("pool not found to a repo, org or enterprise")
	}

	if err != nil {
		return params.Pool{}, errors.Wrap(err, "updating pool")
	}
	return newPool, nil
}

func (r *Runner) ListAllJobs(ctx context.Context) ([]params.Job, error) {
	if !auth.IsAdmin(ctx) {
		return []params.Job{}, runnerErrors.ErrUnauthorized
	}

	jobs, err := r.store.ListAllJobs(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fetching jobs")
	}
	return jobs, nil
}
