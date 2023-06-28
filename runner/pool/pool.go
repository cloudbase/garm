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
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudbase/garm/auth"
	dbCommon "github.com/cloudbase/garm/database/common"
	runnerErrors "github.com/cloudbase/garm/errors"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	providerCommon "github.com/cloudbase/garm/runner/providers/common"
	"github.com/cloudbase/garm/util"

	"github.com/google/go-github/v53/github"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var (
	poolIDLabelprefix     = "runner-pool-id:"
	controllerLabelPrefix = "runner-controller-id:"
	// We tag runners that have been spawned as a result of a queued job with the job ID
	// that spawned them. There is no way to guarantee that the runner spawned in response to a particular
	// job, will be picked up by that job. We mark them so as in the very likely event that the runner
	// has picked up a different job, we can clear the lock on the job that spaned it.
	// The job it picked up would already be transitioned to in_progress so it will be ignored by the
	// consume loop.
	jobLabelPrefix = "in_response_to_job:"
)

const (
	// maxCreateAttempts is the number of times we will attempt to create an instance
	// before we give up.
	// TODO: make this configurable(?)
	maxCreateAttempts = 5
)

type keyMutex struct {
	muxes sync.Map
}

func (k *keyMutex) TryLock(key string) bool {
	mux, _ := k.muxes.LoadOrStore(key, &sync.Mutex{})
	keyMux := mux.(*sync.Mutex)
	return keyMux.TryLock()
}

func (k *keyMutex) Unlock(key string, remove bool) {
	mux, ok := k.muxes.Load(key)
	if !ok {
		return
	}
	keyMux := mux.(*sync.Mutex)
	if remove {
		k.Delete(key)
	}
	keyMux.Unlock()
}

func (k *keyMutex) Delete(key string) {
	k.muxes.Delete(key)
}

type basePoolManager struct {
	ctx          context.Context
	controllerID string

	store dbCommon.Store

	providers map[string]common.Provider
	tools     []*github.RunnerApplicationDownload
	quit      chan struct{}

	helper       poolHelper
	credsDetails params.GithubCredentials

	managerIsRunning   bool
	managerErrorReason string

	mux    sync.Mutex
	wg     *sync.WaitGroup
	keyMux *keyMutex
}

func (r *basePoolManager) HandleWorkflowJob(job params.WorkflowJob) error {
	if err := r.helper.ValidateOwner(job); err != nil {
		return errors.Wrap(err, "validating owner")
	}

	var jobParams params.Job
	var err error
	var triggeredBy int64
	defer func() {
		// we're updating the job in the database, regardless of whether it was successful or not.
		// or if it was meant for this pool or not. Github will send the same job data to all hierarchies
		// that have been configured to work with garm. Updating the job at all levels should yield the same
		// outcome in the db.
		if jobParams.ID == 0 {
			return
		}

		potentialPools, err := r.store.FindPoolsMatchingAllTags(r.ctx, r.helper.PoolType(), r.helper.ID(), jobParams.Labels)
		if err != nil {
			log.Printf("[Pool mgr %s] failed to find pools matching tags %s: %s; not recording job", r.helper.String(), strings.Join(jobParams.Labels, ", "), err)
			return
		}
		if len(potentialPools) == 0 {
			log.Printf("[Pool mgr %s] no pools matching tags %s; not recording job", r.helper.String(), strings.Join(jobParams.Labels, ", "))
			return
		}

		if _, jobErr := r.store.CreateOrUpdateJob(r.ctx, jobParams); jobErr != nil {
			log.Printf("[Pool mgr %s] failed to update job %d: %s", r.helper.String(), jobParams.ID, jobErr)
		}

		if triggeredBy != 0 && jobParams.ID != triggeredBy {
			// The triggeredBy value is only set by the "in_progress" webhook. The runner that
			// transitioned to in_progress was created as a result of a different queued job. If that job is
			// still queued and we don't remove the lock, it will linger until the lock timeout is reached.
			// That may take a long time, so we break the lock here and allow it to be scheduled again.
			if err := r.store.BreakLockJobIsQueued(r.ctx, triggeredBy); err != nil {
				log.Printf("failed to break lock for job %d: %s", triggeredBy, err)
			}
		}
	}()

	switch job.Action {
	case "queued":
		// Record the job in the database. Queued jobs will be picked up by the consumeQueuedJobs() method
		// when reconciling.
		jobParams, err = r.paramsWorkflowJobToParamsJob(job)
		if err != nil {
			return errors.Wrap(err, "converting job to params")
		}
	case "completed":
		jobParams, err = r.paramsWorkflowJobToParamsJob(job)
		if err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				// Unassigned jobs will have an empty runner_name.
				// We also need to ignore not found errors, as we may get a webhook regarding
				// a workflow that is handled by a runner at a different hierarchy level.
				return nil
			}
			return errors.Wrap(err, "converting job to params")
		}

		// update instance workload state.
		if _, err := r.setInstanceRunnerStatus(jobParams.RunnerName, providerCommon.RunnerTerminated); err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				return nil
			}
			r.log("failed to update runner %s status: %s", util.SanitizeLogEntry(jobParams.RunnerName), err)
			return errors.Wrap(err, "updating runner")
		}
		r.log("marking instance %s as pending_delete", util.SanitizeLogEntry(jobParams.RunnerName))
		if _, err := r.setInstanceStatus(jobParams.RunnerName, providerCommon.InstancePendingDelete, nil); err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				return nil
			}
			r.log("failed to update runner %s status: %s", util.SanitizeLogEntry(jobParams.RunnerName), err)
			return errors.Wrap(err, "updating runner")
		}
	case "in_progress":
		jobParams, err = r.paramsWorkflowJobToParamsJob(job)
		if err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				// This is most likely a runner we're not managing. If we define a repo from within an org
				// and also define that same org, we will get a hook from github from both the repo and the org
				// regarding the same workflow. We look for the runner in the database, and make sure it exists and is
				// part of a pool that this manager is responsible for. A not found error here will most likely mean
				// that we are not responsible for that runner, and we should ignore it.
				return nil
			}
			return errors.Wrap(err, "converting job to params")
		}

		// update instance workload state.
		instance, err := r.setInstanceRunnerStatus(jobParams.RunnerName, providerCommon.RunnerActive)
		if err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				return nil
			}
			r.log("failed to update runner %s status: %s", util.SanitizeLogEntry(jobParams.RunnerName), err)
			return errors.Wrap(err, "updating runner")
		}
		// Set triggeredBy here so we break the lock on any potential queued job.
		triggeredBy = jobIdFromLabels(instance.AditionalLabels)

		// A runner has picked up the job, and is now running it. It may need to be replaced if the pool has
		// a minimum number of idle runners configured.
		pool, err := r.store.GetPoolByID(r.ctx, instance.PoolID)
		if err != nil {
			return errors.Wrap(err, "getting pool")
		}
		if err := r.ensureIdleRunnersForOnePool(pool); err != nil {
			log.Printf("error ensuring idle runners for pool %s: %s", pool.ID, err)
		}
	}
	return nil
}

