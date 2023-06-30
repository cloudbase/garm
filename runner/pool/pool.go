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
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var (
	poolIDLabelprefix     = "runner-pool-id:"
	controllerLabelPrefix = "runner-controller-id:"
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

func (r *basePoolManager) HandleWorkflowJob(job params.WorkflowJob) (err error) {
	if err := r.helper.ValidateOwner(job); err != nil {
		return errors.Wrap(err, "validating owner")
	}

	defer func() {
		if err != nil && errors.Is(err, runnerErrors.ErrUnauthorized) {
			r.setPoolRunningState(false, fmt.Sprintf("failed to handle job: %q", err))
		}
	}()

	switch job.Action {
	case "queued":
		// Create instance in database and set it to pending create.
		// If we already have an idle runner around, that runner will pick up the job
		// and trigger an "in_progress" update from github (see bellow), which in turn will set the
		// runner state of the instance to "active". The ensureMinIdleRunners() function will
		// exclude that runner from available runners and attempt to ensure
		// the needed number of runners.
		if err := r.acquireNewInstance(job); err != nil {
			log.Printf("failed to add instance: %s", err)
		}
	case "completed":
		// ignore the error here. A completed job may not have a runner name set
		// if it was never assigned to a runner, and was canceled.
		runnerInfo, err := r.getRunnerDetailsFromJob(job)
		if err != nil {
			if !errors.Is(err, runnerErrors.ErrUnauthorized) {
				// Unassigned jobs will have an empty runner_name.
				// We also need to ignore not found errors, as we may get a webhook regarding
				// a workflow that is handled by a runner at a different hierarchy level.
				return nil
			}
			return errors.Wrap(err, "updating runner")
		}

		// update instance workload state.
		if err := r.setInstanceRunnerStatus(runnerInfo.Name, providerCommon.RunnerTerminated); err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				return nil
			}
			log.Printf("failed to update runner %s status", util.SanitizeLogEntry(runnerInfo.Name))
			return errors.Wrap(err, "updating runner")
		}
		log.Printf("marking instance %s as pending_delete", util.SanitizeLogEntry(runnerInfo.Name))
		if err := r.setInstanceStatus(runnerInfo.Name, providerCommon.InstancePendingDelete, nil); err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				return nil
			}
			log.Printf("failed to update runner %s status", util.SanitizeLogEntry(runnerInfo.Name))
			return errors.Wrap(err, "updating runner")
		}
	case "in_progress":
		// in_progress jobs must have a runner name/ID assigned. Sometimes github will send a hook without
		// a runner set. In such cases, we attemt to fetch it from the API.
		runnerInfo, err := r.getRunnerDetailsFromJob(job)
		if err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				// This is most likely a runner we're not managing. If we define a repo from within an org
				// and also define that same org, we will get a hook from github from both the repo and the org
				// regarding the same workflow. We look for the runner in the database, and make sure it exists and is
				// part of a pool that this manager is responsible for. A not found error here will most likely mean
				// that we are not responsible for that runner, and we should ignore it.
				return nil
			}
			return errors.Wrap(err, "determining runner name")
		}

		// update instance workload state.
		if err := r.setInstanceRunnerStatus(runnerInfo.Name, providerCommon.RunnerActive); err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				return nil
			}
			log.Printf("failed to update runner %s status", util.SanitizeLogEntry(runnerInfo.Name))
			return errors.Wrap(err, "updating runner")
		}
	}
	return nil
}

