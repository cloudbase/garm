// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package scaleset

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/cache"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/locking"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

func NewWorker(ctx context.Context, store dbCommon.Store, scaleSet params.ScaleSet, provider common.Provider) (*Worker, error) {
	consumerID := fmt.Sprintf("scaleset-worker-%s-%d", scaleSet.Name, scaleSet.ID)
	controllerInfo, err := store.ControllerInfo()
	if err != nil {
		return nil, fmt.Errorf("getting controller info: %w", err)
	}
	return &Worker{
		ctx:            ctx,
		controllerInfo: controllerInfo,
		consumerID:     consumerID,
		store:          store,
		provider:       provider,
		scaleSet:       scaleSet,
		runners:        make(map[string]params.Instance),
	}, nil
}

type Worker struct {
	ctx            context.Context
	consumerID     string
	controllerInfo params.ControllerInfo

	provider common.Provider
	store    dbCommon.Store
	scaleSet params.ScaleSet
	runners  map[string]params.Instance

	consumer dbCommon.Consumer

	listener *scaleSetListener

	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func (w *Worker) Stop() error {
	slog.DebugContext(w.ctx, "stopping scale set worker", "scale_set", w.consumerID)
	w.mux.Lock()
	defer w.mux.Unlock()

	if !w.running {
		return nil
	}

	w.consumer.Close()
	w.running = false
	if w.quit != nil {
		close(w.quit)
	}
	w.listener.Stop()
	return nil
}

func (w *Worker) Start() (err error) {
	slog.DebugContext(w.ctx, "starting scale set worker", "scale_set", w.consumerID)
	w.mux.Lock()
	defer w.mux.Unlock()

	if w.running {
		return nil
	}

	instances, err := w.store.ListScaleSetInstances(w.ctx, w.scaleSet.ID)
	if err != nil {
		return fmt.Errorf("listing scale set instances: %w", err)
	}

	for _, instance := range instances {
		if instance.Status == commonParams.InstanceCreating {
			// We're just starting up. We found an instance stuck in creating.
			// When a provider creates an instance, it sets the db instance to
			// creating and then issues an API call to the IaaS to create the
			// instance using some userdata it needs to come up. But the instance
			// will still need to call back home to fetch aditional metadata and
			// complete its setup. We should remove the instance as it is not
			// possible to reliably determine the state of the instance (if it's in
			// mid boot before it reached the phase where it runs the metadtata, or
			// if it already failed).
			instanceState := commonParams.InstancePendingDelete
			locking.Lock(instance.Name, w.consumerID)
			if instance.AgentID != 0 {
				scaleSetCli, err := w.GetScaleSetClient()
				if err != nil {
					slog.ErrorContext(w.ctx, "error getting scale set client", "error", err)
					return fmt.Errorf("getting scale set client: %w", err)
				}
				if err := scaleSetCli.RemoveRunner(w.ctx, instance.AgentID); err != nil {
					// scale sets use JIT runners. This means that we create the runner in github
					// before we create the actual instance that will use the credentials. We need
					// to remove the runner from github if it exists.
					if !errors.Is(err, runnerErrors.ErrNotFound) {
						if errors.Is(err, runnerErrors.ErrUnauthorized) {
							// we don't have access to remove the runner. This implies that our
							// credentials may have expired or ar incorrect.
							//
							// nolint:golangci-lint,godox
							// TODO(gabriel-samfira): we need to set the scale set as inactive and stop the listener (if any).
							slog.ErrorContext(w.ctx, "error removing runner", "runner_name", instance.Name, "error", err)
							w.runners[instance.ID] = instance
							locking.Unlock(instance.Name, false)
							continue
						}
						// The runner may have come up, registered and is currently running a
						// job, in which case, github will not allow us to remove it.
						runnerInstance, err := scaleSetCli.GetRunner(w.ctx, instance.AgentID)
						if err != nil {
							if !errors.Is(err, runnerErrors.ErrNotFound) {
								// We could not get info about the runner and it wasn't not found
								slog.ErrorContext(w.ctx, "error getting runner details", "error", err)
								w.runners[instance.ID] = instance
								locking.Unlock(instance.Name, false)
								continue
							}
						}
						if runnerInstance.Status == string(params.RunnerIdle) ||
							runnerInstance.Status == string(params.RunnerActive) {
							// This is a highly unlikely scenario, but let's account for it anyway.
							//
							// The runner is running a job or is idle. Mark it as running, as
							// it appears that it finished booting and is now running.
							//
							// NOTE: if the instance was in creating and it managed to boot, there
							// is a high chance that the we do not have a provider ID for the runner
							// inside our database. When removing the runner, the provider will attempt
							// to use the instance name instead of the provider ID, the same as when
							// creation of the instance fails and we try to clean up any lingering resources
							// in the provider.
							slog.DebugContext(w.ctx, "runner is running a job or is idle; not removing", "runner_name", instance.Name)
							instanceState = commonParams.InstanceRunning
						}
					}
				}
			}
			runnerUpdateParams := params.UpdateInstanceParams{
				Status: instanceState,
			}
			instance, err = w.store.UpdateInstance(w.ctx, instance.Name, runnerUpdateParams)
			if err != nil {
				if !errors.Is(err, runnerErrors.ErrNotFound) {
					locking.Unlock(instance.Name, false)
					return fmt.Errorf("updating runner %s: %w", instance.Name, err)
				}
			}
		} else if instance.Status == commonParams.InstanceDeleting {
			// Set the instance in deleting. It is assumed that the runner was already
			// removed from github either by github or by garm. Deleting status indicates
			// that it was already being handled by the provider. There should be no entry on
			// github for the runner if that was the case.
			// Setting it in pending_delete will cause the provider to try again, an operation
			// which is idempotent (if it's already deleted, the provider reports success).
			runnerUpdateParams := params.UpdateInstanceParams{
				Status: commonParams.InstancePendingDelete,
			}
			instance, err = w.store.UpdateInstance(w.ctx, instance.Name, runnerUpdateParams)
			if err != nil {
				if !errors.Is(err, runnerErrors.ErrNotFound) {
					locking.Unlock(instance.Name, false)
					return fmt.Errorf("updating runner %s: %w", instance.Name, err)
				}
			}
		}
		w.runners[instance.ID] = instance
		locking.Unlock(instance.Name, false)
	}

	consumer, err := watcher.RegisterConsumer(
		w.ctx, w.consumerID,
		watcher.WithAny(
			watcher.WithAll(
				watcher.WithScaleSetFilter(w.scaleSet),
				watcher.WithOperationTypeFilter(dbCommon.UpdateOperation),
			),
			watcher.WithScaleSetInstanceFilter(w.scaleSet),
		),
	)
	if err != nil {
		return fmt.Errorf("error registering consumer: %w", err)
	}
	defer func() {
		if err != nil {
			consumer.Close()
		}
	}()

	slog.DebugContext(w.ctx, "creating scale set listener")
	listener := newListener(w.ctx, w)

	if w.scaleSet.Enabled {
		slog.DebugContext(w.ctx, "starting scale set listener")
		if err := listener.Start(); err != nil {
			return fmt.Errorf("error starting listener: %w", err)
		}
	} else {
		slog.InfoContext(w.ctx, "scale set is disabled; not starting listener")
	}

	w.listener = listener
	w.consumer = consumer
	w.running = true
	w.quit = make(chan struct{})

	slog.DebugContext(w.ctx, "starting scale set worker loops", "scale_set", w.consumerID)
	go w.loop()
	go w.keepListenerAlive()
	go w.handleAutoScale()
	return nil
}

func (w *Worker) runnerByName() map[string]params.Instance {
	runners := make(map[string]params.Instance)
	for _, runner := range w.runners {
		runners[runner.Name] = runner
	}
	return runners
}

func (w *Worker) setRunnerDBStatus(runner string, status commonParams.InstanceStatus) (params.Instance, error) {
	updateParams := params.UpdateInstanceParams{
		Status: status,
	}
	newDbInstance, err := w.store.UpdateInstance(w.ctx, runner, updateParams)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return params.Instance{}, fmt.Errorf("updating runner %s: %w", runner, err)
		}
	}
	return newDbInstance, nil
}

