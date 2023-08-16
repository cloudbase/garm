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
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util"

	"github.com/google/go-github/v53/github"
	"github.com/pkg/errors"
)

// test that we implement PoolManager
var _ poolHelper = &repository{}

func NewRepositoryPoolManager(ctx context.Context, cfg params.Repository, cfgInternal params.Internal, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	ghc, _, err := util.GithubClient(ctx, cfgInternal.OAuth2Token, cfgInternal.GithubCredentialsDetails)
	if err != nil {
		return nil, errors.Wrap(err, "getting github client")
	}

	wg := &sync.WaitGroup{}
	keyMuxes := &keyMutex{}

	helper := &repository{
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

var _ poolHelper = &repository{}

type repository struct {
	cfg         params.Repository
	cfgInternal params.Internal
	ctx         context.Context
	ghcli       common.GithubClient
	id          string
	store       dbCommon.Store

	mux sync.Mutex
}

func (r *repository) GithubCLI() common.GithubClient {
	return r.ghcli
}

func (r *repository) PoolType() params.PoolType {
	return params.RepositoryPool
}

func (r *repository) GetRunnerInfoFromWorkflow(job params.WorkflowJob) (params.RunnerInfo, error) {
	if err := r.ValidateOwner(job); err != nil {
		return params.RunnerInfo{}, errors.Wrap(err, "validating owner")
	}
	workflow, ghResp, err := r.ghcli.GetWorkflowJobByID(r.ctx, job.Repository.Owner.Login, job.Repository.Name, job.WorkflowJob.ID)
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

func (r *repository) UpdateState(param params.UpdatePoolStateParams) error {
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

func (r *repository) GetGithubToken() string {
	return r.cfgInternal.OAuth2Token
}

func (r *repository) GetGithubRunners() ([]*github.Runner, error) {
	opts := github.ListOptions{
		PerPage: 100,
	}

	var allRunners []*github.Runner
	for {
		runners, ghResp, err := r.ghcli.ListRunners(r.ctx, r.cfg.Owner, r.cfg.Name, &opts)
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

func (r *repository) FetchTools() ([]*github.RunnerApplicationDownload, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	tools, ghResp, err := r.ghcli.ListRunnerApplicationDownloads(r.ctx, r.cfg.Owner, r.cfg.Name)
	if err != nil {
		if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
			return nil, errors.Wrap(runnerErrors.ErrUnauthorized, "fetching tools")
		}
		return nil, errors.Wrap(err, "fetching runner tools")
	}

	return tools, nil
}

func (r *repository) FetchDbInstances() ([]params.Instance, error) {
	return r.store.ListRepoInstances(r.ctx, r.id)
}

func (r *repository) RemoveGithubRunner(runnerID int64) (*github.Response, error) {
	return r.ghcli.RemoveRunner(r.ctx, r.cfg.Owner, r.cfg.Name, runnerID)
}

func (r *repository) ListPools() ([]params.Pool, error) {
	pools, err := r.store.ListRepoPools(r.ctx, r.id)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}
	return pools, nil
}

func (r *repository) GithubURL() string {
	return fmt.Sprintf("%s/%s/%s", r.cfgInternal.GithubCredentialsDetails.BaseURL, r.cfg.Owner, r.cfg.Name)
}

func (r *repository) JwtToken() string {
	return r.cfgInternal.JWTSecret
}

func (r *repository) GetGithubRegistrationToken() (string, error) {
	tk, ghResp, err := r.ghcli.CreateRegistrationToken(r.ctx, r.cfg.Owner, r.cfg.Name)

	if err != nil {
		if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
			return "", errors.Wrap(runnerErrors.ErrUnauthorized, "fetching token")
		}
		return "", errors.Wrap(err, "creating runner token")
	}
	return *tk.Token, nil
}

func (r *repository) String() string {
	return fmt.Sprintf("%s/%s", r.cfg.Owner, r.cfg.Name)
}

func (r *repository) WebhookSecret() string {
	return r.cfg.WebhookSecret
}

func (r *repository) FindPoolByTags(labels []string) (params.Pool, error) {
	pool, err := r.store.FindRepositoryPoolByTags(r.ctx, r.id, labels)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching suitable pool")
	}
	return pool, nil
}

func (r *repository) GetPoolByID(poolID string) (params.Pool, error) {
	pool, err := r.store.GetRepositoryPool(r.ctx, r.id, poolID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool, nil
}

func (r *repository) ValidateOwner(job params.WorkflowJob) error {
	if !strings.EqualFold(job.Repository.Name, r.cfg.Name) || !strings.EqualFold(job.Repository.Owner.Login, r.cfg.Owner) {
		return runnerErrors.NewBadRequestError("job not meant for this pool manager")
	}
	return nil
}

func (r *repository) ID() string {
	return r.id
}

func (r *repository) listHooks(ctx context.Context) ([]*github.Hook, error) {
	opts := github.ListOptions{
		PerPage: 100,
	}
	var allHooks []*github.Hook
	for {
		hooks, ghResp, err := r.ghcli.ListRepoHooks(ctx, r.cfg.Owner, r.cfg.Name, &opts)
		if err != nil {
			if ghResp != nil && ghResp.StatusCode == http.StatusNotFound {
				return nil, runnerErrors.NewBadRequestError("repository not found or your PAT does not have access to manage webhooks")
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

func (r *repository) InstallHook(ctx context.Context, req *github.Hook) (params.HookInfo, error) {
	allHooks, err := r.listHooks(ctx)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "listing hooks")
	}

	if err := validateHookRequest(r.cfgInternal.ControllerID, r.cfgInternal.BaseWebhookURL, allHooks, req); err != nil {
		return params.HookInfo{}, errors.Wrap(err, "validating hook request")
	}

	hook, _, err := r.ghcli.CreateRepoHook(ctx, r.cfg.Owner, r.cfg.Name, req)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "creating repository hook")
	}

	if _, err := r.ghcli.PingRepoHook(ctx, r.cfg.Owner, r.cfg.Name, hook.GetID()); err != nil {
		log.Printf("failed to ping hook %d: %v", hook.GetID(), err)
	}

	return hookToParamsHookInfo(hook), nil
}

func (r *repository) UninstallHook(ctx context.Context, url string) error {
	allHooks, err := r.listHooks(ctx)
	if err != nil {
		return errors.Wrap(err, "listing hooks")
	}

	for _, hook := range allHooks {
		if hook.Config["url"] == url {
			_, err = r.ghcli.DeleteRepoHook(ctx, r.cfg.Owner, r.cfg.Name, hook.GetID())
			if err != nil {
				return errors.Wrap(err, "deleting hook")
			}
			return nil
		}
	}
	return nil
}

func (r *repository) GetHookInfo(ctx context.Context) (params.HookInfo, error) {
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
