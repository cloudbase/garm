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

func (r *Runner) CreateEnterprise(ctx context.Context, param params.CreateEnterpriseParams) (enterprise params.Enterprise, err error) {
	if !auth.IsAdmin(ctx) {
		return enterprise, runnerErrors.ErrUnauthorized
	}

	err = param.Validate()
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error validating params: %w", err)
	}

	creds, err := r.store.GetGithubCredentialsByName(ctx, param.CredentialsName, true)
	if err != nil {
		return params.Enterprise{}, runnerErrors.NewBadRequestError("credentials %s not defined", param.CredentialsName)
	}

	_, err = r.store.GetEnterprise(ctx, param.Name, creds.Endpoint.Name)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return params.Enterprise{}, fmt.Errorf("error fetching enterprise: %w", err)
		}
	} else {
		return params.Enterprise{}, runnerErrors.NewConflictError("enterprise %s already exists", param.Name)
	}

	enterprise, err = r.store.CreateEnterprise(ctx, param.Name, creds, param.WebhookSecret, param.PoolBalancerType)
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error creating enterprise: %w", err)
	}

	defer func() {
		if err != nil {
			if deleteErr := r.store.DeleteEnterprise(ctx, enterprise.ID); deleteErr != nil {
				slog.With(slog.Any("error", deleteErr)).ErrorContext(
					ctx, "failed to delete enterprise",
					"enterprise_id", enterprise.ID)
			}
		}
	}()

	// Use the admin context in the pool manager. Any access control is already done above when
	// updating the store.
	var poolMgr common.PoolManager
	poolMgr, err = r.poolManagerCtrl.CreateEnterprisePoolManager(r.ctx, enterprise, r.providers, r.store)
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error creating enterprise pool manager: %w", err)
	}
	if err := poolMgr.Start(); err != nil {
		if deleteErr := r.poolManagerCtrl.DeleteEnterprisePoolManager(enterprise); deleteErr != nil {
			slog.With(slog.Any("error", deleteErr)).ErrorContext(
				ctx, "failed to cleanup pool manager for enterprise",
				"enterprise_id", enterprise.ID)
		}
		return params.Enterprise{}, fmt.Errorf("error starting enterprise pool manager: %w", err)
	}
	return enterprise, nil
}

func (r *Runner) ListEnterprises(ctx context.Context, filter params.EnterpriseFilter) ([]params.Enterprise, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	enterprises, err := r.store.ListEnterprises(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("error listing enterprises: %w", err)
	}

	var allEnterprises []params.Enterprise

	for _, enterprise := range enterprises {
		poolMgr, err := r.poolManagerCtrl.GetEnterprisePoolManager(enterprise)
		if err != nil {
			enterprise.PoolManagerStatus.IsRunning = false
			enterprise.PoolManagerStatus.FailureReason = fmt.Sprintf("failed to get pool manager: %q", err)
		} else {
			enterprise.PoolManagerStatus = poolMgr.Status()
		}
		allEnterprises = append(allEnterprises, enterprise)
	}

	return allEnterprises, nil
}

func (r *Runner) GetEnterpriseByID(ctx context.Context, enterpriseID string) (params.Enterprise, error) {
	if !auth.IsAdmin(ctx) {
		return params.Enterprise{}, runnerErrors.ErrUnauthorized
	}

	enterprise, err := r.store.GetEnterpriseByID(ctx, enterpriseID)
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error fetching enterprise: %w", err)
	}
	poolMgr, err := r.poolManagerCtrl.GetEnterprisePoolManager(enterprise)
	if err != nil {
		enterprise.PoolManagerStatus.IsRunning = false
		enterprise.PoolManagerStatus.FailureReason = fmt.Sprintf("failed to get pool manager: %q", err)
	}
	enterprise.PoolManagerStatus = poolMgr.Status()
	return enterprise, nil
}

func (r *Runner) DeleteEnterprise(ctx context.Context, enterpriseID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	enterprise, err := r.store.GetEnterpriseByID(ctx, enterpriseID)
	if err != nil {
		return fmt.Errorf("error fetching enterprise: %w", err)
	}

	entity, err := enterprise.GetEntity()
	if err != nil {
		return fmt.Errorf("error getting entity: %w", err)
	}

	pools, err := r.store.ListEntityPools(ctx, entity)
	if err != nil {
		return fmt.Errorf("error fetching enterprise pools: %w", err)
	}

	if len(pools) > 0 {
		poolIDs := []string{}
		for _, pool := range pools {
			poolIDs = append(poolIDs, pool.ID)
		}

		return runnerErrors.NewBadRequestError("enterprise has pools defined (%s)", strings.Join(poolIDs, ", "))
	}

	scaleSets, err := r.store.ListEntityScaleSets(ctx, entity)
	if err != nil {
		return fmt.Errorf("error fetching enterprise scale sets: %w", err)
	}

	if len(scaleSets) > 0 {
		return runnerErrors.NewBadRequestError("enterprise has scale sets defined; delete them first")
	}

	if err := r.poolManagerCtrl.DeleteEnterprisePoolManager(enterprise); err != nil {
		return fmt.Errorf("error deleting enterprise pool manager: %w", err)
	}

	if err := r.store.DeleteEnterprise(ctx, enterpriseID); err != nil {
		return fmt.Errorf("error removing enterprise %s: %w", enterpriseID, err)
	}
	return nil
}

