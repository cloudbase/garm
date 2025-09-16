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
	"errors"
	"fmt"
	"hash"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

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
	"github.com/cloudbase/garm/util/github"
	"github.com/cloudbase/garm/util/github/scalesets"
)

func NewRunner(ctx context.Context, cfg config.Config, db dbCommon.Store, token auth.InstanceTokenGetter) (*Runner, error) {
	ctrlID, err := db.ControllerInfo()
	if err != nil {
		return nil, fmt.Errorf("error fetching controller info: %w", err)
	}

	providers, err := providers.LoadProvidersFromConfig(ctx, cfg, ctrlID.ControllerID.String())
	if err != nil {
		return nil, fmt.Errorf("error loading providers: %w", err)
	}

	creds := map[string]config.Github{}

	for _, ghcreds := range cfg.Github {
		creds[ghcreds.Name] = ghcreds
	}

	poolManagerCtrl := &poolManagerCtrl{
		config:        cfg,
		store:         db,
		token:         token,
		repositories:  map[string]common.PoolManager{},
		organizations: map[string]common.PoolManager{},
		enterprises:   map[string]common.PoolManager{},
	}
	runner := &Runner{
		ctx:             ctx,
		config:          cfg,
		store:           db,
		tokenGetter:     token,
		poolManagerCtrl: poolManagerCtrl,
		providers:       providers,
	}

	if err := runner.loadReposOrgsAndEnterprises(); err != nil {
		return nil, fmt.Errorf("error loading pool managers: %w", err)
	}

	return runner, nil
}

type poolManagerCtrl struct {
	mux sync.Mutex

	config config.Config
	store  dbCommon.Store
	token  auth.InstanceTokenGetter

	repositories  map[string]common.PoolManager
	organizations map[string]common.PoolManager
	enterprises   map[string]common.PoolManager
}

func (p *poolManagerCtrl) CreateRepoPoolManager(ctx context.Context, repo params.Repository, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	p.mux.Lock()
	defer p.mux.Unlock()

	entity, err := repo.GetEntity()
	if err != nil {
		return nil, fmt.Errorf("error getting entity: %w", err)
	}

	poolManager, err := pool.NewEntityPoolManager(ctx, entity, p.token, providers, store)
	if err != nil {
		return nil, fmt.Errorf("error creating repo pool manager: %w", err)
	}
	p.repositories[repo.ID] = poolManager
	return poolManager, nil
}

func (p *poolManagerCtrl) GetRepoPoolManager(repo params.Repository) (common.PoolManager, error) {
	if repoPoolMgr, ok := p.repositories[repo.ID]; ok {
		return repoPoolMgr, nil
	}
	return nil, fmt.Errorf("repository %s/%s pool manager not loaded: %w", repo.Owner, repo.Name, runnerErrors.ErrNotFound)
}

