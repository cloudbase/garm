package pool

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"runner-manager/auth"
	"runner-manager/config"
	dbCommon "runner-manager/database/common"
	runnerErrors "runner-manager/errors"
	"runner-manager/params"
	"runner-manager/runner/common"
	providerCommon "runner-manager/runner/providers/common"
	"runner-manager/util"

	"github.com/google/go-github/v43/github"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// test that we implement PoolManager
var _ common.PoolManager = &Repository{}

func NewRepositoryPoolManager(ctx context.Context, cfg params.Repository, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error) {
	ghc, err := util.GithubClient(ctx, cfg.Internal.OAuth2Token)
	if err != nil {
		return nil, errors.Wrap(err, "getting github client")
	}
	pools := map[string]params.Pool{}

	for _, val := range cfg.Pools {
		pools[val.ID] = val
	}
	repo := &Repository{
		ctx:          ctx,
		cfg:          cfg,
		ghcli:        ghc,
		store:        store,
		providers:    providers,
		pools:        pools,
		controllerID: cfg.Internal.ControllerID,
		quit:         make(chan struct{}),
		done:         make(chan struct{}),
	}
	return repo, nil
}

type Repository struct {
	ctx          context.Context
	controllerID string
	cfg          params.Repository
	store        dbCommon.Store
	ghcli        *github.Client
	providers    map[string]common.Provider
	tools        []*github.RunnerApplicationDownload
	quit         chan struct{}
	done         chan struct{}
	id           string
	pools        map[string]params.Pool

	mux sync.Mutex
}