func (w *Worker) removeRunnerFromGithubAndSetPendingDelete(runnerName string, agentID int64) error {
	scaleSetCli, err := w.GetScaleSetClient()
	if err != nil {
		return fmt.Errorf("getting scale set client: %w", err)
	}
	if err := scaleSetCli.RemoveRunner(w.ctx, agentID); err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return fmt.Errorf("removing runner %s: %w", runnerName, err)
		}
	}
	instance, err := w.setRunnerDBStatus(runnerName, commonParams.InstancePendingDelete)
	if err != nil {
		return fmt.Errorf("updating runner %s: %w", instance.Name, err)
	}
	w.runners[instance.ID] = instance
	return nil
}

func (w *Worker) reapTimedOutRunners(runners map[string]params.RunnerReference) (func(), error) {
	lockNames := []string{}

	unlockFn := func() {
		for _, name := range lockNames {
			slog.DebugContext(w.ctx, "unlockFn unlocking runner", "runner_name", name)
			locking.Unlock(name, false)
		}
	}

	for _, runner := range w.runners {
		if time.Since(runner.UpdatedAt).Minutes() < float64(w.scaleSet.RunnerBootstrapTimeout) {
			continue
		}
		switch runner.Status {
		case commonParams.InstancePendingDelete, commonParams.InstancePendingForceDelete,
			commonParams.InstanceDeleting, commonParams.InstanceDeleted:
			continue
		}

		if runner.RunnerStatus != params.RunnerPending && runner.RunnerStatus != params.RunnerInstalling {
			slog.DebugContext(w.ctx, "runner is not pending or installing; skipping", "runner_name", runner.Name)
			continue
		}
		if ghRunner, ok := runners[runner.Name]; !ok || ghRunner.GetStatus() == params.RunnerOffline {
			if ok := locking.TryLock(runner.Name, w.consumerID); !ok {
				slog.DebugContext(w.ctx, "runner is locked; skipping", "runner_name", runner.Name)
				continue
			}
			lockNames = append(lockNames, runner.Name)

			slog.InfoContext(
				w.ctx, "reaping timed-out/failed runner",
				"runner_name", runner.Name)

			if err := w.removeRunnerFromGithubAndSetPendingDelete(runner.Name, runner.AgentID); err != nil {
				slog.ErrorContext(w.ctx, "error removing runner", "runner_name", runner.Name, "error", err)
				unlockFn()
				return nil, fmt.Errorf("removing runner %s: %w", runner.Name, err)
			}
		}
	}
	return unlockFn, nil
}