func (p *poolManagerCtrl) DeleteRepoPoolManager(repo params.Repository) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	poolMgr, ok := p.repositories[repo.ID]
	if ok {
		if err := poolMgr.Stop(); err != nil {
			return fmt.Errorf("error stopping repo pool manager: %w", err)
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

	entity, err := org.GetEntity()
	if err != nil {
		return nil, fmt.Errorf("error getting entity: %w", err)
	}

	poolManager, err := pool.NewEntityPoolManager(ctx, entity, p.token, providers, store)
	if err != nil {
		return nil, fmt.Errorf("error creating org pool manager: %w", err)
	}
	p.organizations[org.ID] = poolManager
	return poolManager, nil
}

func (p *poolManagerCtrl) GetOrgPoolManager(org params.Organization) (common.PoolManager, error) {
	if orgPoolMgr, ok := p.organizations[org.ID]; ok {
		return orgPoolMgr, nil
	}
	return nil, fmt.Errorf("organization %s pool manager not loaded: %w", org.Name, runnerErrors.ErrNotFound)
}

func (p *poolManagerCtrl) DeleteOrgPoolManager(org params.Organization) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	poolMgr, ok := p.organizations[org.ID]
	if ok {
		if err := poolMgr.Stop(); err != nil {
			return fmt.Errorf("error stopping org pool manager: %w", err)
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

	entity, err := enterprise.GetEntity()
	if err != nil {
		return nil, fmt.Errorf("error getting entity: %w", err)
	}

	poolManager, err := pool.NewEntityPoolManager(ctx, entity, p.token, providers, store)
	if err != nil {
		return nil, fmt.Errorf("error creating enterprise pool manager: %w", err)
	}
	p.enterprises[enterprise.ID] = poolManager
	return poolManager, nil
}

func (p *poolManagerCtrl) GetEnterprisePoolManager(enterprise params.Enterprise) (common.PoolManager, error) {
	if enterprisePoolMgr, ok := p.enterprises[enterprise.ID]; ok {
		return enterprisePoolMgr, nil
	}
	return nil, fmt.Errorf("enterprise %s pool manager not loaded: %w", enterprise.Name, runnerErrors.ErrNotFound)
}

func (p *poolManagerCtrl) DeleteEnterprisePoolManager(enterprise params.Enterprise) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	poolMgr, ok := p.enterprises[enterprise.ID]
	if ok {
		if err := poolMgr.Stop(); err != nil {
			return fmt.Errorf("error stopping enterprise pool manager: %w", err)
		}
		delete(p.enterprises, enterprise.ID)
	}
	return nil
}

func (p *poolManagerCtrl) GetEnterprisePoolManagers() (map[string]common.PoolManager, error) {
	return p.enterprises, nil
}

type Runner struct {
	mux sync.Mutex

	config      config.Config
	ctx         context.Context
	store       dbCommon.Store
	tokenGetter auth.InstanceTokenGetter

	poolManagerCtrl PoolManagerController

	providers map[string]common.Provider
}

// UpdateController will update the controller settings.
func (r *Runner) UpdateController(ctx context.Context, param params.UpdateControllerParams) (params.ControllerInfo, error) {
	if !auth.IsAdmin(ctx) {
		return params.ControllerInfo{}, runnerErrors.ErrUnauthorized
	}

	if err := param.Validate(); err != nil {
		return params.ControllerInfo{}, fmt.Errorf("error validating controller update params: %w", err)
	}

	info, err := r.store.UpdateController(param)
	if err != nil {
		return params.ControllerInfo{}, fmt.Errorf("error updating controller info: %w", err)
	}
	return info, nil
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
	var err error
	for range 10 {
		hostname, err = os.Hostname()
		if err != nil {
			select {
			case <-time.After(10 * time.Millisecond):
				continue
			case <-ctx.Done():
			}
			return params.ControllerInfo{}, fmt.Errorf("error fetching hostname: %w", err)
		}
		break
	}
	if err != nil {
		return params.ControllerInfo{}, fmt.Errorf("error fetching hostname: %w", err)
	}

	info, err := r.store.ControllerInfo()
	if err != nil {
		return params.ControllerInfo{}, fmt.Errorf("error fetching controller info: %w", err)
	}

	// This is temporary. Right now, GARM is a single-instance deployment. When we add the
	// ability to scale out, the hostname field will be moved form here to a dedicated node
	// object. As a single controller will be made up of multiple nodes, we will need to model
	// that aspect of GARM.
	info.Hostname = hostname
	return info, nil
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

	repos, err := r.store.ListRepositories(r.ctx, params.RepositoryFilter{})
	if err != nil {
		return fmt.Errorf("error fetching repositories: %w", err)
	}

	orgs, err := r.store.ListOrganizations(r.ctx, params.OrganizationFilter{})
	if err != nil {
		return fmt.Errorf("error fetching organizations: %w", err)
	}

	enterprises, err := r.store.ListEnterprises(r.ctx, params.EnterpriseFilter{})
	if err != nil {
		return fmt.Errorf("error fetching enterprises: %w", err)
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
		return fmt.Errorf("error fetch repo pool managers: %w", err)
	}

	organizations, err := r.poolManagerCtrl.GetOrgPoolManagers()
	if err != nil {
		return fmt.Errorf("error fetch org pool managers: %w", err)
	}

	enterprises, err := r.poolManagerCtrl.GetEnterprisePoolManagers()
	if err != nil {
		return fmt.Errorf("error fetch enterprise pool managers: %w", err)
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
	timer := time.NewTimer(60 * time.Second)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		return fmt.Errorf("timed out waiting for pool manager start")
	}
}

func (r *Runner) Stop() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	repos, err := r.poolManagerCtrl.GetRepoPoolManagers()
	if err != nil {
		return fmt.Errorf("error fetching repo pool managers: %w", err)
	}

	orgs, err := r.poolManagerCtrl.GetOrgPoolManagers()
	if err != nil {
		return fmt.Errorf("error fetching org pool managers: %w", err)
	}

	enterprises, err := r.poolManagerCtrl.GetEnterprisePoolManagers()
	if err != nil {
		return fmt.Errorf("error fetching enterprise pool managers: %w", err)
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
		return fmt.Errorf("error fetching repo pool managers: %w", err)
	}

	orgs, err := r.poolManagerCtrl.GetOrgPoolManagers()
	if err != nil {
		return fmt.Errorf("error fetching org pool managers: %w", err)
	}

	enterprises, err := r.poolManagerCtrl.GetEnterprisePoolManagers()
	if err != nil {
		return fmt.Errorf("error fetching enterprise pool managers: %w", err)
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
		return fmt.Errorf("failed to compute sha256: %w", err)
	}
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sigParts[1]), []byte(expectedMAC)) {
		return runnerErrors.NewUnauthorizedError("signature missmatch")
	}

	return nil
}

