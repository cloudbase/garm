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
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io/ioutil"
	"log"
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
	"garm/runner/pool"
	"garm/runner/providers"
	providerCommon "garm/runner/providers/common"
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

	poolManagerCtrl := &poolManagerCtrl{
		controllerID:  ctrlId.ControllerID.String(),
		config:        cfg,
		credentials:   creds,
		repositories:  map[string]common.PoolManager{},
		organizations: map[string]common.PoolManager{},
	}
	runner := &Runner{
		ctx:             ctx,
		config:          cfg,
		store:           db,
		poolManagerCtrl: poolManagerCtrl,
		providers:       providers,
		credentials:     creds,
	}

	if err := runner.loadReposAndOrgs(); err != nil {
		return nil, errors.Wrap(err, "loading pool managers")
	}

	return runner, nil
}

type poolManagerCtrl struct {
	mux sync.Mutex

	controllerID string
	config       config.Config
	credentials  map[string]config.Github

	repositories  map[string]common.PoolManager
	organizations map[string]common.PoolManager
}

func (p *poolManagerCtrl) CreateRepoPoolManager(ctx context.Context, repo params.Repository, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	p.mux.Lock()
	defer p.mux.Unlock()

	cfgInternal, err := p.getInternalConfig(repo.CredentialsName)
	if err != nil {
		return nil, errors.Wrap(err, "fetching internal config")
	}
	poolManager, err := pool.NewRepositoryPoolManager(ctx, repo, cfgInternal, providers, store)
	if err != nil {
		return nil, errors.Wrap(err, "creating repo pool manager")
	}
	p.repositories[repo.ID] = poolManager
	return poolManager, nil
}

func (p *poolManagerCtrl) GetRepoPoolManager(repo params.Repository) (common.PoolManager, error) {
	if repoPoolMgr, ok := p.repositories[repo.ID]; ok {
		return repoPoolMgr, nil
	}
	return nil, errors.Wrapf(runnerErrors.ErrNotFound, "repository %s/%s pool manager not loaded", repo.Owner, repo.Name)
}

func (p *poolManagerCtrl) DeleteRepoPoolManager(repo params.Repository) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	poolMgr, ok := p.repositories[repo.ID]
	if ok {
		if err := poolMgr.Stop(); err != nil {
			return errors.Wrap(err, "stopping repo pool manager")
		}
		delete(p.repositories, repo.ID)
	}
	return nil
}

func (p *poolManagerCtrl) GetRepoPoolManagers() (map[string]common.PoolManager, error) {
	return p.repositories, nil
}

func (p *poolManagerCtrl) CreateOrgPoolManager(ctx context.Context, org params.Organization, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	p.mux.Lock()
	defer p.mux.Unlock()

	cfgInternal, err := p.getInternalConfig(org.CredentialsName)
	if err != nil {
		return nil, errors.Wrap(err, "fetching internal config")
	}
	poolManager, err := pool.NewOrganizationPoolManager(ctx, org, cfgInternal, providers, store)
	if err != nil {
		return nil, errors.Wrap(err, "creating org pool manager")
	}
	p.organizations[org.ID] = poolManager
	return poolManager, nil
}

func (p *poolManagerCtrl) GetOrgPoolManager(org params.Organization) (common.PoolManager, error) {
	if orgPoolMgr, ok := p.organizations[org.ID]; ok {
		return orgPoolMgr, nil
	}
	return nil, errors.Wrapf(runnerErrors.ErrNotFound, "organization %s pool manager not loaded", org.Name)
}

func (p *poolManagerCtrl) DeleteOrgPoolManager(org params.Organization) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	poolMgr, ok := p.organizations[org.ID]
	if ok {
		if err := poolMgr.Stop(); err != nil {
			return errors.Wrap(err, "stopping org pool manager")
		}
		delete(p.organizations, org.ID)
	}
	return nil
}

func (p *poolManagerCtrl) GetOrgPoolManagers() (map[string]common.PoolManager, error) {
	return p.organizations, nil
}

func (p *poolManagerCtrl) getInternalConfig(credsName string) (params.Internal, error) {
	creds, ok := p.credentials[credsName]
	if !ok {
		return params.Internal{}, runnerErrors.NewBadRequestError("invalid credential name (%s)", credsName)
	}

	return params.Internal{
		OAuth2Token:         creds.OAuth2Token,
		ControllerID:        p.controllerID,
		InstanceCallbackURL: p.config.Default.CallbackURL,
		JWTSecret:           p.config.JWTAuth.Secret,
	}, nil
}

type Runner struct {
	mux sync.Mutex

	config config.Config
	ctx    context.Context
	store  dbCommon.Store

	poolManagerCtrl PoolManagerController

	providers   map[string]common.Provider
	credentials map[string]config.Github
}

func (r *Runner) ListCredentials(ctx context.Context) ([]params.GithubCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}
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
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}
	ret := []params.Provider{}

	for _, val := range r.providers {
		ret = append(ret, val.AsParams())
	}
	return ret, nil
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
		return errors.Wrap(err, "fetching organizations")
	}

	expectedReplies := len(repos) + len(orgs)
	errChan := make(chan error, expectedReplies)

	for _, repo := range repos {
		go func(repo params.Repository) {
			log.Printf("creating pool manager for repo %s/%s", repo.Owner, repo.Name)
			_, err := r.poolManagerCtrl.CreateRepoPoolManager(r.ctx, repo, r.providers, r.store)
			errChan <- err
		}(repo)
	}

	for _, org := range orgs {
		go func(org params.Organization) {
			log.Printf("creating pool manager for organization %s", org.Name)
			_, err := r.poolManagerCtrl.CreateOrgPoolManager(r.ctx, org, r.providers, r.store)
			errChan <- err
		}(org)
	}

	for i := 0; i < expectedReplies; i++ {
		select {
		case err := <-errChan:
			if err != nil {
				return errors.Wrap(err, "failed to load pool managers for repos and orgs")
			}
		case <-time.After(60 * time.Second):
			return fmt.Errorf("timed out waiting for pool manager load")
		}
	}

	return nil
}

