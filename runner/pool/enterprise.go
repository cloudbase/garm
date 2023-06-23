package pool

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	dbCommon "github.com/cloudbase/garm/database/common"
	runnerErrors "github.com/cloudbase/garm/errors"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util"

	"github.com/google/go-github/v53/github"
	"github.com/pkg/errors"
)

// test that we implement PoolManager
var _ poolHelper = &enterprise{}

func NewEnterprisePoolManager(ctx context.Context, cfg params.Enterprise, cfgInternal params.Internal, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	ghc, ghEnterpriseClient, err := util.GithubClient(ctx, cfgInternal.OAuth2Token, cfgInternal.GithubCredentialsDetails)
	if err != nil {
		return nil, errors.Wrap(err, "getting github client")
	}

	wg := &sync.WaitGroup{}

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
		quit:         make(chan struct{}),
		helper:       helper,
		credsDetails: cfgInternal.GithubCredentialsDetails,
		wg:           wg,
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

func (r *enterprise) GetRunnerInfoFromWorkflow(job params.WorkflowJob) (params.RunnerInfo, error) {
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

func (r *enterprise) UpdateState(param params.UpdatePoolStateParams) error {
	r.mux.Lock()
	defer r.mux.Unlock()

	r.cfg.WebhookSecret = param.WebhookSecret

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
		runners, ghResp, err := r.ghcEnterpriseCli.ListRunners(r.ctx, r.cfg.Name, &opts)
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

func (r *enterprise) FetchTools() ([]*github.RunnerApplicationDownload, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	tools, ghResp, err := r.ghcEnterpriseCli.ListRunnerApplicationDownloads(r.ctx, r.cfg.Name)
	if err != nil {
		if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
			return nil, errors.Wrap(runnerErrors.ErrUnauthorized, "fetching runners")
		}
		return nil, errors.Wrap(err, "fetching runner tools")
	}

	return tools, nil
}

func (r *enterprise) FetchDbInstances() ([]params.Instance, error) {
	return r.store.ListEnterpriseInstances(r.ctx, r.id)
}

func (r *enterprise) RemoveGithubRunner(runnerID int64) (*github.Response, error) {
	return r.ghcEnterpriseCli.RemoveRunner(r.ctx, r.cfg.Name, runnerID)
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
	tk, ghResp, err := r.ghcEnterpriseCli.CreateRegistrationToken(r.ctx, r.cfg.Name)

	if err != nil {
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

func (r *enterprise) GetCallbackURL() string {
	return r.cfgInternal.InstanceCallbackURL
}

func (r *enterprise) GetMetadataURL() string {
	return r.cfgInternal.InstanceMetadataURL
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