func (r *Runner) findEndpointForJob(job params.WorkflowJob, forgeType params.EndpointType) (params.ForgeEndpoint, error) {
	uri, err := url.ParseRequestURI(job.WorkflowJob.HTMLURL)
	if err != nil {
		return params.ForgeEndpoint{}, fmt.Errorf("error parsing job URL: %w", err)
	}
	baseURI := fmt.Sprintf("%s://%s", uri.Scheme, uri.Host)

	// Note(gabriel-samfira): Endpoints should be cached. We don't expect to have a large number
	// of endpoints. In most cases there will be just one (github.com). In cases where there is
	// a GHES involved, those users will have just one extra endpoint or 2 (if they also have a
	// test env). But there should be a relatively small number, regardless. So we don't really care
	// that much about the performance of this function.
	var endpoints []params.ForgeEndpoint
	switch forgeType {
	case params.GithubEndpointType:
		endpoints, err = r.store.ListGithubEndpoints(r.ctx)
	case params.GiteaEndpointType:
		endpoints, err = r.store.ListGiteaEndpoints(r.ctx)
	default:
		return params.ForgeEndpoint{}, runnerErrors.NewBadRequestError("unknown forge type %s", forgeType)
	}

	if err != nil {
		return params.ForgeEndpoint{}, fmt.Errorf("error fetching github endpoints: %w", err)
	}
	for _, ep := range endpoints {
		slog.DebugContext(r.ctx, "checking endpoint", "base_uri", baseURI, "endpoint", ep.BaseURL)
		epBaseURI := strings.TrimSuffix(ep.BaseURL, "/")
		if epBaseURI == baseURI {
			return ep, nil
		}
	}

	return params.ForgeEndpoint{}, runnerErrors.NewNotFoundError("no endpoint found for job")
}

func (r *Runner) DispatchWorkflowJob(hookTargetType, signature string, forgeType params.EndpointType, jobData []byte) error {
	if len(jobData) == 0 {
		slog.ErrorContext(r.ctx, "missing job data")
		return runnerErrors.NewBadRequestError("missing job data")
	}

	var job params.WorkflowJob
	if err := json.Unmarshal(jobData, &job); err != nil {
		slog.ErrorContext(r.ctx, "failed to unmarshal job data", "error", err)
		return fmt.Errorf("invalid job data %s: %w", err, runnerErrors.ErrBadRequest)
	}

	endpoint, err := r.findEndpointForJob(job, forgeType)
	if err != nil {
		slog.ErrorContext(r.ctx, "failed to find endpoint for job", "error", err)
		return fmt.Errorf("error finding endpoint for job: %w", err)
	}

	var poolManager common.PoolManager

	switch HookTargetType(hookTargetType) {
	case RepoHook:
		slog.DebugContext(
			r.ctx, "got hook for repo",
			"repo_owner", util.SanitizeLogEntry(job.Repository.Owner.Login),
			"repo_name", util.SanitizeLogEntry(job.Repository.Name),
			"endpoint", endpoint.Name)
		poolManager, err = r.findRepoPoolManager(job.Repository.Owner.Login, job.Repository.Name, endpoint.Name)
	case OrganizationHook:
		slog.DebugContext(
			r.ctx, "got hook for organization",
			"organization", util.SanitizeLogEntry(job.GetOrgName(forgeType)),
			"endpoint", endpoint.Name)
		poolManager, err = r.findOrgPoolManager(job.GetOrgName(forgeType), endpoint.Name)
	case EnterpriseHook:
		slog.DebugContext(
			r.ctx, "got hook for enterprise",
			"enterprise", util.SanitizeLogEntry(job.Enterprise.Slug),
			"endpoint", endpoint.Name)
		poolManager, err = r.findEnterprisePoolManager(job.Enterprise.Slug, endpoint.Name)
	default:
		return runnerErrors.NewBadRequestError("cannot handle hook target type %s", hookTargetType)
	}

	slog.DebugContext(r.ctx, "found pool manager", "pool_manager", poolManager.ID())
	if err != nil {
		slog.ErrorContext(r.ctx, "failed to find pool manager", "error", err, "hook_target_type", hookTargetType)
		// We don't have a repository or organization configured that
		// can handle this workflow job.
		return fmt.Errorf("error fetching poolManager: %w", err)
	}

	// We found a pool. Validate the webhook job. If a secret is configured,
	// we make sure that the source of this workflow job is valid.
	secret := poolManager.WebhookSecret()
	if err := r.validateHookBody(signature, secret, jobData); err != nil {
		slog.ErrorContext(r.ctx, "failed to validate webhook data", "error", err)
		return fmt.Errorf("error validating webhook data: %w", err)
	}

	if err := poolManager.HandleWorkflowJob(job); err != nil {
		slog.ErrorContext(r.ctx, "failed to handle workflow job", "error", err)
		return fmt.Errorf("error handling workflow job: %w", err)
	}

	return nil
}

