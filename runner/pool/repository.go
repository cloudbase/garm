package pool

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"runner-manager/config"
	runnerErrors "runner-manager/errors"
	"runner-manager/params"
	"runner-manager/runner/common"

	"github.com/google/go-github/v43/github"
	"github.com/pkg/errors"
)

// test that we implement PoolManager
var _ common.PoolManager = &Repository{}

func NewRepositoryRunnerPool(ctx context.Context, cfg config.Repository, provider common.Provider, ghcli *github.Client, controllerID string) (common.PoolManager, error) {
	queueSize := cfg.Pool.QueueSize
	if queueSize == 0 {
		queueSize = config.DefaultPoolQueueSize
	}
	repo := &Repository{
		ctx:          ctx,
		cfg:          cfg,
		ghcli:        ghcli,
		provider:     provider,
		controllerID: controllerID,
		jobQueue:     make(chan params.WorkflowJob, queueSize),
		quit:         make(chan struct{}),
		done:         make(chan struct{}),
	}

	if err := repo.fetchTools(); err != nil {
		return nil, errors.Wrap(err, "initializing tools")
	}
	return repo, nil
}

type Repository struct {
	ctx          context.Context
	controllerID string
	cfg          config.Repository
	ghcli        *github.Client
	provider     common.Provider
	tools        []*github.RunnerApplicationDownload
	jobQueue     chan params.WorkflowJob
	quit         chan struct{}
	done         chan struct{}
	mux          sync.Mutex
}

func (r *Repository) getGithubRunners() ([]*github.Runner, error) {
	runners, _, err := r.ghcli.Actions.ListRunners(r.ctx, r.cfg.Owner, r.cfg.Name, nil)
	if err != nil {
		return nil, errors.Wrap(err, "fetching runners")
	}

	return runners.Runners, nil
}

func (r *Repository) getProviderInstances() ([]params.Instance, error) {
	return nil, nil
}

func (r *Repository) Start() error {
	go r.loop()
	return nil
}

func (r *Repository) Stop() error {
	close(r.quit)
	return nil
}

func (r *Repository) fetchTools() error {
	r.mux.Lock()
	defer r.mux.Unlock()
	tools, _, err := r.ghcli.Actions.ListRunnerApplicationDownloads(r.ctx, r.cfg.Owner, r.cfg.Name)
	if err != nil {
		return errors.Wrap(err, "fetching runner tools")
	}
	r.tools = tools
	return nil
}

func (r *Repository) Wait() error {
	select {
	case <-r.done:
	case <-time.After(20 * time.Second):
		return errors.Wrap(runnerErrors.ErrTimeout, "waiting for pool to stop")
	}
	return nil
}

func (r *Repository) loop() {
	defer close(r.done)
	// TODO: Consolidate runners on loop start. Provider runners must match runners
	// in github and DB. When a Workflow job is received, we will first create/update
	// an entity in the database, before sending the request to the provider to create/delete
	// an instance. If a "queued" job is received, we create an entity in the db with
	// a state of "pending_create". Once that instance is up and calls home, it is marked
	// as "active". If a "completed" job is received from github, we mark the instance
	// as "pending_delete". Once the provider deletes the instance, we mark it as "deleted"
	// in the database.
	// We also ensure we have runners created based on pool characteristics. This is where
	// we spin up "MinWorkers" for each runner type.

	for {
		select {
		case job, ok := <-r.jobQueue:
			if !ok {
				// queue was closed. return.
				return
			}
			// We handle jobs synchronously (for now)
			switch job.Action {
			case "queued":
				// Create instance.
			case "completed":
				// Remove instance.
			case "in_progress":
				// update state
			}
			fmt.Println(job)
		case <-time.After(3 * time.Hour):
			// Update tools cache.
			if err := r.fetchTools(); err != nil {
				log.Printf("failed to update tools for repo %s: %s", r.cfg.String(), err)
			}
		case <-r.ctx.Done():
			// daemon is shutting down.
			return
		case <-r.quit:
			// this worker was stopped.
			return
		}
	}
}

// addJobToQueue adds a new workflow job to the queue of jobs that need to be
// processed by this pool. Jobs are added by github webhooks, so it makes no sense
// to return an error when that happens. But we do need to log any error that comes
// up. The queue size is configurable. If we hit that limit, new jobs will be discarded
// and logged.
// TODO: setup a state pipeline that will send back updates to the runner and update the
// database as needed.
func (r *Repository) addJobToQueue(job params.WorkflowJob) {
	select {
	case r.jobQueue <- job:
	case <-time.After(1 * time.Second):
		log.Printf("timed out accepting job. Queue is full.")
	}
}

func (r *Repository) WebhookSecret() string {
	return r.cfg.WebhookSecret
}

func (r *Repository) HandleWorkflowJob(job params.WorkflowJob) error {
	if job.Repository.FullName != r.cfg.String() {
		return runnerErrors.NewBadRequestError("job not meant for this pool")
	}
	r.addJobToQueue(job)
	return nil
}

func (r *Repository) ListInstances() ([]params.Instance, error) {
	return nil, nil
}

func (r *Repository) GetInstance() (params.Instance, error) {
	return params.Instance{}, nil
}
func (r *Repository) DeleteInstance() error {
	return nil
}
func (r *Repository) StopInstance() error {
	return nil
}
func (r *Repository) StartInstance() error {
	return nil
}
