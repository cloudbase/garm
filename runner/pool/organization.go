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

func (r *organization) FetchTools() ([]*github.RunnerApplicationDownload, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	tools, ghResp, err := r.ghcli.ListOrganizationRunnerApplicationDownloads(r.ctx, r.cfg.Name)
	if err != nil {
		if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
			return nil, errors.Wrap(runnerErrors.ErrUnauthorized, "fetching tools")
		}
		return nil, errors.Wrap(err, "fetching runner tools")
	}

	return tools, nil
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
				return nil, errors.Wrap(runnerErrors.ErrNotFound, "fetching hooks")
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

func (r *organization) InstallHook(ctx context.Context, req *github.Hook) error {
	allHooks, err := r.listHooks(ctx)
	if err != nil {
		return errors.Wrap(err, "listing hooks")
	}

	for _, hook := range allHooks {
		if hook.Config["url"] == req.Config["url"] {
			return fmt.Errorf("hook already installed: %w", runnerErrors.ErrBadRequest)
		}
	}

	_, _, err = r.ghcli.CreateOrgHook(ctx, r.cfg.Name, req)
	if err != nil {
		return errors.Wrap(err, "creating organization hook")
	}
	return nil
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
