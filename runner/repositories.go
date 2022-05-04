package runner

import (
	"context"
	"garm/auth"
	runnerErrors "garm/errors"
	"garm/params"
	"garm/runner/common"
	"garm/runner/pool"
	"log"
	"strings"

	"github.com/pkg/errors"
)

func (r *Runner) CreateRepository(ctx context.Context, param params.CreateRepoParams) (repo params.Repository, err error) {
	if !auth.IsAdmin(ctx) {
		return repo, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.Repository{}, errors.Wrap(err, "validating params")
	}

	creds, ok := r.credentials[param.CredentialsName]
	if !ok {
		return params.Repository{}, runnerErrors.NewBadRequestError("credentials %s not defined", param.CredentialsName)
	}

	_, err = r.store.GetRepository(ctx, param.Owner, param.Name)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return params.Repository{}, errors.Wrap(err, "fetching repo")
		}
	} else {
		return params.Repository{}, runnerErrors.NewConflictError("repository %s/%s already exists", param.Owner, param.Name)
	}

	repo, err = r.store.CreateRepository(ctx, param.Owner, param.Name, creds.Name, param.WebhookSecret)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "creating repository")
	}

	defer func() {
		if err != nil {
			r.store.DeleteRepository(ctx, repo.ID)
		}
	}()

	poolMgr, err := r.loadRepoPoolManager(repo)
	if err := poolMgr.Start(); err != nil {
		return params.Repository{}, errors.Wrap(err, "starting pool manager")
	}
	r.repositories[repo.ID] = poolMgr
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

	return repos, nil
}

func (r *Runner) GetRepositoryByID(ctx context.Context, repoID string) (params.Repository, error) {
	if !auth.IsAdmin(ctx) {
		return params.Repository{}, runnerErrors.ErrUnauthorized
	}

	repo, err := r.store.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "fetching repository")
	}
	return repo, nil
}

func (r *Runner) DeleteRepository(ctx context.Context, repoID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	repo, err := r.store.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return errors.Wrap(err, "fetching repo")
	}

	poolMgr, ok := r.repositories[repo.ID]
	if ok {
		if err := poolMgr.Stop(); err != nil {
			log.Printf("failed to stop pool for repo %s", repo.ID)
		}
		delete(r.repositories, repoID)
	}

	pools, err := r.store.ListRepoPools(ctx, repoID)
	if err != nil {
		return errors.Wrap(err, "fetching repo pools")
	}

	if len(pools) > 0 {
		poolIds := []string{}
		for _, pool := range pools {
			poolIds = append(poolIds, pool.ID)
		}

		return runnerErrors.NewBadRequestError("repo has pools defined (%s)", strings.Join(poolIds, ", "))
	}

	if err := r.store.DeleteRepository(ctx, repoID); err != nil {
		return errors.Wrap(err, "removing repository")
	}
	return nil
}

func (r *Runner) UpdateRepository(ctx context.Context, repoID string, param params.UpdateRepositoryParams) (params.Repository, error) {
	if !auth.IsAdmin(ctx) {
		return params.Repository{}, runnerErrors.ErrUnauthorized
	}

	r.mux.Lock()
	defer r.mux.Unlock()

	repo, err := r.store.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "fetching repo")
	}

	if param.CredentialsName != "" {
		// Check that credentials are set before saving to db
		if _, ok := r.credentials[param.CredentialsName]; !ok {
			return params.Repository{}, runnerErrors.NewBadRequestError("invalid credentials (%s) for repo %s/%s", param.CredentialsName, repo.Owner, repo.Name)
		}
	}

	repo, err = r.store.UpdateRepository(ctx, repoID, param)
	if err != nil {
		return params.Repository{}, errors.Wrap(err, "updating repo")
	}

	poolMgr, ok := r.repositories[repo.ID]
	if ok {
		internalCfg, err := r.getInternalConfig(repo.CredentialsName)
		if err != nil {
			return params.Repository{}, errors.Wrap(err, "fetching internal config")
		}
		newState := params.UpdatePoolStateParams{
			WebhookSecret: repo.WebhookSecret,
			Internal:      internalCfg,
		}
		repo.Internal = internalCfg
		// stop the pool mgr
		if err := poolMgr.RefreshState(newState); err != nil {
			return params.Repository{}, errors.Wrap(err, "updating pool manager")
		}
	} else {
		poolMgr, err := r.loadRepoPoolManager(repo)
		if err != nil {
			return params.Repository{}, errors.Wrap(err, "loading pool manager")
		}
		r.repositories[repo.ID] = poolMgr
	}

	return repo, nil
}

