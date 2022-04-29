package runner

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hash"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"runner-manager/auth"
	"runner-manager/config"
	"runner-manager/database"
	dbCommon "runner-manager/database/common"
	runnerErrors "runner-manager/errors"
	"runner-manager/params"
	"runner-manager/runner/common"
	"runner-manager/runner/pool"
	"runner-manager/runner/providers"
	"runner-manager/util"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func NewRunner(ctx context.Context, cfg config.Config) (*Runner, error) {
	// ghc, err := util.GithubClient(ctx, cfg.Github.OAuth2Token)
	// if err != nil {
	// 	return nil, errors.Wrap(err, "getting github client")
	// }

	providers, err := providers.LoadProvidersFromConfig(ctx, cfg, "")
	if err != nil {
		return nil, errors.Wrap(err, "loading providers")
	}
	db, err := database.NewDatabase(ctx, cfg.Database)
	if err != nil {
		log.Fatal(err)
	}

	creds := map[string]config.Github{}

	for _, ghcreds := range cfg.Github {
		creds[ghcreds.Name] = ghcreds
	}
	runner := &Runner{
		ctx:    ctx,
		config: cfg,
		store:  db,
		// ghc:       ghc,
		repositories:  map[string]common.PoolManager{},
		organizations: map[string]common.PoolManager{},
		providers:     providers,
		credentials:   creds,
	}

	if err := runner.ensureSSHKeys(); err != nil {
		return nil, errors.Wrap(err, "ensuring SSH keys")
	}

	if err := runner.loadReposAndOrgs(); err != nil {
		return nil, errors.Wrap(err, "loading pool managers")
	}

	return runner, nil
}

type Runner struct {
	mux sync.Mutex

	ctx context.Context
	// ghc *github.Client
	store dbCommon.Store

	controllerID string

	config        config.Config
	repositories  map[string]common.PoolManager
	organizations map[string]common.PoolManager
	providers     map[string]common.Provider
	credentials   map[string]config.Github
}

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

	log.Printf("post-update webhook secret: %s", repo.WebhookSecret)
	poolMgr, ok := r.repositories[repo.ID]
	if ok {
		internalCfg, err := r.getInternalConfig(repo)
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

func (r *Runner) CreateRepoPool(ctx context.Context) error {
	return nil
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

func (r *Runner) ListCredentials(ctx context.Context) ([]params.GithubCredentials, error) {
	ret := []params.GithubCredentials{}

	for _, val := range r.config.Github {
		ret = append(ret, params.GithubCredentials{
			Name:        val.Name,
			Description: val.Description,
		})
	}
	return ret, nil
}

func (r *Runner) ListProviders(ctx context.Context) ([]params.Provider, error) {
	ret := []params.Provider{}

	for _, val := range r.providers {
		ret = append(ret, val.AsParams())
	}
	return ret, nil
}

func (r *Runner) getInternalConfig(repo params.Repository) (params.Internal, error) {
	creds, ok := r.credentials[repo.CredentialsName]
	if !ok {
		return params.Internal{}, runnerErrors.NewBadRequestError("invalid credentials (%s) for repo %s/%s", repo.CredentialsName, repo.Owner, repo.Name)
	}

	return params.Internal{
		OAuth2Token:         creds.OAuth2Token,
		ControllerID:        r.controllerID,
		InstanceCallbackURL: r.config.Default.CallbackURL,
		JWTSecret:           r.config.JWTAuth.Secret,
	}, nil
}

func (r *Runner) loadRepoPoolManager(repo params.Repository) (common.PoolManager, error) {
	cfg, err := r.getInternalConfig(repo)
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

func (r *Runner) loadReposAndOrgs() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	repos, err := r.store.ListRepositories(r.ctx)
	if err != nil {
		return errors.Wrap(err, "fetching repositories")
	}

	for _, repo := range repos {
		log.Printf("creating pool manager for %s/%s", repo.Owner, repo.Name)
		poolManager, err := r.loadRepoPoolManager(repo)
		if err != nil {
			return errors.Wrap(err, "loading repo pool manager")
		}
		r.repositories[repo.ID] = poolManager
	}

	return nil
}

func (r *Runner) Start() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	for _, repo := range r.repositories {
		if err := repo.Start(); err != nil {
			return errors.Wrap(err, "starting repo pool manager")
		}
	}

	for _, org := range r.organizations {
		if err := org.Start(); err != nil {
			return errors.Wrap(err, "starting org pool manager")
		}
	}
	return nil
}

func (r *Runner) Stop() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	for _, repo := range r.repositories {
		if err := repo.Stop(); err != nil {
			return errors.Wrap(err, "starting repo pool manager")
		}
	}

	for _, org := range r.organizations {
		if err := org.Stop(); err != nil {
			return errors.Wrap(err, "starting org pool manager")
		}
	}
	return nil
}

func (r *Runner) Wait() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	var wg sync.WaitGroup

	for poolId, repo := range r.repositories {
		wg.Add(1)
		go func(id string, poolMgr common.PoolManager) {
			defer wg.Done()
			if err := poolMgr.Wait(); err != nil {
				log.Printf("timed out waiting for pool manager %s to exit", id)
			}
		}(poolId, repo)
	}

	for poolId, org := range r.organizations {
		wg.Add(1)
		go func(id string, poolMgr common.PoolManager) {
			defer wg.Done()
			if err := poolMgr.Wait(); err != nil {
				log.Printf("timed out waiting for pool manager %s to exit", id)
			}
		}(poolId, org)
	}
	wg.Wait()
	return nil
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