func jobIdFromLabels(labels []string) int64 {
	for _, lbl := range labels {
		if strings.HasPrefix(lbl, jobLabelPrefix) {
			jobId, err := strconv.ParseInt(lbl[len(jobLabelPrefix):], 10, 64)
			if err != nil {
				return 0
			}
			return jobId
		}
	}
	return 0
}

func (r *basePoolManager) startLoopForFunction(f func() error, interval time.Duration, name string, alwaysRun bool) {
	r.log("starting %s loop for %s", name, r.helper.String())
	ticker := time.NewTicker(interval)
	r.wg.Add(1)

	defer func() {
		r.log("%s loop exited for pool %s", name, r.helper.String())
		ticker.Stop()
		r.wg.Done()
	}()

	for {
		shouldRun := r.managerIsRunning
		if alwaysRun {
			shouldRun = true
		}
		switch shouldRun {
		case true:
			select {
			case <-ticker.C:
				if err := f(); err != nil {
					r.log("error in loop %s: %q", name, err)
					if errors.Is(err, runnerErrors.ErrUnauthorized) {
						r.setPoolRunningState(false, err.Error())
					}
				}
			case <-r.ctx.Done():
				// daemon is shutting down.
				return
			case <-r.quit:
				// this worker was stopped.
				return
			}
		default:
			select {
			case <-r.ctx.Done():
				// daemon is shutting down.
				return
			case <-r.quit:
				// this worker was stopped.
				return
			default:
				r.waitForTimeoutOrCanceled(common.UnauthorizedBackoffTimer)
			}
		}
	}
}

func (r *basePoolManager) updateTools() error {
	// Update tools cache.
	tools, err := r.helper.FetchTools()
	if err != nil {
		r.setPoolRunningState(false, err.Error())
		if errors.Is(err, runnerErrors.ErrUnauthorized) {
			r.waitForTimeoutOrCanceled(common.UnauthorizedBackoffTimer)
		} else {
			r.waitForTimeoutOrCanceled(60 * time.Second)
		}
		return fmt.Errorf("failed to update tools for repo %s: %w", r.helper.String(), err)
	}
	r.mux.Lock()
	r.tools = tools
	r.mux.Unlock()

	r.setPoolRunningState(true, "")
	return err
}

func controllerIDFromLabels(labels []string) string {
	for _, lbl := range labels {
		if strings.HasPrefix(lbl, controllerLabelPrefix) {
			return lbl[len(controllerLabelPrefix):]
		}
	}
	return ""
}

func labelsFromRunner(runner *github.Runner) []string {
	if runner == nil || runner.Labels == nil {
		return []string{}
	}

	var labels []string
	for _, val := range runner.Labels {
		if val == nil {
			continue
		}
		labels = append(labels, val.GetName())
	}
	return labels
}

// isManagedRunner returns true if labels indicate the runner belongs to a pool
// this manager is responsible for.
func (r *basePoolManager) isManagedRunner(labels []string) bool {
	runnerControllerID := controllerIDFromLabels(labels)
	return runnerControllerID == r.controllerID
}

// cleanupOrphanedProviderRunners compares runners in github with local runners and removes
// any local runners that are not present in Github. Runners that are "idle" in our
// provider, but do not exist in github, will be removed. This can happen if the
// garm was offline while a job was executed by a github action. When this
// happens, github will remove the ephemeral worker and send a webhook our way.
// If we were offline and did not process the webhook, the instance will linger.
// We need to remove it from the provider and database.
func (r *basePoolManager) cleanupOrphanedProviderRunners(runners []*github.Runner) error {
	dbInstances, err := r.helper.FetchDbInstances()
	if err != nil {
		return errors.Wrap(err, "fetching instances from db")
	}

	runnerNames := map[string]bool{}
	for _, run := range runners {
		if !r.isManagedRunner(labelsFromRunner(run)) {
			r.log("runner %s is not managed by a pool belonging to %s", *run.Name, r.helper.String())
			continue
		}
		runnerNames[*run.Name] = true
	}

	for _, instance := range dbInstances {
		lockAcquired := r.keyMux.TryLock(instance.Name)
		if !lockAcquired {
			r.log("failed to acquire lock for instance %s", instance.Name)
			continue
		}
		defer r.keyMux.Unlock(instance.Name, false)

		switch providerCommon.InstanceStatus(instance.Status) {
		case providerCommon.InstancePendingCreate,
			providerCommon.InstancePendingDelete:
			// this instance is in the process of being created or is awaiting deletion.
			// Instances in pending_create did not get a chance to register themselves in,
			// github so we let them be for now.
			continue
		}

		switch instance.RunnerStatus {
		case providerCommon.RunnerPending, providerCommon.RunnerInstalling:
			// runner is still installing. We give it a chance to finish.
			r.log("runner %s is still installing, give it a chance to finish", instance.Name)
			continue
		}

		if time.Since(instance.UpdatedAt).Minutes() < 5 {
			// instance was updated recently. We give it a chance to register itself in github.
			r.log("instance %s was updated recently, skipping check", instance.Name)
			continue
		}

		if ok := runnerNames[instance.Name]; !ok {
			// Set pending_delete on DB field. Allow consolidate() to remove it.
			if _, err := r.setInstanceStatus(instance.Name, providerCommon.InstancePendingDelete, nil); err != nil {
				r.log("failed to update runner %s status: %s", instance.Name, err)
				return errors.Wrap(err, "updating runner")
			}
		}
	}
	return nil
}

// reapTimedOutRunners will mark as pending_delete any runner that has a status
// of "running" in the provider, but that has not registered with Github, and has
// received no new updates in the configured timeout interval.
func (r *basePoolManager) reapTimedOutRunners(runners []*github.Runner) error {
	dbInstances, err := r.helper.FetchDbInstances()
	if err != nil {
		return errors.Wrap(err, "fetching instances from db")
	}

	runnersByName := map[string]*github.Runner{}
	for _, run := range runners {
		if !r.isManagedRunner(labelsFromRunner(run)) {
			r.log("runner %s is not managed by a pool belonging to %s", *run.Name, r.helper.String())
			continue
		}
		runnersByName[*run.Name] = run
	}

	for _, instance := range dbInstances {
		r.log("attempting to lock instance %s", instance.Name)
		lockAcquired := r.keyMux.TryLock(instance.Name)
		if !lockAcquired {
			r.log("failed to acquire lock for instance %s", instance.Name)
			continue
		}
		defer r.keyMux.Unlock(instance.Name, false)

		pool, err := r.store.GetPoolByID(r.ctx, instance.PoolID)
		if err != nil {
			return errors.Wrap(err, "fetching instance pool info")
		}
		if time.Since(instance.UpdatedAt).Minutes() < float64(pool.RunnerTimeout()) {
			continue
		}

		// There are 2 cases (currently) where we consider a runner as timed out:
		//   * The runner never joined github within the pool timeout
		//   * The runner managed to join github, but the setup process failed later and the runner
		//     never started on the instance.
		//
		// There are several steps in the user data that sets up the runner:
		//   * Download and unarchive the runner from github (or used the cached version)
		//   * Configure runner (connects to github). At this point the runner is seen as offline.
		//   * Install the service
		//   * Set SELinux context (if SELinux is enabled)
		//   * Start the service (if successful, the runner will transition to "online")
		//   * Get the runner ID
		//
		// If we fail getting the runner ID after it's started, garm will set the runner status to "failed",
		// even though, technically the runner is online and fully functional. This is why we check here for
		// both the runner status as reported by GitHub and the runner status as reported by the provider.
		// If the runner is "offline" and marked as "failed", it should be safe to reap it.
		if runner, ok := runnersByName[instance.Name]; !ok || (runner.GetStatus() == "offline" && instance.RunnerStatus == providerCommon.RunnerFailed) {
			r.log("reaping timed-out/failed runner %s", instance.Name)
			if err := r.ForceDeleteRunner(instance); err != nil {
				r.log("failed to update runner %s status: %s", instance.Name, err)
				return errors.Wrap(err, "updating runner")
			}
		}
	}
	return nil
}

