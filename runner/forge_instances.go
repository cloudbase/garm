// Copyright 2026 Cloudbase Solutions SRL
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
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util/appdefaults"
)

func (r *Runner) CreateForgeInstance(ctx context.Context, param params.CreateForgeInstanceParams) (forgeInstance params.ForgeInstance, err error) {
	if !auth.IsAdmin(ctx) {
		return forgeInstance, runnerErrors.ErrUnauthorized
	}

	err = param.Validate()
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error validating params: %w", err)
	}

	var creds params.ForgeCredentials
	switch param.ForgeType {
	case params.GiteaEndpointType:
		creds, err = r.store.GetGiteaCredentialsByName(ctx, param.CredentialsName, true)
	default:
		return params.ForgeInstance{}, runnerErrors.NewBadRequestError("unsupported forge type: %s", param.ForgeType)
	}
	if err != nil {
		return params.ForgeInstance{}, runnerErrors.NewBadRequestError("credentials %s not defined", param.CredentialsName)
	}

	if creds.Endpoint.Name != param.EndpointName {
		return params.ForgeInstance{}, runnerErrors.NewBadRequestError("credentials endpoint %q does not match requested endpoint %q", creds.Endpoint.Name, param.EndpointName)
	}

	_, err = r.store.GetForgeInstance(ctx, param.EndpointName)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return params.ForgeInstance{}, fmt.Errorf("error fetching forge instance: %w", err)
		}
	} else {
		return params.ForgeInstance{}, runnerErrors.NewConflictError("forge instance for endpoint %s already exists", param.EndpointName)
	}

	forgeInstance, err = r.store.CreateForgeInstance(ctx, param.EndpointName, creds, param.WebhookSecret, param.PoolBalancerType, param.AgentMode)
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error creating forge instance: %w", err)
	}

	defer func() {
		if err != nil {
			if deleteErr := r.store.DeleteForgeInstance(ctx, forgeInstance.ID); deleteErr != nil {
				slog.With(slog.Any("error", deleteErr)).ErrorContext(
					ctx, "failed to delete forge instance",
					"forge_instance_id", forgeInstance.ID)
			}
		}
	}()

	var poolMgr common.PoolManager
	poolMgr, err = r.poolManagerCtrl.CreateForgeInstancePoolManager(r.ctx, forgeInstance, r.providers, r.store)
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error creating forge instance pool manager: %w", err)
	}
	if err := poolMgr.Start(); err != nil {
		if deleteErr := r.poolManagerCtrl.DeleteForgeInstancePoolManager(forgeInstance); deleteErr != nil {
			slog.With(slog.Any("error", deleteErr)).ErrorContext(
				ctx, "failed to cleanup pool manager for forge instance",
				"forge_instance_id", forgeInstance.ID)
		}
		return params.ForgeInstance{}, fmt.Errorf("error starting forge instance pool manager: %w", err)
	}
	return forgeInstance, nil
}

func (r *Runner) ListForgeInstances(ctx context.Context, filter params.ForgeInstanceFilter) ([]params.ForgeInstance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	forgeInstances, err := r.store.ListForgeInstances(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("error listing forge instances: %w", err)
	}

	return forgeInstances, nil
}

func (r *Runner) GetForgeInstanceByID(ctx context.Context, forgeInstanceID string) (params.ForgeInstance, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeInstance{}, runnerErrors.ErrUnauthorized
	}

	forgeInstance, err := r.store.GetForgeInstanceByID(ctx, forgeInstanceID)
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error fetching forge instance: %w", err)
	}

	return forgeInstance, nil
}

func (r *Runner) DeleteForgeInstance(ctx context.Context, forgeInstanceID string, keepWebhook bool) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	forgeInstance, err := r.store.GetForgeInstanceByID(ctx, forgeInstanceID)
	if err != nil {
		return fmt.Errorf("error fetching forge instance: %w", err)
	}

	entity, err := forgeInstance.GetEntity()
	if err != nil {
		return fmt.Errorf("error getting entity: %w", err)
	}

	pools, err := r.store.ListEntityPools(ctx, entity)
	if err != nil {
		return fmt.Errorf("error fetching forge instance pools: %w", err)
	}

	if len(pools) > 0 {
		poolIDs := []string{}
		for _, pool := range pools {
			poolIDs = append(poolIDs, pool.ID)
		}

		return runnerErrors.NewBadRequestError("forge instance has pools defined (%s)", strings.Join(poolIDs, ", "))
	}

	if !keepWebhook && r.config.Default.EnableWebhookManagement {
		poolMgr, err := r.poolManagerCtrl.GetForgeInstancePoolManager(forgeInstance)
		if err != nil {
			return fmt.Errorf("error fetching pool manager: %w", err)
		}

		if err := poolMgr.UninstallWebhook(ctx); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(
				ctx, "failed to uninstall webhook",
				"forge_instance_id", forgeInstance.ID)
		}
	}

	if err := r.poolManagerCtrl.DeleteForgeInstancePoolManager(forgeInstance); err != nil {
		return fmt.Errorf("error deleting forge instance pool manager: %w", err)
	}

	if err := r.store.DeleteForgeInstance(ctx, forgeInstanceID); err != nil {
		return fmt.Errorf("error removing forge instance %s: %w", forgeInstanceID, err)
	}
	return nil
}

