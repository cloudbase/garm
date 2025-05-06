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

func (r *Runner) CreateEnterprise(ctx context.Context, param params.CreateEnterpriseParams) (enterprise params.Enterprise, err error) {
	if !auth.IsAdmin(ctx) {
		return enterprise, runnerErrors.ErrUnauthorized
	}

	err = param.Validate()
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "validating params")
	}

	creds, err := r.store.GetGithubCredentialsByName(ctx, param.CredentialsName, true)
	if err != nil {
		return params.Enterprise{}, runnerErrors.NewBadRequestError("credentials %s not defined", param.CredentialsName)
	}

	_, err = r.store.GetEnterprise(ctx, param.Name, creds.Endpoint.Name)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
		}
	} else {
		return params.Enterprise{}, runnerErrors.NewConflictError("enterprise %s already exists", param.Name)
	}

	enterprise, err = r.store.CreateEnterprise(ctx, param.Name, creds.Name, param.WebhookSecret, param.PoolBalancerType)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "creating enterprise")
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
		return params.Enterprise{}, errors.Wrap(err, "creating enterprise pool manager")
	}
	if err := poolMgr.Start(); err != nil {
		if deleteErr := r.poolManagerCtrl.DeleteEnterprisePoolManager(enterprise); deleteErr != nil {
			slog.With(slog.Any("error", deleteErr)).ErrorContext(
				ctx, "failed to cleanup pool manager for enterprise",
				"enterprise_id", enterprise.ID)
		}
		return params.Enterprise{}, errors.Wrap(err, "starting enterprise pool manager")
	}
	return enterprise, nil
}

func (r *Runner) ListEnterprises(ctx context.Context) ([]params.Enterprise, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	enterprises, err := r.store.ListEnterprises(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "listing enterprises")
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
		return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
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
		return errors.Wrap(err, "fetching enterprise")
	}

	entity, err := enterprise.GetEntity()
	if err != nil {
		return errors.Wrap(err, "getting entity")
	}

	pools, err := r.store.ListEntityPools(ctx, entity)
	if err != nil {
		return errors.Wrap(err, "fetching enterprise pools")
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
		return errors.Wrap(err, "fetching enterprise scale sets")
	}

	if len(scaleSets) > 0 {
		return runnerErrors.NewBadRequestError("enterprise has scale sets defined; delete them first")
	}

	if err := r.poolManagerCtrl.DeleteEnterprisePoolManager(enterprise); err != nil {
		return errors.Wrap(err, "deleting enterprise pool manager")
	}

	if err := r.store.DeleteEnterprise(ctx, enterpriseID); err != nil {
		return errors.Wrapf(err, "removing enterprise %s", enterpriseID)
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
		return params.Enterprise{}, errors.Wrap(err, "updating enterprise")
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

	entity := params.GithubEntity{
		ID:         enterpriseID,
		EntityType: params.GithubEntityTypeEnterprise,
	}

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
	entity := params.GithubEntity{
		ID:         enterpriseID,
		EntityType: params.GithubEntityTypeEnterprise,
	}
	pool, err := r.store.GetEntityPool(ctx, entity, poolID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool, nil
}

func (r *Runner) DeleteEnterprisePool(ctx context.Context, enterpriseID, poolID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	entity := params.GithubEntity{
		ID:         enterpriseID,
		EntityType: params.GithubEntityTypeEnterprise,
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

func (r *Runner) ListEnterprisePools(ctx context.Context, enterpriseID string) ([]params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return []params.Pool{}, runnerErrors.ErrUnauthorized
	}

	entity := params.GithubEntity{
		ID:         enterpriseID,
		EntityType: params.GithubEntityTypeEnterprise,
	}
	pools, err := r.store.ListEntityPools(ctx, entity)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}
	return pools, nil
}

func (r *Runner) UpdateEnterprisePool(ctx context.Context, enterpriseID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	entity := params.GithubEntity{
		ID:         enterpriseID,
		EntityType: params.GithubEntityTypeEnterprise,
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

func (r *Runner) ListEnterpriseInstances(ctx context.Context, enterpriseID string) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}
	entity := params.GithubEntity{
		ID:         enterpriseID,
		EntityType: params.GithubEntityTypeEnterprise,
	}
	instances, err := r.store.ListEntityInstances(ctx, entity)
	if err != nil {
		return []params.Instance{}, errors.Wrap(err, "fetching instances")
	}
	return instances, nil
}

func (r *Runner) findEnterprisePoolManager(name, endpointName string) (common.PoolManager, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	enterprise, err := r.store.GetEnterprise(r.ctx, name, endpointName)
	if err != nil {
		return nil, errors.Wrap(err, "fetching enterprise")
	}

	poolManager, err := r.poolManagerCtrl.GetEnterprisePoolManager(enterprise)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pool manager for enterprise")
	}
	return poolManager, nil
}