func (r *Runner) appendTagsToCreatePoolParams(param params.CreatePoolParams) (params.CreatePoolParams, error) {
	if err := param.Validate(); err != nil {
		return params.CreatePoolParams{}, fmt.Errorf("failed to validate params (%q): %w", err, runnerErrors.ErrBadRequest)
		// errors.Wrapf(runnerErrors.ErrBadRequest, "validating params: %s", err)
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

	return param, nil
}

func (r *Runner) GetInstance(ctx context.Context, instanceName string) (params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return params.Instance{}, runnerErrors.ErrUnauthorized
	}

	instance, err := r.store.GetInstance(ctx, instanceName)
	if err != nil {
		return params.Instance{}, fmt.Errorf("error fetching instance: %w", err)
	}
	return instance, nil
}

func (r *Runner) ListAllInstances(ctx context.Context) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	instances, err := r.store.ListAllInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching instances: %w", err)
	}
	return instances, nil
}

func (r *Runner) AddInstanceStatusMessage(ctx context.Context, param params.InstanceUpdateMessage) error {
	instanceName := auth.InstanceName(ctx)
	if instanceName == "" {
		return runnerErrors.ErrUnauthorized
	}

	if err := r.store.AddInstanceEvent(ctx, instanceName, params.StatusEvent, params.EventInfo, param.Message); err != nil {
		return fmt.Errorf("error adding status update: %w", err)
	}

	updateParams := params.UpdateInstanceParams{
		RunnerStatus: param.Status,
	}

	if param.AgentID != nil {
		updateParams.AgentID = *param.AgentID
	}

	if _, err := r.store.UpdateInstance(r.ctx, instanceName, updateParams); err != nil {
		return fmt.Errorf("error updating runner agent ID: %w", err)
	}

	return nil
}

