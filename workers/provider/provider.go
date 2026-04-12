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
package provider

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	garmUtil "github.com/cloudbase/garm/util"
	workersCommon "github.com/cloudbase/garm/workers/common"
)

const retryLoopInterval = 5 * time.Second

type runnerEntry struct {
	instance params.Instance
	manager  *instanceManager
	mux      sync.Mutex
}

func (r *runnerEntry) SetManager(m *instanceManager) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.manager = m
}

func (r *runnerEntry) SetInstance(inst params.Instance) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.instance = inst
}

func (r *runnerEntry) Stop() error {
	r.mux.Lock()
	defer r.mux.Unlock()
	if r.manager == nil {
		return nil
	}
	return r.manager.Stop()
}

func NewWorker(ctx context.Context, store dbCommon.Store, providers map[string]common.Provider, tokenGetter auth.InstanceTokenGetter) (*Provider, error) {
	consumerID := "provider-worker"

	ctx = garmUtil.WithSlogContext(
		ctx,
		slog.Any("worker", consumerID))

	return &Provider{
		ctx:         ctx,
		store:       store,
		consumerID:  consumerID,
		providers:   providers,
		tokenGetter: tokenGetter,
		backoff:     workersCommon.NewBackoff(workersCommon.DefaultBackoffConfig()),
	}, nil
}