func (w *Worker) consolidateRunnerState(runners []params.RunnerReference) error {
	w.mux.Lock()
	defer w.mux.Unlock()

	ghRunnersByName := make(map[string]params.RunnerReference)
	for _, runner := range runners {
		ghRunnersByName[runner.Name] = runner
	}

	scaleSetCli, err := w.GetScaleSetClient()
	if err != nil {
		return fmt.Errorf("getting scale set client: %w", err)
	}
	dbRunnersByName := w.runnerByName()
	// Cross check what exists in github with what we have in the database.
	for name, runner := range ghRunnersByName {
		status := runner.GetStatus()
		if _, ok := dbRunnersByName[name]; !ok {
			// runner appears to be active. Is it not managed by GARM?
			if status != params.RunnerIdle && status != params.RunnerActive {
				slog.InfoContext(w.ctx, "runner does not exist in GARM; removing from github", "runner_name", name)
				if err := scaleSetCli.RemoveRunner(w.ctx, runner.ID); err != nil {
					if errors.Is(err, runnerErrors.ErrNotFound) {
						continue
					}
					slog.ErrorContext(w.ctx, "error removing runner", "runner_name", runner.Name, "error", err)
				}
			}
			continue
		}
	}

	unlockFn, err := w.reapTimedOutRunners(ghRunnersByName)
	if err != nil {
		return fmt.Errorf("reaping timed out runners: %w", err)
	}
	defer unlockFn()

	// refresh the map. It may have been mutated above.
	dbRunnersByName = w.runnerByName()
	// Cross check what exists in the database with what we have in github.
	for name, runner := range dbRunnersByName {
		// in the case of scale sets, JIT configs re used. There is no situation
		// in which we create a runner in the DB and one does not exist in github.
		// We can safely assume that if the runner is not in github anymore, it can
		// be removed from the provider and the DB.
		switch runner.Status {
		case commonParams.InstancePendingDelete, commonParams.InstancePendingForceDelete,
			commonParams.InstanceDeleting, commonParams.InstanceDeleted:
			continue
		}

		if _, ok := ghRunnersByName[name]; !ok {
			if ok := locking.TryLock(name, w.consumerID); !ok {
				slog.DebugContext(w.ctx, "runner is locked; skipping", "runner_name", name)
				continue
			}
			// unlock the runner only after this function returns. This function also cross
			// checks between the provider and the database, and removes left over runners.
			// If we unlock early, the provider worker will attempt to remove runners that
			// we set in pending_delete. This function holds the mutex, so we won't see those
			// changes until we return. So we hold the instance lock here until we are done.
			// That way, even if the provider sees the pending_delete status, it won't act on
			// it until it manages to lock the instance.
			defer locking.Unlock(name, false)

			slog.InfoContext(w.ctx, "runner does not exist in github; removing from provider", "runner_name", name)
			instance, err := w.setRunnerDBStatus(runner.Name, commonParams.InstancePendingDelete)
			if err != nil {
				if !errors.Is(err, runnerErrors.ErrNotFound) {
					return fmt.Errorf("updating runner %s: %w", instance.Name, err)
				}
			}
			// We will get an update event anyway from the watcher, but updating the runner
			// here, will prevent race conditions if some other event is already in the queue
			// which involves this runner. For the duration of the lifetime of this function, we
			// hold the lock, so no race condition can occur.
			w.runners[runner.ID] = instance
		}
	}

	// Cross check what exists in the provider with the DB.
	pseudoPoolID, err := w.pseudoPoolID()
	if err != nil {
		return fmt.Errorf("getting pseudo pool ID: %w", err)
	}
	listParams := common.ListInstancesParams{
		ListInstancesV011: common.ListInstancesV011Params{
			ProviderBaseParams: common.ProviderBaseParams{
				ControllerInfo: w.controllerInfo,
			},
		},
	}

	providerRunners, err := w.provider.ListInstances(w.ctx, pseudoPoolID, listParams)
	if err != nil {
		return fmt.Errorf("listing instances: %w", err)
	}

	providerRunnersByName := make(map[string]commonParams.ProviderInstance)
	for _, runner := range providerRunners {
		providerRunnersByName[runner.Name] = runner
	}

	deleteInstanceParams := common.DeleteInstanceParams{
		DeleteInstanceV011: common.DeleteInstanceV011Params{
			ProviderBaseParams: common.ProviderBaseParams{
				ControllerInfo: w.controllerInfo,
			},
		},
	}

	// refresh the map. It may have been mutated above.
	dbRunnersByName = w.runnerByName()
	for _, runner := range providerRunners {
		if _, ok := dbRunnersByName[runner.Name]; !ok {
			slog.InfoContext(w.ctx, "runner does not exist in database; removing from provider", "runner_name", runner.Name)
			// There is no situation in which the runner will disappear from the provider
			// after it was removed from the database. The provider worker will remove the
			// instance from the provider nd mark the instance as deleted in the database.
			// It is the responsibility of the scaleset worker to then clean up the runners
			// in the deleted state.
			// That means that if we have a runner in the provider but not the DB, it is most
			// likely an inconsistency.
			if err := w.provider.DeleteInstance(w.ctx, runner.Name, deleteInstanceParams); err != nil {
				slog.ErrorContext(w.ctx, "error removing instance", "instance_name", runner.Name, "error", err)
			}
			continue
		}
	}

	for _, runner := range dbRunnersByName {
		switch runner.Status {
		case commonParams.InstancePendingDelete, commonParams.InstancePendingForceDelete,
			commonParams.InstanceDeleting, commonParams.InstanceDeleted:
			// This instance is already being deleted.
			continue
		}

		locked := locking.TryLock(runner.Name, w.consumerID)
		if !locked {
			slog.DebugContext(w.ctx, "runner is locked; skipping", "runner_name", runner.Name)
			continue
		}
		defer locking.Unlock(runner.Name, false)

		if _, ok := providerRunnersByName[runner.Name]; !ok {
			// The runner is not in the provider anymore. Remove it from the DB.
			slog.InfoContext(w.ctx, "runner does not exist in provider; removing from database", "runner_name", runner.Name)
			if err := w.removeRunnerFromGithubAndSetPendingDelete(runner.Name, runner.AgentID); err != nil {
				return fmt.Errorf("removing runner %s: %w", runner.Name, err)
			}
		}
	}

	return nil
}

