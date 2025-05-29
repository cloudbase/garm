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

func (r *Runner) CreateOrganization(ctx context.Context, param params.CreateOrgParams) (org params.Organization, err error) {
	if !auth.IsAdmin(ctx) {
		return org, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.Organization{}, errors.Wrap(err, "validating params")
	}

	var creds params.ForgeCredentials
	switch param.ForgeType {
	case params.GithubEndpointType:
		slog.DebugContext(ctx, "getting github credentials")
		creds, err = r.store.GetGithubCredentialsByName(ctx, param.CredentialsName, true)
	case params.GiteaEndpointType:
		slog.DebugContext(ctx, "getting gitea credentials")
		creds, err = r.store.GetGiteaCredentialsByName(ctx, param.CredentialsName, true)
	default:
		creds, err = r.ResolveForgeCredentialByName(ctx, param.CredentialsName)
	}

	if err != nil {
		return params.Organization{}, runnerErrors.NewBadRequestError("credentials %s not defined", param.CredentialsName)
	}

	_, err = r.store.GetOrganization(ctx, param.Name, creds.Endpoint.Name)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return params.Organization{}, errors.Wrap(err, "fetching org")
		}
	} else {
		return params.Organization{}, runnerErrors.NewConflictError("organization %s already exists", param.Name)
	}

	org, err = r.store.CreateOrganization(ctx, param.Name, creds, param.WebhookSecret, param.PoolBalancerType)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "creating organization")
	}

	defer func() {
		if err != nil {
			if deleteErr := r.store.DeleteOrganization(ctx, org.ID); deleteErr != nil {
				slog.With(slog.Any("error", deleteErr)).ErrorContext(
					ctx, "failed to delete org",
					"org_id", org.ID)
			}
		}
	}()

	// Use the admin context in the pool manager. Any access control is already done above when
	// updating the store.
	poolMgr, err := r.poolManagerCtrl.CreateOrgPoolManager(r.ctx, org, r.providers, r.store)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "creating org pool manager")
	}
	if err := poolMgr.Start(); err != nil {
		if deleteErr := r.poolManagerCtrl.DeleteOrgPoolManager(org); deleteErr != nil {
			slog.With(slog.Any("error", deleteErr)).ErrorContext(
				ctx, "failed to cleanup pool manager for org",
				"org_id", org.ID)
		}
		return params.Organization{}, errors.Wrap(err, "starting org pool manager")
	}
	return org, nil
}

func (r *Runner) ListOrganizations(ctx context.Context) ([]params.Organization, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	orgs, err := r.store.ListOrganizations(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "listing organizations")
	}

	var allOrgs []params.Organization

	for _, org := range orgs {
		poolMgr, err := r.poolManagerCtrl.GetOrgPoolManager(org)
		if err != nil {
			org.PoolManagerStatus.IsRunning = false
			org.PoolManagerStatus.FailureReason = fmt.Sprintf("failed to get pool manager: %q", err)
		} else {
			org.PoolManagerStatus = poolMgr.Status()
		}

		allOrgs = append(allOrgs, org)
	}

	return allOrgs, nil
}

func (r *Runner) GetOrganizationByID(ctx context.Context, orgID string) (params.Organization, error) {
	if !auth.IsAdmin(ctx) {
		return params.Organization{}, runnerErrors.ErrUnauthorized
	}

	org, err := r.store.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "fetching organization")
	}

	poolMgr, err := r.poolManagerCtrl.GetOrgPoolManager(org)
	if err != nil {
		org.PoolManagerStatus.IsRunning = false
		org.PoolManagerStatus.FailureReason = fmt.Sprintf("failed to get pool manager: %q", err)
	}
	org.PoolManagerStatus = poolMgr.Status()
	return org, nil
}

func (r *Runner) DeleteOrganization(ctx context.Context, orgID string, keepWebhook bool) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	org, err := r.store.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return errors.Wrap(err, "fetching org")
	}

	entity, err := org.GetEntity()
	if err != nil {
		return errors.Wrap(err, "getting entity")
	}

	pools, err := r.store.ListEntityPools(ctx, entity)
	if err != nil {
		return errors.Wrap(err, "fetching org pools")
	}

	if len(pools) > 0 {
		poolIDs := []string{}
		for _, pool := range pools {
			poolIDs = append(poolIDs, pool.ID)
		}

		return runnerErrors.NewBadRequestError("org has pools defined (%s)", strings.Join(poolIDs, ", "))
	}

	scaleSets, err := r.store.ListEntityScaleSets(ctx, entity)
	if err != nil {
		return errors.Wrap(err, "fetching organization scale sets")
	}

	if len(scaleSets) > 0 {
		return runnerErrors.NewBadRequestError("organization has scale sets defined; delete them first")
	}

	if !keepWebhook && r.config.Default.EnableWebhookManagement {
		poolMgr, err := r.poolManagerCtrl.GetOrgPoolManager(org)
		if err != nil {
			return errors.Wrap(err, "fetching pool manager")
		}

		if err := poolMgr.UninstallWebhook(ctx); err != nil {
			// nolint:golangci-lint,godox
			// TODO(gabriel-samfira): Should we error out here?
			slog.With(slog.Any("error", err)).ErrorContext(
				ctx, "failed to uninstall webhook",
				"org_id", org.ID)
		}
	}

	if err := r.poolManagerCtrl.DeleteOrgPoolManager(org); err != nil {
		return errors.Wrap(err, "deleting org pool manager")
	}

	if err := r.store.DeleteOrganization(ctx, orgID); err != nil {
		return errors.Wrapf(err, "removing organization %s", orgID)
	}
	return nil
}