type Provider struct {
	ctx        context.Context
	consumerID string

	consumer dbCommon.Consumer
	// nolint:golangci-lint,godox
	// TODO: not all workers should have access to the store.
	// We need to implement way to RPC from workers to controllers
	// and abstract that into something we can use to eventually
	// scale out.
	store       dbCommon.Store
	tokenGetter auth.InstanceTokenGetter

	providers map[string]common.Provider
	// A cache of all scale sets kept updated by the watcher.
	// This helps us avoid a bunch of queries to the database.
	// sync.Map[uint]params.ScaleSet
	scaleSets sync.Map
	// sync.Map[string]*runnerEntry
	runners sync.Map

	eventQueue *workersCommon.UnboundedChan[dbCommon.ChangePayload]
	backoff    *workersCommon.Backoff

	scaleSetsLoaded bool
	runnersLoaded   bool

	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func (p *Provider) loadAllScaleSets() error {
	scaleSets, err := p.store.ListAllScaleSets(p.ctx)
	if err != nil {
		return fmt.Errorf("fetching scale sets: %w", err)
	}

	for _, scaleSet := range scaleSets {
		p.scaleSets.Store(scaleSet.ID, scaleSet)
	}
	return nil
}

// loadAllRunners loads all runners from the database. At this stage we only
// care about runners created by scale sets, but in the future, we will migrate
// the pool manager to the same model.
func (p *Provider) loadAllRunners() error {
	runners, err := p.store.ListAllInstances(p.ctx)
	if err != nil {
		return fmt.Errorf("fetching runners: %w", err)
	}

	for _, runner := range runners {
		// Skip non scale set instances for now. This condition needs to be
		// removed once we replace the current pool manager.
		if runner.ScaleSetID == 0 {
			continue
		}
		// Ignore runners in "creating" state. If we're just starting up and
		// we find a runner in "creating" it was most likely interrupted while
		// creating. It is unlikely that it is still usable. We allow the scale set
		// worker to clean it up. It will eventually be marked as pending delete and
		// this worker will get an update to clean up any resources left behing by
		// an incomplete creation event.
		if runner.Status == commonParams.InstanceCreating {
			continue
		}
		if runner.Status == commonParams.InstanceDeleting || runner.Status == commonParams.InstanceDeleted {
			continue
		}

		// Skip runners that already have entries (idempotent on re-call).
		if _, ok := p.runners.Load(runner.Name); ok {
			continue
		}

		entry := &runnerEntry{instance: runner}

		val, ok := p.scaleSets.Load(runner.ScaleSetID)
		if !ok {
			slog.ErrorContext(p.ctx, "scale set not found", "scale_set_id", runner.ScaleSetID, "runner_name", runner.Name)
			p.runners.Store(runner.Name, entry)
			p.backoff.RecordFailure(runner.Name)
			continue
		}
		scaleSet := val.(params.ScaleSet)
		provider, ok := p.providers[scaleSet.ProviderName]
		if !ok {
			// The provider map is static — it only changes on app restart.
			// No point storing an entry that will never succeed.
			slog.ErrorContext(p.ctx, "provider not configured, skipping runner", "provider_name", scaleSet.ProviderName, "runner_name", runner.Name)
			continue
		}

		manager, err := newInstanceManager(p.ctx, runner, scaleSet, provider, p)
		if err != nil {
			slog.ErrorContext(p.ctx, "creating instance manager", "runner_name", runner.Name, "error", err)
			p.runners.Store(runner.Name, entry)
			p.backoff.RecordFailure(runner.Name)
			continue
		}
		if err := manager.Start(); err != nil {
			slog.ErrorContext(p.ctx, "starting instance manager", "runner_name", runner.Name, "error", err)
			p.runners.Store(runner.Name, entry)
			p.backoff.RecordFailure(runner.Name)
			continue
		}

		entry.SetManager(manager)
		p.runners.Store(runner.Name, entry)
	}
	return nil
}

func (p *Provider) Start() error {
	p.mux.Lock()
	defer p.mux.Unlock()

	if p.running {
		return nil
	}

	// Register the consumer first. It must be watching before entity/scaleset
	// workers start creating instances. This is the only fatal error.
	consumer, err := watcher.RegisterConsumer(
		p.ctx, p.consumerID, composeProviderWatcher())
	if err != nil {
		return fmt.Errorf("registering consumer: %w", err)
	}
	p.consumer = consumer
	p.quit = make(chan struct{})
	p.running = true
	p.eventQueue = workersCommon.NewUnboundedChan[dbCommon.ChangePayload](p.ctx, p.quit)

	// Best-effort initial DB load. The retry loop handles failures.
	if err := p.loadAllScaleSets(); err != nil {
		slog.ErrorContext(p.ctx, "initial scale set load failed, will retry", "error", err)
	} else {
		p.scaleSetsLoaded = true
	}
	if p.scaleSetsLoaded {
		if err := p.loadAllRunners(); err != nil {
			slog.ErrorContext(p.ctx, "initial runner load failed, will retry", "error", err)
		} else {
			p.runnersLoaded = true
		}
	}

	go p.loop()
	go p.eventQueue.Process(p.handleWatcherEvent)
	go workersCommon.RetryLoop(p.ctx, p.quit, retryLoopInterval, p.retryFailedRunners)

	return nil
}

func (p *Provider) Stop() error {
	p.mux.Lock()
	defer p.mux.Unlock()

	if !p.running {
		return nil
	}

	p.runners.Range(func(key, value any) bool {
		name := key.(string)
		entry := value.(*runnerEntry)
		if err := entry.Stop(); err != nil {
			slog.ErrorContext(p.ctx, "stopping instance manager", "runner_name", name, "error", err)
		}
		return true
	})

	p.running = false
	close(p.quit)
	p.consumer.Close()
	return nil
}

func (p *Provider) retryFailedRunners() {
	p.mux.Lock()
	defer p.mux.Unlock()

	if !p.running {
		return
	}

	// Retry DB load for scale sets if needed.
	if !p.scaleSetsLoaded {
		if err := p.loadAllScaleSets(); err != nil {
			slog.ErrorContext(p.ctx, "retrying scale set load", "error", err)
			return // Can't proceed without scale sets.
		}
		slog.InfoContext(p.ctx, "scale sets loaded successfully after retry")
		p.scaleSetsLoaded = true
	}

	// Retry DB load for runners if needed.
	if !p.runnersLoaded {
		if err := p.loadAllRunners(); err != nil {
			slog.ErrorContext(p.ctx, "retrying runner load", "error", err)
		} else {
			slog.InfoContext(p.ctx, "runners loaded successfully after retry")
			p.runnersLoaded = true
		}
	}

	// Retry individual failed instance managers.
	p.runners.Range(func(key, value any) bool {
		name := key.(string)
		entry := value.(*runnerEntry)

		entry.mux.Lock()
		manager := entry.manager
		instance := entry.instance
		entry.mux.Unlock()

		if manager != nil && manager.running.Load() {
			return true
		}

		// Clean up terminated instances.
		if instance.Status == commonParams.InstanceDeleted ||
			instance.Status == commonParams.InstanceDeleting {
			p.runners.Delete(name)
			p.backoff.RecordSuccess(name)
			return true
		}

		if !p.backoff.ShouldRetry(name) {
			return true
		}

		val, ok := p.scaleSets.Load(instance.ScaleSetID)
		if !ok {
			slog.ErrorContext(p.ctx, "scale set not found for retry", "runner_name", name, "scale_set_id", instance.ScaleSetID)
			p.backoff.RecordFailure(name)
			return true
		}
		scaleSet := val.(params.ScaleSet)
		provider, ok := p.providers[scaleSet.ProviderName]
		if !ok {
			// The provider map is static — retrying will never help.
			// Remove the entry entirely.
			slog.ErrorContext(p.ctx, "provider not configured, removing runner entry", "runner_name", name, "provider_name", scaleSet.ProviderName)
			p.runners.Delete(name)
			p.backoff.RecordSuccess(name)
			return true
		}

		if manager == nil {
			var err error
			manager, err = newInstanceManager(p.ctx, instance, scaleSet, provider, p)
			if err != nil {
				slog.ErrorContext(p.ctx, "retry: creating instance manager", "runner_name", name, "error", err)
				p.backoff.RecordFailure(name)
				return true
			}
			entry.SetManager(manager)
		}

		if err := manager.Start(); err != nil {
			slog.ErrorContext(p.ctx, "retry: starting instance manager", "runner_name", name, "error", err)
			p.backoff.RecordFailure(name)
			return true
		}

		slog.InfoContext(p.ctx, "instance manager started after retry", "runner_name", name)
		p.backoff.RecordSuccess(name)
		return true
	})
}

// loop drains the watcher consumer channel as fast as possible into the
// event queue. It exits when the context is cancelled or quit is closed.
func (p *Provider) loop() {
	defer p.Stop()
	for {
		select {
		case payload := <-p.consumer.Watch():
			slog.DebugContext(p.ctx, "received payload, queuing for processing")
			select {
			case p.eventQueue.In() <- payload:
			case <-p.ctx.Done():
				return
			case <-p.quit:
				return
			}
		case <-p.ctx.Done():
			return
		case <-p.quit:
			return
		}
	}
}

func (p *Provider) handleWatcherEvent(payload dbCommon.ChangePayload) {
	p.mux.Lock()
	defer p.mux.Unlock()

	if !p.running {
		return
	}

	switch payload.EntityType {
	case dbCommon.ScaleSetEntityType:
		p.handleScaleSetEvent(payload)
	case dbCommon.InstanceEntityType:
		p.handleInstanceEvent(payload)
	default:
		slog.ErrorContext(p.ctx, "invalid entity type", "entity_type", payload.EntityType)
	}
}

func (p *Provider) handleScaleSetEvent(event dbCommon.ChangePayload) {
	scaleSet, ok := event.Payload.(params.ScaleSet)
	if !ok {
		slog.ErrorContext(p.ctx, "invalid payload type", "payload_type", fmt.Sprintf("%T", event.Payload))
		return
	}

	switch event.Operation {
	case dbCommon.CreateOperation, dbCommon.UpdateOperation:
		slog.DebugContext(p.ctx, "got create/update operation")
		p.scaleSets.Store(scaleSet.ID, scaleSet)
	case dbCommon.DeleteOperation:
		slog.DebugContext(p.ctx, "got delete operation")
		p.scaleSets.Delete(scaleSet.ID)
	default:
		slog.ErrorContext(p.ctx, "invalid operation type", "operation_type", event.Operation)
		return
	}
}

func (p *Provider) handleInstanceAdded(instance params.Instance) {
	// If an entry already exists, update its instance data and let the
	// retry loop handle restarting it if needed.
	if val, ok := p.runners.Load(instance.Name); ok {
		entry := val.(*runnerEntry)
		entry.mux.Lock()
		running := entry.manager != nil && entry.manager.running.Load()
		entry.mux.Unlock()

		if running {
			slog.DebugContext(p.ctx, "instance manager already running", "instance_name", instance.Name)
			return
		}
		entry.SetInstance(instance)
		p.backoff.RecordSuccess(instance.Name)
		return
	}

	entry := &runnerEntry{instance: instance}

	val, ok := p.scaleSets.Load(instance.ScaleSetID)
	if !ok {
		slog.ErrorContext(p.ctx, "scale set not found for instance", "instance_name", instance.Name, "scale_set_id", instance.ScaleSetID)
		p.runners.Store(instance.Name, entry)
		p.backoff.RecordFailure(instance.Name)
		return
	}
	scaleSet := val.(params.ScaleSet)

	provider, ok := p.providers[scaleSet.ProviderName]
	if !ok {
		// The provider map is static — no point storing an entry that will never work.
		slog.ErrorContext(p.ctx, "provider not configured, skipping instance", "instance_name", instance.Name, "provider_name", scaleSet.ProviderName)
		return
	}

	manager, err := newInstanceManager(
		p.ctx, instance, scaleSet, provider, p)
	if err != nil {
		slog.ErrorContext(p.ctx, "creating instance manager", "instance_name", instance.Name, "error", err)
		p.runners.Store(instance.Name, entry)
		p.backoff.RecordFailure(instance.Name)
		return
	}
	if err := manager.Start(); err != nil {
		slog.ErrorContext(p.ctx, "starting instance manager", "instance_name", instance.Name, "error", err)
		p.runners.Store(instance.Name, entry)
		p.backoff.RecordFailure(instance.Name)
		return
	}

	entry.SetManager(manager)
	p.runners.Store(instance.Name, entry)
}

func (p *Provider) stopAndDeleteInstance(instance params.Instance) {
	if instance.Status != commonParams.InstanceDeleted {
		return
	}
	val, loaded := p.runners.LoadAndDelete(instance.Name)
	if !loaded {
		return
	}
	entry := val.(*runnerEntry)
	if err := entry.Stop(); err != nil {
		// Re-store the entry so it can be retried.
		p.runners.Store(instance.Name, entry)
		slog.ErrorContext(p.ctx, "failed to stop instance manager", "runner_name", instance.Name, "error", err)
		return
	}
	p.backoff.RecordSuccess(instance.Name)
}

func (p *Provider) handleInstanceEvent(event dbCommon.ChangePayload) {
	instance, ok := event.Payload.(params.Instance)
	if !ok {
		slog.ErrorContext(p.ctx, "invalid payload type", "payload_type", fmt.Sprintf("%T", event.Payload))
		return
	}

	if instance.ScaleSetID == 0 {
		slog.DebugContext(p.ctx, "skipping instance event for non scale set instance")
		return
	}

	slog.DebugContext(p.ctx, "handling instance event", "instance_name", instance.Name, "operation", event.Operation)
	switch event.Operation {
	case dbCommon.CreateOperation:
		slog.DebugContext(p.ctx, "got create operation")
		p.handleInstanceAdded(instance)
	case dbCommon.UpdateOperation:
		slog.DebugContext(p.ctx, "got update operation")

		val, ok := p.runners.Load(instance.Name)
		if !ok {
			if instance.Status == commonParams.InstanceDeleted {
				// No entry for this instance and it's already deleted.
				return
			}
			slog.DebugContext(p.ctx, "instance not found, creating new instance", "instance_name", instance.Name)
			p.handleInstanceAdded(instance)
			return
		}

		entry := val.(*runnerEntry)
		slog.DebugContext(p.ctx, "updating instance", "instance_name", instance.Name)

		if instance.Status == commonParams.InstanceDeleted {
			p.stopAndDeleteInstance(instance)
			return
		}

		entry.SetInstance(instance)

		entry.mux.Lock()
		manager := entry.manager
		entry.mux.Unlock()

		if manager != nil && manager.running.Load() {
			if err := manager.Update(event); err != nil {
				slog.ErrorContext(p.ctx, "failed to update instance", "error", err, "instance_name", instance.Name)
			}
		} else {
			// Manager not running. Clear backoff so the retry loop
			// picks up the updated instance data immediately.
			p.backoff.RecordSuccess(instance.Name)
		}
	case dbCommon.DeleteOperation:
		slog.DebugContext(p.ctx, "got delete operation", "instance_name", instance.Name)
		p.stopAndDeleteInstance(instance)
	default:
		slog.ErrorContext(p.ctx, "invalid operation type", "operation_type", event.Operation)
		return
	}
}