func (w *Worker) pseudoPoolID() (string, error) {
	// This is temporary. We need to extend providers to know about scale sets.
	entity, err := w.scaleSet.GetEntity()
	if err != nil {
		return "", fmt.Errorf("getting entity: %w", err)
	}
	return fmt.Sprintf("%s-%s", w.scaleSet.Name, entity.ID), nil
}

func (w *Worker) handleScaleSetEvent(event dbCommon.ChangePayload) {
	scaleSet, ok := event.Payload.(params.ScaleSet)
	if !ok {
		slog.ErrorContext(w.ctx, "invalid payload for scale set type", "scale_set_type", event.EntityType, "payload", event.Payload)
		return
	}
	switch event.Operation {
	case dbCommon.UpdateOperation:
		slog.DebugContext(w.ctx, "got update operation")
		w.mux.Lock()

		if scaleSet.MaxRunners < w.scaleSet.MaxRunners || !scaleSet.Enabled {
			// we stop the listener if the scale set is disabled or if the max runners
			// is decreased. In the case where max runners changes but the scale set
			// is still enabled, we rely on the keepListenerAlive to restart the listener
			// which will listen for new messages with the changed max runners. This way
			// we don't have to potentially wait for 50 second for the max runner value
			// to be updated, in which time we might get more runners spawned than the
			// new max runner value.
			if err := w.listener.Stop(); err != nil {
				slog.ErrorContext(w.ctx, "error stopping listener", "error", err)
			}
		}
		w.scaleSet = scaleSet
		w.mux.Unlock()
	default:
		slog.DebugContext(w.ctx, "invalid operation type; ignoring", "operation_type", event.Operation)
	}
}

