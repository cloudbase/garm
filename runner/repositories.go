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
	"log/slog"
	"strings"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
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
		return params.Repository{}, fmt.Errorf("error validating params: %w", err)
	}

	var creds params.ForgeCredentials
	switch param.ForgeType {
	case params.GithubEndpointType:
		creds, err = r.store.GetGithubCredentialsByName(ctx, param.CredentialsName, true)
	case params.GiteaEndpointType:
		creds, err = r.store.GetGiteaCredentialsByName(ctx, param.CredentialsName, true)
	default:
		creds, err = r.ResolveForgeCredentialByName(ctx, param.CredentialsName)
	}

	if err != nil {
		return params.Repository{}, runnerErrors.NewBadRequestError("credentials %s not defined", param.CredentialsName)
	}

	_, err = r.store.GetRepository(ctx, param.Owner, param.Name, creds.Endpoint.Name)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return params.Repository{}, fmt.Errorf("error fetching repo: %w", err)
		}
	} else {
		return params.Repository{}, runnerErrors.NewConflictError("repository %s/%s already exists", param.Owner, param.Name)
	}

	repo, err = r.store.CreateRepository(ctx, param.Owner, param.Name, creds, param.WebhookSecret, param.PoolBalancerType)
	if err != nil {
		return params.Repository{}, fmt.Errorf("error creating repository: %w", err)
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
		return params.Repository{}, fmt.Errorf("error creating repo pool manager: %w", err)
	}
	if err := poolMgr.Start(); err != nil {
		if deleteErr := r.poolManagerCtrl.DeleteRepoPoolManager(repo); deleteErr != nil {
			slog.With(slog.Any("error", deleteErr)).ErrorContext(
				ctx, "failed to cleanup pool manager for repo",
				"repository_id", repo.ID)
		}
		return params.Repository{}, fmt.Errorf("error starting repo pool manager: %w", err)
	}
	return repo, nil
}