func (r *Runner) UpdateForgeInstance(ctx context.Context, forgeInstanceID string, param params.UpdateEntityParams) (params.ForgeInstance, error) {
	if !auth.IsAdmin(ctx) {
		return params.ForgeInstance{}, runnerErrors.ErrUnauthorized
	}

	r.mux.Lock()
	defer r.mux.Unlock()

	switch param.PoolBalancerType {
	case params.PoolBalancerTypeRoundRobin, params.PoolBalancerTypePack, params.PoolBalancerTypeNone:
	default:
		return params.ForgeInstance{}, runnerErrors.NewBadRequestError("invalid pool balancer type: %s", param.PoolBalancerType)
	}

	forgeInstance, err := r.store.UpdateForgeInstance(ctx, forgeInstanceID, param)
	if err != nil {
		return params.ForgeInstance{}, fmt.Errorf("error updating forge instance: %w", err)
	}

	return forgeInstance, nil
}

func (r *Runner) CreateForgeInstancePool(ctx context.Context, forgeInstanceID string, param params.CreatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	createPoolParams, err := r.appendTagsToCreatePoolParams(param)
	if err != nil {
		return params.Pool{}, fmt.Errorf("failed to append tags to create pool params: %w", err)
	}

	if param.RunnerBootstrapTimeout == 0 {
		param.RunnerBootstrapTimeout = appdefaults.DefaultRunnerBootstrapTimeout
	}

	entity, err := r.store.GetForgeEntity(ctx, params.ForgeEntityTypeInstance, forgeInstanceID)
	if err != nil {
		return params.Pool{}, fmt.Errorf("failed to get forge instance: %w", err)
	}

	template, err := r.findTemplate(ctx, entity, param.OSType, param.TemplateID)
	if err != nil {
		return params.Pool{}, fmt.Errorf("failed to find suitable template: %w", err)
	}

	createPoolParams.TemplateID = &template.ID
	pool, err := r.store.CreateEntityPool(ctx, entity, createPoolParams)
	if err != nil {
		return params.Pool{}, fmt.Errorf("failed to create forge instance pool: %w", err)
	}

	return pool, nil
}

func (r *Runner) GetForgeInstancePoolByID(ctx context.Context, forgeInstanceID, poolID string) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}
	entity := params.ForgeEntity{
		ID:         forgeInstanceID,
		EntityType: params.ForgeEntityTypeInstance,
	}
	pool, err := r.store.GetEntityPool(ctx, entity, poolID)
	if err != nil {
		return params.Pool{}, fmt.Errorf("error fetching pool: %w", err)
	}
	return pool, nil
}

func (r *Runner) DeleteForgeInstancePool(ctx context.Context, forgeInstanceID, poolID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	entity := params.ForgeEntity{
		ID:         forgeInstanceID,
		EntityType: params.ForgeEntityTypeInstance,
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

func (r *Runner) ListForgeInstancePools(ctx context.Context, forgeInstanceID string) ([]params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return []params.Pool{}, runnerErrors.ErrUnauthorized
	}

	entity := params.ForgeEntity{
		ID:         forgeInstanceID,
		EntityType: params.ForgeEntityTypeInstance,
	}
	pools, err := r.store.ListEntityPools(ctx, entity)
	if err != nil {
		return nil, fmt.Errorf("error fetching pools: %w", err)
	}
	return pools, nil
}

func (r *Runner) UpdateForgeInstancePool(ctx context.Context, forgeInstanceID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}
	entity, err := r.store.GetForgeEntity(ctx, params.ForgeEntityTypeInstance, forgeInstanceID)
	if err != nil {
		return params.Pool{}, fmt.Errorf("failed to get forge instance: %w", err)
	}

	if param.TemplateID != nil {
		template, err := r.findTemplate(ctx, entity, param.OSType, param.TemplateID)
		if err != nil {
			return params.Pool{}, fmt.Errorf("failed to find suitable template: %w", err)
		}
		param.TemplateID = &template.ID
	}
	pool, err := r.store.GetEntityPool(ctx, entity, poolID)
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

	if minIdleRunners > maxRunners {
		return params.Pool{}, runnerErrors.NewBadRequestError("min_idle_runners cannot be larger than max_runners")
	}

	newPool, err := r.store.UpdateEntityPool(ctx, entity, poolID, param)
	if err != nil {
		return params.Pool{}, fmt.Errorf("error updating pool: %w", err)
	}
	return newPool, nil
}

