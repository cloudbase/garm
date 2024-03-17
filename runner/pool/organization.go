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
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/google/go-github/v57/github"
	"github.com/pkg/errors"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util"
)

// test that we implement PoolManager
var _ poolHelper = &organization{}

func NewOrganizationPoolManager(ctx context.Context, cfg params.Organization, cfgInternal params.Internal, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	ctx = util.WithContext(ctx, slog.Any("pool_mgr", cfg.Name), slog.Any("pool_type", params.GithubEntityTypeOrganization))
	ghc, _, err := util.GithubClient(ctx, cfgInternal.GithubCredentialsDetails)
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
		ctx: ctx,
		entity: params.GithubEntity{
			Name:       cfg.Name,
			EntityType: params.GithubEntityTypeOrganization,
		},
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

func (o *organization) PoolBalancerType() params.PoolBalancerType {
	if o.cfgInternal.PoolBalancerType == "" {
		return params.PoolBalancerTypeRoundRobin
	}
	return o.cfgInternal.PoolBalancerType
}

func (o *organization) findRunnerGroupByName(name string) (*github.RunnerGroup, error) {
	// nolint:golangci-lint,godox
	// TODO(gabriel-samfira): implement caching
	opts := github.ListOrgRunnerGroupOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		metrics.GithubOperationCount.WithLabelValues(
			"ListOrganizationRunnerGroups",       // label: operation
			params.MetricsLabelOrganizationScope, // label: scope
		).Inc()
		runnerGroups, ghResp, err := o.ghcli.ListOrganizationRunnerGroups(o.ctx, o.cfg.Name, &opts)
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"ListOrganizationRunnerGroups",       // label: operation
				params.MetricsLabelOrganizationScope, // label: scope
			).Inc()
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

func (o *organization) GetJITConfig(ctx context.Context, instance string, pool params.Pool, labels []string) (jitConfigMap map[string]string, runner *github.Runner, err error) {
	var rg int64 = 1
	if pool.GitHubRunnerGroup != "" {
		runnerGroup, err := o.findRunnerGroupByName(pool.GitHubRunnerGroup)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find runner group: %w", err)
		}
		rg = runnerGroup.GetID()
	}

	req := github.GenerateJITConfigRequest{
		Name:          instance,
		RunnerGroupID: rg,
		Labels:        labels,
		// nolint:golangci-lint,godox
		// TODO(gabriel-samfira): Should we make this configurable?
		WorkFolder: github.String("_work"),
	}
	metrics.GithubOperationCount.WithLabelValues(
		"GenerateOrgJITConfig",               // label: operation
		params.MetricsLabelOrganizationScope, // label: scope
	).Inc()
	jitConfig, resp, err := o.ghcli.GenerateOrgJITConfig(ctx, o.cfg.Name, &req)
	if err != nil {
		metrics.GithubOperationFailedCount.WithLabelValues(
			"GenerateOrgJITConfig",               // label: operation
			params.MetricsLabelOrganizationScope, // label: scope
		).Inc()
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil, nil, fmt.Errorf("failed to get JIT config: %w", err)
		}
		return nil, nil, fmt.Errorf("failed to get JIT config: %w", err)
	}

	runner = jitConfig.GetRunner()
	defer func() {
		if err != nil && runner != nil {
			metrics.GithubOperationCount.WithLabelValues(
				"RemoveOrganizationRunner",           // label: operation
				params.MetricsLabelOrganizationScope, // label: scope
			).Inc()
			_, innerErr := o.ghcli.RemoveOrganizationRunner(o.ctx, o.cfg.Name, runner.GetID())
			if innerErr != nil {
				metrics.GithubOperationFailedCount.WithLabelValues(
					"RemoveOrganizationRunner",           // label: operation
					params.MetricsLabelOrganizationScope, // label: scope
				).Inc()
			}
			slog.With(slog.Any("error", innerErr)).ErrorContext(
				ctx, "failed to remove runner",
				"runner_id", runner.GetID(), "organization", o.cfg.Name)
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

func (o *organization) GetRunnerInfoFromWorkflow(job params.WorkflowJob) (params.RunnerInfo, error) {
	if err := o.ValidateOwner(job); err != nil {
		return params.RunnerInfo{}, errors.Wrap(err, "validating owner")
	}
	metrics.GithubOperationCount.WithLabelValues(
		"GetWorkflowJobByID",                 // label: operation
		params.MetricsLabelOrganizationScope, // label: scope
	).Inc()
	workflow, ghResp, err := o.ghcli.GetWorkflowJobByID(o.ctx, job.Organization.Login, job.Repository.Name, job.WorkflowJob.ID)
	if err != nil {
		metrics.GithubOperationFailedCount.WithLabelValues(
			"GetWorkflowJobByID",                 // label: operation
			params.MetricsLabelOrganizationScope, // label: scope
		).Inc()
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

func (o *organization) UpdateState(param params.UpdatePoolStateParams) error {
	o.mux.Lock()
	defer o.mux.Unlock()

	o.cfg.WebhookSecret = param.WebhookSecret
	if param.InternalConfig != nil {
		o.cfgInternal = *param.InternalConfig
	}

	ghc, _, err := util.GithubClient(o.ctx, o.cfgInternal.GithubCredentialsDetails)
	if err != nil {
		return errors.Wrap(err, "getting github client")
	}
	o.ghcli = ghc
	return nil
}

func (o *organization) GetGithubRunners() ([]*github.Runner, error) {
	opts := github.ListOptions{
		PerPage: 100,
	}

	var allRunners []*github.Runner
	for {
		metrics.GithubOperationCount.WithLabelValues(
			"ListOrganizationRunners",            // label: operation
			params.MetricsLabelOrganizationScope, // label: scope
		).Inc()
		runners, ghResp, err := o.ghcli.ListOrganizationRunners(o.ctx, o.cfg.Name, &opts)
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"ListOrganizationRunners",            // label: operation
				params.MetricsLabelOrganizationScope, // label: scope
			).Inc()
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

func (o *organization) FetchTools() ([]commonParams.RunnerApplicationDownload, error) {
	o.mux.Lock()
	defer o.mux.Unlock()
	metrics.GithubOperationCount.WithLabelValues(
		"ListOrganizationRunnerApplicationDownloads", // label: operation
		params.MetricsLabelOrganizationScope,         // label: scope
	).Inc()
	tools, ghResp, err := o.ghcli.ListOrganizationRunnerApplicationDownloads(o.ctx, o.cfg.Name)
	if err != nil {
		metrics.GithubOperationFailedCount.WithLabelValues(
			"ListOrganizationRunnerApplicationDownloads", // label: operation
			params.MetricsLabelOrganizationScope,         // label: scope
		).Inc()
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

func (o *organization) FetchDbInstances() ([]params.Instance, error) {
	return o.store.ListOrgInstances(o.ctx, o.id)
}

func (o *organization) RemoveGithubRunner(runnerID int64) (*github.Response, error) {
	metrics.GithubOperationCount.WithLabelValues(
		"RemoveRunner",                       // label: operation
		params.MetricsLabelOrganizationScope, // label: scope
	).Inc()

	ghResp, err := o.ghcli.RemoveOrganizationRunner(o.ctx, o.cfg.Name, runnerID)
	if err != nil {
		metrics.GithubOperationFailedCount.WithLabelValues(
			"RemoveRunner",                       // label: operation
			params.MetricsLabelOrganizationScope, // label: scope
		).Inc()
		return nil, err
	}

	return ghResp, nil
}

func (o *organization) ListPools() ([]params.Pool, error) {
	pools, err := o.store.ListOrgPools(o.ctx, o.id)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}
	return pools, nil
}

func (o *organization) GithubURL() string {
	return fmt.Sprintf("%s/%s", o.cfgInternal.GithubCredentialsDetails.BaseURL, o.cfg.Name)
}

func (o *organization) JwtToken() string {
	return o.cfgInternal.JWTSecret
}

func (o *organization) GetGithubRegistrationToken() (string, error) {
	metrics.GithubOperationCount.WithLabelValues(
		"CreateOrganizationRegistrationToken", // label: operation
		params.MetricsLabelOrganizationScope,  // label: scope
	).Inc()
	tk, ghResp, err := o.ghcli.CreateOrganizationRegistrationToken(o.ctx, o.cfg.Name)
	if err != nil {
		metrics.GithubOperationFailedCount.WithLabelValues(
			"CreateOrganizationRegistrationToken", // label: operation
			params.MetricsLabelOrganizationScope,  // label: scope
		).Inc()
		if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
			return "", errors.Wrap(runnerErrors.ErrUnauthorized, "fetching token")
		}

		return "", errors.Wrap(err, "creating runner token")
	}
	return *tk.Token, nil
}

func (o *organization) String() string {
	return o.cfg.Name
}

func (o *organization) WebhookSecret() string {
	return o.cfg.WebhookSecret
}

func (o *organization) GetPoolByID(poolID string) (params.Pool, error) {
	pool, err := o.store.GetOrganizationPool(o.ctx, o.id, poolID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool, nil
}

func (o *organization) ValidateOwner(job params.WorkflowJob) error {
	if !strings.EqualFold(job.Organization.Login, o.cfg.Name) {
		return runnerErrors.NewBadRequestError("job not meant for this pool manager")
	}
	return nil
}

func (o *organization) ID() string {
	return o.id
}

func (o *organization) listHooks(ctx context.Context) ([]*github.Hook, error) {
	opts := github.ListOptions{
		PerPage: 100,
	}
	var allHooks []*github.Hook
	for {
		metrics.GithubOperationCount.WithLabelValues(
			"ListOrgHooks",                       // label: operation
			params.MetricsLabelOrganizationScope, // label: scope
		).Inc()
		hooks, ghResp, err := o.ghcli.ListOrgHooks(ctx, o.cfg.Name, &opts)
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"ListOrgHooks",                       // label: operation
				params.MetricsLabelOrganizationScope, // label: scope
			).Inc()
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

func (o *organization) InstallHook(ctx context.Context, req *github.Hook) (params.HookInfo, error) {
	allHooks, err := o.listHooks(ctx)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "listing hooks")
	}

	if err := validateHookRequest(o.cfgInternal.ControllerID, o.cfgInternal.BaseWebhookURL, allHooks, req); err != nil {
		return params.HookInfo{}, errors.Wrap(err, "validating hook request")
	}

	metrics.GithubOperationCount.WithLabelValues(
		"CreateOrgHook",                      // label: operation
		params.MetricsLabelOrganizationScope, // label: scope
	).Inc()

	hook, _, err := o.ghcli.CreateOrgHook(ctx, o.cfg.Name, req)
	if err != nil {
		metrics.GithubOperationFailedCount.WithLabelValues(
			"CreateOrgHook",                      // label: operation
			params.MetricsLabelOrganizationScope, // label: scope
		).Inc()
		return params.HookInfo{}, errors.Wrap(err, "creating organization hook")
	}

	metrics.GithubOperationCount.WithLabelValues(
		"PingOrgHook",                        // label: operation
		params.MetricsLabelOrganizationScope, // label: scope
	).Inc()

	if _, err := o.ghcli.PingOrgHook(ctx, o.cfg.Name, hook.GetID()); err != nil {
		metrics.GithubOperationFailedCount.WithLabelValues(
			"PingOrgHook",                        // label: operation
			params.MetricsLabelOrganizationScope, // label: scope
		).Inc()
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to ping hook", "hook_id", hook.GetID())
	}

	return hookToParamsHookInfo(hook), nil
}

func (o *organization) UninstallHook(ctx context.Context, url string) error {
	allHooks, err := o.listHooks(ctx)
	if err != nil {
		return errors.Wrap(err, "listing hooks")
	}

	for _, hook := range allHooks {
		if hook.Config["url"] == url {
			metrics.GithubOperationCount.WithLabelValues(
				"DeleteOrgHook",                      // label: operation
				params.MetricsLabelOrganizationScope, // label: scope
			).Inc()
			_, err = o.ghcli.DeleteOrgHook(ctx, o.cfg.Name, hook.GetID())
			if err != nil {
				metrics.GithubOperationFailedCount.WithLabelValues(
					"DeleteOrgHook",                      // label: operation
					params.MetricsLabelOrganizationScope, // label: scope
				).Inc()
				return errors.Wrap(err, "deleting hook")
			}
			return nil
		}
	}
	return nil
}

func (o *organization) GetHookInfo(ctx context.Context) (params.HookInfo, error) {
	allHooks, err := o.listHooks(ctx)
	if err != nil {
		return params.HookInfo{}, errors.Wrap(err, "listing hooks")
	}

	for _, hook := range allHooks {
		hookInfo := hookToParamsHookInfo(hook)
		if strings.EqualFold(hookInfo.URL, o.cfgInternal.ControllerWebhookURL) {
			return hookInfo, nil
		}
	}

	return params.HookInfo{}, runnerErrors.NewNotFoundError("hook not found")
}
