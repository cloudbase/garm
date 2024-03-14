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
	"crypto/sha1" //nolint:golangci-lint,gosec // sha1 is used for github webhooks
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/juju/clock"
	"github.com/juju/retry"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/config"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/runner/pool"
	"github.com/cloudbase/garm/runner/providers"
)

func NewRunner(ctx context.Context, cfg config.Config, db dbCommon.Store) (*Runner, error) {
	ctrlID, err := db.ControllerInfo()
	if err != nil {
		return nil, errors.Wrap(err, "fetching controller info")
	}

	providers, err := providers.LoadProvidersFromConfig(ctx, cfg, ctrlID.ControllerID.String())
	if err != nil {
		return nil, errors.Wrap(err, "loading providers")
	}

	creds := map[string]config.Github{}

	for _, ghcreds := range cfg.Github {
		creds[ghcreds.Name] = ghcreds
	}

	poolManagerCtrl := &poolManagerCtrl{
		controllerID:  ctrlID.ControllerID.String(),
		config:        cfg,
		credentials:   creds,
		repositories:  map[string]common.PoolManager{},
		organizations: map[string]common.PoolManager{},
		enterprises:   map[string]common.PoolManager{},
	}
	runner := &Runner{
		ctx:             ctx,
		config:          cfg,
		store:           db,
		poolManagerCtrl: poolManagerCtrl,
		providers:       providers,
		credentials:     creds,
		controllerID:    ctrlID.ControllerID,
	}

	if err := runner.loadReposOrgsAndEnterprises(); err != nil {
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
	enterprises   map[string]common.PoolManager
}

func (p *poolManagerCtrl) CreateRepoPoolManager(ctx context.Context, repo params.Repository, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	p.mux.Lock()
	defer p.mux.Unlock()

	cfgInternal, err := p.getInternalConfig(ctx, repo.CredentialsName, repo.GetBalancerType())
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

func (p *poolManagerCtrl) UpdateRepoPoolManager(ctx context.Context, repo params.Repository) (common.PoolManager, error) {
	p.mux.Lock()
	defer p.mux.Unlock()

	poolMgr, ok := p.repositories[repo.ID]
	if !ok {
		return nil, errors.Wrapf(runnerErrors.ErrNotFound, "repository %s/%s pool manager not loaded", repo.Owner, repo.Name)
	}

	internalCfg, err := p.getInternalConfig(ctx, repo.CredentialsName, repo.GetBalancerType())
	if err != nil {
		return nil, errors.Wrap(err, "fetching internal config")
	}

	newState := params.UpdatePoolStateParams{
		WebhookSecret:  repo.WebhookSecret,
		InternalConfig: &internalCfg,
	}

	if err := poolMgr.RefreshState(newState); err != nil {
		return nil, errors.Wrap(err, "updating repo pool manager")
	}
	return poolMgr, nil
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

	cfgInternal, err := p.getInternalConfig(ctx, org.CredentialsName, org.GetBalancerType())
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

func (p *poolManagerCtrl) UpdateOrgPoolManager(ctx context.Context, org params.Organization) (common.PoolManager, error) {
	p.mux.Lock()
	defer p.mux.Unlock()

	poolMgr, ok := p.organizations[org.ID]
	if !ok {
		return nil, errors.Wrapf(runnerErrors.ErrNotFound, "org %s pool manager not loaded", org.Name)
	}

	internalCfg, err := p.getInternalConfig(ctx, org.CredentialsName, org.GetBalancerType())
	if err != nil {
		return nil, errors.Wrap(err, "fetching internal config")
	}

	newState := params.UpdatePoolStateParams{
		WebhookSecret:  org.WebhookSecret,
		InternalConfig: &internalCfg,
	}

	if err := poolMgr.RefreshState(newState); err != nil {
		return nil, errors.Wrap(err, "updating repo pool manager")
	}
	return poolMgr, nil
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

func (p *poolManagerCtrl) CreateEnterprisePoolManager(ctx context.Context, enterprise params.Enterprise, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	p.mux.Lock()
	defer p.mux.Unlock()

	cfgInternal, err := p.getInternalConfig(ctx, enterprise.CredentialsName, enterprise.GetBalancerType())
	if err != nil {
		return nil, errors.Wrap(err, "fetching internal config")
	}
	poolManager, err := pool.NewEnterprisePoolManager(ctx, enterprise, cfgInternal, providers, store)
	if err != nil {
		return nil, errors.Wrap(err, "creating enterprise pool manager")
	}
	p.enterprises[enterprise.ID] = poolManager
	return poolManager, nil
}

func (p *poolManagerCtrl) UpdateEnterprisePoolManager(ctx context.Context, enterprise params.Enterprise) (common.PoolManager, error) {
	p.mux.Lock()
	defer p.mux.Unlock()

	poolMgr, ok := p.enterprises[enterprise.ID]
	if !ok {
		return nil, errors.Wrapf(runnerErrors.ErrNotFound, "enterprise %s pool manager not loaded", enterprise.Name)
	}

	internalCfg, err := p.getInternalConfig(ctx, enterprise.CredentialsName, enterprise.GetBalancerType())
	if err != nil {
		return nil, errors.Wrap(err, "fetching internal config")
	}

	newState := params.UpdatePoolStateParams{
		WebhookSecret:  enterprise.WebhookSecret,
		InternalConfig: &internalCfg,
	}

	if err := poolMgr.RefreshState(newState); err != nil {
		return nil, errors.Wrap(err, "updating repo pool manager")
	}
	return poolMgr, nil
}

func (p *poolManagerCtrl) GetEnterprisePoolManager(enterprise params.Enterprise) (common.PoolManager, error) {
	if enterprisePoolMgr, ok := p.enterprises[enterprise.ID]; ok {
		return enterprisePoolMgr, nil
	}
	return nil, errors.Wrapf(runnerErrors.ErrNotFound, "enterprise %s pool manager not loaded", enterprise.Name)
}

func (p *poolManagerCtrl) DeleteEnterprisePoolManager(enterprise params.Enterprise) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	poolMgr, ok := p.enterprises[enterprise.ID]
	if ok {
		if err := poolMgr.Stop(); err != nil {
			return errors.Wrap(err, "stopping enterprise pool manager")
		}
		delete(p.enterprises, enterprise.ID)
	}
	return nil
}

func (p *poolManagerCtrl) GetEnterprisePoolManagers() (map[string]common.PoolManager, error) {
	return p.enterprises, nil
}

func (p *poolManagerCtrl) getInternalConfig(ctx context.Context, credsName string, poolBalancerType params.PoolBalancerType) (params.Internal, error) {
	creds, ok := p.credentials[credsName]
	if !ok {
		return params.Internal{}, runnerErrors.NewBadRequestError("invalid credential name (%s)", credsName)
	}

	caBundle, err := creds.CACertBundle()
	if err != nil {
		return params.Internal{}, fmt.Errorf("fetching CA bundle for creds: %w", err)
	}

	var controllerWebhookURL string
	if p.config.Default.WebhookURL != "" {
		controllerWebhookURL = fmt.Sprintf("%s/%s", p.config.Default.WebhookURL, p.controllerID)
	}
	httpClient, err := creds.HTTPClient(ctx)
	if err != nil {
		return params.Internal{}, fmt.Errorf("fetching http client for creds: %w", err)
	}
	return params.Internal{
		ControllerID:         p.controllerID,
		InstanceCallbackURL:  p.config.Default.CallbackURL,
		InstanceMetadataURL:  p.config.Default.MetadataURL,
		BaseWebhookURL:       p.config.Default.WebhookURL,
		ControllerWebhookURL: controllerWebhookURL,
		JWTSecret:            p.config.JWTAuth.Secret,
		PoolBalancerType:     poolBalancerType,
		GithubCredentialsDetails: params.GithubCredentials{
			Name:          creds.Name,
			Description:   creds.Description,
			BaseURL:       creds.BaseEndpoint(),
			APIBaseURL:    creds.APIEndpoint(),
			UploadBaseURL: creds.UploadEndpoint(),
			CABundle:      caBundle,
			HTTPClient:    httpClient,
		},
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

	controllerInfo params.ControllerInfo
	controllerID   uuid.UUID
}

// GetControllerInfo returns the controller id and the hostname.
// This data might be used in metrics and logging.
func (r *Runner) GetControllerInfo(ctx context.Context) (params.ControllerInfo, error) {
	if !auth.IsAdmin(ctx) {
		return params.ControllerInfo{}, runnerErrors.ErrUnauthorized
	}
	// It is unlikely that fetching the hostname will encounter an error on a standard
	// linux (or Windows) system, but if os.Hostname() can fail, we need to at least retry
	// a few times before giving up.
	// This retries 10 times within one second. While it has the potential to give us a
	// one second delay before returning either the hostname or an error, I expect this
	// to succeed on the first try.
	// As a side note, Windows requires a reboot for the hostname change to take effect,
	// so if we'll ever support Windows as a target system, the hostname can be cached.
	var hostname string
	err := retry.Call(retry.CallArgs{
		Func: func() error {
			var err error
			hostname, err = os.Hostname()
			if err != nil {
				return errors.Wrap(err, "fetching hostname")
			}
			return nil
		},
		Attempts: 10,
		Delay:    100 * time.Millisecond,
		Clock:    clock.WallClock,
	})
	if err != nil {
		return params.ControllerInfo{}, errors.Wrap(err, "fetching hostname")
	}
	r.controllerInfo.Hostname = hostname
	var controllerWebhook string
	if r.controllerID != uuid.Nil && r.config.Default.WebhookURL != "" {
		controllerWebhook = fmt.Sprintf("%s/%s", r.config.Default.WebhookURL, r.controllerID.String())
	}
	return params.ControllerInfo{
		ControllerID:         r.controllerID,
		Hostname:             hostname,
		MetadataURL:          r.config.Default.MetadataURL,
		CallbackURL:          r.config.Default.CallbackURL,
		WebhookURL:           r.config.Default.WebhookURL,
		ControllerWebhookURL: controllerWebhook,
	}, nil
}

func (r *Runner) ListCredentials(ctx context.Context) ([]params.GithubCredentials, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}
	ret := []params.GithubCredentials{}

	for _, val := range r.config.Github {
		ret = append(ret, params.GithubCredentials{
			Name:          val.Name,
			Description:   val.Description,
			BaseURL:       val.BaseEndpoint(),
			APIBaseURL:    val.APIEndpoint(),
			UploadBaseURL: val.UploadEndpoint(),
			AuthType:      params.GithubAuthType(val.AuthType),
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

func (r *Runner) loadReposOrgsAndEnterprises() error {
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

	enterprises, err := r.store.ListEnterprises(r.ctx)
	if err != nil {
		return errors.Wrap(err, "fetching enterprises")
	}

	g, _ := errgroup.WithContext(r.ctx)
	for _, repo := range repos {
		repo := repo
		g.Go(func() error {
			slog.InfoContext(
				r.ctx, "creating pool manager for repo",
				"repo_owner", repo.Owner, "repo_name", repo.Name)
			_, err := r.poolManagerCtrl.CreateRepoPoolManager(r.ctx, repo, r.providers, r.store)
			return err
		})
	}

	for _, org := range orgs {
		org := org
		g.Go(func() error {
			slog.InfoContext(r.ctx, "creating pool manager for organization", "org_name", org.Name)
			_, err := r.poolManagerCtrl.CreateOrgPoolManager(r.ctx, org, r.providers, r.store)
			return err
		})
	}

	for _, enterprise := range enterprises {
		enterprise := enterprise
		g.Go(func() error {
			slog.InfoContext(r.ctx, "creating pool manager for enterprise", "enterprise_name", enterprise.Name)
			_, err := r.poolManagerCtrl.CreateEnterprisePoolManager(r.ctx, enterprise, r.providers, r.store)
			return err
		})
	}

	if err := r.waitForErrorGroupOrTimeout(g); err != nil {
		return fmt.Errorf("failed to create pool managers: %w", err)
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

	enterprises, err := r.poolManagerCtrl.GetEnterprisePoolManagers()
	if err != nil {
		return errors.Wrap(err, "fetch enterprise pool managers")
	}

	g, _ := errgroup.WithContext(r.ctx)
	for _, repo := range repositories {
		repo := repo
		g.Go(func() error {
			return repo.Start()
		})
	}

	for _, org := range organizations {
		org := org
		g.Go(func() error {
			return org.Start()
		})
	}

	for _, enterprise := range enterprises {
		enterprise := enterprise
		g.Go(func() error {
			return enterprise.Start()
		})
	}

	if err := r.waitForErrorGroupOrTimeout(g); err != nil {
		return fmt.Errorf("failed to start pool managers: %w", err)
	}
	return nil
}

func (r *Runner) waitForErrorGroupOrTimeout(g *errgroup.Group) error {
	if g == nil {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- g.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(60 * time.Second):
		return fmt.Errorf("timed out waiting for pool manager start")
	}
}

func (r *Runner) Stop() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	repos, err := r.poolManagerCtrl.GetRepoPoolManagers()
	if err != nil {
		return errors.Wrap(err, "fetch repo pool managers")
	}

	orgs, err := r.poolManagerCtrl.GetOrgPoolManagers()
	if err != nil {
		return errors.Wrap(err, "fetch org pool managers")
	}

	enterprises, err := r.poolManagerCtrl.GetEnterprisePoolManagers()
	if err != nil {
		return errors.Wrap(err, "fetch enterprise pool managers")
	}

	g, _ := errgroup.WithContext(r.ctx)

	for _, repo := range repos {
		poolMgr := repo
		g.Go(func() error {
			err := poolMgr.Stop()
			if err != nil {
				return fmt.Errorf("failed to stop repo pool manager: %w", err)
			}
			return poolMgr.Wait()
		})
	}

	for _, org := range orgs {
		poolMgr := org
		g.Go(func() error {
			err := poolMgr.Stop()
			if err != nil {
				return fmt.Errorf("failed to stop org pool manager: %w", err)
			}
			return poolMgr.Wait()
		})
	}

	for _, enterprise := range enterprises {
		poolMgr := enterprise
		g.Go(func() error {
			err := poolMgr.Stop()
			if err != nil {
				return fmt.Errorf("failed to stop enterprise pool manager: %w", err)
			}
			return poolMgr.Wait()
		})
	}

	if err := r.waitForErrorGroupOrTimeout(g); err != nil {
		return fmt.Errorf("failed to stop pool managers: %w", err)
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

	orgs, err := r.poolManagerCtrl.GetOrgPoolManagers()
	if err != nil {
		return errors.Wrap(err, "fetch org pool managers")
	}

	enterprises, err := r.poolManagerCtrl.GetEnterprisePoolManagers()
	if err != nil {
		return errors.Wrap(err, "fetch enterprise pool managers")
	}

	for poolID, repo := range repos {
		wg.Add(1)
		go func(id string, poolMgr common.PoolManager) {
			defer wg.Done()
			if err := poolMgr.Wait(); err != nil {
				slog.With(slog.Any("error", err)).ErrorContext(r.ctx, "timed out waiting for pool manager to exit", "pool_id", id, "pool_mgr_id", poolMgr.ID())
			}
		}(poolID, repo)
	}

	for poolID, org := range orgs {
		wg.Add(1)
		go func(id string, poolMgr common.PoolManager) {
			defer wg.Done()
			if err := poolMgr.Wait(); err != nil {
				slog.With(slog.Any("error", err)).ErrorContext(r.ctx, "timed out waiting for pool manager to exit", "pool_id", id)
			}
		}(poolID, org)
	}

	for poolID, enterprise := range enterprises {
		wg.Add(1)
		go func(id string, poolMgr common.PoolManager) {
			defer wg.Done()
			if err := poolMgr.Wait(); err != nil {
				slog.With(slog.Any("error", err)).ErrorContext(r.ctx, "timed out waiting for pool manager to exit", "pool_id", id)
			}
		}(poolID, enterprise)
	}

	wg.Wait()
	return nil
}

func (r *Runner) validateHookBody(signature, secret string, body []byte) error {
	if secret == "" {
		return runnerErrors.NewMissingSecretError("missing secret to validate webhook signature")
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
		slog.DebugContext(
			r.ctx, "got hook for repo",
			"repo_owner", util.SanitizeLogEntry(job.Repository.Owner.Login),
			"repo_name", util.SanitizeLogEntry(job.Repository.Name))
		poolManager, err = r.findRepoPoolManager(job.Repository.Owner.Login, job.Repository.Name)
	case OrganizationHook:
		slog.DebugContext(
			r.ctx, "got hook for organization",
			"organization", util.SanitizeLogEntry(job.Organization.Login))
		poolManager, err = r.findOrgPoolManager(job.Organization.Login)
	case EnterpriseHook:
		slog.DebugContext(
			r.ctx, "got hook for enterprise",
			"enterprise", util.SanitizeLogEntry(job.Enterprise.Slug))
		poolManager, err = r.findEnterprisePoolManager(job.Enterprise.Slug)
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

	newTags, err := r.processTags(string(param.OSArch), param.OSType, param.Tags)
	if err != nil {
		return params.CreatePoolParams{}, errors.Wrap(err, "processing tags")
	}

	param.Tags = newTags

	return param, nil
}

func (r *Runner) processTags(osArch string, osType commonParams.OSType, tags []string) ([]string, error) {
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

	ghOSType, err := util.ResolveToGithubTag(osType)
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

	if err := r.store.AddInstanceEvent(ctx, instanceID, params.StatusEvent, params.EventInfo, param.Message); err != nil {
		return errors.Wrap(err, "adding status update")
	}

	updateParams := params.UpdateInstanceParams{
		RunnerStatus: param.Status,
	}

	if param.AgentID != nil {
		updateParams.AgentID = *param.AgentID
	}

	if _, err := r.store.UpdateInstance(r.ctx, instanceID, updateParams); err != nil {
		return errors.Wrap(err, "updating runner agent ID")
	}

	return nil
}

func (r *Runner) UpdateSystemInfo(ctx context.Context, param params.UpdateSystemInfoParams) error {
	instanceID := auth.InstanceID(ctx)
	if instanceID == "" {
		slog.ErrorContext(ctx, "missing instance ID")
		return runnerErrors.ErrUnauthorized
	}

	if param.OSName == "" && param.OSVersion == "" && param.AgentID == nil {
		// Nothing to update
		return nil
	}

	updateParams := params.UpdateInstanceParams{
		OSName:    param.OSName,
		OSVersion: param.OSVersion,
	}

	if param.AgentID != nil {
		updateParams.AgentID = *param.AgentID
	}

	if _, err := r.store.UpdateInstance(r.ctx, instanceID, updateParams); err != nil {
		return errors.Wrap(err, "updating runner system info")
	}

	return nil
}

func (r *Runner) getPoolManagerFromInstance(ctx context.Context, instance params.Instance) (common.PoolManager, error) {
	pool, err := r.store.GetPoolByID(ctx, instance.PoolID)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pool")
	}

	var poolMgr common.PoolManager

	switch {
	case pool.RepoID != "":
		repo, err := r.store.GetRepositoryByID(ctx, pool.RepoID)
		if err != nil {
			return nil, errors.Wrap(err, "fetching repo")
		}
		poolMgr, err = r.findRepoPoolManager(repo.Owner, repo.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching pool manager for repo %s", pool.RepoName)
		}
	case pool.OrgID != "":
		org, err := r.store.GetOrganizationByID(ctx, pool.OrgID)
		if err != nil {
			return nil, errors.Wrap(err, "fetching org")
		}
		poolMgr, err = r.findOrgPoolManager(org.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching pool manager for org %s", pool.OrgName)
		}
	case pool.EnterpriseID != "":
		enterprise, err := r.store.GetEnterpriseByID(ctx, pool.EnterpriseID)
		if err != nil {
			return nil, errors.Wrap(err, "fetching enterprise")
		}
		poolMgr, err = r.findEnterprisePoolManager(enterprise.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching pool manager for enterprise %s", pool.EnterpriseName)
		}
	}

	return poolMgr, nil
}

// DeleteRunner removes a runner from a pool. If forceDelete is true, GARM will ignore any provider errors
// that may occur, and attempt to remove the runner from GitHub and then the database, regardless of provider
// errors.
func (r *Runner) DeleteRunner(ctx context.Context, instanceName string, forceDelete, bypassGithubUnauthorized bool) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	instance, err := r.store.GetInstanceByName(ctx, instanceName)
	if err != nil {
		return errors.Wrap(err, "fetching instance")
	}

	switch instance.Status {
	case commonParams.InstanceRunning, commonParams.InstanceError,
		commonParams.InstancePendingForceDelete, commonParams.InstancePendingDelete:
	default:
		validStates := []string{
			string(commonParams.InstanceRunning),
			string(commonParams.InstanceError),
			string(commonParams.InstancePendingForceDelete),
			string(commonParams.InstancePendingDelete),
		}
		return runnerErrors.NewBadRequestError("runner must be in one of the following states: %q", strings.Join(validStates, ", "))
	}

	poolMgr, err := r.getPoolManagerFromInstance(ctx, instance)
	if err != nil {
		return errors.Wrap(err, "fetching pool manager for instance")
	}

	if err := poolMgr.DeleteRunner(instance, forceDelete, bypassGithubUnauthorized); err != nil {
		return errors.Wrap(err, "removing runner")
	}
	return nil
}