func (r *Repository) AddPool(ctx context.Context, pool params.Pool) error {
	r.mux.Lock()
	defer r.mux.Unlock()

	if _, ok := r.pools[pool.ID]; ok {
		return nil
	}

	// start pool loop
	r.pools[pool.ID] = pool
	return nil
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
	if err := r.fetchTools(); err != nil {
		return errors.Wrap(err, "initializing tools")
	}

	runners, err := r.getGithubRunners()
	if err != nil {
		return errors.Wrap(err, "fetching github runners")
	}
	if err := r.cleanupOrphanedProviderRunners(runners); err != nil {
		return errors.Wrap(err, "cleaning orphaned instances")
	}

	if err := r.cleanupOrphanedGithubRunners(runners); err != nil {
		return errors.Wrap(err, "cleaning orphaned github runners")
	}
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

func (r *Repository) consolidate() {
	r.mux.Lock()
	defer r.mux.Unlock()

	r.deletePendingInstances()
	r.addPendingInstances()
	r.ensureMinIdleRunners()
}

func (r *Repository) addPendingInstances() {
	// TODO: filter instances by status.
	instances, err := r.store.ListRepoInstances(r.ctx, r.id)
	if err != nil {
		log.Printf("failed to fetch instances from store: %s", err)
		return
	}

	for _, instance := range instances {
		if instance.Status != providerCommon.InstancePendingCreate {
			// not in pending_create status. Skip.
			continue
		}

		if err := r.addInstanceToProvider(instance); err != nil {
			log.Printf("failed to create instance in provider: %s", err)
		}
	}
}

func (r *Repository) deletePendingInstances() {
	instances, err := r.store.ListRepoInstances(r.ctx, r.id)
	if err != nil {
		log.Printf("failed to fetch instances from store: %s", err)
		return
	}

	for _, instance := range instances {
		if instance.Status != providerCommon.InstancePendingDelete {
			// not in pending_delete status. Skip.
			continue
		}

		if err := r.deleteInstanceFromProvider(instance); err != nil {
			log.Printf("failed to delete instance from provider: %s", err)
		}
	}
}

func (r *Repository) poolIDFromLabels(labels []*github.RunnerLabels) (string, error) {
	for _, lbl := range labels {
		if strings.HasPrefix(*lbl.Name, poolIDLabelprefix) {
			labelName := *lbl.Name
			return labelName[len(poolIDLabelprefix):], nil
		}
	}
	return "", runnerErrors.ErrNotFound
}

// cleanupOrphanedGithubRunners will forcefully remove any github runners that appear
// as offline and for which we no longer have a local instance.
// This may happen if someone manually deletes the instance in the provider. We need to
// first remove the instance from github, and then from our database.
func (r *Repository) cleanupOrphanedGithubRunners(runners []*github.Runner) error {
	for _, runner := range runners {
		status := runner.GetStatus()
		if status != "offline" {
			// Runner is online. Ignore it.
			continue
		}

		removeRunner := false
		poolID, err := r.poolIDFromLabels(runner.Labels)
		if err != nil {
			if !errors.Is(err, runnerErrors.ErrNotFound) {
				return errors.Wrap(err, "finding pool")
			}
			// not a runner we manage
			continue
		}

		pool, ok := r.pools[poolID]
		if !ok {
			// not a pool we manage.
			continue
		}

		dbInstance, err := r.store.GetInstanceByName(r.ctx, poolID, *runner.Name)
		if err != nil {
			if !errors.Is(err, runnerErrors.ErrNotFound) {
				return errors.Wrap(err, "fetching instance from DB")
			}
			// We no longer have a DB entry for this instance. Previous forceful
			// removal may have failed?
			removeRunner = true
		} else {
			if providerCommon.InstanceStatus(dbInstance.Status) == providerCommon.InstancePendingDelete {
				// already marked for deleting. Let consolidate take care of it.
				continue
			}
			// check if the provider still has the instance.
			provider, ok := r.providers[pool.ProviderName]
			if !ok {
				return fmt.Errorf("unknown provider %s for pool %s", pool.ProviderName, pool.ID)
			}

			if providerCommon.InstanceStatus(dbInstance.Status) == providerCommon.InstanceRunning {
				// instance is running, but github reports runner as offline. Log the event.
				// This scenario requires manual intervention.
				// Perhaps it just came online and github did not yet change it's status?
				log.Printf("instance %s is online but github reports runner as offline", dbInstance.Name)
				continue
			}
			//start the instance
			if err := provider.Start(r.ctx, dbInstance.ProviderID); err != nil {
				return errors.Wrapf(err, "starting instance %s", dbInstance.ProviderID)
			}
			// we started the instance. Give it a chance to come online
			continue
		}

		if removeRunner {
			if _, err := r.ghcli.Actions.RemoveRunner(r.ctx, r.cfg.Owner, r.cfg.Name, *runner.ID); err != nil {
				return errors.Wrap(err, "removing runner")
			}
		}
	}
	return nil
}

// cleanupOrphanedProviderRunners compares runners in github with local runners and removes
// any local runners that are not present in Github. Runners that are "idle" in our
// provider, but do not exist in github, will be removed. This can happen if the
// runner-manager was offline while a job was executed by a github action. When this
// happens, github will remove the ephemeral worker and send a webhook our way.
// If we were offline and did not process the webhook, the instance will linger.
// We need to remove it from the provider and database.
func (r *Repository) cleanupOrphanedProviderRunners(runners []*github.Runner) error {
	// runners, err := r.getGithubRunners()
	// if err != nil {
	// 	return errors.Wrap(err, "fetching github runners")
	// }

	dbInstances, err := r.store.ListRepoInstances(r.ctx, r.id)
	if err != nil {
		return errors.Wrap(err, "fetching instances from db")
	}

	runnerNames := map[string]bool{}
	for _, run := range runners {
		runnerNames[*run.Name] = true
	}

	for _, instance := range dbInstances {
		if providerCommon.InstanceStatus(instance.Status) == providerCommon.InstancePendingCreate || providerCommon.InstanceStatus(instance.Status) == providerCommon.InstancePendingDelete {
			// this instance is in the process of being created or is awaiting deletion.
			// Instances in pending_Create did not get a chance to register themselves in,
			// github so we let them be for now.
			continue
		}
		if ok := runnerNames[instance.Name]; !ok {
			// Set pending_delete on DB field. Allow consolidate() to remove it.
			_, err = r.store.UpdateInstance(r.ctx, instance.ID, params.UpdateInstanceParams{})
			if err != nil {
				return errors.Wrap(err, "syncing local state with github")
			}
		}
	}
	return nil
}

func (r *Repository) ensureMinIdleRunners() {
	for poolID, pool := range r.pools {
		existingInstances, err := r.store.ListInstances(r.ctx, poolID)
		if err != nil {
			log.Printf("failed to ensure minimum idle workers for pool %s: %s", poolID, err)
			return
		}

		if uint(len(existingInstances)) >= pool.MaxRunners {
			log.Printf("max workers reached for pool %s, skipping idle worker creation", poolID)
			continue
		}

		idleOrPendingWorkers := []params.Instance{}
		for _, inst := range existingInstances {
			if providerCommon.RunnerStatus(inst.RunnerStatus) == providerCommon.RunnerIdle || providerCommon.RunnerStatus(inst.RunnerStatus) == providerCommon.RunnerPending {
				idleOrPendingWorkers = append(idleOrPendingWorkers, inst)
			}
		}

		var required int
		if len(idleOrPendingWorkers) < int(pool.MinIdleRunners) {
			// get the needed delta.
			required = int(pool.MinIdleRunners) - len(idleOrPendingWorkers)

			projectedInstanceCount := len(existingInstances) + required
			if uint(projectedInstanceCount) > pool.MaxRunners {
				// ensure we don't go above max workers
				delta := projectedInstanceCount - int(pool.MaxRunners)
				required = required - delta
			}
		}

		for i := 0; i < required; i++ {
			log.Printf("addind new idle worker to pool %s", poolID)
			if err := r.AddRunner(r.ctx, poolID); err != nil {
				log.Printf("failed to add new instance for pool %s: %s", poolID, err)
			}
		}
	}
}

func (r *Repository) githubURL() string {
	return fmt.Sprintf("%s/%s/%s", config.GithubBaseURL, r.cfg.Owner, r.cfg.Name)
}

func (r *Repository) poolLabel() string {
	return fmt.Sprintf("%s%s", poolIDLabelprefix, r.id)
}

func (r *Repository) controllerLabel() string {
	return fmt.Sprintf("%s%s", controllerLabelPrefix, r.controllerID)
}

func (r *Repository) getLabels() []string {
	return []string{
		r.poolLabel(),
		r.controllerLabel(),
	}
}

func (r *Repository) updateArgsFromProviderInstance(providerInstance params.Instance) params.UpdateInstanceParams {
	return params.UpdateInstanceParams{
		ProviderID:   providerInstance.ProviderID,
		OSName:       providerInstance.OSName,
		OSVersion:    providerInstance.OSName,
		Addresses:    providerInstance.Addresses,
		Status:       providerInstance.Status,
		RunnerStatus: providerInstance.RunnerStatus,
	}
}

func (r *Repository) deleteInstanceFromProvider(instance params.Instance) error {
	pool, ok := r.pools[instance.PoolID]
	if !ok {
		return runnerErrors.NewNotFoundError("invalid pool ID")
	}

	provider, ok := r.providers[pool.ProviderName]
	if !ok {
		return runnerErrors.NewNotFoundError("invalid provider ID")
	}

	if err := provider.DeleteInstance(r.ctx, instance.ProviderID); err != nil {
		return errors.Wrap(err, "removing instance")
	}

	if err := r.store.DeleteInstance(r.ctx, pool.ID, instance.ID); err != nil {
		return errors.Wrap(err, "deleting instance from database")
	}
	return nil
}

func (r *Repository) addInstanceToProvider(instance params.Instance) error {
	pool, ok := r.pools[instance.PoolID]
	if !ok {
		return runnerErrors.NewNotFoundError("invalid pool ID")
	}

	provider, ok := r.providers[pool.ProviderName]
	if !ok {
		return runnerErrors.NewNotFoundError("invalid provider ID")
	}

	labels := []string{}
	for _, tag := range pool.Tags {
		labels = append(labels, tag.Name)
	}
	labels = append(labels, r.getLabels()...)

	tk, _, err := r.ghcli.Actions.CreateRegistrationToken(r.ctx, r.cfg.Owner, r.cfg.Name)

	if err != nil {
		return errors.Wrap(err, "creating runner token")
	}

	entity := fmt.Sprintf("%s/%s", r.cfg.Owner, r.cfg.Name)
	jwtToken, err := auth.NewInstanceJWTToken(instance, r.cfg.Internal.JWTSecret, entity, common.RepositoryPool)
	if err != nil {
		return errors.Wrap(err, "fetching instance jwt token")
	}

	bootstrapArgs := params.BootstrapInstance{
		Tools:                   r.tools,
		RepoURL:                 r.githubURL(),
		GithubRunnerAccessToken: *tk.Token,
		CallbackURL:             instance.CallbackURL,
		InstanceToken:           jwtToken,
		OSArch:                  pool.OSArch,
		Flavor:                  pool.Flavor,
		Image:                   pool.Image,
		Labels:                  labels,
	}

	providerInstance, err := provider.CreateInstance(r.ctx, bootstrapArgs)
	if err != nil {
		return errors.Wrap(err, "creating instance")
	}

	updateInstanceArgs := r.updateArgsFromProviderInstance(providerInstance)
	if _, err := r.store.UpdateInstance(r.ctx, instance.ID, updateInstanceArgs); err != nil {
		return errors.Wrap(err, "updating instance")
	}
	return nil
}

// TODO: add function to set runner status to idle when instance calls home on callback url

func (r *Repository) AddRunner(ctx context.Context, poolID string) error {
	pool, ok := r.pools[poolID]
	if !ok {
		return runnerErrors.NewNotFoundError("invalid provider ID")
	}

	name := fmt.Sprintf("runner-manager-%s", uuid.New())

	createParams := params.CreateInstanceParams{
		Name:         name,
		Pool:         poolID,
		Status:       providerCommon.InstancePendingCreate,
		RunnerStatus: providerCommon.RunnerPending,
		OSArch:       pool.OSArch,
		OSType:       pool.OSType,
		CallbackURL:  r.cfg.Internal.InstanceCallbackURL,
	}

	_, err := r.store.CreateInstance(r.ctx, poolID, createParams)
	if err != nil {
		return errors.Wrap(err, "creating instance")
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
		case <-time.After(5 * time.Second):
			// consolidate.
			r.consolidate()
		case <-time.After(3 * time.Hour):
			// Update tools cache.
			if err := r.fetchTools(); err != nil {
				log.Printf("failed to update tools for repo %s/%s: %s", r.cfg.Owner, r.cfg.Name, err)
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

func (r *Repository) WebhookSecret() string {
	return r.cfg.WebhookSecret
}

func (r *Repository) HandleWorkflowJob(job params.WorkflowJob) error {
	if job.Repository.Name != r.cfg.Name || job.Repository.Owner.Login != r.cfg.Owner {
		return runnerErrors.NewBadRequestError("job not meant for this pool manager")
	}

	switch job.Action {
	case "queued":
		// Create instance in database and set it to pending create.
	case "completed":
		// Set instance in database to pending delete. Unassigned jobs will have
		// an empty runner_name. There is nothing to to in that case.
	case "in_progress":
		// update instance workload state. Set job_id in instance state.
	}
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
