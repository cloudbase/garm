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
	"log/slog"
	"strings"

	"github.com/pkg/errors"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util/appdefaults"
)

func (r *Runner) CreateRepository(ctx context.Context, param params.CreateRepoParams) (repo params.Repository, err error) {
	if !auth.IsAdmin(ctx) {
		return repo, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.Repository{}, errors.Wrap(err, "validating params")
	}

	creds, err := r.store.GetGithubCredentialsByName(ctx, param.CredentialsName, true)
	if err != nil {
		return params.Repository{}, runnerErrors.NewBadRequestError("credentials %s not defined", param.CredentialsName)
	}

	_, err = r.store.GetRepository(ctx, param.Owner, param.Name, creds.Endpoint.Name)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return params.Repository{}, errors.Wrap(err, "fetching repo")
		}
	} else {
		return params.Repository{}, runnerErrors.NewConflictError("repository %s/%s already exists", param.Owner, param.Name)
	}

	repo, err = r.store.CreateRepository(ctx, param.Owner, param.Name, creds.Name, param.WebhookSecret, param.PoolBalancerType)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "creating repository")
	}

	defer func() {
		if err != nil {
			if deleteErr := r.store.DeleteRepository(ctx, repo.ID); deleteErr != nil {
				slog.With(slog.Any("error", deleteErr)).ErrorContext(
					ctx, "failed to delete repository",
					"repository_id", repo.ID)
			}
		}
	}()

	// Use the admin context in the pool manager. Any access control is already done above when
	// updating the store.
	poolMgr, err := r.poolManagerCtrl.CreateRepoPoolManager(r.ctx, repo, r.providers, r.store)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "creating repo pool manager")
	}
	if err := poolMgr.Start(); err != nil {
		if deleteErr := r.poolManagerCtrl.DeleteRepoPoolManager(repo); deleteErr != nil {
			slog.With(slog.Any("error", deleteErr)).ErrorContext(
				ctx, "failed to cleanup pool manager for repo",
				"repository_id", repo.ID)
		}
		return params.Repository{}, errors.Wrap(err, "starting repo pool manager")
	}
	return repo, nil
}

func (r *Runner) ListRepositories(ctx context.Context) ([]params.Repository, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	repos, err := r.store.ListRepositories(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "listing repositories")
	}

	var allRepos []params.Repository

	for _, repo := range repos {
		poolMgr, err := r.poolManagerCtrl.GetRepoPoolManager(repo)
		if err != nil {
			repo.PoolManagerStatus.IsRunning = false
			repo.PoolManagerStatus.FailureReason = fmt.Sprintf("failed to get pool manager: %q", err)
		} else {
			repo.PoolManagerStatus = poolMgr.Status()
		}
		allRepos = append(allRepos, repo)
	}

	return allRepos, nil
}

func (r *Runner) GetRepositoryByID(ctx context.Context, repoID string) (params.Repository, error) {
	if !auth.IsAdmin(ctx) {
		return params.Repository{}, runnerErrors.ErrUnauthorized
	}

	repo, err := r.store.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "fetching repository")
	}

	poolMgr, err := r.poolManagerCtrl.GetRepoPoolManager(repo)
	if err != nil {
		repo.PoolManagerStatus.IsRunning = false
		repo.PoolManagerStatus.FailureReason = fmt.Sprintf("failed to get pool manager: %q", err)
	}
	repo.PoolManagerStatus = poolMgr.Status()
	return repo, nil
}

