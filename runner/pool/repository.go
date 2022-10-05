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
	"strings"
	"sync"

	"garm/config"
	dbCommon "garm/database/common"
	runnerErrors "garm/errors"
	"garm/params"
	"garm/runner/common"
	"garm/util"

	"github.com/google/go-github/v43/github"
	"github.com/pkg/errors"
)

// test that we implement PoolManager
var _ poolHelper = &repository{}

func NewRepositoryPoolManager(ctx context.Context, cfg params.Repository, cfgInternal params.Internal, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	ghc, err := util.GithubClient(ctx, cfgInternal.OAuth2Token)
	if err != nil {
		return nil, errors.Wrap(err, "getting github client")
	}

	helper := &repository{
		cfg:         cfg,
		cfgInternal: cfgInternal,
		ctx:         ctx,
		ghcli:       ghc,
		id:          cfg.ID,
		store:       store,
	}

	repo := &basePool{
		ctx:          ctx,
		store:        store,
		providers:    providers,
		controllerID: cfgInternal.ControllerID,
		quit:         make(chan struct{}),
		done:         make(chan struct{}),
		helper:       helper,
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

func (r *repository) GetRunnerNameFromWorkflow(job params.WorkflowJob) (string, error) {
	workflow, _, err := r.ghcli.GetWorkflowJobByID(r.ctx, job.Repository.Owner.Login, job.Repository.Name, job.WorkflowJob.ID)
	if err != nil {
		return "", errors.Wrap(err, "fetching workflow info")
	}
	if workflow.RunnerName != nil {
		return *workflow.RunnerName, nil
	}
	return "", fmt.Errorf("failed to find runner name from workflow")
}

func (r *repository) UpdateState(param params.UpdatePoolStateParams) error {
	r.mux.Lock()
	defer r.mux.Unlock()

	r.cfg.WebhookSecret = param.WebhookSecret

	ghc, err := util.GithubClient(r.ctx, r.GetGithubToken())
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
	runners, _, err := r.ghcli.ListRunners(r.ctx, r.cfg.Owner, r.cfg.Name, nil)
	if err != nil {
		return nil, errors.Wrap(err, "fetching runners")
	}

	return runners.Runners, nil
}

func (r *repository) FetchTools() ([]*github.RunnerApplicationDownload, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	tools, _, err := r.ghcli.ListRunnerApplicationDownloads(r.ctx, r.cfg.Owner, r.cfg.Name)
	if err != nil {
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
	return fmt.Sprintf("%s/%s/%s", config.GithubBaseURL, r.cfg.Owner, r.cfg.Name)
}

func (r *repository) JwtToken() string {
	return r.cfgInternal.JWTSecret
}

func (r *repository) GetGithubRegistrationToken() (string, error) {
	tk, _, err := r.ghcli.CreateRegistrationToken(r.ctx, r.cfg.Owner, r.cfg.Name)

	if err != nil {
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

func (r *repository) GetCallbackURL() string {
	return r.cfgInternal.InstanceCallbackURL
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