func (w *Worker) handleInstanceCleanup(instance params.Instance) error {
	if instance.Status == commonParams.InstanceDeleted {
		if err := w.store.DeleteInstanceByName(w.ctx, instance.Name); err != nil {
			if !errors.Is(err, runnerErrors.ErrNotFound) {
				return fmt.Errorf("deleting instance %s: %w", instance.ID, err)
			}
		}
	}
	return nil
}

func (w *Worker) handleInstanceEntityEvent(event dbCommon.ChangePayload) {
	instance, ok := event.Payload.(params.Instance)
	if !ok {
		slog.ErrorContext(w.ctx, "invalid payload for instance type", "instance_type", event.EntityType, "payload", event.Payload)
		return
	}
	switch event.Operation {
	case dbCommon.CreateOperation:
		slog.DebugContext(w.ctx, "got create operation")
		w.mux.Lock()
		w.runners[instance.ID] = instance
		w.mux.Unlock()
	case dbCommon.UpdateOperation:
		slog.DebugContext(w.ctx, "got update operation")
		w.mux.Lock()
		if instance.Status == commonParams.InstanceDeleted {
			if err := w.handleInstanceCleanup(instance); err != nil {
				slog.ErrorContext(w.ctx, "error cleaning up instance", "instance_id", instance.ID, "error", err)
			}
			w.mux.Unlock()
			return
		}
		oldInstance, ok := w.runners[instance.ID]
		w.runners[instance.ID] = instance

		if !ok {
			slog.DebugContext(w.ctx, "instance not found in local cache; ignoring", "instance_id", instance.ID)
			w.mux.Unlock()
			return
		}
		scaleSetCli, err := w.GetScaleSetClient()
		if err != nil {
			slog.ErrorContext(w.ctx, "error getting scale set client", "error", err)
			return
		}
		if oldInstance.RunnerStatus != instance.RunnerStatus && instance.RunnerStatus == params.RunnerIdle {
			serviceRuner, err := scaleSetCli.GetRunner(w.ctx, instance.AgentID)
			if err != nil {
				slog.ErrorContext(w.ctx, "error getting runner details", "error", err)
				w.mux.Unlock()
				return
			}
			status, ok := serviceRuner.Status.(string)
			if !ok {
				slog.ErrorContext(w.ctx, "error getting runner status", "runner_id", instance.AgentID)
				w.mux.Unlock()
				return
			}
			if status != string(params.RunnerIdle) && status != string(params.RunnerActive) {
				// nolint:golangci-lint,godox
				// TODO: Wait for the status to change for a while (30 seconds?). Mark the instance as
				// pending_delete if the runner never comes online.
				w.mux.Unlock()
				return
			}
		}
		w.mux.Unlock()
	case dbCommon.DeleteOperation:
		slog.DebugContext(w.ctx, "got delete operation")
		w.mux.Lock()
		delete(w.runners, instance.ID)
		w.mux.Unlock()
	default:
		slog.DebugContext(w.ctx, "invalid operation type; ignoring", "operation_type", event.Operation)
	}
}

