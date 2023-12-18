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

package pool

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util"

	"github.com/google/go-github/v57/github"
	"github.com/pkg/errors"
)

// test that we implement PoolManager
var _ poolHelper = &organization{}

func NewOrganizationPoolManager(ctx context.Context, cfg params.Organization, cfgInternal params.Internal, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	ghc, _, err := util.GithubClient(ctx, cfgInternal.OAuth2Token, cfgInternal.GithubCredentialsDetails)
	if err != nil {
		return nil, errors.Wrap(err, "getting github client")
	}

	wg := &sync.WaitGroup{}
	keyMuxes := &keyMutex{}

	helper := &organization{
		cfg:         cfg,
		cfgInternal: cfgInternal,
		ctx:         ctx,
		ghcli:       ghc,
		id:          cfg.ID,
		store:       store,
	}

	repo := &basePoolManager{
		ctx:          ctx,
		store:        store,
		providers:    providers,
		controllerID: cfgInternal.ControllerID,
		urls: urls{
			webhookURL:           cfgInternal.BaseWebhookURL,
			callbackURL:          cfgInternal.InstanceCallbackURL,
			metadataURL:          cfgInternal.InstanceMetadataURL,
			controllerWebhookURL: cfgInternal.ControllerWebhookURL,
		},
		quit:         make(chan struct{}),
		helper:       helper,
		credsDetails: cfgInternal.GithubCredentialsDetails,
		wg:           wg,
		keyMux:       keyMuxes,
	}
	return repo, nil
}

type organization struct {
	cfg         params.Organization
	cfgInternal params.Internal
	ctx         context.Context
	ghcli       common.GithubClient
	id          string
	store       dbCommon.Store

	mux sync.Mutex
}

func (r *organization) findRunnerGroupByName(ctx context.Context, name string) (*github.RunnerGroup, error) {
	// TODO(gabriel-samfira): implement caching
	opts := github.ListOrgRunnerGroupOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		runnerGroups, ghResp, err := r.ghcli.ListOrganizationRunnerGroups(r.ctx, r.cfg.Name, &opts)
		if err != nil {
			if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
				return nil, errors.Wrap(runnerErrors.ErrUnauthorized, "fetching runners")
			}
			return nil, errors.Wrap(err, "fetching runners")
		}
		for _, runnerGroup := range runnerGroups.RunnerGroups {
			if runnerGroup.GetName() == name {
				return runnerGroup, nil
			}
		}
		if ghResp.NextPage == 0 {
			break
		}
		opts.Page = ghResp.NextPage
	}

	return nil, errors.Wrap(runnerErrors.ErrNotFound, "runner group not found")
}

func (r *organization) GetJITConfig(ctx context.Context, instance string, pool params.Pool, labels []string) (jitConfigMap map[string]string, runner *github.Runner, err error) {
	var rg int64 = 1
	if pool.GitHubRunnerGroup != "" {
		runnerGroup, err := r.findRunnerGroupByName(ctx, pool.GitHubRunnerGroup)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find runner group: %w", err)
		}
		rg = runnerGroup.GetID()
	}

	req := github.GenerateJITConfigRequest{
		Name:          instance,
		RunnerGroupID: rg,
		Labels:        labels,
		// TODO(gabriel-samfira): Should we make this configurable?
		WorkFolder: github.String("_work"),
	}
	jitConfig, resp, err := r.ghcli.GenerateOrgJITConfig(ctx, r.cfg.Name, &req)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil, nil, fmt.Errorf("failed to get JIT config: %w", err)
		}
		return nil, nil, fmt.Errorf("failed to get JIT config: %w", err)
	}

	runner = jitConfig.GetRunner()
	defer func() {
		if err != nil && runner != nil {
			_, innerErr := r.ghcli.RemoveOrganizationRunner(r.ctx, r.cfg.Name, runner.GetID())
			log.Printf("failed to remove runner: %v", innerErr)
		}
	}()

	decoded, err := base64.StdEncoding.DecodeString(jitConfig.GetEncodedJITConfig())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode JIT config: %w", err)
	}

	var ret map[string]string
	if err := json.Unmarshal(decoded, &ret); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal JIT config: %w", err)
	}

	return ret, runner, nil
}