func (r *Runner) UpdateOrganization(ctx context.Context, orgID string, param params.UpdateEntityParams) (params.Organization, error) {
	if !auth.IsAdmin(ctx) {
		return params.Organization{}, runnerErrors.ErrUnauthorized
	}

	r.mux.Lock()
	defer r.mux.Unlock()

	switch param.PoolBalancerType {
	case params.PoolBalancerTypeRoundRobin, params.PoolBalancerTypePack, params.PoolBalancerTypeNone:
	default:
		return params.Organization{}, runnerErrors.NewBadRequestError("invalid pool balancer type: %s", param.PoolBalancerType)
	}

	org, err := r.store.UpdateOrganization(ctx, orgID, param)
	if err != nil {
		return params.Organization{}, errors.Wrap(err, "updating org")
	}

	poolMgr, err := r.poolManagerCtrl.GetOrgPoolManager(org)
	if err != nil {
		return params.Organization{}, fmt.Errorf("failed to get org pool manager: %w", err)
	}

	org.PoolManagerStatus = poolMgr.Status()
	return org, nil
}

func (r *Runner) CreateOrgPool(ctx context.Context, orgID string, param params.CreatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	createPoolParams, err := r.appendTagsToCreatePoolParams(param)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool params")
	}

	if param.RunnerBootstrapTimeout == 0 {
		param.RunnerBootstrapTimeout = appdefaults.DefaultRunnerBootstrapTimeout
	}

	entity := params.ForgeEntity{
		ID:         orgID,
		EntityType: params.ForgeEntityTypeOrganization,
	}

	pool, err := r.store.CreateEntityPool(ctx, entity, createPoolParams)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "creating pool")
	}

	return pool, nil
}

func (r *Runner) GetOrgPoolByID(ctx context.Context, orgID, poolID string) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	entity := params.ForgeEntity{
		ID:         orgID,
		EntityType: params.ForgeEntityTypeOrganization,
	}

	pool, err := r.store.GetEntityPool(ctx, entity, poolID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}

	return pool, nil
}

func (r *Runner) DeleteOrgPool(ctx context.Context, orgID, poolID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	entity := params.ForgeEntity{
		ID:         orgID,
		EntityType: params.ForgeEntityTypeOrganization,
	}

	pool, err := r.store.GetEntityPool(ctx, entity, poolID)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return errors.Wrap(err, "fetching pool")
		}
		return nil
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

func (r *Runner) ListOrgPools(ctx context.Context, orgID string) ([]params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return []params.Pool{}, runnerErrors.ErrUnauthorized
	}
	entity := params.ForgeEntity{
		ID:         orgID,
		EntityType: params.ForgeEntityTypeOrganization,
	}
	pools, err := r.store.ListEntityPools(ctx, entity)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}
	return pools, nil
}

func (r *Runner) UpdateOrgPool(ctx context.Context, orgID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	entity := params.ForgeEntity{
		ID:         orgID,
		EntityType: params.ForgeEntityTypeOrganization,
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

func (r *Runner) ListOrgInstances(ctx context.Context, orgID string) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	entity := params.ForgeEntity{
		ID:         orgID,
		EntityType: params.ForgeEntityTypeOrganization,
	}

	instances, err := r.store.ListEntityInstances(ctx, entity)
	if err != nil {
		return []params.Instance{}, errors.Wrap(err, "fetching instances")
	}
	return instances, nil
}

func (r *Runner) findOrgPoolManager(name, endpointName string) (common.PoolManager, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	org, err := r.store.GetOrganization(r.ctx, name, endpointName)
	if err != nil {
		return nil, errors.Wrap(err, "fetching org")
	}

	poolManager, err := r.poolManagerCtrl.GetOrgPoolManager(org)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pool manager for org")
	}
	return poolManager, nil
}

func (r *Runner) InstallOrgWebhook(ctx context.Context, orgID string, param params.InstallWebhookParams) (params.HookInfo, error) {
	if !auth.IsAdmin(ctx) {
		return params.HookInfo{}, runnerErrors.ErrUnauthorized
	}

	org, err := r.store.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "fetching org")
	}

	poolMgr, err := r.poolManagerCtrl.GetOrgPoolManager(org)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "fetching pool manager for org")
	}

	info, err := poolMgr.InstallWebhook(ctx, param)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "installing webhook")
	}
	return info, nil
}

func (r *Runner) UninstallOrgWebhook(ctx context.Context, orgID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	org, err := r.store.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return errors.Wrap(err, "fetching org")
	}

	poolMgr, err := r.poolManagerCtrl.GetOrgPoolManager(org)
	if err != nil {
		return errors.Wrap(err, "fetching pool manager for org")
	}

	if err := poolMgr.UninstallWebhook(ctx); err != nil {
		return errors.Wrap(err, "uninstalling webhook")
	}
	return nil
}

func (r *Runner) GetOrgWebhookInfo(ctx context.Context, orgID string) (params.HookInfo, error) {
	if !auth.IsAdmin(ctx) {
		return params.HookInfo{}, runnerErrors.ErrUnauthorized
	}

	org, err := r.store.GetOrganizationByID(ctx, orgID)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "fetching org")
	}

	poolMgr, err := r.poolManagerCtrl.GetOrgPoolManager(org)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "fetching pool manager for org")
	}

	info, err := poolMgr.GetWebhookInfo(ctx)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "fetching webhook info")
	}
	return info, nil
}