func (w *Worker) handleEvent(event dbCommon.ChangePayload) {
	switch event.EntityType {
	case dbCommon.ScaleSetEntityType:
		slog.DebugContext(w.ctx, "got scaleset event")
		w.handleScaleSetEvent(event)
	case dbCommon.InstanceEntityType:
		slog.DebugContext(w.ctx, "got instance event")
		w.handleInstanceEntityEvent(event)
	default:
		slog.DebugContext(w.ctx, "invalid entity type; ignoring", "entity_type", event.EntityType)
	}
}

func (w *Worker) loop() {
	defer w.Stop()

	for {
		select {
		case <-w.quit:
			return
		case event, ok := <-w.consumer.Watch():
			if !ok {
				slog.InfoContext(w.ctx, "consumer channel closed")
				return
			}
			go w.handleEvent(event)
		case <-w.ctx.Done():
			slog.DebugContext(w.ctx, "context done")
			return
		}
	}
}

func (w *Worker) sleepWithCancel(sleepTime time.Duration) (canceled bool) {
	ticker := time.NewTicker(sleepTime)
	defer ticker.Stop()

	select {
	case <-ticker.C:
		return false
	case <-w.quit:
	case <-w.ctx.Done():
	}
	return true
}

func (w *Worker) keepListenerAlive() {
	var backoff time.Duration
	for {
		w.mux.Lock()
		if !w.scaleSet.Enabled {
			if canceled := w.sleepWithCancel(2 * time.Second); canceled {
				slog.DebugContext(w.ctx, "worker is stopped; exiting keepListenerAlive")
				w.mux.Unlock()
				return
			}
			w.mux.Unlock()
			continue
		}
		// noop if already started. If the scaleset was just enabled, we need to
		// start the listener here, or the <-w.listener.Wait() channel receive bellow
		// will block forever, even if we start the listener, as a nil channel will
		// block forever.
		w.listener.Start()
		w.mux.Unlock()

		select {
		case <-w.quit:
			return
		case <-w.ctx.Done():
			return
		case <-w.listener.Wait():
			slog.DebugContext(w.ctx, "listener is stopped; attempting to restart")
			w.mux.Lock()
			if !w.scaleSet.Enabled {
				w.mux.Unlock()
				continue
			}
			w.mux.Unlock()
			for {
				w.mux.Lock()
				w.listener.Stop() // cleanup
				if !w.scaleSet.Enabled {
					w.mux.Unlock()
					break
				}
				slog.DebugContext(w.ctx, "attempting to restart")
				if err := w.listener.Start(); err != nil {
					w.mux.Unlock()
					slog.ErrorContext(w.ctx, "error restarting listener", "error", err)
					switch {
					case backoff > 60*time.Second:
						backoff = 60 * time.Second
					case backoff == 0:
						backoff = 5 * time.Second
						slog.InfoContext(w.ctx, "backing off restart attempt", "backoff", backoff)
					default:
						backoff *= 2
					}
					slog.ErrorContext(w.ctx, "error restarting listener", "error", err, "backoff", backoff)
					if canceled := w.sleepWithCancel(backoff); canceled {
						slog.DebugContext(w.ctx, "listener restart canceled")
						return
					}
					continue
				}
				w.mux.Unlock()
				break
			}
		}
	}
}