func instanceInList(instanceName string, instances []params.Instance) (params.Instance, bool) {
	for _, val := range instances {
		if val.Name == instanceName {
			return val, true
		}
	}
	return params.Instance{}, false
}

// cleanupOrphanedGithubRunners will forcefully remove any github runners that appear
// as offline and for which we no longer have a local instance.
// This may happen if someone manually deletes the instance in the provider. We need to
// first remove the instance from github, and then from our database.
func (r *basePoolManager) cleanupOrphanedGithubRunners(runners []*github.Runner) error {
	poolInstanceCache := map[string][]params.Instance{}
	g, ctx := errgroup.WithContext(r.ctx)
	for _, runner := range runners {
		if !r.isManagedRunner(labelsFromRunner(runner)) {
			r.log("runner %s is not managed by a pool belonging to %s", *runner.Name, r.helper.String())
			continue
		}

		status := runner.GetStatus()
		if status != "offline" {
			// Runner is online. Ignore it.
			continue
		}

		dbInstance, err := r.store.GetInstanceByName(r.ctx, *runner.Name)
		if err != nil {
			if !errors.Is(err, runnerErrors.ErrNotFound) {
				return errors.Wrap(err, "fetching instance from DB")
			}
			// We no longer have a DB entry for this instance, and the runner appears offline in github.
			// Previous forceful removal may have failed?
			r.log("Runner %s has no database entry in garm, removing from github", *runner.Name)
			resp, err := r.helper.RemoveGithubRunner(*runner.ID)
			if err != nil {
				// Removed in the meantime?
				if resp != nil && resp.StatusCode == http.StatusNotFound {
					continue
				}
				return errors.Wrap(err, "removing runner")
			}
			continue
		}

		switch providerCommon.InstanceStatus(dbInstance.Status) {
		case providerCommon.InstancePendingDelete, providerCommon.InstanceDeleting:
			// already marked for deletion or is in the process of being deleted.
			// Let consolidate take care of it.
			continue
		}

		pool, err := r.helper.GetPoolByID(dbInstance.PoolID)
		if err != nil {
			return errors.Wrap(err, "fetching pool")
		}

		// check if the provider still has the instance.
		provider, ok := r.providers[pool.ProviderName]
		if !ok {
			return fmt.Errorf("unknown provider %s for pool %s", pool.ProviderName, pool.ID)
		}

		var poolInstances []params.Instance
		poolInstances, ok = poolInstanceCache[pool.ID]
		if !ok {
			r.log("updating instances cache for pool %s", pool.ID)
			poolInstances, err = provider.ListInstances(r.ctx, pool.ID)
			if err != nil {
				return errors.Wrapf(err, "fetching instances for pool %s", pool.ID)
			}
			poolInstanceCache[pool.ID] = poolInstances
		}

		lockAcquired := r.keyMux.TryLock(dbInstance.Name)
		if !lockAcquired {
			r.log("failed to acquire lock for instance %s", dbInstance.Name)
			continue
		}

		// See: https://golang.org/doc/faq#closures_and_goroutines
		runner := runner
		g.Go(func() error {
			deleteMux := false
			defer func() {
				r.keyMux.Unlock(dbInstance.Name, deleteMux)
			}()
			providerInstance, ok := instanceInList(dbInstance.Name, poolInstances)
			if !ok {
				// The runner instance is no longer on the provider, and it appears offline in github.
				// It should be safe to force remove it.
				r.log("Runner instance for %s is no longer on the provider, removing from github", dbInstance.Name)
				resp, err := r.helper.RemoveGithubRunner(*runner.ID)
				if err != nil {
					// Removed in the meantime?
					if resp != nil && resp.StatusCode == http.StatusNotFound {
						r.log("runner dissapeared from github")
					} else {
						return errors.Wrap(err, "removing runner from github")
					}
				}
				// Remove the database entry for the runner.
				r.log("Removing %s from database", dbInstance.Name)
				if err := r.store.DeleteInstance(ctx, dbInstance.PoolID, dbInstance.Name); err != nil {
					return errors.Wrap(err, "removing runner from database")
				}
				deleteMux = true
				return nil
			}

			if providerInstance.Status == providerCommon.InstanceRunning {
				// instance is running, but github reports runner as offline. Log the event.
				// This scenario may require manual intervention.
				// Perhaps it just came online and github did not yet change it's status?
				r.log("instance %s is online but github reports runner as offline", dbInstance.Name)
				return nil
			} else {
				r.log("instance %s was found in stopped state; starting", dbInstance.Name)
				//start the instance
				if err := provider.Start(r.ctx, dbInstance.ProviderID); err != nil {
					return errors.Wrapf(err, "starting instance %s", dbInstance.ProviderID)
				}
			}
			return nil
		})
	}
	if err := r.waitForErrorGroupOrContextCancelled(g); err != nil {
		return errors.Wrap(err, "removing orphaned github runners")
	}
	return nil
}

func (r *basePoolManager) waitForErrorGroupOrContextCancelled(g *errgroup.Group) error {
	if g == nil {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		waitErr := g.Wait()
		done <- waitErr
	}()

	select {
	case err := <-done:
		return err
	case <-r.ctx.Done():
		return r.ctx.Err()
	}
}

func (r *basePoolManager) fetchInstance(runnerName string) (params.Instance, error) {
	runner, err := r.store.GetInstanceByName(r.ctx, runnerName)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching instance")
	}

	_, err = r.helper.GetPoolByID(runner.PoolID)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching pool")
	}

	return runner, nil
}

func (r *basePoolManager) setInstanceRunnerStatus(runnerName string, status providerCommon.RunnerStatus) (params.Instance, error) {
	updateParams := params.UpdateInstanceParams{
		RunnerStatus: status,
	}

	instance, err := r.updateInstance(runnerName, updateParams)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "updating runner state")
	}
	return instance, nil
}

func (r *basePoolManager) updateInstance(runnerName string, update params.UpdateInstanceParams) (params.Instance, error) {
	runner, err := r.fetchInstance(runnerName)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "fetching instance")
	}

	instance, err := r.store.UpdateInstance(r.ctx, runner.ID, update)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "updating runner state")
	}
	return instance, nil
}

func (r *basePoolManager) setInstanceStatus(runnerName string, status providerCommon.InstanceStatus, providerFault []byte) (params.Instance, error) {
	updateParams := params.UpdateInstanceParams{
		Status:        status,
		ProviderFault: providerFault,
	}

	instance, err := r.updateInstance(runnerName, updateParams)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "updating runner state")
	}
	return instance, nil
}

