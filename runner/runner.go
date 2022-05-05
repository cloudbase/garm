package runner

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"garm/auth"
	"garm/config"
	"garm/database"
	dbCommon "garm/database/common"
	runnerErrors "garm/errors"
	"garm/params"
	"garm/runner/common"
	"garm/runner/providers"
	"garm/util"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func NewRunner(ctx context.Context, cfg config.Config) (*Runner, error) {
	db, err := database.NewDatabase(ctx, cfg.Database)
	if err != nil {
		return nil, errors.Wrap(err, "creating db connection")
	}

	ctrlId, err := db.ControllerInfo()
	if err != nil {
		return nil, errors.Wrap(err, "fetching controller info")
	}

	providers, err := providers.LoadProvidersFromConfig(ctx, cfg, ctrlId.ControllerID.String())
	if err != nil {
		return nil, errors.Wrap(err, "loading providers")
	}

	creds := map[string]config.Github{}

	for _, ghcreds := range cfg.Github {
		creds[ghcreds.Name] = ghcreds
	}
	runner := &Runner{
		ctx:           ctx,
		config:        cfg,
		store:         db,
		repositories:  map[string]common.PoolManager{},
		organizations: map[string]common.PoolManager{},
		providers:     providers,
		controllerID:  ctrlId.ControllerID.String(),
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

	config       config.Config
	controllerID string
	ctx          context.Context
	store        dbCommon.Store

	repositories  map[string]common.PoolManager
	organizations map[string]common.PoolManager
	providers     map[string]common.Provider
	credentials   map[string]config.Github
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

func (r *Runner) getInternalConfig(credsName string) (params.Internal, error) {
	creds, ok := r.credentials[credsName]
	if !ok {
		return params.Internal{}, runnerErrors.NewBadRequestError("invalid credential name (%s)", credsName)
	}

	return params.Internal{
		OAuth2Token:         creds.OAuth2Token,
		ControllerID:        r.controllerID,
		InstanceCallbackURL: r.config.Default.CallbackURL,
		JWTSecret:           r.config.JWTAuth.Secret,
	}, nil
}

func (r *Runner) loadReposAndOrgs() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	repos, err := r.store.ListRepositories(r.ctx)
	if err != nil {
		return errors.Wrap(err, "fetching repositories")
	}

	orgs, err := r.store.ListOrganizations(r.ctx)
	if err != nil {
		return errors.Wrap(err, "fetching repositories")
	}

	expectedReplies := len(repos) + len(orgs)
	repoPoolMgrChan := make(chan common.PoolManager, len(repos))
	orgPoolMgrChan := make(chan common.PoolManager, len(orgs))
	errChan := make(chan error, expectedReplies)

	for _, repo := range repos {
		go func(repo params.Repository) {
			log.Printf("creating pool manager for %s/%s", repo.Owner, repo.Name)
			poolManager, err := r.loadRepoPoolManager(repo)
			if err != nil {
				errChan <- err
				return
			}
			repoPoolMgrChan <- poolManager
		}(repo)
	}

	for _, org := range orgs {
		go func(org params.Organization) {
			log.Printf("creating pool manager for organization %s", org.Name)
			poolManager, err := r.loadOrgPoolManager(org)
			if err != nil {
				errChan <- err
				return
			}
			orgPoolMgrChan <- poolManager
		}(org)
	}

	for i := 0; i < expectedReplies; i++ {
		select {
		case repoPool := <-repoPoolMgrChan:
			r.repositories[repoPool.ID()] = repoPool
		case orgPool := <-orgPoolMgrChan:
			r.organizations[orgPool.ID()] = orgPool
		case err := <-errChan:
			return errors.Wrap(err, "failed to load repos and pools")
		case <-time.After(60 * time.Second):
			return fmt.Errorf("timed out waiting for pool mamager load")
		}
	}

	return nil
}

func (r *Runner) Start() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	expectedReplies := len(r.repositories) + len(r.organizations)
	errChan := make(chan error, expectedReplies)

	for _, repo := range r.repositories {
		go func(repo common.PoolManager) {
			err := repo.Start()
			errChan <- err

		}(repo)
	}

	for _, org := range r.organizations {
		go func(org common.PoolManager) {
			err := org.Start()
			errChan <- err
		}(org)

	}

	for i := 0; i < expectedReplies; i++ {
		select {
		case err := <-errChan:
			if err != nil {
				return errors.Wrap(err, "starting pool manager")
			}
		case <-time.After(60 * time.Second):
			return fmt.Errorf("timed out waiting for pool mamager start")
		}
	}
	return nil
}