func (r *basePoolManager) startLoopForFunction(f func() error, interval time.Duration, name string) {
	log.Printf("starting %s loop for %s", name, r.helper.String())
	ticker := time.NewTicker(interval)
	r.wg.Add(1)

	defer func() {
		log.Printf("%s loop exited for pool %s", name, r.helper.String())
		ticker.Stop()
		r.wg.Done()
	}()

	for {
		switch r.managerIsRunning {
		case true:
			select {
			case <-ticker.C:
				if err := f(); err != nil {
					log.Printf("%s: %q", name, err)
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
		return fmt.Errorf("failed to update tools for repo %s: %w", r.helper.String(), err)
	}
	r.mux.Lock()
	r.tools = tools
	r.mux.Unlock()
	return nil
}

func (r *basePoolManager) checkCanAuthenticateToGithub() error {
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

	err = r.runnerCleanup()
	if err != nil {
		if errors.Is(err, runnerErrors.ErrUnauthorized) {
			r.setPoolRunningState(false, err.Error())
			r.waitForTimeoutOrCanceled(common.UnauthorizedBackoffTimer)
			return fmt.Errorf("failed to clean runners for %s: %w", r.helper.String(), err)
		}
	}
	// We still set the pool as running, even if we failed to clean up runners.
	// We only set the pool as not running if we fail to authenticate to github. This is done
	// to avoid being rate limited by github when we have a bad token.
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
			log.Printf("runner %s is not managed by a pool belonging to %s", *run.Name, r.helper.String())
			continue
		}
		runnerNames[*run.Name] = true
	}

	for _, instance := range dbInstances {
		lockAcquired := r.keyMux.TryLock(instance.Name)
		if !lockAcquired {
			log.Printf("failed to acquire lock for instance %s", instance.Name)
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
			log.Printf("runner %s is still installing, give it a chance to finish", instance.Name)
			continue
		}

		if time.Since(instance.UpdatedAt).Minutes() < 5 {
			// instance was updated recently. We give it a chance to register itself in github.
			log.Printf("instance %s was updated recently, skipping check", instance.Name)
			continue
		}

		if ok := runnerNames[instance.Name]; !ok {
			// Set pending_delete on DB field. Allow consolidate() to remove it.
			if err := r.setInstanceStatus(instance.Name, providerCommon.InstancePendingDelete, nil); err != nil {
				log.Printf("failed to update runner %s status", instance.Name)
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
			log.Printf("runner %s is not managed by a pool belonging to %s", *run.Name, r.helper.String())
			continue
		}
		runnersByName[*run.Name] = run
	}

	for _, instance := range dbInstances {
		lockAcquired := r.keyMux.TryLock(instance.Name)
		if !lockAcquired {
			log.Printf("failed to acquire lock for instance %s", instance.Name)
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
			log.Printf("reaping timed-out/failed runner %s", instance.Name)
			if err := r.ForceDeleteRunner(instance); err != nil {
				log.Printf("failed to update runner %s status", instance.Name)
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
			log.Printf("runner %s is not managed by a pool belonging to %s", *runner.Name, r.helper.String())
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
			log.Printf("Runner %s has no database entry in garm, removing from github", *runner.Name)
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
			log.Printf("updating instances cache for pool %s", pool.ID)
			poolInstances, err = provider.ListInstances(r.ctx, pool.ID)
			if err != nil {
				return errors.Wrapf(err, "fetching instances for pool %s", pool.ID)
			}
			poolInstanceCache[pool.ID] = poolInstances
		}

		lockAcquired := r.keyMux.TryLock(dbInstance.Name)
		if !lockAcquired {
			log.Printf("failed to acquire lock for instance %s", dbInstance.Name)
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
				log.Printf("Runner instance for %s is no longer on the provider, removing from github", dbInstance.Name)
				resp, err := r.helper.RemoveGithubRunner(*runner.ID)
				if err != nil {
					// Removed in the meantime?
					if resp != nil && resp.StatusCode == http.StatusNotFound {
						log.Printf("runner dissapeared from github")
					} else {
						return errors.Wrap(err, "removing runner from github")
					}
				}
				// Remove the database entry for the runner.
				log.Printf("Removing %s from database", dbInstance.Name)
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
				log.Printf("instance %s is online but github reports runner as offline", dbInstance.Name)
				return nil
			} else {
				log.Printf("instance %s was found in stopped state; starting", dbInstance.Name)
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

func (r *basePoolManager) setInstanceRunnerStatus(runnerName string, status providerCommon.RunnerStatus) error {
	updateParams := params.UpdateInstanceParams{
		RunnerStatus: status,
	}

	if err := r.updateInstance(runnerName, updateParams); err != nil {
		return errors.Wrap(err, "updating runner state")
	}
	return nil
}

func (r *basePoolManager) updateInstance(runnerName string, update params.UpdateInstanceParams) error {
	runner, err := r.fetchInstance(runnerName)
	if err != nil {
		return errors.Wrap(err, "fetching instance")
	}

	if _, err := r.store.UpdateInstance(r.ctx, runner.ID, update); err != nil {
		return errors.Wrap(err, "updating runner state")
	}
	return nil
}

func (r *basePoolManager) setInstanceStatus(runnerName string, status providerCommon.InstanceStatus, providerFault []byte) error {
	updateParams := params.UpdateInstanceParams{
		Status:        status,
		ProviderFault: providerFault,
	}

	if err := r.updateInstance(runnerName, updateParams); err != nil {
		return errors.Wrap(err, "updating runner state")
	}
	return nil
}

func (r *basePoolManager) acquireNewInstance(job params.WorkflowJob) error {
	requestedLabels := job.WorkflowJob.Labels
	if len(requestedLabels) == 0 {
		// no labels were requested.
		return nil
	}

	pool, err := r.helper.FindPoolByTags(requestedLabels)
	if err != nil {
		if errors.Is(err, runnerErrors.ErrNotFound) {
			log.Printf("failed to find an enabled pool with required labels: %s", strings.Join(requestedLabels, ", "))
			return nil
		}
		return errors.Wrap(err, "fetching suitable pool")
	}
	log.Printf("adding new runner with requested tags %s in pool %s", util.SanitizeLogEntry(strings.Join(job.WorkflowJob.Labels, ", ")), util.SanitizeLogEntry(pool.ID))

	if !pool.Enabled {
		log.Printf("selected pool (%s) is disabled", pool.ID)
		return nil
	}

	poolInstances, err := r.store.PoolInstanceCount(r.ctx, pool.ID)
	if err != nil {
		return errors.Wrap(err, "fetching instances")
	}

	if poolInstances >= int64(pool.MaxRunners) {
		log.Printf("max_runners (%d) reached for pool %s, skipping...", pool.MaxRunners, pool.ID)
		return nil
	}

	instances, err := r.store.ListPoolInstances(r.ctx, pool.ID)
	if err != nil {
		return errors.Wrap(err, "fetching instances")
	}

	idleWorkers := 0
	for _, inst := range instances {
		if providerCommon.RunnerStatus(inst.RunnerStatus) == providerCommon.RunnerIdle &&
			providerCommon.InstanceStatus(inst.Status) == providerCommon.InstanceRunning {
			idleWorkers++
		}
	}

	// Skip creating a new runner if we have at least one idle runner and the minimum is already satisfied.
	// This should work even for pools that define a MinIdleRunner of 0.
	if int64(idleWorkers) > 0 && int64(idleWorkers) >= int64(pool.MinIdleRunners) {
		log.Printf("we have enough min_idle_runners (%d) for pool %s, skipping...", pool.MinIdleRunners, pool.ID)
		return nil
	}

	if err := r.AddRunner(r.ctx, pool.ID); err != nil {
		log.Printf("failed to add runner to pool %s", pool.ID)
		return errors.Wrap(err, "adding runner")
	}
	return nil
}

func (r *basePoolManager) AddRunner(ctx context.Context, poolID string) error {
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
	log.Printf("sleeping for %.2f minutes", timeout.Minutes())
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
					log.Printf("failed to cleanup instance: %s", instanceIDToDelete)
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
		log.Printf("runner name not found in workflow job, attempting to fetch from API")
		runnerInfo, err = r.helper.GetRunnerInfoFromWorkflow(job)
		if err != nil {
			return params.RunnerInfo{}, errors.Wrap(err, "fetching runner name from API")
		}
	}

	runnerDetails, err := r.store.GetInstanceByName(context.Background(), runnerInfo.Name)
	if err != nil {
		log.Printf("could not find runner details for %s", util.SanitizeLogEntry(runnerInfo.Name))
		return params.RunnerInfo{}, errors.Wrap(err, "fetching runner details")
	}

	if _, err := r.helper.GetPoolByID(runnerDetails.PoolID); err != nil {
		log.Printf("runner %s (pool ID: %s) does not belong to any pool we manage: %s", runnerDetails.Name, runnerDetails.PoolID, err)
		return params.RunnerInfo{}, errors.Wrap(err, "fetching pool for instance")
	}
	return runnerInfo, nil
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
	if !pool.Enabled {
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
		if providerCommon.RunnerStatus(inst.RunnerStatus) == providerCommon.RunnerIdle &&
			providerCommon.InstanceStatus(inst.Status) == providerCommon.InstanceRunning &&
			time.Since(inst.UpdatedAt).Minutes() > 5 {
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
			log.Printf("failed to acquire lock for instance %s", instanceToDelete.Name)
			continue
		}
		defer r.keyMux.Unlock(instanceToDelete.Name, false)

		g.Go(func() error {
			log.Printf("scaling down idle worker %s from pool %s\n", instanceToDelete.Name, pool.ID)
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

func (r *basePoolManager) ensureIdleRunnersForOnePool(pool params.Pool) error {
	if !pool.Enabled {
		return nil
	}
	existingInstances, err := r.store.ListPoolInstances(r.ctx, pool.ID)
	if err != nil {
		return fmt.Errorf("failed to ensure minimum idle workers for pool %s: %w", pool.ID, err)
	}

	if uint(len(existingInstances)) >= pool.MaxRunners {
		log.Printf("max workers (%d) reached for pool %s, skipping idle worker creation", pool.MaxRunners, pool.ID)
		return nil
	}

	idleOrPendingWorkers := []params.Instance{}
	for _, inst := range existingInstances {
		if providerCommon.RunnerStatus(inst.RunnerStatus) != providerCommon.RunnerActive {
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
		log.Printf("adding new idle worker to pool %s", pool.ID)
		if err := r.AddRunner(r.ctx, pool.ID); err != nil {
			return fmt.Errorf("failed to add new instance for pool %s: %w", pool.ID, err)
		}
	}
	return nil
}

func (r *basePoolManager) retryFailedInstancesForOnePool(ctx context.Context, pool params.Pool) error {
	if !pool.Enabled {
		return nil
	}

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

		lockAcquired := r.keyMux.TryLock(instance.Name)
		if !lockAcquired {
			log.Printf("failed to acquire lock for instance %s", instance.Name)
			continue
		}

		instance := instance
		g.Go(func() error {
			defer r.keyMux.Unlock(instance.Name, false)
			// NOTE(gabriel-samfira): this is done in parallel. If there are many failed instances
			// this has the potential to create many API requests to the target provider.
			// TODO(gabriel-samfira): implement request throttling.
			if err := r.deleteInstanceFromProvider(errCtx, instance); err != nil {
				log.Printf("failed to delete instance %s from provider: %s", instance.Name, err)
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
			log.Printf("queueing previously failed instance %s for retry", instance.Name)
			// Set instance to pending create and wait for retry.
			if err := r.updateInstance(instance.Name, updateParams); err != nil {
				log.Printf("failed to update runner %s status", instance.Name)
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

	for _, instance := range instances {
		if instance.Status != providerCommon.InstancePendingDelete {
			// not in pending_delete status. Skip.
			continue
		}

		lockAcquired := r.keyMux.TryLock(instance.Name)
		if !lockAcquired {
			log.Printf("failed to acquire lock for instance %s", instance.Name)
			continue
		}

		// Set the status to deleting before launching the goroutine that removes
		// the runner from the provider (which can take a long time).
		if err := r.setInstanceStatus(instance.Name, providerCommon.InstanceDeleting, nil); err != nil {
			log.Printf("failed to update runner %s status: %s", instance.Name, err)
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
					// failed to remove from provider. Set the status back to pending_delete, which
					// will retry the operation.
					if err := r.setInstanceStatus(instance.Name, providerCommon.InstancePendingDelete, nil); err != nil {
						log.Printf("failed to update runner %s status", instance.Name)
					}
				}
			}(instance)

			err = r.deleteInstanceFromProvider(r.ctx, instance)
			if err != nil {
				return fmt.Errorf("failed to remove instance from provider: %w", err)
			}

			if deleteErr := r.store.DeleteInstance(r.ctx, instance.PoolID, instance.Name); deleteErr != nil {
				return fmt.Errorf("failed to delete instance from database: %w", deleteErr)
			}
			deleteMux = true
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

		lockAcquired := r.keyMux.TryLock(instance.Name)
		if !lockAcquired {
			log.Printf("failed to acquire lock for instance %s", instance.Name)
			continue
		}

		// Set the instance to "creating" before launching the goroutine. This will ensure that addPendingInstances()
		// won't attempt to create the runner a second time.
		if err := r.setInstanceStatus(instance.Name, providerCommon.InstanceCreating, nil); err != nil {
			log.Printf("failed to update runner %s status: %s", instance.Name, err)
			r.keyMux.Unlock(instance.Name, false)
			// We failed to transition the instance to Creating. This means that garm will retry to create this instance
			// when the loop runs again and we end up with multiple instances.
			continue
		}

		go func(instance params.Instance) {
			defer r.keyMux.Unlock(instance.Name, false)
			log.Printf("creating instance %s in pool %s", instance.Name, instance.PoolID)
			if err := r.addInstanceToProvider(instance); err != nil {
				log.Printf("failed to add instance to provider: %s", err)
				errAsBytes := []byte(err.Error())
				if err := r.setInstanceStatus(instance.Name, providerCommon.InstanceError, errAsBytes); err != nil {
					log.Printf("failed to update runner %s status: %s", instance.Name, err)
				}
				log.Printf("failed to create instance in provider: %s", err)
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
	r.checkCanAuthenticateToGithub() //nolint

	go r.startLoopForFunction(r.runnerCleanup, common.PoolReapTimeoutInterval, "timeout_reaper")
	go r.startLoopForFunction(r.scaleDown, common.PoolScaleDownInterval, "scale_down")
	go r.startLoopForFunction(r.deletePendingInstances, common.PoolConsilitationInterval, "consolidate[delete_pending]")
	go r.startLoopForFunction(r.addPendingInstances, common.PoolConsilitationInterval, "consolidate[add_pending]")
	go r.startLoopForFunction(r.ensureMinIdleRunners, common.PoolConsilitationInterval, "consolidate[ensure_min_idle]")
	go r.startLoopForFunction(r.retryFailedInstances, common.PoolConsilitationInterval, "consolidate[retry_failed]")
	go r.startLoopForFunction(r.updateTools, common.PoolToolUpdateInterval, "update_tools")
	go r.startLoopForFunction(r.checkCanAuthenticateToGithub, common.UnauthorizedBackoffTimer, "bad_auth_backoff")
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
					log.Printf("runner with agent id %d was not found in github", runner.AgentID)
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
	log.Printf("setting instance status for: %v", runner.Name)

	if err := r.setInstanceStatus(runner.Name, providerCommon.InstancePendingDelete, nil); err != nil {
		log.Printf("failed to update runner %s status", runner.Name)
		return errors.Wrap(err, "updating runner")
	}
	return nil
}