func (w *Worker) handleScaleUp(target, current uint) {
	if !w.scaleSet.Enabled {
		slog.DebugContext(w.ctx, "scale set is disabled; not scaling up")
		return
	}

	if target <= current {
		slog.DebugContext(w.ctx, "target is less than or equal to current; not scaling up")
		return
	}

	controllerConfig, err := w.store.ControllerInfo()
	if err != nil {
		slog.ErrorContext(w.ctx, "error getting controller config", "error", err)
		return
	}

	scaleSetCli, err := w.GetScaleSetClient()
	if err != nil {
		slog.ErrorContext(w.ctx, "error getting scale set client", "error", err)
		return
	}
	for i := current; i < target; i++ {
		newRunnerName := fmt.Sprintf("%s-%s", w.scaleSet.GetRunnerPrefix(), util.NewID())
		jitConfig, err := scaleSetCli.GenerateJitRunnerConfig(w.ctx, newRunnerName, w.scaleSet.ScaleSetID)
		if err != nil {
			slog.ErrorContext(w.ctx, "error generating jit config", "error", err)
			continue
		}
		slog.DebugContext(w.ctx, "creating new runner", "runner_name", newRunnerName)
		decodedJit, err := jitConfig.DecodedJITConfig()
		if err != nil {
			slog.ErrorContext(w.ctx, "error decoding jit config", "error", err)
			continue
		}
		runnerParams := params.CreateInstanceParams{
			Name:              newRunnerName,
			Status:            commonParams.InstancePendingCreate,
			RunnerStatus:      params.RunnerPending,
			OSArch:            w.scaleSet.OSArch,
			OSType:            w.scaleSet.OSType,
			CallbackURL:       controllerConfig.CallbackURL,
			MetadataURL:       controllerConfig.MetadataURL,
			CreateAttempt:     1,
			GitHubRunnerGroup: w.scaleSet.GitHubRunnerGroup,
			JitConfiguration:  decodedJit,
			AgentID:           jitConfig.Runner.ID,
		}

		dbInstance, err := w.store.CreateScaleSetInstance(w.ctx, w.scaleSet.ID, runnerParams)
		if err != nil {
			slog.ErrorContext(w.ctx, "error creating instance", "error", err)
			if err := scaleSetCli.RemoveRunner(w.ctx, jitConfig.Runner.ID); err != nil {
				slog.ErrorContext(w.ctx, "error deleting runner", "error", err)
			}
			continue
		}
		w.runners[dbInstance.ID] = dbInstance

		_, err = scaleSetCli.GetRunner(w.ctx, jitConfig.Runner.ID)
		if err != nil {
			slog.ErrorContext(w.ctx, "error getting runner details", "error", err)
			continue
		}
	}
}

func (w *Worker) waitForToolsOrCancel() (hasTools, stopped bool) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	select {
	case <-ticker.C:
		entity, err := w.scaleSet.GetEntity()
		if err != nil {
			slog.ErrorContext(w.ctx, "error getting entity", "error", err)
		}
		if _, err := cache.GetGithubToolsCache(entity.ID); err != nil {
			slog.DebugContext(w.ctx, "tools not found in cache; waiting for tools")
			return false, false
		}
		return true, false
	case <-w.quit:
		return false, true
	case <-w.ctx.Done():
		return false, true
	}
}