func (r *Runner) UpdateEnterprise(ctx context.Context, enterpriseID string, param params.UpdateEntityParams) (params.Enterprise, error) {
	if !auth.IsAdmin(ctx) {
		return params.Enterprise{}, runnerErrors.ErrUnauthorized
	}

	r.mux.Lock()
	defer r.mux.Unlock()

	switch param.PoolBalancerType {
	case params.PoolBalancerTypeRoundRobin, params.PoolBalancerTypePack, params.PoolBalancerTypeNone:
	default:
		return params.Enterprise{}, runnerErrors.NewBadRequestError("invalid pool balancer type: %s", param.PoolBalancerType)
	}

	enterprise, err := r.store.UpdateEnterprise(ctx, enterpriseID, param)
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("error updating enterprise: %w", err)
	}

	poolMgr, err := r.poolManagerCtrl.GetEnterprisePoolManager(enterprise)
	if err != nil {
		return params.Enterprise{}, fmt.Errorf("failed to get enterprise pool manager: %w", err)
	}

	enterprise.PoolManagerStatus = poolMgr.Status()
	return enterprise, nil
}

func (r *Runner) CreateEnterprisePool(ctx context.Context, enterpriseID string, param params.CreatePoolParams) (params.Pool, error) {
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

	entity, err := r.store.GetForgeEntity(ctx, params.ForgeEntityTypeEnterprise, enterpriseID)
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
		return params.Pool{}, fmt.Errorf("failed to create enterprise pool: %w", err)
	}

	return pool, nil
}

func (r *Runner) GetEnterprisePoolByID(ctx context.Context, enterpriseID, poolID string) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}
	entity := params.ForgeEntity{
		ID:         enterpriseID,
		EntityType: params.ForgeEntityTypeEnterprise,
	}
	pool, err := r.store.GetEntityPool(ctx, entity, poolID)
	if err != nil {
		return params.Pool{}, fmt.Errorf("error fetching pool: %w", err)
	}
	return pool, nil
}

func (r *Runner) DeleteEnterprisePool(ctx context.Context, enterpriseID, poolID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	entity := params.ForgeEntity{
		ID:         enterpriseID,
		EntityType: params.ForgeEntityTypeEnterprise,
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

func (r *Runner) ListEnterprisePools(ctx context.Context, enterpriseID string) ([]params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return []params.Pool{}, runnerErrors.ErrUnauthorized
	}

	entity := params.ForgeEntity{
		ID:         enterpriseID,
		EntityType: params.ForgeEntityTypeEnterprise,
	}
	pools, err := r.store.ListEntityPools(ctx, entity)
	if err != nil {
		return nil, fmt.Errorf("error fetching pools: %w", err)
	}
	return pools, nil
}

func (r *Runner) UpdateEnterprisePool(ctx context.Context, enterpriseID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}
	entity, err := r.store.GetForgeEntity(ctx, params.ForgeEntityTypeEnterprise, enterpriseID)
	if err != nil {
		return params.Pool{}, fmt.Errorf("failed to get repo: %w", err)
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

func (r *Runner) ListEnterpriseInstances(ctx context.Context, enterpriseID string) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}
	entity := params.ForgeEntity{
		ID:         enterpriseID,
		EntityType: params.ForgeEntityTypeEnterprise,
	}
	instances, err := r.store.ListEntityInstances(ctx, entity)
	if err != nil {
		return []params.Instance{}, fmt.Errorf("error fetching instances: %w", err)
	}
	return instances, nil
}

func (r *Runner) findEnterprisePoolManager(name, endpointName string) (common.PoolManager, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	enterprise, err := r.store.GetEnterprise(r.ctx, name, endpointName)
	if err != nil {
		return nil, fmt.Errorf("error fetching enterprise: %w", err)
	}

	poolManager, err := r.poolManagerCtrl.GetEnterprisePoolManager(enterprise)
	if err != nil {
		return nil, fmt.Errorf("error fetching pool manager for enterprise: %w", err)
	}
	return poolManager, nil
}