func (r *organization) GithubCLI() common.GithubClient {
	return r.ghcli
}

func (o *organization) PoolType() params.PoolType {
	return params.OrganizationPool
}

func (r *organization) GetRunnerInfoFromWorkflow(job params.WorkflowJob) (params.RunnerInfo, error) {
	if err := r.ValidateOwner(job); err != nil {
		return params.RunnerInfo{}, errors.Wrap(err, "validating owner")
	}
	workflow, ghResp, err := r.ghcli.GetWorkflowJobByID(r.ctx, job.Organization.Login, job.Repository.Name, job.WorkflowJob.ID)
	if err != nil {
		if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
			return params.RunnerInfo{}, errors.Wrap(runnerErrors.ErrUnauthorized, "fetching workflow info")
		}
		return params.RunnerInfo{}, errors.Wrap(err, "fetching workflow info")
	}

	if workflow.RunnerName != nil {
		return params.RunnerInfo{
			Name:   *workflow.RunnerName,
			Labels: workflow.Labels,
		}, nil
	}
	return params.RunnerInfo{}, fmt.Errorf("failed to find runner name from workflow")
}

func (r *organization) UpdateState(param params.UpdatePoolStateParams) error {
	r.mux.Lock()
	defer r.mux.Unlock()

	r.cfg.WebhookSecret = param.WebhookSecret
	if param.InternalConfig != nil {
		r.cfgInternal = *param.InternalConfig
	}

	ghc, _, err := util.GithubClient(r.ctx, r.GetGithubToken(), r.cfgInternal.GithubCredentialsDetails)
	if err != nil {
		return errors.Wrap(err, "getting github client")
	}
	r.ghcli = ghc
	return nil
}

func (r *organization) GetGithubToken() string {
	return r.cfgInternal.OAuth2Token
}

func (r *organization) GetGithubRunners() ([]*github.Runner, error) {
	opts := github.ListOptions{
		PerPage: 100,
	}

	var allRunners []*github.Runner
	for {
		runners, ghResp, err := r.ghcli.ListOrganizationRunners(r.ctx, r.cfg.Name, &opts)
		if err != nil {
			if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
				return nil, errors.Wrap(runnerErrors.ErrUnauthorized, "fetching runners")
			}
			return nil, errors.Wrap(err, "fetching runners")
		}
		allRunners = append(allRunners, runners.Runners...)
		if ghResp.NextPage == 0 {
			break
		}
		opts.Page = ghResp.NextPage
	}

	return allRunners, nil
}

func (r *organization) FetchTools() ([]commonParams.RunnerApplicationDownload, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	tools, ghResp, err := r.ghcli.ListOrganizationRunnerApplicationDownloads(r.ctx, r.cfg.Name)
	if err != nil {
		if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
			return nil, errors.Wrap(runnerErrors.ErrUnauthorized, "fetching tools")
		}
		return nil, errors.Wrap(err, "fetching runner tools")
	}

	ret := []commonParams.RunnerApplicationDownload{}
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		ret = append(ret, commonParams.RunnerApplicationDownload(*tool))
	}

	return ret, nil
}

func (r *organization) FetchDbInstances() ([]params.Instance, error) {
	return r.store.ListOrgInstances(r.ctx, r.id)
}

func (r *organization) RemoveGithubRunner(runnerID int64) (*github.Response, error) {
	return r.ghcli.RemoveOrganizationRunner(r.ctx, r.cfg.Name, runnerID)
}

func (r *organization) ListPools() ([]params.Pool, error) {
	pools, err := r.store.ListOrgPools(r.ctx, r.id)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}
	return pools, nil
}

func (r *organization) GithubURL() string {
	return fmt.Sprintf("%s/%s", r.cfgInternal.GithubCredentialsDetails.BaseURL, r.cfg.Name)
}