func (r *Runner) findOrgPoolManager(name string) (common.PoolManager, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	org, err := r.store.GetOrganization(r.ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, "fetching repo")
	}

	if orgPoolMgr, ok := r.organizations[org.ID]; ok {
		return orgPoolMgr, nil
	}
	return nil, errors.Wrapf(runnerErrors.ErrNotFound, "organization %s not configured", name)
}

func (r *Runner) validateHookBody(signature, secret string, body []byte) error {
	if secret == "" {
		// A secret was not set. Skip validation of body.
		return nil
	}

	if signature == "" {
		// A secret was set in our config, but a signature was not received
		// from Github. Authentication of the body cannot be done.
		return runnerErrors.NewUnauthorizedError("missing github signature")
	}

	sigParts := strings.SplitN(signature, "=", 2)
	if len(sigParts) != 2 {
		// We expect the signature from github to be of the format:
		// hashType=hashValue
		// ie: sha256=1fc917c7ad66487470e466c0ad40ddd45b9f7730a4b43e1b2542627f0596bbdc
		return runnerErrors.NewBadRequestError("invalid signature format")
	}

	var hashFunc func() hash.Hash
	switch sigParts[0] {
	case "sha256":
		hashFunc = sha256.New
	case "sha1":
		hashFunc = sha1.New
	default:
		return runnerErrors.NewBadRequestError("unknown signature type")
	}

	mac := hmac.New(hashFunc, []byte(secret))
	_, err := mac.Write(body)
	if err != nil {
		return errors.Wrap(err, "failed to compute sha256")
	}
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sigParts[1]), []byte(expectedMAC)) {
		return runnerErrors.NewUnauthorizedError("signature missmatch")
	}

	return nil
}

func (r *Runner) DispatchWorkflowJob(hookTargetType, signature string, jobData []byte) error {
	if jobData == nil || len(jobData) == 0 {
		return runnerErrors.NewBadRequestError("missing job data")
	}

	var job params.WorkflowJob
	if err := json.Unmarshal(jobData, &job); err != nil {
		return errors.Wrapf(runnerErrors.ErrBadRequest, "invalid job data: %s", err)
	}

	var poolManager common.PoolManager
	var err error

	switch HookTargetType(hookTargetType) {
	case RepoHook:
		poolManager, err = r.findRepoPoolManager(job.Repository.Owner.Login, job.Repository.Name)
	case OrganizationHook:
		poolManager, err = r.findOrgPoolManager(job.Organization.Login)
	default:
		return runnerErrors.NewBadRequestError("cannot handle hook target type %s", hookTargetType)
	}

	if err != nil {
		// We don't have a repository or organization configured that
		// can handle this workflow job.
		return errors.Wrap(err, "fetching poolManager")
	}

	// We found a pool. Validate the webhook job. If a secret is configured,
	// we make sure that the source of this workflow job is valid.
	secret := poolManager.WebhookSecret()
	if err := r.validateHookBody(signature, secret, jobData); err != nil {
		return errors.Wrap(err, "validating webhook data")
	}

	if err := poolManager.HandleWorkflowJob(job); err != nil {
		return errors.Wrap(err, "handling workflow job")
	}

	return nil
}

func (r *Runner) sshDir() string {
	return filepath.Join(r.config.Default.ConfigDir, "ssh")
}

func (r *Runner) sshKeyPath() string {
	keyPath := filepath.Join(r.sshDir(), "runner_rsa_key")
	return keyPath
}

func (r *Runner) sshPubKeyPath() string {
	keyPath := filepath.Join(r.sshDir(), "runner_rsa_key.pub")
	return keyPath
}

func (r *Runner) parseSSHKey() (ssh.Signer, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	key, err := ioutil.ReadFile(r.sshKeyPath())
	if err != nil {
		return nil, errors.Wrapf(err, "reading private key %s", r.sshKeyPath())
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing private key %s", r.sshKeyPath())
	}

	return signer, nil
}

func (r *Runner) sshPubKey() ([]byte, error) {
	key, err := ioutil.ReadFile(r.sshPubKeyPath())
	if err != nil {
		return nil, errors.Wrapf(err, "reading public key %s", r.sshPubKeyPath())
	}
	return key, nil
}

func (r *Runner) ensureSSHKeys() error {
	sshDir := r.sshDir()

	if _, err := os.Stat(sshDir); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return errors.Wrapf(err, "checking SSH dir %s", sshDir)
		}
		if err := os.MkdirAll(sshDir, 0o700); err != nil {
			return errors.Wrapf(err, "creating ssh dir %s", sshDir)
		}
	}

	privKeyFile := r.sshKeyPath()
	pubKeyFile := r.sshPubKeyPath()

	if _, err := os.Stat(privKeyFile); err == nil {
		return nil
	}

	pubKey, privKey, err := util.GenerateSSHKeyPair()
	if err != nil {
		errors.Wrap(err, "generating keypair")
	}

	if err := ioutil.WriteFile(privKeyFile, privKey, 0o600); err != nil {
		return errors.Wrap(err, "writing private key")
	}

	if err := ioutil.WriteFile(pubKeyFile, pubKey, 0o600); err != nil {
		return errors.Wrap(err, "writing public key")
	}

	return nil
}