func (r *basePoolManager) AddRunner(ctx context.Context, poolID string, aditionalLabels []string) error {
	pool, err := r.helper.GetPoolByID(poolID)
	if err != nil {
		return errors.Wrap(err, "fetching pool")
	}

	name := fmt.Sprintf("%s-%s", pool.GetRunnerPrefix(), util.NewID())

	createParams := params.CreateInstanceParams{
		Name:              name,
		Status:            providerCommon.InstancePendingCreate,
		RunnerStatus:      providerCommon.RunnerPending,
		OSArch:            pool.OSArch,
		OSType:            pool.OSType,
		CallbackURL:       r.helper.GetCallbackURL(),
		MetadataURL:       r.helper.GetMetadataURL(),
		CreateAttempt:     1,
		GitHubRunnerGroup: pool.GitHubRunnerGroup,
		AditionalLabels:   aditionalLabels,
	}

	_, err = r.store.CreateInstance(r.ctx, poolID, createParams)
	if err != nil {
		return errors.Wrap(err, "creating instance")
	}

	return nil
}

func (r *basePoolManager) Status() params.PoolManagerStatus {
	r.mux.Lock()
	defer r.mux.Unlock()
	return params.PoolManagerStatus{
		IsRunning:     r.managerIsRunning,
		FailureReason: r.managerErrorReason,
	}
}

func (r *basePoolManager) waitForTimeoutOrCanceled(timeout time.Duration) {
	r.log("sleeping for %.2f minutes", timeout.Minutes())
	select {
	case <-time.After(timeout):
	case <-r.ctx.Done():
	case <-r.quit:
	}
}

func (r *basePoolManager) setPoolRunningState(isRunning bool, failureReason string) {
	r.mux.Lock()
	r.managerErrorReason = failureReason
	r.managerIsRunning = isRunning
	r.mux.Unlock()
}

func (r *basePoolManager) addInstanceToProvider(instance params.Instance) error {
	pool, err := r.helper.GetPoolByID(instance.PoolID)
	if err != nil {
		return errors.Wrap(err, "fetching pool")
	}

	provider, ok := r.providers[pool.ProviderName]
	if !ok {
		return fmt.Errorf("unknown provider %s for pool %s", pool.ProviderName, pool.ID)
	}

	labels := []string{}
	for _, tag := range pool.Tags {
		labels = append(labels, tag.Name)
	}
	labels = append(labels, r.controllerLabel())
	labels = append(labels, r.poolLabel(pool.ID))

	if len(instance.AditionalLabels) > 0 {
		labels = append(labels, instance.AditionalLabels...)
	}

	jwtValidity := pool.RunnerTimeout()

	entity := r.helper.String()
	jwtToken, err := auth.NewInstanceJWTToken(instance, r.helper.JwtToken(), entity, pool.PoolType(), jwtValidity)
	if err != nil {
		return errors.Wrap(err, "fetching instance jwt token")
	}

	bootstrapArgs := params.BootstrapInstance{
		Name:              instance.Name,
		Tools:             r.tools,
		RepoURL:           r.helper.GithubURL(),
		MetadataURL:       instance.MetadataURL,
		CallbackURL:       instance.CallbackURL,
		InstanceToken:     jwtToken,
		OSArch:            pool.OSArch,
		OSType:            pool.OSType,
		Flavor:            pool.Flavor,
		Image:             pool.Image,
		ExtraSpecs:        pool.ExtraSpecs,
		Labels:            labels,
		PoolID:            instance.PoolID,
		CACertBundle:      r.credsDetails.CABundle,
		GitHubRunnerGroup: instance.GitHubRunnerGroup,
	}

	var instanceIDToDelete string

	defer func() {
		if instanceIDToDelete != "" {
			if err := provider.DeleteInstance(r.ctx, instanceIDToDelete); err != nil {
				if !errors.Is(err, runnerErrors.ErrNotFound) {
					r.log("failed to cleanup instance: %s", instanceIDToDelete)
				}
			}
		}
	}()

	providerInstance, err := provider.CreateInstance(r.ctx, bootstrapArgs)
	if err != nil {
		instanceIDToDelete = instance.Name
		return errors.Wrap(err, "creating instance")
	}

	if providerInstance.Status == providerCommon.InstanceError {
		instanceIDToDelete = instance.ProviderID
		if instanceIDToDelete == "" {
			instanceIDToDelete = instance.Name
		}
	}

	updateInstanceArgs := r.updateArgsFromProviderInstance(providerInstance)
	if _, err := r.store.UpdateInstance(r.ctx, instance.ID, updateInstanceArgs); err != nil {
		return errors.Wrap(err, "updating instance")
	}
	return nil
}

func (r *basePoolManager) getRunnerDetailsFromJob(job params.WorkflowJob) (params.RunnerInfo, error) {
	runnerInfo := params.RunnerInfo{
		Name:   job.WorkflowJob.RunnerName,
		Labels: job.WorkflowJob.Labels,
	}

	var err error
	if job.WorkflowJob.RunnerName == "" {
		if job.WorkflowJob.Conclusion == "skipped" || job.WorkflowJob.Conclusion == "canceled" {
			// job was skipped or canceled before a runner was allocated. No point in continuing.
			return params.RunnerInfo{}, fmt.Errorf("job %d was skipped or canceled before a runner was allocated: %w", job.WorkflowJob.ID, runnerErrors.ErrNotFound)
		}
		// Runner name was not set in WorkflowJob by github. We can still attempt to
		// fetch the info we need, using the workflow run ID, from the API.
		r.log("runner name not found in workflow job, attempting to fetch from API")
		runnerInfo, err = r.helper.GetRunnerInfoFromWorkflow(job)
		if err != nil {
			return params.RunnerInfo{}, errors.Wrap(err, "fetching runner name from API")
		}
	}

	runnerDetails, err := r.store.GetInstanceByName(context.Background(), runnerInfo.Name)
	if err != nil {
		r.log("could not find runner details for %s", util.SanitizeLogEntry(runnerInfo.Name))
		return params.RunnerInfo{}, errors.Wrap(err, "fetching runner details")
	}

	if _, err := r.helper.GetPoolByID(runnerDetails.PoolID); err != nil {
		r.log("runner %s (pool ID: %s) does not belong to any pool we manage: %s", runnerDetails.Name, runnerDetails.PoolID, err)
		return params.RunnerInfo{}, errors.Wrap(err, "fetching pool for instance")
	}
	return runnerInfo, nil
}