func (r *Runner) ListRepositories(ctx context.Context, filter params.RepositoryFilter) ([]params.Repository, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	repos, err := r.store.ListRepositories(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("error listing repositories: %w", err)
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
		return params.Repository{}, fmt.Errorf("error fetching repository: %w", err)
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
		return fmt.Errorf("error fetching repo: %w", err)
	}

	entity, err := repo.GetEntity()
	if err != nil {
		return fmt.Errorf("error getting entity: %w", err)
	}

	pools, err := r.store.ListEntityPools(ctx, entity)
	if err != nil {
		return fmt.Errorf("error fetching repo pools: %w", err)
	}

	if len(pools) > 0 {
		poolIDs := []string{}
		for _, pool := range pools {
			poolIDs = append(poolIDs, pool.ID)
		}

		return runnerErrors.NewBadRequestError("repo has pools defined (%s)", strings.Join(poolIDs, ", "))
	}

	scaleSets, err := r.store.ListEntityScaleSets(ctx, entity)
	if err != nil {
		return fmt.Errorf("error fetching repo scale sets: %w", err)
	}

	if len(scaleSets) > 0 {
		return runnerErrors.NewBadRequestError("repo has scale sets defined; delete them first")
	}

	if !keepWebhook && r.config.Default.EnableWebhookManagement {
		poolMgr, err := r.poolManagerCtrl.GetRepoPoolManager(repo)
		if err != nil {
			return fmt.Errorf("error fetching pool manager: %w", err)
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
		return fmt.Errorf("error deleting repo pool manager: %w", err)
	}

	if err := r.store.DeleteRepository(ctx, repoID); err != nil {
		return fmt.Errorf("error removing repository: %w", err)
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
		return params.Repository{}, fmt.Errorf("error updating repo: %w", err)
	}

	poolMgr, err := r.poolManagerCtrl.GetRepoPoolManager(repo)
	if err != nil {
		return params.Repository{}, fmt.Errorf("error getting pool manager: %w", err)
	}

	repo.PoolManagerStatus = poolMgr.Status()
	return repo, nil
}

func (r *Runner) findTemplate(ctx context.Context, entity params.ForgeEntity, osType commonParams.OSType, templateID *uint) (params.Template, error) {
	var template params.Template
	if templateID != nil {
		dbTpl, err := r.store.GetTemplate(ctx, *templateID)
		if err != nil {
			return params.Template{}, fmt.Errorf("failed to get template: %w", err)
		}
		template = dbTpl
	} else {
		tpls, err := r.store.ListTemplates(ctx, &osType, &entity.Credentials.ForgeType, nil)
		if err != nil {
			return params.Template{}, fmt.Errorf("failed to list templates: %w", err)
		}
		if len(tpls) == 0 {
			return params.Template{}, runnerErrors.NewBadRequestError("no template ID supplied and no default template can be found")
		}
		for _, val := range tpls {
			slog.InfoContext(ctx, "considering template", "name", val.Name, "os_type", val.OSType, "pool_os_type", osType, "forge_type", val.ForgeType, "pool_forge_type", entity.Credentials.ForgeType, "owner", val.Owner)
			if val.OSType == osType && val.ForgeType == entity.Credentials.ForgeType && val.Owner == "system" {
				template = val
				break
			}
		}
	}
	if template.ID == 0 {
		return params.Template{}, runnerErrors.NewBadRequestError("no template ID supplied and no default template can be found")
	}

	if template.OSType != osType {
		return params.Template{}, runnerErrors.NewBadRequestError("selected template OS type (%s) and pool OS type (%s) do not match", template.OSType, osType)
	}

	if template.ForgeType != entity.Credentials.ForgeType {
		return params.Template{}, runnerErrors.NewBadRequestError("selected template forge type (%s) and pool forge type (%s) do not match", template.ForgeType, entity.Credentials.ForgeType)
	}
	return template, nil
}

func (r *Runner) CreateRepoPool(ctx context.Context, repoID string, param params.CreatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	createPoolParams, err := r.appendTagsToCreatePoolParams(param)
	if err != nil {
		return params.Pool{}, fmt.Errorf("error appending tags to create pool params: %w", err)
	}

	if createPoolParams.RunnerBootstrapTimeout == 0 {
		createPoolParams.RunnerBootstrapTimeout = appdefaults.DefaultRunnerBootstrapTimeout
	}

	entity, err := r.store.GetForgeEntity(ctx, params.ForgeEntityTypeRepository, repoID)
	if err != nil {
		return params.Pool{}, fmt.Errorf("failed to get repo: %w", err)
	}

	template, err := r.findTemplate(ctx, entity, param.OSType, param.TemplateID)
	if err != nil {
		return params.Pool{}, fmt.Errorf("failed to find suitable template: %w", err)
	}

	createPoolParams.TemplateID = &template.ID
	pool, err := r.store.CreateEntityPool(ctx, entity, createPoolParams)
	if err != nil {
		return params.Pool{}, fmt.Errorf("error creating pool: %w", err)
	}

	return pool, nil
}

func (r *Runner) GetRepoPoolByID(ctx context.Context, repoID, poolID string) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	entity := params.ForgeEntity{
		ID:         repoID,
		EntityType: params.ForgeEntityTypeRepository,
	}

	pool, err := r.store.GetEntityPool(ctx, entity, poolID)
	if err != nil {
		return params.Pool{}, fmt.Errorf("error fetching pool: %w", err)
	}

	return pool, nil
}

func (r *Runner) DeleteRepoPool(ctx context.Context, repoID, poolID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	entity := params.ForgeEntity{
		ID:         repoID,
		EntityType: params.ForgeEntityTypeRepository,
	}
	pool, err := r.store.GetEntityPool(ctx, entity, poolID)
	if err != nil {
		return fmt.Errorf("error fetching pool: %w", err)
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
		return fmt.Errorf("error deleting pool: %w", err)
	}
	return nil
}

func (r *Runner) ListRepoPools(ctx context.Context, repoID string) ([]params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return []params.Pool{}, runnerErrors.ErrUnauthorized
	}
	entity := params.ForgeEntity{
		ID:         repoID,
		EntityType: params.ForgeEntityTypeRepository,
	}
	pools, err := r.store.ListEntityPools(ctx, entity)
	if err != nil {
		return nil, fmt.Errorf("error fetching pools: %w", err)
	}
	return pools, nil
}