func (r *Runner) CreateRepoPool(ctx context.Context, repoID string, param params.CreatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	r.mux.Lock()
	defer r.mux.Unlock()

	_, ok := r.repositories[repoID]
	if !ok {
		return params.Pool{}, runnerErrors.ErrNotFound
	}

	createPoolParams, err := r.appendTagsToCreatePoolParams(param)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool params")
	}

	pool, err := r.store.CreateRepositoryPool(ctx, repoID, createPoolParams)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "creating pool")
	}

	return pool, nil
}

func (r *Runner) GetRepoPoolByID(ctx context.Context, repoID, poolID string) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	pool, err := r.store.GetRepositoryPool(ctx, repoID, poolID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool, nil
}

func (r *Runner) DeleteRepoPool(ctx context.Context, repoID, poolID string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	pool, err := r.store.GetRepositoryPool(ctx, repoID, poolID)
	if err != nil {
		return errors.Wrap(err, "fetching pool")
	}

	instances, err := r.store.ListInstances(ctx, pool.ID)
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

	if err := r.store.DeleteRepositoryPool(ctx, repoID, poolID); err != nil {
		return errors.Wrap(err, "deleting pool")
	}
	return nil
}

func (r *Runner) ListRepoPools(ctx context.Context, repoID string) ([]params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return []params.Pool{}, runnerErrors.ErrUnauthorized
	}

	pools, err := r.store.ListRepoPools(ctx, repoID)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}
	return pools, nil
}

func (r *Runner) ListPoolInstances(ctx context.Context, poolID string) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	instances, err := r.store.ListInstances(ctx, poolID)
	if err != nil {
		return []params.Instance{}, errors.Wrap(err, "fetching instances")
	}
	return instances, nil
}

func (r *Runner) UpdateRepoPool(ctx context.Context, repoID, poolID string, param params.UpdatePoolParams) (params.Pool, error) {
	if !auth.IsAdmin(ctx) {
		return params.Pool{}, runnerErrors.ErrUnauthorized
	}

	pool, err := r.store.GetRepositoryPool(ctx, repoID, poolID)
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

	newPool, err := r.store.UpdateRepositoryPool(ctx, repoID, poolID, param)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "updating pool")
	}
	return newPool, nil
}

func (r *Runner) ListRepoInstances(ctx context.Context, repoID string) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	instances, err := r.store.ListRepoInstances(ctx, repoID)
	if err != nil {
		return []params.Instance{}, errors.Wrap(err, "fetching instances")
	}
	return instances, nil
}

func (r *Runner) loadRepoPoolManager(repo params.Repository) (common.PoolManager, error) {
	cfg, err := r.getInternalConfig(repo.CredentialsName)
	if err != nil {
		return nil, errors.Wrap(err, "fetching internal config")
	}
	repo.Internal = cfg
	poolManager, err := pool.NewRepositoryPoolManager(r.ctx, repo, r.providers, r.store)
	if err != nil {
		return nil, errors.Wrap(err, "creating pool manager")
	}
	return poolManager, nil
}

func (r *Runner) findRepoPoolManager(owner, name string) (common.PoolManager, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	repo, err := r.store.GetRepository(r.ctx, owner, name)
	if err != nil {
		return nil, errors.Wrap(err, "fetching repo")
	}

	if repo, ok := r.repositories[repo.ID]; ok {
		return repo, nil
	}
	return nil, errors.Wrapf(runnerErrors.ErrNotFound, "repository %s/%s not configured", owner, name)
}