// paramsWorkflowJobToParamsJob returns a params.Job from a params.WorkflowJob, and aditionally determines
// if the runner belongs to this pool or not. It will always return a valid params.Job, even if it errs out.
// This allows us to still update the job in the database, even if we determined that it wasn't necessarily meant
// for this pool.
// If garm manages multiple hierarchies (repos, org, enterprise) which involve the same repo, we will get a hook
// whenever a job involving our repo triggers a hook. So even if the job is picked up by a runner at the enterprise
// level, the repo and org still get a hook.
// We even get a hook if a particular job is picked up by a GitHub hosted runner. We don't know who will pick up the job
// until the "in_progress" event is sent and we can see which runner picked it up.
//
// We save the details of that job at every level, because we want to at least update the status of the job. We make
// decissions based on the status of saved jobs. A "queued" job will prompt garm to search for an appropriate pool
// and spin up a runner there if no other idle runner exists to pick it up.
func (r *basePoolManager) paramsWorkflowJobToParamsJob(job params.WorkflowJob) (params.Job, error) {
	asUUID, err := uuid.Parse(r.ID())
	if err != nil {
		return params.Job{}, errors.Wrap(err, "parsing pool ID as UUID")
	}

	jobParams := params.Job{
		ID:              job.WorkflowJob.ID,
		Action:          job.Action,
		RunID:           job.WorkflowJob.RunID,
		Status:          job.WorkflowJob.Status,
		Conclusion:      job.WorkflowJob.Conclusion,
		StartedAt:       job.WorkflowJob.StartedAt,
		CompletedAt:     job.WorkflowJob.CompletedAt,
		Name:            job.WorkflowJob.Name,
		GithubRunnerID:  job.WorkflowJob.RunnerID,
		RunnerGroupID:   job.WorkflowJob.RunnerGroupID,
		RunnerGroupName: job.WorkflowJob.RunnerGroupName,
		RepositoryName:  job.Repository.Name,
		RepositoryOwner: job.Repository.Owner.Login,
		Labels:          job.WorkflowJob.Labels,
	}

	runnerName := job.WorkflowJob.RunnerName
	if job.Action != "queued" && runnerName == "" {
		if job.WorkflowJob.Conclusion != "skipped" && job.WorkflowJob.Conclusion != "canceled" {
			// Runner name was not set in WorkflowJob by github. We can still attempt to fetch the info we need,
			// using the workflow run ID, from the API.
			// We may still get no runner name. In situations such as jobs being cancelled before a runner had the chance
			// to pick up the job, the runner name is not available from the API.
			runnerInfo, err := r.getRunnerDetailsFromJob(job)
			if err != nil && !errors.Is(err, runnerErrors.ErrNotFound) {
				return jobParams, errors.Wrap(err, "fetching runner details")
			}
			runnerName = runnerInfo.Name
		}
	}

	jobParams.RunnerName = runnerName

	switch r.helper.PoolType() {
	case params.EnterprisePool:
		jobParams.EnterpriseID = &asUUID
	case params.RepositoryPool:
		jobParams.RepoID = &asUUID
	case params.OrganizationPool:
		jobParams.OrgID = &asUUID
	default:
		return jobParams, errors.Errorf("unknown pool type: %s", r.helper.PoolType())
	}

	return jobParams, nil
}

func (r *basePoolManager) poolLabel(poolID string) string {
	return fmt.Sprintf("%s%s", poolIDLabelprefix, poolID)
}

func (r *basePoolManager) controllerLabel() string {
	return fmt.Sprintf("%s%s", controllerLabelPrefix, r.controllerID)
}

func (r *basePoolManager) updateArgsFromProviderInstance(providerInstance params.Instance) params.UpdateInstanceParams {
	return params.UpdateInstanceParams{
		ProviderID:    providerInstance.ProviderID,
		OSName:        providerInstance.OSName,
		OSVersion:     providerInstance.OSVersion,
		Addresses:     providerInstance.Addresses,
		Status:        providerInstance.Status,
		RunnerStatus:  providerInstance.RunnerStatus,
		ProviderFault: providerInstance.ProviderFault,
	}
}
func (r *basePoolManager) scaleDownOnePool(ctx context.Context, pool params.Pool) error {
	log.Printf("scaling down pool %s", pool.ID)
	if !pool.Enabled {
		log.Printf("pool %s is disabled, skipping scale down", pool.ID)
		return nil
	}

	existingInstances, err := r.store.ListPoolInstances(r.ctx, pool.ID)
	if err != nil {
		return fmt.Errorf("failed to ensure minimum idle workers for pool %s: %w", pool.ID, err)
	}

	idleWorkers := []params.Instance{}
	for _, inst := range existingInstances {
		// Idle runners that have been spawned and are still idle after 5 minutes, are take into
		// consideration for scale-down. The 5 minute grace period prevents a situation where a
		// "queued" workflow triggers the creation of a new idle runner, and this routine reaps
		// an idle runner before they have a chance to pick up a job.
		if inst.RunnerStatus == providerCommon.RunnerIdle && inst.Status == providerCommon.InstanceRunning && time.Since(inst.UpdatedAt).Minutes() > 2 {
			idleWorkers = append(idleWorkers, inst)
		}
	}

	if len(idleWorkers) == 0 {
		return nil
	}

	surplus := float64(len(idleWorkers) - int(pool.MinIdleRunners))

	if surplus <= 0 {
		return nil
	}

	scaleDownFactor := 0.5 // could be configurable
	numScaleDown := int(math.Ceil(surplus * scaleDownFactor))

	if numScaleDown <= 0 || numScaleDown > len(idleWorkers) {
		return fmt.Errorf("invalid number of instances to scale down: %v, check your scaleDownFactor: %v", numScaleDown, scaleDownFactor)
	}

	g, _ := errgroup.WithContext(ctx)

	for _, instanceToDelete := range idleWorkers[:numScaleDown] {
		instanceToDelete := instanceToDelete

		lockAcquired := r.keyMux.TryLock(instanceToDelete.Name)
		if !lockAcquired {
			r.log("failed to acquire lock for instance %s", instanceToDelete.Name)
			continue
		}
		defer r.keyMux.Unlock(instanceToDelete.Name, false)

		g.Go(func() error {
			r.log("scaling down idle worker %s from pool %s\n", instanceToDelete.Name, pool.ID)
			if err := r.ForceDeleteRunner(instanceToDelete); err != nil {
				return fmt.Errorf("failed to delete instance %s: %w", instanceToDelete.ID, err)
			}
			return nil
		})
	}

	if err := r.waitForErrorGroupOrContextCancelled(g); err != nil {
		return fmt.Errorf("failed to scale down pool %s: %w", pool.ID, err)
	}
	return nil
}

func (r *basePoolManager) addRunnerToPool(pool params.Pool, aditionalLabels []string) error {
	if !pool.Enabled {
		return fmt.Errorf("pool %s is disabled", pool.ID)
	}

	poolInstanceCount, err := r.store.PoolInstanceCount(r.ctx, pool.ID)
	if err != nil {
		return fmt.Errorf("failed to list pool instances: %w", err)
	}

	if poolInstanceCount >= int64(pool.MaxRunners) {
		return fmt.Errorf("max workers (%d) reached for pool %s", pool.MaxRunners, pool.ID)
	}

	if err := r.AddRunner(r.ctx, pool.ID, aditionalLabels); err != nil {
		return fmt.Errorf("failed to add new instance for pool %s: %s", pool.ID, err)
	}
	return nil
}