func (r *Runner) DeleteRepository(ctx context.Context, repoID string, keepWebhook bool) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	repo, err := r.store.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return errors.Wrap(err, "fetching repo")
	}

	entity, err := repo.GetEntity()
	if err != nil {
		return errors.Wrap(err, "getting entity")
	}

	pools, err := r.store.ListEntityPools(ctx, entity)
	if err != nil {
		return errors.Wrap(err, "fetching repo pools")
	}

	if len(pools) > 0 {
		poolIDs := []string{}
		for _, pool := range pools {
			poolIDs = append(poolIDs, pool.ID)
		}

		return runnerErrors.NewBadRequestError("repo has pools defined (%s)", strings.Join(poolIDs, ", "))
	}

	if !keepWebhook && r.config.Default.EnableWebhookManagement {
		poolMgr, err := r.poolManagerCtrl.GetRepoPoolManager(repo)
		if err != nil {
			return errors.Wrap(err, "fetching pool manager")
		}

		if err := poolMgr.UninstallWebhook(ctx); err != nil {
			// nolint:golangci-lint,godox
			// TODO(gabriel-samfira): Should we error out here?
			slog.With(slog.Any("error", err)).ErrorContext(
				ctx, "failed to uninstall webhook",
				"pool_manager_id", poolMgr.ID())
		}
	}

	if err := r.poolManagerCtrl.DeleteRepoPoolManager(repo); err != nil {
		return errors.Wrap(err, "deleting repo pool manager")
	}

	if err := r.store.DeleteRepository(ctx, repoID); err != nil {
		return errors.Wrap(err, "removing repository")
	}
	return nil
}

func (r *Runner) UpdateRepository(ctx context.Context, repoID string, param params.UpdateEntityParams) (params.Repository, error) {
	if !auth.IsAdmin(ctx) {
		return params.Repository{}, runnerErrors.ErrUnauthorized
	}

	r.mux.Lock()
	defer r.mux.Unlock()

	switch param.PoolBalancerType {
	case params.PoolBalancerTypeRoundRobin, params.PoolBalancerTypePack, params.PoolBalancerTypeNone:
	default:
		return params.Repository{}, runnerErrors.NewBadRequestError("invalid pool balancer type: %s", param.PoolBalancerType)
	}

	slog.InfoContext(ctx, "updating repository", "repo_id", repoID, "param", param)
	repo, err := r.store.UpdateRepository(ctx, repoID, param)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "updating repo")
	}

	poolMgr, err := r.poolManagerCtrl.GetRepoPoolManager(repo)
	if err != nil {
		return params.Repository{}, fmt.Errorf("failed to get pool manager: %w", err)
	}

	repo.PoolManagerStatus = poolMgr.Status()
	return repo, nil
}

func (r *Runner) CreateRepoPool(ctx context.Context, repoID string, param params.CreatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	createPoolParams, err := r.appendTagsToCreatePoolParams(param)
	if err != nil {
		return params.Pool{}, fmt.Errorf("failed to append tags to create pool params: %w", err)
	}

	if createPoolParams.RunnerBootstrapTimeout == 0 {
		createPoolParams.RunnerBootstrapTimeout = appdefaults.DefaultRunnerBootstrapTimeout
	}

	entity := params.GithubEntity{
		ID:         repoID,
		EntityType: params.GithubEntityTypeRepository,
	}

	pool, err := r.store.CreateEntityPool(ctx, entity, createPoolParams)
	if err != nil {
		return params.Pool{}, fmt.Errorf("failed to create pool: %w", err)
	}

	return pool, nil
}

func (r *Runner) GetRepoPoolByID(ctx context.Context, repoID, poolID string) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	entity := params.GithubEntity{
		ID:         repoID,
		EntityType: params.GithubEntityTypeRepository,
	}

	pool, err := r.store.GetEntityPool(ctx, entity, poolID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}

	return pool, nil
}

func (r *Runner) DeleteRepoPool(ctx context.Context, repoID, poolID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	entity := params.GithubEntity{
		ID:         repoID,
		EntityType: params.GithubEntityTypeRepository,
	}
	pool, err := r.store.GetEntityPool(ctx, entity, poolID)
	if err != nil {
		return errors.Wrap(err, "fetching pool")
	}

	// nolint:golangci-lint,godox
	// TODO: implement a count function
	if len(pool.Instances) > 0 {
		runnerIDs := []string{}
		for _, run := range pool.Instances {
			runnerIDs = append(runnerIDs, run.ID)
		}
		return runnerErrors.NewBadRequestError("pool has runners: %s", strings.Join(runnerIDs, ", "))
	}

	if err := r.store.DeleteEntityPool(ctx, entity, poolID); err != nil {
		return errors.Wrap(err, "deleting pool")
	}
	return nil
}