func (w *Worker) handleScaleDown(target, current uint) {
	delta := current - target
	if delta <= 0 {
		return
	}
	removed := 0
	candidates := []params.Instance{}
	for _, runner := range w.runners {
		locked := locking.TryLock(runner.Name, w.consumerID)
		if !locked {
			slog.DebugContext(w.ctx, "runner is locked; skipping", "runner_name", runner.Name)
			continue
		}
		switch runner.Status {
		case commonParams.InstanceRunning:
			if runner.RunnerStatus != params.RunnerActive {
				candidates = append(candidates, runner)
			}
		case commonParams.InstancePendingDelete, commonParams.InstancePendingForceDelete,
			commonParams.InstanceDeleting, commonParams.InstanceDeleted:
			removed++
			locking.Unlock(runner.Name, true)
			continue
		default:
			slog.DebugContext(w.ctx, "runner is not in a valid state; skipping", "runner_name", runner.Name, "runner_status", runner.Status)
			locking.Unlock(runner.Name, false)
			continue
		}
		locking.Unlock(runner.Name, false)
	}

	if removed >= int(delta) {
		return
	}

	for _, runner := range candidates {
		if removed >= int(delta) {
			break
		}

		locked := locking.TryLock(runner.Name, w.consumerID)
		if !locked {
			slog.DebugContext(w.ctx, "runner is locked; skipping", "runner_name", runner.Name)
			continue
		}

		switch runner.Status {
		case commonParams.InstancePendingCreate, commonParams.InstanceRunning:
		case commonParams.InstancePendingDelete, commonParams.InstancePendingForceDelete,
			commonParams.InstanceDeleting, commonParams.InstanceDeleted:
			removed++
			locking.Unlock(runner.Name, true)
			continue
		default:
			slog.DebugContext(w.ctx, "runner is not in a valid state; skipping", "runner_name", runner.Name, "runner_status", runner.Status)
			locking.Unlock(runner.Name, false)
			continue
		}

		switch runner.RunnerStatus {
		case params.RunnerTerminated, params.RunnerActive:
			slog.DebugContext(w.ctx, "runner is not in a valid state; skipping", "runner_name", runner.Name, "runner_status", runner.RunnerStatus)
			locking.Unlock(runner.Name, false)
			continue
		}

		scaleSetCli, err := w.GetScaleSetClient()
		if err != nil {
			slog.ErrorContext(w.ctx, "error getting scale set client", "error", err)
			return
		}
		slog.DebugContext(w.ctx, "removing runner", "runner_name", runner.Name)
		if err := scaleSetCli.RemoveRunner(w.ctx, runner.AgentID); err != nil {
			if !errors.Is(err, runnerErrors.ErrNotFound) {
				slog.ErrorContext(w.ctx, "error removing runner", "runner_name", runner.Name, "error", err)
				locking.Unlock(runner.Name, false)
				continue
			}
		}
		runnerUpdateParams := params.UpdateInstanceParams{
			Status: commonParams.InstancePendingDelete,
		}
		if _, err := w.store.UpdateInstance(w.ctx, runner.Name, runnerUpdateParams); err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				// The error seems to be that the instance was removed from the database. We still had it in our
				// state, so either the update never came from the watcher or something else happened.
				// Remove it from the local cache.
				delete(w.runners, runner.ID)
				removed++
				locking.Unlock(runner.Name, true)
				continue
			}
			// nolint:golangci-lint,godox
			// TODO: This should not happen, unless there is some issue with the database.
			// The UpdateInstance() function should add tenacity, but even in that case, if it
			// still errors out, we need to handle it somehow.
			slog.ErrorContext(w.ctx, "error updating runner", "runner_name", runner.Name, "error", err)
			locking.Unlock(runner.Name, false)
			continue
		}
		removed++
		locking.Unlock(runner.Name, false)
	}
}

func (w *Worker) handleAutoScale() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastMsg := ""
	lastMsgDebugLog := func(msg string, targetRunners, currentRunners uint) {
		if lastMsg != msg {
			slog.DebugContext(w.ctx, msg, "current_runners", currentRunners, "target_runners", targetRunners)
			lastMsg = msg
		}
	}

	for {
		hasTools, stopped := w.waitForToolsOrCancel()
		if stopped {
			slog.DebugContext(w.ctx, "worker is stopped; exiting handleAutoScale")
			return
		}

		if !hasTools {
			w.sleepWithCancel(1 * time.Second)
			continue
		}

		select {
		case <-w.quit:
			return
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.mux.Lock()
			for _, instance := range w.runners {
				if err := w.handleInstanceCleanup(instance); err != nil {
					slog.ErrorContext(w.ctx, "error cleaning up instance", "instance_id", instance.ID, "error", err)
				}
			}
			var desiredRunners uint
			if w.scaleSet.DesiredRunnerCount > 0 {
				desiredRunners = uint(w.scaleSet.DesiredRunnerCount)
			}
			targetRunners := min(w.scaleSet.MinIdleRunners+desiredRunners, w.scaleSet.MaxRunners)

			currentRunners := uint(len(w.runners))
			if currentRunners == targetRunners {
				lastMsgDebugLog("desired runner count reached", targetRunners, currentRunners)
				w.mux.Unlock()
				continue
			}

			if currentRunners < targetRunners {
				lastMsgDebugLog("scaling up", targetRunners, currentRunners)
				w.handleScaleUp(targetRunners, currentRunners)
			} else {
				lastMsgDebugLog("attempting to scale down", targetRunners, currentRunners)
				w.handleScaleDown(targetRunners, currentRunners)
			}
			w.mux.Unlock()
		}
	}
}