func (r *basePoolManager) ensureIdleRunnersForOnePool(pool params.Pool) error {
	if !pool.Enabled || pool.MinIdleRunners == 0 {
		return nil
	}

	existingInstances, err := r.store.ListPoolInstances(r.ctx, pool.ID)
	if err != nil {
		return fmt.Errorf("failed to ensure minimum idle workers for pool %s: %w", pool.ID, err)

	}

	if uint(len(existingInstances)) >= pool.MaxRunners {
		r.log("max workers (%d) reached for pool %s, skipping idle worker creation", pool.MaxRunners, pool.ID)
		return nil
	}

	idleOrPendingWorkers := []params.Instance{}
	for _, inst := range existingInstances {
		if inst.RunnerStatus != providerCommon.RunnerActive && inst.RunnerStatus != providerCommon.RunnerTerminated {
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
		r.log("adding new idle worker to pool %s", pool.ID)
		if err := r.AddRunner(r.ctx, pool.ID, nil); err != nil {
			return fmt.Errorf("failed to add new instance for pool %s: %w", pool.ID, err)
		}
	}
	return nil
}

func (r *basePoolManager) retryFailedInstancesForOnePool(ctx context.Context, pool params.Pool) error {
	if !pool.Enabled {
		return nil
	}
	r.log("running retry failed instances for pool %s", pool.ID)

	existingInstances, err := r.store.ListPoolInstances(r.ctx, pool.ID)
	if err != nil {
		return fmt.Errorf("failed to list instances for pool %s: %w", pool.ID, err)
	}

	g, errCtx := errgroup.WithContext(ctx)
	for _, instance := range existingInstances {
		if instance.Status != providerCommon.InstanceError {
			continue
		}
		if instance.CreateAttempt >= maxCreateAttempts {
			continue
		}

		r.log("attempting to retry failed instance %s", instance.Name)
		lockAcquired := r.keyMux.TryLock(instance.Name)
		if !lockAcquired {
			r.log("failed to acquire lock for instance %s", instance.Name)
			continue
		}

		instance := instance
		g.Go(func() error {
			defer r.keyMux.Unlock(instance.Name, false)
			// NOTE(gabriel-samfira): this is done in parallel. If there are many failed instances
			// this has the potential to create many API requests to the target provider.
			// TODO(gabriel-samfira): implement request throttling.
			if err := r.deleteInstanceFromProvider(errCtx, instance); err != nil {
				r.log("failed to delete instance %s from provider: %s", instance.Name, err)
				// Bail here, otherwise we risk creating multiple failing instances, and losing track
				// of them. If Create instance failed to return a proper provider ID, we rely on the
				// name to delete the instance. If we don't bail here, and end up with multiple
				// instances with the same name, using the name to clean up failed instances will fail
				// on any subsequent call, unless the external or native provider takes into account
				// non unique names and loops over all of them. Something which is extremely hacky and
				// which we would rather avoid.
				return err
			}

			// TODO(gabriel-samfira): Incrementing CreateAttempt should be done within a transaction.
			// It's fairly safe to do here (for now), as there should be no other code path that updates
			// an instance in this state.
			var tokenFetched bool = false
			updateParams := params.UpdateInstanceParams{
				CreateAttempt: instance.CreateAttempt + 1,
				TokenFetched:  &tokenFetched,
				Status:        providerCommon.InstancePendingCreate,
			}
			r.log("queueing previously failed instance %s for retry", instance.Name)
			// Set instance to pending create and wait for retry.
			if _, err := r.updateInstance(instance.Name, updateParams); err != nil {
				r.log("failed to update runner %s status: %s", instance.Name, err)
			}
			return nil
		})
	}
	if err := r.waitForErrorGroupOrContextCancelled(g); err != nil {
		return fmt.Errorf("failed to retry failed instances for pool %s: %w", pool.ID, err)
	}
	return nil
}

func (r *basePoolManager) retryFailedInstances() error {
	pools, err := r.helper.ListPools()
	if err != nil {
		return fmt.Errorf("error listing pools: %w", err)
	}
	g, ctx := errgroup.WithContext(r.ctx)
	for _, pool := range pools {
		pool := pool
		g.Go(func() error {
			if err := r.retryFailedInstancesForOnePool(ctx, pool); err != nil {
				return fmt.Errorf("retrying failed instances for pool %s: %w", pool.ID, err)
			}
			return nil
		})
	}

	if err := r.waitForErrorGroupOrContextCancelled(g); err != nil {
		return fmt.Errorf("retrying failed instances: %w", err)
	}

	return nil
}

func (r *basePoolManager) scaleDown() error {
	pools, err := r.helper.ListPools()
	if err != nil {
		return fmt.Errorf("error listing pools: %w", err)
	}
	g, ctx := errgroup.WithContext(r.ctx)
	for _, pool := range pools {
		pool := pool
		g.Go(func() error {
			r.log("running scale down for pool %s", pool.ID)
			return r.scaleDownOnePool(ctx, pool)
		})
	}
	if err := r.waitForErrorGroupOrContextCancelled(g); err != nil {
		return fmt.Errorf("failed to scale down: %w", err)
	}
	return nil
}

func (r *basePoolManager) ensureMinIdleRunners() error {
	pools, err := r.helper.ListPools()
	if err != nil {
		return fmt.Errorf("error listing pools: %w", err)
	}

	g, _ := errgroup.WithContext(r.ctx)
	for _, pool := range pools {
		pool := pool
		g.Go(func() error {
			return r.ensureIdleRunnersForOnePool(pool)
		})
	}

	if err := r.waitForErrorGroupOrContextCancelled(g); err != nil {
		return fmt.Errorf("failed to ensure minimum idle workers: %w", err)
	}
	return nil
}

func (r *basePoolManager) deleteInstanceFromProvider(ctx context.Context, instance params.Instance) error {
	pool, err := r.helper.GetPoolByID(instance.PoolID)
	if err != nil {
		return errors.Wrap(err, "fetching pool")
	}

	provider, ok := r.providers[pool.ProviderName]
	if !ok {
		return fmt.Errorf("unknown provider %s for pool %s", pool.ProviderName, pool.ID)
	}

	identifier := instance.ProviderID
	if identifier == "" {
		// provider did not return a provider ID?
		// try with name
		identifier = instance.Name
	}

	if err := provider.DeleteInstance(ctx, identifier); err != nil {
		return errors.Wrap(err, "removing instance")
	}

	return nil
}

func (r *basePoolManager) deletePendingInstances() error {
	instances, err := r.helper.FetchDbInstances()
	if err != nil {
		return fmt.Errorf("failed to fetch instances from store: %w", err)
	}

	r.log("removing instances in pending_delete")
	for _, instance := range instances {
		if instance.Status != providerCommon.InstancePendingDelete {
			// not in pending_delete status. Skip.
			continue
		}

		r.log("removing instance %s in pool %s", instance.Name, instance.PoolID)
		lockAcquired := r.keyMux.TryLock(instance.Name)
		if !lockAcquired {
			r.log("failed to acquire lock for instance %s", instance.Name)
			continue
		}

		// Set the status to deleting before launching the goroutine that removes
		// the runner from the provider (which can take a long time).
		if _, err := r.setInstanceStatus(instance.Name, providerCommon.InstanceDeleting, nil); err != nil {
			r.log("failed to update runner %s status: %q", instance.Name, err)
			r.keyMux.Unlock(instance.Name, false)
			continue
		}

		go func(instance params.Instance) (err error) {
			deleteMux := false
			defer func() {
				r.keyMux.Unlock(instance.Name, deleteMux)
			}()
			defer func(instance params.Instance) {
				if err != nil {
					r.log("failed to remove instance %s: %s", instance.Name, err)
					// failed to remove from provider. Set the status back to pending_delete, which
					// will retry the operation.
					if _, err := r.setInstanceStatus(instance.Name, providerCommon.InstancePendingDelete, nil); err != nil {
						r.log("failed to update runner %s status: %s", instance.Name, err)
					}
				}
			}(instance)

			r.log("removing instance %s from provider", instance.Name)
			err = r.deleteInstanceFromProvider(r.ctx, instance)
			if err != nil {
				return fmt.Errorf("failed to remove instance from provider: %w", err)
			}
			r.log("removing instance %s from database", instance.Name)
			if deleteErr := r.store.DeleteInstance(r.ctx, instance.PoolID, instance.Name); deleteErr != nil {
				return fmt.Errorf("failed to delete instance from database: %w", deleteErr)
			}
			deleteMux = true
			r.log("instance %s was successfully removed", instance.Name)
			return nil
		}(instance) //nolint
	}

	return nil
}

func (r *basePoolManager) addPendingInstances() error {
	// TODO: filter instances by status.
	instances, err := r.helper.FetchDbInstances()
	if err != nil {
		return fmt.Errorf("failed to fetch instances from store: %w", err)
	}
	for _, instance := range instances {
		if instance.Status != providerCommon.InstancePendingCreate {
			// not in pending_create status. Skip.
			continue
		}

		r.log("attempting to acquire lock for instance %s (create)", instance.Name)
		lockAcquired := r.keyMux.TryLock(instance.Name)
		if !lockAcquired {
			r.log("failed to acquire lock for instance %s", instance.Name)
			continue
		}

		// Set the instance to "creating" before launching the goroutine. This will ensure that addPendingInstances()
		// won't attempt to create the runner a second time.
		if _, err := r.setInstanceStatus(instance.Name, providerCommon.InstanceCreating, nil); err != nil {
			r.log("failed to update runner %s status: %s", instance.Name, err)
			r.keyMux.Unlock(instance.Name, false)
			// We failed to transition the instance to Creating. This means that garm will retry to create this instance
			// when the loop runs again and we end up with multiple instances.
			continue
		}

		go func(instance params.Instance) {
			defer r.keyMux.Unlock(instance.Name, false)
			r.log("creating instance %s in pool %s", instance.Name, instance.PoolID)
			if err := r.addInstanceToProvider(instance); err != nil {
				r.log("failed to add instance to provider: %s", err)
				errAsBytes := []byte(err.Error())
				if _, err := r.setInstanceStatus(instance.Name, providerCommon.InstanceError, errAsBytes); err != nil {
					r.log("failed to update runner %s status: %s", instance.Name, err)
				}
				r.log("failed to create instance in provider: %s", err)
			}
		}(instance)
	}

	return nil
}

func (r *basePoolManager) Wait() error {
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		return errors.Wrap(runnerErrors.ErrTimeout, "waiting for pool to stop")
	}
	return nil
}