func (r *Runner) ListRepoPools(ctx context.Context, repoID string) ([]params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return []params.Pool{}, runnerErrors.ErrUnauthorized
	}
	entity := params.GithubEntity{
		ID:         repoID,
		EntityType: params.GithubEntityTypeRepository,
	}
	pools, err := r.store.ListEntityPools(ctx, entity)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}
	return pools, nil
}

func (r *Runner) ListPoolInstances(ctx context.Context, poolID string) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	instances, err := r.store.ListPoolInstances(ctx, poolID)
	if err != nil {
		return []params.Instance{}, errors.Wrap(err, "fetching instances")
	}
	return instances, nil
}

func (r *Runner) UpdateRepoPool(ctx context.Context, repoID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	entity := params.GithubEntity{
		ID:         repoID,
		EntityType: params.GithubEntityTypeRepository,
	}
	pool, err := r.store.GetEntityPool(ctx, entity, poolID)
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

	if minIdleRunners > maxRunners {
		return params.Pool{}, runnerErrors.NewBadRequestError("min_idle_runners cannot be larger than max_runners")
	}

	newPool, err := r.store.UpdateEntityPool(ctx, entity, poolID, param)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "updating pool")
	}
	return newPool, nil
}

func (r *Runner) ListRepoInstances(ctx context.Context, repoID string) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}
	entity := params.GithubEntity{
		ID:         repoID,
		EntityType: params.GithubEntityTypeRepository,
	}
	instances, err := r.store.ListEntityInstances(ctx, entity)
	if err != nil {
		return []params.Instance{}, errors.Wrap(err, "fetching instances")
	}
	return instances, nil
}

func (r *Runner) findRepoPoolManager(owner, name, endpointName string) (common.PoolManager, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	repo, err := r.store.GetRepository(r.ctx, owner, name, endpointName)
	if err != nil {
		return nil, errors.Wrap(err, "fetching repo")
	}

	poolManager, err := r.poolManagerCtrl.GetRepoPoolManager(repo)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pool manager for repo")
	}
	return poolManager, nil
}

func (r *Runner) InstallRepoWebhook(ctx context.Context, repoID string, param params.InstallWebhookParams) (params.HookInfo, error) {
	if !auth.IsAdmin(ctx) {
		return params.HookInfo{}, runnerErrors.ErrUnauthorized
	}

	repo, err := r.store.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "fetching repo")
	}

	poolManager, err := r.poolManagerCtrl.GetRepoPoolManager(repo)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "fetching pool manager for repo")
	}

	info, err := poolManager.InstallWebhook(ctx, param)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "installing webhook")
	}
	return info, nil
}

func (r *Runner) UninstallRepoWebhook(ctx context.Context, repoID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	repo, err := r.store.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return errors.Wrap(err, "fetching repo")
	}

	poolManager, err := r.poolManagerCtrl.GetRepoPoolManager(repo)
	if err != nil {
		return errors.Wrap(err, "fetching pool manager for repo")
	}

	if err := poolManager.UninstallWebhook(ctx); err != nil {
		return errors.Wrap(err, "uninstalling webhook")
	}
	return nil
}

func (r *Runner) GetRepoWebhookInfo(ctx context.Context, repoID string) (params.HookInfo, error) {
	if !auth.IsAdmin(ctx) {
		return params.HookInfo{}, runnerErrors.ErrUnauthorized
	}

	repo, err := r.store.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "fetching repo")
	}

	poolManager, err := r.poolManagerCtrl.GetRepoPoolManager(repo)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "fetching pool manager for repo")
	}

	info, err := poolManager.GetWebhookInfo(ctx)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "getting webhook info")
	}
	return info, nil
}