func (r *Runner) ListForgeInstanceInstances(ctx context.Context, forgeInstanceID string) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}
	entity := params.ForgeEntity{
		ID:         forgeInstanceID,
		EntityType: params.ForgeEntityTypeInstance,
	}
	instances, err := r.store.ListEntityInstances(ctx, entity)
	if err != nil {
		return []params.Instance{}, fmt.Errorf("error fetching instances: %w", err)
	}
	return instances, nil
}

func (r *Runner) InstallForgeInstanceWebhook(ctx context.Context, forgeInstanceID string, param params.InstallWebhookParams) (params.HookInfo, error) {
	if !auth.IsAdmin(ctx) {
		return params.HookInfo{}, runnerErrors.ErrUnauthorized
	}

	fi, err := r.store.GetForgeInstanceByID(ctx, forgeInstanceID)
	if err != nil {
		return params.HookInfo{}, fmt.Errorf("error fetching forge instance: %w", err)
	}

	poolMgr, err := r.poolManagerCtrl.GetForgeInstancePoolManager(fi)
	if err != nil {
		return params.HookInfo{}, fmt.Errorf("error fetching pool manager for forge instance: %w", err)
	}

	info, err := poolMgr.InstallWebhook(ctx, param)
	if err != nil {
		return params.HookInfo{}, fmt.Errorf("error installing webhook: %w", err)
	}
	return info, nil
}

func (r *Runner) UninstallForgeInstanceWebhook(ctx context.Context, forgeInstanceID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	fi, err := r.store.GetForgeInstanceByID(ctx, forgeInstanceID)
	if err != nil {
		return fmt.Errorf("error fetching forge instance: %w", err)
	}

	poolMgr, err := r.poolManagerCtrl.GetForgeInstancePoolManager(fi)
	if err != nil {
		return fmt.Errorf("error fetching pool manager for forge instance: %w", err)
	}

	if err := poolMgr.UninstallWebhook(ctx); err != nil {
		return fmt.Errorf("error uninstalling webhook: %w", err)
	}
	return nil
}

func (r *Runner) GetForgeInstanceWebhookInfo(ctx context.Context, forgeInstanceID string) (params.HookInfo, error) {
	if !auth.IsAdmin(ctx) {
		return params.HookInfo{}, runnerErrors.ErrUnauthorized
	}

	fi, err := r.store.GetForgeInstanceByID(ctx, forgeInstanceID)
	if err != nil {
		return params.HookInfo{}, fmt.Errorf("error fetching forge instance: %w", err)
	}

	poolMgr, err := r.poolManagerCtrl.GetForgeInstancePoolManager(fi)
	if err != nil {
		return params.HookInfo{}, fmt.Errorf("error fetching pool manager for forge instance: %w", err)
	}

	info, err := poolMgr.GetWebhookInfo(ctx)
	if err != nil {
		return params.HookInfo{}, fmt.Errorf("error fetching webhook info: %w", err)
	}
	return info, nil
}

func (r *Runner) findForgeInstancePoolManager(endpointName string) (common.PoolManager, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	forgeInstance, err := r.store.GetForgeInstance(r.ctx, endpointName)
	if err != nil {
		return nil, fmt.Errorf("error fetching forge instance: %w", err)
	}

	poolManager, err := r.poolManagerCtrl.GetForgeInstancePoolManager(forgeInstance)
	if err != nil {
		return nil, fmt.Errorf("error fetching pool manager for forge instance: %w", err)
	}
	return poolManager, nil
}

func (r *Runner) createAndStartForgeInstancePoolManager(forgeInstance params.ForgeInstance) {
	slog.InfoContext(r.ctx, "creating pool manager for forge instance", "forge_instance_id", forgeInstance.ID, "endpoint", forgeInstance.Endpoint.Name)
	poolMgr, err := r.poolManagerCtrl.CreateForgeInstancePoolManager(r.ctx, forgeInstance, r.providers, r.store)
	if err != nil {
		slog.ErrorContext(r.ctx, "creating forge instance pool manager", "forge_instance_id", forgeInstance.ID, "error", err)
		r.failedEntities[forgeInstance.ID] = failedEntity{entityType: params.ForgeEntityTypeInstance, id: forgeInstance.ID}
		r.backoff.RecordFailure(forgeInstance.ID)
		return
	}
	if err := poolMgr.Start(); err != nil {
		slog.ErrorContext(r.ctx, "starting forge instance pool manager", "forge_instance_id", forgeInstance.ID, "error", err)
		if deleteErr := r.poolManagerCtrl.DeleteForgeInstancePoolManager(forgeInstance); deleteErr != nil {
			slog.ErrorContext(r.ctx, "cleaning up failed forge instance pool manager", "forge_instance_id", forgeInstance.ID, "error", deleteErr)
		}
		r.failedEntities[forgeInstance.ID] = failedEntity{entityType: params.ForgeEntityTypeInstance, id: forgeInstance.ID}
		r.backoff.RecordFailure(forgeInstance.ID)
		return
	}
	delete(r.failedEntities, forgeInstance.ID)
	r.backoff.RecordSuccess(forgeInstance.ID)
}