func (r *Runner) Stop() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	for _, repo := range r.repositories {
		if err := repo.Stop(); err != nil {
			return errors.Wrap(err, "stopping repo pool manager")
		}
	}

	for _, org := range r.organizations {
		if err := org.Stop(); err != nil {
			return errors.Wrap(err, "stopping org pool manager")
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

func (r *Runner) appendTagsToCreatePoolParams(param params.CreatePoolParams) (params.CreatePoolParams, error) {
	if err := param.Validate(); err != nil {
		return params.CreatePoolParams{}, errors.Wrapf(runnerErrors.ErrBadRequest, "validating params: %s", err)
	}

	if !IsSupportedOSType(param.OSType) {
		return params.CreatePoolParams{}, runnerErrors.NewBadRequestError("invalid OS type %s", param.OSType)
	}

	if !IsSupportedArch(param.OSArch) {
		return params.CreatePoolParams{}, runnerErrors.NewBadRequestError("invalid OS architecture %s", param.OSArch)
	}

	_, ok := r.providers[param.ProviderName]
	if !ok {
		return params.CreatePoolParams{}, runnerErrors.NewBadRequestError("no such provider %s", param.ProviderName)
	}

	// github automatically adds the "self-hosted" tag as well as the OS type (linux, windows, etc)
	// and architecture (arm, x64, etc) to all self hosted runners. When a workflow job comes in, we try
	// to find a pool based on the labels that are set in the workflow. If we don't explicitly define these
	// default tags for each pool, and the user targets these labels, we won't be able to match any pools.
	// The downside is that all pools with the same OS and arch will have these default labels. Users should
	// set distinct and unique labels on each pool, and explicitly target those labels, or risk assigning
	// the job to the wrong worker type.
	ghArch, err := util.ResolveToGithubArch(string(param.OSArch))
	if err != nil {
		return params.CreatePoolParams{}, errors.Wrap(err, "invalid arch")
	}

	osType, err := util.ResolveToGithubOSType(string(param.OSType))
	if err != nil {
		return params.CreatePoolParams{}, errors.Wrap(err, "invalid os type")
	}

	extraLabels := []string{
		"self-hosted",
		ghArch,
		osType,
	}

	param.Tags = append(param.Tags, extraLabels...)

	return param, nil
}

func (r *Runner) GetInstance(ctx context.Context, instanceName string) (params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return params.Instance{}, runnerErrors.ErrUnauthorized
	}

	instance, err := r.store.GetInstanceByName(ctx, instanceName)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching instance")
	}
	return instance, nil
}

func (r *Runner) ListAllInstances(ctx context.Context) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	instances, err := r.store.ListAllInstances(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fetcing instances")
	}
	return instances, nil
}

func (r *Runner) AddInstanceStatusMessage(ctx context.Context, param params.InstanceUpdateMessage) error {
	instanceID := auth.InstanceID(ctx)
	if instanceID == "" {
		return runnerErrors.ErrUnauthorized
	}

	if err := r.store.AddInstanceStatusMessage(ctx, instanceID, param.Message); err != nil {
		return errors.Wrap(err, "adding status update")
	}

	updateParams := params.UpdateInstanceParams{
		RunnerStatus: param.Status,
	}

	if _, err := r.store.UpdateInstance(r.ctx, instanceID, updateParams); err != nil {
		return errors.Wrap(err, "updating runner state")
	}

	return nil
}
