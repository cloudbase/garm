package runner

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cloudbase/garm/auth"
	runnerErrors "github.com/cloudbase/garm/errors"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util/appdefaults"

	"github.com/pkg/errors"
)

func (r *Runner) CreateEnterprise(ctx context.Context, param params.CreateEnterpriseParams) (enterprise params.Enterprise, err error) {
	if !auth.IsAdmin(ctx) {
		return enterprise, runnerErrors.ErrUnauthorized
	}

	err = param.Validate()
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "validating params")
	}

	creds, ok := r.credentials[param.CredentialsName]
	if !ok {
		return params.Enterprise{}, runnerErrors.NewBadRequestError("credentials %s not defined", param.CredentialsName)
	}

	_, err = r.store.GetEnterprise(ctx, param.Name)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
		}
	} else {
		return params.Enterprise{}, runnerErrors.NewConflictError("enterprise %s already exists", param.Name)
	}

	enterprise, err = r.store.CreateEnterprise(ctx, param.Name, creds.Name, param.WebhookSecret)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "creating enterprise")
	}

	defer func() {
		if err != nil {
			if deleteErr := r.store.DeleteEnterprise(ctx, enterprise.ID); deleteErr != nil {
				log.Printf("failed to delete enterprise: %s", deleteErr)
			}
		}
	}()

	var poolMgr common.PoolManager
	poolMgr, err = r.poolManagerCtrl.CreateEnterprisePoolManager(r.ctx, enterprise, r.providers, r.store)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "creating enterprise pool manager")
	}
	if err := poolMgr.Start(); err != nil {
		if deleteErr := r.poolManagerCtrl.DeleteEnterprisePoolManager(enterprise); deleteErr != nil {
			log.Printf("failed to cleanup pool manager for enterprise %s", enterprise.ID)
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

	pools, err := r.store.ListEnterprisePools(ctx, enterpriseID)
	if err != nil {
		return errors.Wrap(err, "fetching enterprise pools")
	}

	if len(pools) > 0 {
		poolIds := []string{}
		for _, pool := range pools {
			poolIds = append(poolIds, pool.ID)
		}

		return runnerErrors.NewBadRequestError("enterprise has pools defined (%s)", strings.Join(poolIds, ", "))
	}

	if err := r.poolManagerCtrl.DeleteEnterprisePoolManager(enterprise); err != nil {
		return errors.Wrap(err, "deleting enterprise pool manager")
	}

	if err := r.store.DeleteEnterprise(ctx, enterpriseID); err != nil {
		return errors.Wrapf(err, "removing enterprise %s", enterpriseID)
	}
	return nil
}

func (r *Runner) UpdateEnterprise(ctx context.Context, enterpriseID string, param params.UpdateRepositoryParams) (params.Enterprise, error) {
	if !auth.IsAdmin(ctx) {
		return params.Enterprise{}, runnerErrors.ErrUnauthorized
	}

	r.mux.Lock()
	defer r.mux.Unlock()

	enterprise, err := r.store.GetEnterpriseByID(ctx, enterpriseID)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "fetching enterprise")
	}

	if param.CredentialsName != "" {
		// Check that credentials are set before saving to db
		if _, ok := r.credentials[param.CredentialsName]; !ok {
			return params.Enterprise{}, runnerErrors.NewBadRequestError("invalid credentials (%s) for enterprise %s", param.CredentialsName, enterprise.Name)
		}
	}

	enterprise, err = r.store.UpdateEnterprise(ctx, enterpriseID, param)
	if err != nil {
		return params.Enterprise{}, errors.Wrap(err, "updating enterprise")
	}

	poolMgr, err := r.poolManagerCtrl.GetEnterprisePoolManager(enterprise)
	if err != nil {
		newState := params.UpdatePoolStateParams{
			WebhookSecret: enterprise.WebhookSecret,
		}
		// stop the pool mgr
		if err := poolMgr.RefreshState(newState); err != nil {
			return params.Enterprise{}, errors.Wrap(err, "updating enterprise pool manager")
		}
	} else {
		if _, err := r.poolManagerCtrl.CreateEnterprisePoolManager(r.ctx, enterprise, r.providers, r.store); err != nil {
			return params.Enterprise{}, errors.Wrap(err, "creating enterprise pool manager")
		}
	}

	return enterprise, nil
}

func (r *Runner) CreateEnterprisePool(ctx context.Context, enterpriseID string, param params.CreatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	r.mux.Lock()
	defer r.mux.Unlock()

	enterprise, err := r.store.GetEnterpriseByID(ctx, enterpriseID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching enterprise")
	}

	if _, err := r.poolManagerCtrl.GetEnterprisePoolManager(enterprise); err != nil {
		return params.Pool{}, runnerErrors.ErrNotFound
	}

	createPoolParams, err := r.appendTagsToCreatePoolParams(param)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool params")
	}

	if param.RunnerBootstrapTimeout == 0 {
		param.RunnerBootstrapTimeout = appdefaults.DefaultRunnerBootstrapTimeout
	}

	pool, err := r.store.CreateEnterprisePool(ctx, enterpriseID, createPoolParams)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "creating pool")
	}

	return pool, nil
}

func (r *Runner) GetEnterprisePoolByID(ctx context.Context, enterpriseID, poolID string) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	pool, err := r.store.GetEnterprisePool(ctx, enterpriseID, poolID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool, nil
}

func (r *Runner) DeleteEnterprisePool(ctx context.Context, enterpriseID, poolID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	// TODO: dedup instance count verification
	pool, err := r.store.GetEnterprisePool(ctx, enterpriseID, poolID)
	if err != nil {
		return errors.Wrap(err, "fetching pool")
	}

	instances, err := r.store.ListPoolInstances(ctx, pool.ID)
	if err != nil {
		return errors.Wrap(err, "fetching instances")
	}

	// TODO: implement a count function
	if len(instances) > 0 {
		runnerIDs := []string{}
		for _, run := range instances {
			runnerIDs = append(runnerIDs, run.ID)
		}
		return runnerErrors.NewBadRequestError("pool has runners: %s", strings.Join(runnerIDs, ", "))
	}

	if err := r.store.DeleteEnterprisePool(ctx, enterpriseID, poolID); err != nil {
		return errors.Wrap(err, "deleting pool")
	}
	return nil
}

func (r *Runner) ListEnterprisePools(ctx context.Context, enterpriseID string) ([]params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return []params.Pool{}, runnerErrors.ErrUnauthorized
	}

	pools, err := r.store.ListEnterprisePools(ctx, enterpriseID)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}
	return pools, nil
}

func (r *Runner) UpdateEnterprisePool(ctx context.Context, enterpriseID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	pool, err := r.store.GetEnterprisePool(ctx, enterpriseID, poolID)
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

	newPool, err := r.store.UpdateEnterprisePool(ctx, enterpriseID, poolID, param)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "updating pool")
	}
	return newPool, nil
}

func (r *Runner) ListEnterpriseInstances(ctx context.Context, enterpriseID string) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	instances, err := r.store.ListEnterpriseInstances(ctx, enterpriseID)
	if err != nil {
		return []params.Instance{}, errors.Wrap(err, "fetching instances")
	}
	return instances, nil
}

func (r *Runner) findEnterprisePoolManager(name string) (common.PoolManager, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	enterprise, err := r.store.GetEnterprise(r.ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, "fetching enterprise")
	}

	poolManager, err := r.poolManagerCtrl.GetEnterprisePoolManager(enterprise)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pool manager for enterprise")
	}
	return poolManager, nil
}
