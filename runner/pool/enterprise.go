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
var _ poolHelper = &enterprise{}

func NewEnterprisePoolManager(ctx context.Context, cfg params.Enterprise, cfgInternal params.Internal, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	ctx = util.WithContext(ctx, slog.Any("pool_mgr", cfg.Name), slog.Any("pool_type", params.EnterprisePool))
	ghc, ghEnterpriseClient, err := util.GithubClient(ctx, cfgInternal.OAuth2Token, cfgInternal.GithubCredentialsDetails)
	if err != nil {
		return nil, errors.Wrap(err, "getting github client")
	}

	wg := &sync.WaitGroup{}
	keyMuxes := &keyMutex{}

	helper := &enterprise{
		cfg:              cfg,
		cfgInternal:      cfgInternal,
		ctx:              ctx,
		ghcli:            ghc,
		ghcEnterpriseCli: ghEnterpriseClient,
		id:               cfg.ID,
		store:            store,
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

type enterprise struct {
	cfg              params.Enterprise
	cfgInternal      params.Internal
	ctx              context.Context
	ghcli            common.GithubClient
	ghcEnterpriseCli common.GithubEnterpriseClient
	id               string
	store            dbCommon.Store

	mux sync.Mutex
}

func (r *enterprise) findRunnerGroupByName(ctx context.Context, name string) (*github.EnterpriseRunnerGroup, error) {
	// TODO(gabriel-samfira): implement caching
	opts := github.ListEnterpriseRunnerGroupOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		metrics.GithubOperationCount.WithLabelValues(
			"ListOrganizationRunnerGroups", // label: operation
			metricsLabelEnterpriseScope,    // label: scope
		).Inc()
		runnerGroups, ghResp, err := r.ghcEnterpriseCli.ListRunnerGroups(r.ctx, r.cfg.Name, &opts)
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"ListOrganizationRunnerGroups", // label: operation
				metricsLabelEnterpriseScope,    // label: scope
			).Inc()
			if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
				return nil, errors.Wrap(runnerErrors.ErrUnauthorized, "fetching runners")
			}
			return nil, errors.Wrap(err, "fetching runners")
		}
		for _, runnerGroup := range runnerGroups.RunnerGroups {
			if runnerGroup.Name != nil && *runnerGroup.Name == name {
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

func (r *enterprise) GetJITConfig(ctx context.Context, instance string, pool params.Pool, labels []string) (jitConfigMap map[string]string, runner *github.Runner, err error) {
	var rg int64 = 1
	if pool.GitHubRunnerGroup != "" {
		runnerGroup, err := r.findRunnerGroupByName(ctx, pool.GitHubRunnerGroup)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find runner group: %w", err)
		}
		rg = *runnerGroup.ID
	}

	req := github.GenerateJITConfigRequest{
		Name:          instance,
		RunnerGroupID: rg,
		Labels:        labels,
		// TODO(gabriel-samfira): Should we make this configurable?
		WorkFolder: github.String("_work"),
	}
	metrics.GithubOperationCount.WithLabelValues(
		"GenerateEnterpriseJITConfig", // label: operation
		metricsLabelEnterpriseScope,   // label: scope
	).Inc()
	jitConfig, resp, err := r.ghcEnterpriseCli.GenerateEnterpriseJITConfig(ctx, r.cfg.Name, &req)
	if err != nil {
		metrics.GithubOperationFailedCount.WithLabelValues(
			"GenerateEnterpriseJITConfig", // label: operation
			metricsLabelEnterpriseScope,   // label: scope
		).Inc()
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil, nil, fmt.Errorf("failed to get JIT config: %w", err)
		}
		return nil, nil, fmt.Errorf("failed to get JIT config: %w", err)
	}

	runner = jitConfig.Runner
	defer func() {
		if err != nil && runner != nil {
			metrics.GithubOperationCount.WithLabelValues(
				"RemoveRunner",              // label: operation
				metricsLabelEnterpriseScope, // label: scope
			).Inc()
			_, innerErr := r.ghcEnterpriseCli.RemoveRunner(r.ctx, r.cfg.Name, runner.GetID())
			if innerErr != nil {
				metrics.GithubOperationFailedCount.WithLabelValues(
					"RemoveRunner",              // label: operation
					metricsLabelEnterpriseScope, // label: scope
				).Inc()
			}
			slog.With(slog.Any("error", innerErr)).ErrorContext(
				ctx, "failed to remove runner",
				"runner_id", runner.GetID(), "organization", r.cfg.Name)
		}
	}()

	decoded, err := base64.StdEncoding.DecodeString(*jitConfig.EncodedJITConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode JIT config: %w", err)
	}

	var ret map[string]string
	if err := json.Unmarshal(decoded, &ret); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal JIT config: %w", err)
	}

	return ret, jitConfig.Runner, nil
}

func (r *enterprise) GithubCLI() common.GithubClient {
	return r.ghcli
}

func (e *enterprise) PoolType() params.PoolType {
	return params.EnterprisePool
}

