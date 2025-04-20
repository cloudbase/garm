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
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/locking"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util/github/scalesets"
)

func NewWorker(ctx context.Context, store dbCommon.Store, scaleSet params.ScaleSet, provider common.Provider, ghCli common.GithubClient) (*Worker, error) {
	consumerID := fmt.Sprintf("scaleset-worker-%s-%d", scaleSet.Name, scaleSet.ID)
	controllerInfo, err := store.ControllerInfo()
	if err != nil {
		return nil, fmt.Errorf("getting controller info: %w", err)
	}
	scaleSetCli, err := scalesets.NewClient(ghCli)
	if err != nil {
		return nil, fmt.Errorf("creating scale set client: %w", err)
	}
	return &Worker{
		ctx:            ctx,
		controllerInfo: controllerInfo,
		consumerID:     consumerID,
		store:          store,
		provider:       provider,
		scaleSet:       scaleSet,
		ghCli:          ghCli,
		scaleSetCli:    scaleSetCli,
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

	ghCli       common.GithubClient
	scaleSetCli *scalesets.ScaleSetClient
	consumer    dbCommon.Consumer

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
		w.quit = nil
	}
	w.listener.Stop()
	w.listener = nil
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
		w.runners[instance.ID] = instance
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
			w.consumer = nil
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

func (w *Worker) SetGithubClient(client common.GithubClient) error {
	w.mux.Lock()
	defer w.mux.Unlock()

	if err := w.listener.Stop(); err != nil {
		slog.ErrorContext(w.ctx, "error stopping listener", "error", err)
	}

	w.ghCli = client
	scaleSetCli, err := scalesets.NewClient(client)
	if err != nil {
		return fmt.Errorf("error creating scale set client: %w", err)
	}
	w.scaleSetCli = scaleSetCli
	return nil
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
		// TODO: should we kick off auto-scaling if desired runner count changes?
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
	case dbCommon.UpdateOperation, dbCommon.CreateOperation:
		slog.DebugContext(w.ctx, "got update operation")
		w.mux.Lock()
		w.runners[instance.ID] = instance
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
		slog.DebugContext(w.ctx, "got scaleset event", "event", event)
		w.handleScaleSetEvent(event)
	case dbCommon.InstanceEntityType:
		slog.DebugContext(w.ctx, "got instance event", "event", event)
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
		// noop if already started
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
				w.listener.Stop() //cleanup
				if !w.scaleSet.Enabled {
					w.mux.Unlock()
					break
				}
				slog.DebugContext(w.ctx, "attempting to restart")
				if err := w.listener.Start(); err != nil {
					w.mux.Unlock()
					slog.ErrorContext(w.ctx, "error restarting listener", "error", err)
					if backoff > 60*time.Second {
						backoff = 60 * time.Second
					} else if backoff == 0 {
						backoff = 5 * time.Second
						slog.InfoContext(w.ctx, "backing off restart attempt", "backoff", backoff)
					} else {
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

	for i := current; i < target; i++ {
		newRunnerName := fmt.Sprintf("%s-%s", w.scaleSet.GetRunnerPrefix(), util.NewID())
		jitConfig, err := w.scaleSetCli.GenerateJitRunnerConfig(w.ctx, newRunnerName, w.scaleSet.ScaleSetID)
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
			AgentID:           int64(jitConfig.Runner.ID),
		}

		if _, err := w.store.CreateScaleSetInstance(w.ctx, w.scaleSet.ID, runnerParams); err != nil {
			slog.ErrorContext(w.ctx, "error creating instance", "error", err)
			if err := w.scaleSetCli.RemoveRunner(w.ctx, jitConfig.Runner.ID); err != nil {
				slog.ErrorContext(w.ctx, "error deleting runner", "error", err)
			}
			continue
		}

		runnerDetails, err := w.scaleSetCli.GetRunner(w.ctx, jitConfig.Runner.ID)
		if err != nil {
			slog.ErrorContext(w.ctx, "error getting runner details", "error", err)
			continue
		}
		slog.DebugContext(w.ctx, "runner details", "runner_details", runnerDetails)
	}
}

func (w *Worker) handleScaleDown(target, current uint) {
	delta := current - target
	if delta <= 0 {
		return
	}
	w.mux.Lock()
	defer w.mux.Unlock()
	removed := 0
	for _, runner := range w.runners {
		if removed >= int(delta) {
			break
		}

		locked, err := locking.TryLock(runner.Name)
		if err != nil || !locked {
			slog.DebugContext(w.ctx, "runner is locked; skipping", "runner_name", runner.Name)
			continue
		}

		switch runner.Status {
		case commonParams.InstancePendingCreate, commonParams.InstanceRunning:
		case commonParams.InstancePendingDelete, commonParams.InstancePendingForceDelete:
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

		slog.DebugContext(w.ctx, "removing runner", "runner_name", runner.Name)
		if err := w.scaleSetCli.RemoveRunner(w.ctx, runner.AgentID); err != nil {
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
			w.mux.Unlock()

			var desiredRunners uint
			if w.scaleSet.DesiredRunnerCount > 0 {
				desiredRunners = uint(w.scaleSet.DesiredRunnerCount)
			}
			targetRunners := min(w.scaleSet.MinIdleRunners+desiredRunners, w.scaleSet.MaxRunners)

			currentRunners := uint(len(w.runners))
			if currentRunners == targetRunners {
				lastMsgDebugLog("desired runner count reached", targetRunners, currentRunners)
				continue
			}

			if currentRunners < targetRunners {
				lastMsgDebugLog("scaling up", targetRunners, currentRunners)
				w.handleScaleUp(targetRunners, currentRunners)
			} else {
				lastMsgDebugLog("attempting to scale down", targetRunners, currentRunners)
				w.handleScaleDown(targetRunners, currentRunners)
			}
		}
	}
}