func (r *Runner) Start() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	repositories, err := r.poolManagerCtrl.GetRepoPoolManagers()
	if err != nil {
		return errors.Wrap(err, "fetch repo pool managers")
	}

	organizations, err := r.poolManagerCtrl.GetOrgPoolManagers()
	if err != nil {
		return errors.Wrap(err, "fetch org pool managers")
	}

	expectedReplies := len(repositories) + len(organizations)
	errChan := make(chan error, expectedReplies)

	for _, repo := range repositories {
		go func(repo common.PoolManager) {
			err := repo.Start()
			errChan <- err

		}(repo)
	}

	for _, org := range organizations {
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

	repos, err := r.poolManagerCtrl.GetRepoPoolManagers()
	if err != nil {
		return errors.Wrap(err, "fetch repo pool managers")
	}
	for _, repo := range repos {
		if err := repo.Stop(); err != nil {
			return errors.Wrap(err, "stopping repo pool manager")
		}
	}

	orgs, err := r.poolManagerCtrl.GetOrgPoolManagers()
	if err != nil {
		return errors.Wrap(err, "fetch org pool managers")
	}
	for _, org := range orgs {
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

	repos, err := r.poolManagerCtrl.GetRepoPoolManagers()
	if err != nil {
		return errors.Wrap(err, "fetch repo pool managers")
	}
	for poolId, repo := range repos {
		wg.Add(1)
		go func(id string, poolMgr common.PoolManager) {
			defer wg.Done()
			if err := poolMgr.Wait(); err != nil {
				log.Printf("timed out waiting for pool manager %s to exit", id)
			}
		}(poolId, repo)
	}

	orgs, err := r.poolManagerCtrl.GetOrgPoolManagers()
	if err != nil {
		return errors.Wrap(err, "fetch org pool managers")
	}
	for poolId, org := range orgs {
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
	if len(jobData) == 0 {
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

	newTags, err := r.processTags(string(param.OSArch), string(param.OSType), param.Tags)
	if err != nil {
		return params.CreatePoolParams{}, errors.Wrap(err, "processing tags")
	}

	param.Tags = newTags

	return param, nil
}

func (r *Runner) processTags(osArch, osType string, tags []string) ([]string, error) {
	// github automatically adds the "self-hosted" tag as well as the OS type (linux, windows, etc)
	// and architecture (arm, x64, etc) to all self hosted runners. When a workflow job comes in, we try
	// to find a pool based on the labels that are set in the workflow. If we don't explicitly define these
	// default tags for each pool, and the user targets these labels, we won't be able to match any pools.
	// The downside is that all pools with the same OS and arch will have these default labels. Users should
	// set distinct and unique labels on each pool, and explicitly target those labels, or risk assigning
	// the job to the wrong worker type.
	ghArch, err := util.ResolveToGithubArch(osArch)
	if err != nil {
		return nil, errors.Wrap(err, "invalid arch")
	}

	ghOSType, err := util.ResolveToGithubOSType(osType)
	if err != nil {
		return nil, errors.Wrap(err, "invalid os type")
	}

	labels := []string{
		"self-hosted",
		ghArch,
		ghOSType,
	}

	for _, val := range tags {
		if val != "self-hosted" && val != ghArch && val != ghOSType {
			labels = append(labels, val)
		}
	}

	return labels, nil
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
		return nil, errors.Wrap(err, "fetching instances")
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

	if param.AgentID != nil {
		updateParams.AgentID = *param.AgentID
	}

	if _, err := r.store.UpdateInstance(r.ctx, instanceID, updateParams); err != nil {
		return errors.Wrap(err, "updating runner state")
	}

	return nil
}

func (r *Runner) ForceDeleteRunner(ctx context.Context, instanceName string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	instance, err := r.store.GetInstanceByName(ctx, instanceName)
	if err != nil {
		return errors.Wrap(err, "fetching instance")
	}

	switch instance.Status {
	case providerCommon.InstanceRunning, providerCommon.InstanceError:
	default:
		return runnerErrors.NewBadRequestError("runner must be in %q or %q state", providerCommon.InstanceRunning, providerCommon.InstanceError)
	}

	pool, err := r.store.GetPoolByID(ctx, instance.PoolID)
	if err != nil {
		return errors.Wrap(err, "fetching pool")
	}

	var poolMgr common.PoolManager

	if pool.RepoID != "" {
		repo, err := r.store.GetRepositoryByID(ctx, pool.RepoID)
		if err != nil {
			return errors.Wrap(err, "fetching repo")
		}
		poolMgr, err = r.findRepoPoolManager(repo.Owner, repo.Name)
		if err != nil {
			return errors.Wrapf(err, "fetching pool manager for repo %s", pool.RepoName)
		}
	} else if pool.OrgID != "" {
		org, err := r.store.GetOrganizationByID(ctx, pool.OrgID)
		if err != nil {
			return errors.Wrap(err, "fetching org")
		}
		poolMgr, err = r.findOrgPoolManager(org.Name)
		if err != nil {
			return errors.Wrapf(err, "fetching pool manager for org %s", pool.OrgName)
		}
	}

	if err := poolMgr.ForceDeleteRunner(instance); err != nil {
		return errors.Wrap(err, "removing runner")
	}
	return nil
}