func (r *enterprise) GetRunnerInfoFromWorkflow(job params.WorkflowJob) (params.RunnerInfo, error) {
	if err := r.ValidateOwner(job); err != nil {
		return params.RunnerInfo{}, errors.Wrap(err, "validating owner")
	}
	metrics.GithubOperationCount.WithLabelValues(
		"GetWorkflowJobByID",        // label: operation
		metricsLabelEnterpriseScope, // label: scope
	).Inc()
	workflow, ghResp, err := r.ghcli.GetWorkflowJobByID(r.ctx, job.Repository.Owner.Login, job.Repository.Name, job.WorkflowJob.ID)
	if err != nil {
		metrics.GithubOperationFailedCount.WithLabelValues(
			"GetWorkflowJobByID",        // label: operation
			metricsLabelEnterpriseScope, // label: scope
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

func (r *enterprise) UpdateState(param params.UpdatePoolStateParams) error {
	r.mux.Lock()
	defer r.mux.Unlock()

	r.cfg.WebhookSecret = param.WebhookSecret
	if param.InternalConfig != nil {
		r.cfgInternal = *param.InternalConfig
	}

	ghc, ghcEnterprise, err := util.GithubClient(r.ctx, r.GetGithubToken(), r.cfgInternal.GithubCredentialsDetails)
	if err != nil {
		return errors.Wrap(err, "getting github client")
	}
	r.ghcli = ghc
	r.ghcEnterpriseCli = ghcEnterprise
	return nil
}

func (r *enterprise) GetGithubToken() string {
	return r.cfgInternal.OAuth2Token
}

func (r *enterprise) GetGithubRunners() ([]*github.Runner, error) {
	opts := github.ListOptions{
		PerPage: 100,
	}

	var allRunners []*github.Runner
	for {
		metrics.GithubOperationCount.WithLabelValues(
			"ListRunners",               // label: operation
			metricsLabelEnterpriseScope, // label: scope
		).Inc()
		runners, ghResp, err := r.ghcEnterpriseCli.ListRunners(r.ctx, r.cfg.Name, &opts)
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"ListRunners",               // label: operation
				metricsLabelEnterpriseScope, // label: scope
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

func (r *enterprise) FetchTools() ([]commonParams.RunnerApplicationDownload, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	metrics.GithubOperationCount.WithLabelValues(
		"ListRunnerApplicationDownloads", // label: operation
		metricsLabelEnterpriseScope,      // label: scope
	).Inc()
	tools, ghResp, err := r.ghcEnterpriseCli.ListRunnerApplicationDownloads(r.ctx, r.cfg.Name)
	if err != nil {
		metrics.GithubOperationFailedCount.WithLabelValues(
			"ListRunnerApplicationDownloads", // label: operation
			metricsLabelEnterpriseScope,      // label: scope
		).Inc()
		if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
			return nil, errors.Wrap(runnerErrors.ErrUnauthorized, "fetching runners")
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

func (r *enterprise) FetchDbInstances() ([]params.Instance, error) {
	return r.store.ListEnterpriseInstances(r.ctx, r.id)
}

func (r *enterprise) RemoveGithubRunner(runnerID int64) (*github.Response, error) {
	metrics.GithubOperationCount.WithLabelValues(
		"RemoveRunner",              // label: operation
		metricsLabelEnterpriseScope, // label: scope
	).Inc()
	ghResp, err := r.ghcEnterpriseCli.RemoveRunner(r.ctx, r.cfg.Name, runnerID)
	if err != nil {
		metrics.GithubOperationFailedCount.WithLabelValues(
			"RemoveRunner",              // label: operation
			metricsLabelEnterpriseScope, // label: scope
		).Inc()
		return nil, err
	}
	return ghResp, nil
}

func (r *enterprise) ListPools() ([]params.Pool, error) {
	pools, err := r.store.ListEnterprisePools(r.ctx, r.id)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}
	return pools, nil
}

func (r *enterprise) GithubURL() string {
	return fmt.Sprintf("%s/enterprises/%s", r.cfgInternal.GithubCredentialsDetails.BaseURL, r.cfg.Name)
}

func (r *enterprise) JwtToken() string {
	return r.cfgInternal.JWTSecret
}

func (r *enterprise) GetGithubRegistrationToken() (string, error) {
	metrics.GithubOperationCount.WithLabelValues(
		"CreateRegistrationToken",   // label: operation
		metricsLabelEnterpriseScope, // label: scope
	).Inc()

	tk, ghResp, err := r.ghcEnterpriseCli.CreateRegistrationToken(r.ctx, r.cfg.Name)
	if err != nil {
		metrics.GithubOperationFailedCount.WithLabelValues(
			"CreateRegistrationToken",   // label: operation
			metricsLabelEnterpriseScope, // label: scope
		).Inc()
		if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
			return "", errors.Wrap(runnerErrors.ErrUnauthorized, "fetching registration token")
		}
		return "", errors.Wrap(err, "creating runner token")
	}
	return *tk.Token, nil
}

func (r *enterprise) String() string {
	return r.cfg.Name
}

func (r *enterprise) WebhookSecret() string {
	return r.cfg.WebhookSecret
}

func (r *enterprise) FindPoolByTags(labels []string) (params.Pool, error) {
	pool, err := r.store.FindEnterprisePoolByTags(r.ctx, r.id, labels)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching suitable pool")
	}
	return pool, nil
}

func (r *enterprise) GetPoolByID(poolID string) (params.Pool, error) {
	pool, err := r.store.GetEnterprisePool(r.ctx, r.id, poolID)
	if err != nil {
		return params.Pool{}, errors.Wrap(err, "fetching pool")
	}
	return pool, nil
}

func (r *enterprise) ValidateOwner(job params.WorkflowJob) error {
	if !strings.EqualFold(job.Enterprise.Slug, r.cfg.Name) {
		return runnerErrors.NewBadRequestError("job not meant for this pool manager")
	}
	return nil
}

func (r *enterprise) ID() string {
	return r.id
}

func (r *enterprise) InstallHook(ctx context.Context, req *github.Hook) (params.HookInfo, error) {
	return params.HookInfo{}, fmt.Errorf("not implemented")
}

func (r *enterprise) UninstallHook(ctx context.Context, url string) error {
	return fmt.Errorf("not implemented")
}

func (r *enterprise) GetHookInfo(ctx context.Context) (params.HookInfo, error) {
	return params.HookInfo{}, fmt.Errorf("not implemented")
}