func (r *basePoolManager) runnerCleanup() (err error) {
	r.log("running runner cleanup")
	runners, err := r.helper.GetGithubRunners()
	if err != nil {
		return fmt.Errorf("failed to fetch github runners: %w", err)
	}

	if err := r.reapTimedOutRunners(runners); err != nil {
		return fmt.Errorf("failed to reap timed out runners: %w", err)
	}

	if err := r.cleanupOrphanedRunners(); err != nil {
		return fmt.Errorf("failed to cleanup orphaned runners: %w", err)
	}

	return nil
}

func (r *basePoolManager) cleanupOrphanedRunners() error {
	runners, err := r.helper.GetGithubRunners()
	if err != nil {
		return errors.Wrap(err, "fetching github runners")
	}
	if err := r.cleanupOrphanedProviderRunners(runners); err != nil {
		return errors.Wrap(err, "cleaning orphaned instances")
	}

	if err := r.cleanupOrphanedGithubRunners(runners); err != nil {
		return errors.Wrap(err, "cleaning orphaned github runners")
	}

	return nil
}

func (r *basePoolManager) Start() error {
	r.updateTools() //nolint

	go r.startLoopForFunction(r.runnerCleanup, common.PoolReapTimeoutInterval, "timeout_reaper", false)
	go r.startLoopForFunction(r.scaleDown, common.PoolScaleDownInterval, "scale_down", false)
	go r.startLoopForFunction(r.deletePendingInstances, common.PoolConsilitationInterval, "consolidate[delete_pending]", false)
	go r.startLoopForFunction(r.addPendingInstances, common.PoolConsilitationInterval, "consolidate[add_pending]", false)
	go r.startLoopForFunction(r.ensureMinIdleRunners, common.PoolConsilitationInterval, "consolidate[ensure_min_idle]", false)
	go r.startLoopForFunction(r.retryFailedInstances, common.PoolConsilitationInterval, "consolidate[retry_failed]", false)
	go r.startLoopForFunction(r.updateTools, common.PoolToolUpdateInterval, "update_tools", true)
	go r.startLoopForFunction(r.consumeQueuedJobs, common.PoolConsilitationInterval, "job_queue_consumer", false)
	return nil
}

func (r *basePoolManager) Stop() error {
	close(r.quit)
	return nil
}

func (r *basePoolManager) RefreshState(param params.UpdatePoolStateParams) error {
	return r.helper.UpdateState(param)
}

func (r *basePoolManager) WebhookSecret() string {
	return r.helper.WebhookSecret()
}

func (r *basePoolManager) GithubRunnerRegistrationToken() (string, error) {
	return r.helper.GetGithubRegistrationToken()
}

func (r *basePoolManager) ID() string {
	return r.helper.ID()
}

func (r *basePoolManager) ForceDeleteRunner(runner params.Instance) error {
	if !r.managerIsRunning {
		return runnerErrors.NewConflictError("pool manager is not running for %s", r.helper.String())
	}
	if runner.AgentID != 0 {
		resp, err := r.helper.RemoveGithubRunner(runner.AgentID)
		if err != nil {
			if resp != nil {
				switch resp.StatusCode {
				case http.StatusUnprocessableEntity:
					return errors.Wrapf(runnerErrors.ErrBadRequest, "removing runner: %q", err)
				case http.StatusNotFound:
					// Runner may have been deleted by a finished job, or manually by the user.
					r.log("runner with agent id %d was not found in github", runner.AgentID)
				case http.StatusUnauthorized:
					// Mark the pool as offline from this point forward
					failureReason := fmt.Sprintf("failed to remove runner: %q", err)
					r.setPoolRunningState(false, failureReason)
					log.Print(failureReason)
					// evaluate the next switch case.
					fallthrough
				default:
					return errors.Wrap(err, "removing runner")
				}
			} else {
				// We got a nil response. Assume we are in error.
				return errors.Wrap(err, "removing runner")
			}
		}
	}
	r.log("setting instance status for: %v", runner.Name)

	if _, err := r.setInstanceStatus(runner.Name, providerCommon.InstancePendingDelete, nil); err != nil {
		r.log("failed to update runner %s status: %s", runner.Name, err)
		return errors.Wrap(err, "updating runner")
	}
	return nil
}