func (r *Runner) UpdateSystemInfo(ctx context.Context, param params.UpdateSystemInfoParams) error {
	instanceName := auth.InstanceName(ctx)
	if instanceName == "" {
		slog.ErrorContext(ctx, "missing instance name")
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

	if _, err := r.store.UpdateInstance(r.ctx, instanceName, updateParams); err != nil {
		return fmt.Errorf("error updating runner system info: %w", err)
	}

	return nil
}

func (r *Runner) getPoolManagerFromInstance(ctx context.Context, instance params.Instance) (common.PoolManager, error) {
	pool, err := r.store.GetPoolByID(ctx, instance.PoolID)
	if err != nil {
		return nil, fmt.Errorf("error fetching pool: %w", err)
	}

	var poolMgr common.PoolManager

	switch {
	case pool.RepoID != "":
		repo, err := r.store.GetRepositoryByID(ctx, pool.RepoID)
		if err != nil {
			return nil, fmt.Errorf("error fetching repo: %w", err)
		}
		poolMgr, err = r.findRepoPoolManager(repo.Owner, repo.Name, repo.Endpoint.Name)
		if err != nil {
			return nil, fmt.Errorf("error fetching pool manager for repo %s: %w", pool.RepoName, err)
		}
	case pool.OrgID != "":
		org, err := r.store.GetOrganizationByID(ctx, pool.OrgID)
		if err != nil {
			return nil, fmt.Errorf("error fetching org: %w", err)
		}
		poolMgr, err = r.findOrgPoolManager(org.Name, org.Endpoint.Name)
		if err != nil {
			return nil, fmt.Errorf("error fetching pool manager for org %s: %w", pool.OrgName, err)
		}
	case pool.EnterpriseID != "":
		enterprise, err := r.store.GetEnterpriseByID(ctx, pool.EnterpriseID)
		if err != nil {
			return nil, fmt.Errorf("error fetching enterprise: %w", err)
		}
		poolMgr, err = r.findEnterprisePoolManager(enterprise.Name, enterprise.Endpoint.Name)
		if err != nil {
			return nil, fmt.Errorf("error fetching pool manager for enterprise %s: %w", pool.EnterpriseName, err)
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

	instance, err := r.store.GetInstance(ctx, instanceName)
	if err != nil {
		return fmt.Errorf("error fetching instance: %w", err)
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

	ghCli, ssCli, err := r.getGHCliFromInstance(ctx, instance)
	if err != nil {
		return fmt.Errorf("error fetching github client: %w", err)
	}

	if instance.AgentID != 0 {
		switch {
		case instance.ScaleSetID != 0:
			err = ssCli.RemoveRunner(ctx, instance.AgentID)
		case instance.PoolID != "":
			err = ghCli.RemoveEntityRunner(ctx, instance.AgentID)
		default:
			return errors.New("instance does not have a pool or scale set")
		}

		if err != nil {
			if !errors.Is(err, runnerErrors.ErrNotFound) {
				if errors.Is(err, runnerErrors.ErrUnauthorized) && instance.PoolID != "" {
					poolMgr, err := r.getPoolManagerFromInstance(ctx, instance)
					if err != nil {
						return fmt.Errorf("error fetching pool manager for instance: %w", err)
					}
					poolMgr.SetPoolRunningState(false, fmt.Sprintf("failed to remove runner: %q", err))
				}
				if !bypassGithubUnauthorized {
					return fmt.Errorf("error removing runner from github: %w", err)
				}
			}
		}
	}

	instanceStatus := commonParams.InstancePendingDelete
	if forceDelete {
		instanceStatus = commonParams.InstancePendingForceDelete
	}

	slog.InfoContext(
		r.ctx, "setting instance status",
		"runner_name", instance.Name,
		"status", instanceStatus)

	updateParams := params.UpdateInstanceParams{
		Status: instanceStatus,
	}
	_, err = r.store.UpdateInstance(r.ctx, instance.Name, updateParams)
	if err != nil {
		return fmt.Errorf("error updating runner state: %w", err)
	}

	return nil
}

func (r *Runner) getGHCliFromInstance(ctx context.Context, instance params.Instance) (common.GithubClient, *scalesets.ScaleSetClient, error) {
	// nolint:golangci-lint,godox
	// TODO(gabriel-samfira): We can probably cache the entity.
	var entityGetter params.EntityGetter
	var err error

	switch {
	case instance.PoolID != "":
		entityGetter, err = r.store.GetPoolByID(ctx, instance.PoolID)
		if err != nil {
			return nil, nil, fmt.Errorf("error fetching pool: %w", err)
		}
	case instance.ScaleSetID != 0:
		entityGetter, err = r.store.GetScaleSetByID(ctx, instance.ScaleSetID)
		if err != nil {
			return nil, nil, fmt.Errorf("error fetching scale set: %w", err)
		}
	default:
		return nil, nil, errors.New("instance does not have a pool or scale set")
	}

	entity, err := entityGetter.GetEntity()
	if err != nil {
		return nil, nil, fmt.Errorf("error fetching entity: %w", err)
	}

	// Fetching the entity from the database will populate all fields, including credentials.
	entity, err = r.store.GetForgeEntity(ctx, entity.EntityType, entity.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("error fetching entity: %w", err)
	}

	ghCli, err := github.Client(ctx, entity)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating github client: %w", err)
	}

	scaleSetCli, err := scalesets.NewClient(ghCli)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating scaleset client: %w", err)
	}
	return ghCli, scaleSetCli, nil
}