func (r *Runner) ListPoolInstances(ctx context.Context, poolID string) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	instances, err := r.store.ListPoolInstances(ctx, poolID)
	if err != nil {
		return []params.Instance{}, fmt.Errorf("error fetching instances: %w", err)
	}
	return instances, nil
}

func (r *Runner) UpdateRepoPool(ctx context.Context, repoID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}
	entity, err := r.store.GetForgeEntity(ctx, params.ForgeEntityTypeRepository, repoID)
	if err != nil {
		return params.Pool{}, fmt.Errorf("failed to get repo: %w", err)
	}

	pool, err := r.store.GetEntityPool(ctx, entity, poolID)
	if err != nil {
		return params.Pool{}, fmt.Errorf("error fetching pool: %w", err)
	}

	if param.TemplateID != nil {
		osType := param.OSType
		if osType == "" {
			osType = pool.OSType
		}
		template, err := r.findTemplate(ctx, entity, osType, param.TemplateID)
		if err != nil {
			return params.Pool{}, fmt.Errorf("failed to find suitable template: %w", err)
		}
		param.TemplateID = &template.ID
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
		return params.Pool{}, fmt.Errorf("error updating pool: %w", err)
	}
	return newPool, nil
}

func (r *Runner) ListRepoInstances(ctx context.Context, repoID string) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}
	entity := params.ForgeEntity{
		ID:         repoID,
		EntityType: params.ForgeEntityTypeRepository,
	}
	instances, err := r.store.ListEntityInstances(ctx, entity)
	if err != nil {
		return []params.Instance{}, fmt.Errorf("error , errfetching instances: %w", err)
	}
	return instances, nil
}

func (r *Runner) findRepoPoolManager(owner, name, endpointName string) (common.PoolManager, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	repo, err := r.store.GetRepository(r.ctx, owner, name, endpointName)
	if err != nil {
		return nil, fmt.Errorf("error fetching repo: %w", err)
	}

	poolManager, err := r.poolManagerCtrl.GetRepoPoolManager(repo)
	if err != nil {
		return nil, fmt.Errorf("error fetching pool manager for repo: %w", err)
	}
	return poolManager, nil
}

func (r *Runner) InstallRepoWebhook(ctx context.Context, repoID string, param params.InstallWebhookParams) (params.HookInfo, error) {
	if !auth.IsAdmin(ctx) {
		return params.HookInfo{}, runnerErrors.ErrUnauthorized
	}

	repo, err := r.store.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return params.HookInfo{}, fmt.Errorf("error fetching repo: %w", err)
	}

	poolManager, err := r.poolManagerCtrl.GetRepoPoolManager(repo)
	if err != nil {
		return params.HookInfo{}, fmt.Errorf("error fetching pool manager for repo: %w", err)
	}

	info, err := poolManager.InstallWebhook(ctx, param)
	if err != nil {
		return params.HookInfo{}, fmt.Errorf("error installing webhook: %w", err)
	}
	return info, nil
}

func (r *Runner) UninstallRepoWebhook(ctx context.Context, repoID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	repo, err := r.store.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("error fetching repo: %w", err)
	}

	poolManager, err := r.poolManagerCtrl.GetRepoPoolManager(repo)
	if err != nil {
		return fmt.Errorf("error fetching pool manager for repo: %w", err)
	}

	if err := poolManager.UninstallWebhook(ctx); err != nil {
		return fmt.Errorf("error uninstalling webhook: %w", err)
	}
	return nil
}

func (r *Runner) GetRepoWebhookInfo(ctx context.Context, repoID string) (params.HookInfo, error) {
	if !auth.IsAdmin(ctx) {
		return params.HookInfo{}, runnerErrors.ErrUnauthorized
	}

	repo, err := r.store.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return params.HookInfo{}, fmt.Errorf("error fetching repo: %w", err)
	}

	poolManager, err := r.poolManagerCtrl.GetRepoPoolManager(repo)
	if err != nil {
		return params.HookInfo{}, fmt.Errorf("error fetching pool manager for repo: %w", err)
	}

	info, err := poolManager.GetWebhookInfo(ctx)
	if err != nil {
		return params.HookInfo{}, fmt.Errorf("error getting webhook info: %w", err)
	}
	return info, nil
}