// consumeQueuedJobs qull pull all the known jobs from the database and attempt to create a new
// runner in one of the pools it manages if it matches the requested labels.
// This is a best effort attempt to consume queued jobs. We do not have any real way to know which
// runner from which pool will pick up a job we react to here. For example, the same job may be received
// by an enterprise manager, an org manager AND a repo manager. If an idle runner from another pool
// picks up the job after we created a runner in this pool, we will have an extra runner that may or may not
// have a job waiting for it.
// This is not a huge problem, as we have scale down logic which should remove any idle runners that have not
// picked up a job within a certain time frame. Also, the logic here should ensure that eventually, all known
// queued jobs will be consumed sooner or later.
//
// NOTE: jobs that were created while the garm instance was down, will be unknown to garm itself and will linger
// in queued state if the pools defined in garm have a minimum idle runner value set to 0. Simply put, garm won't
// know about the queued jobs that we didn't get a webhook for. Listing all jobs on startup is not feasible, as
// an enterprise may have thousands of repos and thousands of jobs in queued state. To fetch all jobs for an
// enterprise, we'd have to list all repos, and for each repo list all jobs currently in queued state. This is
// not desirable by any measure.
func (r *basePoolManager) consumeQueuedJobs() error {
	queued, err := r.store.ListEntityJobsByStatus(r.ctx, r.helper.PoolType(), r.helper.ID(), params.JobStatusQueued)
	if err != nil {
		return errors.Wrap(err, "listing queued jobs")
	}

	poolsCache := poolsForTags{}

	log.Printf("[Pool mgr %s] found %d queued jobs for %s", r.helper.String(), len(queued), r.helper.String())
	for _, job := range queued {
		if job.LockedBy != uuid.Nil && job.LockedBy.String() != r.ID() {
			// Job was handled by us or another entity.
			log.Printf("[Pool mgr %s] job %d is locked by %s", r.helper.String(), job.ID, job.LockedBy.String())
			continue
		}

		if time.Since(job.UpdatedAt) < time.Second*30 {
			// give the idle runners a chance to pick up the job.
			log.Printf("[Pool mgr %s] job %d was updated less than 30 seconds ago. Skipping", r.helper.String(), job.ID)
			continue
		}

		if time.Since(job.UpdatedAt) >= time.Minute*10 {
			// Job has been in queued state for 10 minutes or more. Check if it was consumed by another runner.
			workflow, ghResp, err := r.helper.GithubCLI().GetWorkflowJobByID(r.ctx, job.RepositoryOwner, job.RepositoryName, job.ID)
			if err != nil {
				if ghResp != nil {
					switch ghResp.StatusCode {
					case http.StatusNotFound:
						// Job does not exist in github. Remove it from the database.
						if err := r.store.DeleteJob(r.ctx, job.ID); err != nil {
							return errors.Wrap(err, "deleting job")
						}
					default:
						log.Printf("[Pool mgr %s] failed to fetch job information from github: %q (status code: %d)", r.helper.String(), err, ghResp.StatusCode)
					}
				}
				log.Printf("[Pool mgr %s] error fetching workflow info: %q", r.helper.String(), err)
				continue
			}

			if workflow.GetStatus() != "queued" {
				log.Printf("[Pool mgr %s] job is no longer in queued state on github. New status is: %s", r.helper.String(), workflow.GetStatus())
				job.Action = workflow.GetStatus()
				job.Status = workflow.GetStatus()
				job.Conclusion = workflow.GetConclusion()
				if workflow.RunnerName != nil {
					job.RunnerName = *workflow.RunnerName
				}
				if workflow.RunnerID != nil {
					job.GithubRunnerID = *workflow.RunnerID
				}
				if workflow.RunnerGroupName != nil {
					job.RunnerGroupName = *workflow.RunnerGroupName
				}
				if workflow.RunnerGroupID != nil {
					job.RunnerGroupID = *workflow.RunnerGroupID
				}
				if _, err := r.store.CreateOrUpdateJob(r.ctx, job); err != nil {
					log.Printf("[Pool mgr %s] failed to update job status: %q", r.helper.String(), err)
				}
				continue
			}

			// Job is still queued in our db and in github. Unlock it and try again.
			if err := r.store.UnlockJob(r.ctx, job.ID, r.ID()); err != nil {
				// TODO: Implament a cache? Should we return here?
				log.Printf("[Pool mgr %s] failed to unlock job %d: %q", r.helper.String(), job.ID, err)
				continue
			}
		}

		if job.LockedBy.String() == r.ID() {
			// Job is locked by us. We must have already attepted to create a runner for it. Skip.
			// TODO(gabriel-samfira): create an in-memory state of existing runners that we can easily
			// check for existing pending or idle runners. If we can't find any, attempt to allocate another
			// runner.
			log.Printf("[Pool mgr %s] job %d is locked by us", r.helper.String(), job.ID)
			continue
		}

		poolRR, ok := poolsCache.Get(job.Labels)
		if !ok {
			potentialPools, err := r.store.FindPoolsMatchingAllTags(r.ctx, r.helper.PoolType(), r.helper.ID(), job.Labels)
			if err != nil {
				log.Printf("[Pool mgr %s] error finding pools matching labels: %s", r.helper.String(), err)
				continue
			}
			poolRR = poolsCache.Add(job.Labels, potentialPools)
		}

		if poolRR.Len() == 0 {
			log.Printf("[Pool mgr %s] could not find pools with labels %s", r.helper.String(), strings.Join(job.Labels, ","))
			continue
		}

		runnerCreated := false
		if err := r.store.LockJob(r.ctx, job.ID, r.ID()); err != nil {
			log.Printf("[Pool mgr %s] could not lock job %d: %s", r.helper.String(), job.ID, err)
			continue
		}

		jobLabels := []string{
			fmt.Sprintf("%s%d", jobLabelPrefix, job.ID),
		}
		for i := 0; i < poolRR.Len(); i++ {
			pool, err := poolRR.Next()
			if err != nil {
				log.Printf("[PoolRR %s] could not find a pool to create a runner for job %d: %s", r.helper.String(), job.ID, err)
				break
			}

			log.Printf("[PoolRR %s] attempting to create a runner in pool %s for job %d", r.helper.String(), pool.ID, job.ID)
			if err := r.addRunnerToPool(pool, jobLabels); err != nil {
				log.Printf("[PoolRR] could not add runner to pool %s: %s", pool.ID, err)
				continue
			}
			log.Printf("[PoolRR %s] a new runner was added to pool %s as a response to queued job %d", r.helper.String(), pool.ID, job.ID)
			runnerCreated = true
			break
		}

		if !runnerCreated {
			log.Printf("[Pool mgr %s] could not create a runner for job %d; unlocking", r.helper.String(), job.ID)
			if err := r.store.UnlockJob(r.ctx, job.ID, r.ID()); err != nil {
				log.Printf("[Pool mgr %s] failed to unlock job: %d", r.helper.String(), job.ID)
				return errors.Wrap(err, "unlocking job")
			}
		}
	}

	if err := r.store.DeleteCompletedJobs(r.ctx); err != nil {
		log.Printf("[Pool mgr %s] failed to delete completed jobs: %q", r.helper.String(), err)
	}
	return nil
}
