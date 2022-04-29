package runner

import (
	"context"
	"log"
	"runner-manager/auth"
	runnerErrors "runner-manager/errors"
	"runner-manager/params"
	"runner-manager/runner/common"
	"runner-manager/runner/pool"
	"runner-manager/util"
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
			r.store.DeleteRepository(ctx, repo.ID, true)
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

	if err := r.store.DeleteRepository(ctx, repoID, true); err != nil {
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
		repo.Internal = internalCfg
		// stop the pool mgr
		if err := poolMgr.RefreshState(repo); err != nil {
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

	repo, ok := r.repositories[repoID]
	if !ok {
		return params.Pool{}, runnerErrors.ErrNotFound
	}

	if err := param.Validate(); err != nil {
		return params.Pool{}, errors.Wrapf(runnerErrors.ErrBadRequest, "validating params: %s", err)
	}

	if !IsSupportedOSType(param.OSType) {
		return params.Pool{}, runnerErrors.NewBadRequestError("invalid OS type %s", param.OSType)
	}

	if !IsSupportedArch(param.OSArch) {
		return params.Pool{}, runnerErrors.NewBadRequestError("invalid OS architecture %s", param.OSArch)
	}

	_, ok = r.providers[param.ProviderName]
	if !ok {
		return params.Pool{}, runnerErrors.NewBadRequestError("no such provider %s", param.ProviderName)
	}

	ghArch, err := util.ResolveToGithubArch(string(param.OSArch))
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "invalid arch")
	}

	osType, err := util.ResolveToGithubOSType(string(param.OSType))
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "invalid os type")
	}

	extraLabels := []string{
		"self-hosted",
		ghArch,
		osType,
	}

	param.Tags = append(param.Tags, extraLabels...)

	pool, err := r.store.CreateRepositoryPool(ctx, repoID, param)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "creating pool")
	}

	if err := repo.AddPool(ctx, pool); err != nil {
		return params.Pool{}, errors.Wrap(err, "adding pool to manager")
	}

	return pool, nil
}

func (r *Runner) DeleteRepoPool(ctx context.Context) error {
	return nil
}

func (r *Runner) ListRepoPools(ctx context.Context) error {
	return nil
}

func (r *Runner) UpdateRepoPool(ctx context.Context) error {
	return nil
}

func (r *Runner) ListPoolInstances(ctx context.Context) error {
	return nil
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
