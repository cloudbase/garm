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
var _ poolHelper = &organization{}

func NewOrganizationPoolManager(ctx context.Context, cfg params.Organization, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	ghc, err := util.GithubClient(ctx, cfg.Internal.OAuth2Token)
	if err != nil {
		return nil, errors.Wrap(err, "getting github client")
	}

	helper := &organization{
		cfg:   cfg,
		ctx:   ctx,
		ghcli: ghc,
		id:    cfg.ID,
		store: store,
	}

	repo := &basePool{
		ctx:          ctx,
		store:        store,
		providers:    providers,
		controllerID: cfg.Internal.ControllerID,
		quit:         make(chan struct{}),
		done:         make(chan struct{}),
		helper:       helper,
	}
	return repo, nil
}

type organization struct {
	cfg   params.Organization
	ctx   context.Context
	ghcli common.GithubClient
	id    string
	store dbCommon.Store

	mux sync.Mutex
}

func (r *organization) UpdateState(param params.UpdatePoolStateParams) error {
	r.mux.Lock()
	defer r.mux.Unlock()

	r.cfg.WebhookSecret = param.WebhookSecret
	r.cfg.Internal = param.Internal

	ghc, err := util.GithubClient(r.ctx, r.GetGithubToken())
	if err != nil {
		return errors.Wrap(err, "getting github client")
	}
	r.ghcli = ghc
	return nil
}

func (r *organization) GetGithubToken() string {
	return r.cfg.Internal.OAuth2Token
}

func (r *organization) GetGithubRunners() ([]*github.Runner, error) {
	runners, _, err := r.ghcli.ListOrganizationRunners(r.ctx, r.cfg.Name, nil)
	if err != nil {
		return nil, errors.Wrap(err, "fetching runners")
	}

	return runners.Runners, nil
}

func (r *organization) FetchTools() ([]*github.RunnerApplicationDownload, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	tools, _, err := r.ghcli.ListOrganizationRunnerApplicationDownloads(r.ctx, r.cfg.Name)
	if err != nil {
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
	return fmt.Sprintf("%s/%s", config.GithubBaseURL, r.cfg.Name)
}

func (r *organization) JwtToken() string {
	return r.cfg.Internal.JWTSecret
}

func (r *organization) GetGithubRegistrationToken() (string, error) {
	tk, _, err := r.ghcli.CreateOrganizationRegistrationToken(r.ctx, r.cfg.Name)

	if err != nil {
		return "", errors.Wrap(err, "creating runner token")
	}
	return *tk.Token, nil
}

func (r *organization) String() string {
	return fmt.Sprintf("%s", r.cfg.Name)
}

func (r *organization) WebhookSecret() string {
	return r.cfg.WebhookSecret
}

func (r *organization) GetCallbackURL() string {
	return r.cfg.Internal.InstanceCallbackURL
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
	if job.Organization.Login != r.cfg.Name {
		return runnerErrors.NewBadRequestError("job not meant for this pool manager")
	}
	return nil
}

func (r *organization) ID() string {
	return r.id
}