func (r *organization) JwtToken() string {
	return r.cfgInternal.JWTSecret
}

func (r *organization) GetGithubRegistrationToken() (string, error) {
	tk, ghResp, err := r.ghcli.CreateOrganizationRegistrationToken(r.ctx, r.cfg.Name)

	if err != nil {
		if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
			return "", errors.Wrap(runnerErrors.ErrUnauthorized, "fetching token")
		}

		return "", errors.Wrap(err, "creating runner token")
	}
	return *tk.Token, nil
}

func (r *organization) String() string {
	return r.cfg.Name
}

func (r *organization) WebhookSecret() string {
	return r.cfg.WebhookSecret
}

func (r *organization) FindPoolByTags(labels []string) (params.Pool, error) {
	pool, err := r.store.FindOrganizationPoolByTags(r.ctx, r.id, labels)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching suitable pool")
	}
	return pool, nil
}

func (r *organization) GetPoolByID(poolID string) (params.Pool, error) {
	pool, err := r.store.GetOrganizationPool(r.ctx, r.id, poolID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool, nil
}

func (r *organization) ValidateOwner(job params.WorkflowJob) error {
	if !strings.EqualFold(job.Organization.Login, r.cfg.Name) {
		return runnerErrors.NewBadRequestError("job not meant for this pool manager")
	}
	return nil
}

func (r *organization) ID() string {
	return r.id
}

func (r *organization) listHooks(ctx context.Context) ([]*github.Hook, error) {
	opts := github.ListOptions{
		PerPage: 100,
	}
	var allHooks []*github.Hook
	for {
		hooks, ghResp, err := r.ghcli.ListOrgHooks(ctx, r.cfg.Name, &opts)
		if err != nil {
			if ghResp != nil && ghResp.StatusCode == http.StatusNotFound {
				return nil, runnerErrors.NewBadRequestError("organization not found or your PAT does not have access to manage webhooks")
			}
			return nil, errors.Wrap(err, "fetching hooks")
		}
		allHooks = append(allHooks, hooks...)
		if ghResp.NextPage == 0 {
			break
		}
		opts.Page = ghResp.NextPage
	}
	return allHooks, nil
}

func (r *organization) InstallHook(ctx context.Context, req *github.Hook) (params.HookInfo, error) {
	allHooks, err := r.listHooks(ctx)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "listing hooks")
	}

	if err := validateHookRequest(r.cfgInternal.ControllerID, r.cfgInternal.BaseWebhookURL, allHooks, req); err != nil {
		return params.HookInfo{}, errors.Wrap(err, "validating hook request")
	}

	hook, _, err := r.ghcli.CreateOrgHook(ctx, r.cfg.Name, req)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "creating organization hook")
	}

	if _, err := r.ghcli.PingOrgHook(ctx, r.cfg.Name, hook.GetID()); err != nil {
		log.Printf("failed to ping hook %d: %v", *hook.ID, err)
	}

	return hookToParamsHookInfo(hook), nil
}

func (r *organization) UninstallHook(ctx context.Context, url string) error {
	allHooks, err := r.listHooks(ctx)
	if err != nil {
		return errors.Wrap(err, "listing hooks")
	}

	for _, hook := range allHooks {
		if hook.Config["url"] == url {
			_, err = r.ghcli.DeleteOrgHook(ctx, r.cfg.Name, hook.GetID())
			if err != nil {
				return errors.Wrap(err, "deleting hook")
			}
			return nil
		}
	}
	return nil
}

func (r *organization) GetHookInfo(ctx context.Context) (params.HookInfo, error) {
	allHooks, err := r.listHooks(ctx)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "listing hooks")
	}

	for _, hook := range allHooks {
		hookInfo := hookToParamsHookInfo(hook)
		if strings.EqualFold(hookInfo.URL, r.cfgInternal.ControllerWebhookURL) {
			return hookInfo, nil
		}
	}

	return params.HookInfo{}, runnerErrors.NewNotFoundError("hook not found")
}
