package provider

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	commonParams "github.com/cloudbase/garm-provider-common/params"

	"github.com/cloudbase/garm/auth"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

func NewWorker(ctx context.Context, store dbCommon.Store, providers map[string]common.Provider, tokenGetter auth.InstanceTokenGetter) (*provider, error) {
	consumerID := "provider-worker"
	return &provider{
		ctx:         context.Background(),
		store:       store,
		consumerID:  consumerID,
		providers:   providers,
		tokenGetter: tokenGetter,
		scaleSets:   make(map[uint]params.ScaleSet),
		runners:     make(map[string]*instanceManager),
	}, nil
}

type provider struct {
	ctx        context.Context
	consumerID string

	consumer dbCommon.Consumer
	// TODO: not all workers should have access to the store.
	// We need to implement way to RPC from workers to controllers
	// and abstract that into something we can use to eventually
	// scale out.
	store       dbCommon.Store
	tokenGetter auth.InstanceTokenGetter

	providers map[string]common.Provider
	// A cache of all scale sets kept updated by the watcher.
	// This helps us avoid a bunch of queries to the database.
	scaleSets map[uint]params.ScaleSet
	runners   map[string]*instanceManager

	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func (p *provider) loadAllScaleSets() error {
	scaleSets, err := p.store.ListAllScaleSets(p.ctx)
	if err != nil {
		return fmt.Errorf("fetching scale sets: %w", err)
	}

	for _, scaleSet := range scaleSets {
		p.scaleSets[scaleSet.ID] = scaleSet
	}

	return nil
}

// loadAllRunners loads all runners from the database. At this stage we only
// care about runners created by scale sets, but in the future, we will migrate
// the pool manager to the same model.
func (p *provider) loadAllRunners() error {
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

		scaleSet, ok := p.scaleSets[runner.ScaleSetID]
		if !ok {
			slog.ErrorContext(p.ctx, "scale set not found", "scale_set_id", runner.ScaleSetID)
			continue
		}
		provider, ok := p.providers[scaleSet.ProviderName]
		if !ok {
			slog.ErrorContext(p.ctx, "provider not found", "provider_name", runner.ProviderName)
			continue
		}
		instanceManager, err := NewInstanceManager(
			p.ctx, runner, scaleSet, provider, p)
		if err != nil {
			return fmt.Errorf("creating instance manager: %w", err)
		}
		if err := instanceManager.Start(); err != nil {
			return fmt.Errorf("starting instance manager: %w", err)
		}

		p.runners[runner.Name] = instanceManager
	}

	return nil
}

func (p *provider) Start() error {
	p.mux.Lock()
	defer p.mux.Unlock()

	if p.running {
		return nil
	}

	if err := p.loadAllScaleSets(); err != nil {
		return fmt.Errorf("loading all scale sets: %w", err)
	}

	if err := p.loadAllRunners(); err != nil {
		return fmt.Errorf("loading all runners: %w", err)
	}

	consumer, err := watcher.RegisterConsumer(
		p.ctx, p.consumerID, composeProviderWatcher())
	if err != nil {
		return fmt.Errorf("registering consumer: %w", err)
	}
	p.consumer = consumer

	p.quit = make(chan struct{})
	p.running = true
	go p.loop()

	return nil
}

func (p *provider) Stop() error {
	p.mux.Lock()
	defer p.mux.Unlock()

	if !p.running {
		return nil
	}

	p.consumer.Close()
	close(p.quit)
	p.running = false
	return nil
}

func (p *provider) loop() {
	defer p.Stop()
	for {
		select {
		case payload, ok := <-p.consumer.Watch():
			if !ok {
				slog.ErrorContext(p.ctx, "watcher channel closed")
				return
			}
			slog.InfoContext(p.ctx, "received payload")
			go p.handleWatcherEvent(payload)
		case <-p.ctx.Done():
			return
		case <-p.quit:
			return
		}
	}
}

func (p *provider) handleWatcherEvent(payload dbCommon.ChangePayload) {
	switch payload.EntityType {
	case dbCommon.ScaleSetEntityType:
		p.handleScaleSetEvent(payload)
	case dbCommon.InstanceEntityType:
		p.handleInstanceEvent(payload)
	default:
		slog.ErrorContext(p.ctx, "invalid entity type", "entity_type", payload.EntityType)
	}
}

func (p *provider) handleScaleSetEvent(event dbCommon.ChangePayload) {
	p.mux.Lock()
	defer p.mux.Unlock()

	scaleSet, ok := event.Payload.(params.ScaleSet)
	if !ok {
		slog.ErrorContext(p.ctx, "invalid payload type", "payload_type", fmt.Sprintf("%T", event.Payload))
		return
	}

	switch event.Operation {
	case dbCommon.CreateOperation, dbCommon.UpdateOperation:
		slog.DebugContext(p.ctx, "got create/update operation")
		p.scaleSets[scaleSet.ID] = scaleSet
	case dbCommon.DeleteOperation:
		slog.DebugContext(p.ctx, "got delete operation")
		delete(p.scaleSets, scaleSet.ID)
	default:
		slog.ErrorContext(p.ctx, "invalid operation type", "operation_type", event.Operation)
		return
	}
}

func (p *provider) handleInstanceAdded(instance params.Instance) error {
	scaleSet, ok := p.scaleSets[instance.ScaleSetID]
	if !ok {
		return fmt.Errorf("scale set not found for instance %s", instance.Name)
	}
	instanceManager, err := NewInstanceManager(
		p.ctx, instance, scaleSet, p.providers[instance.ProviderName], p)
	if err != nil {
		return fmt.Errorf("creating instance manager: %w", err)
	}
	if err := instanceManager.Start(); err != nil {
		return fmt.Errorf("starting instance manager: %w", err)
	}
	p.runners[instance.Name] = instanceManager
	return nil
}

func (p *provider) handleInstanceEvent(event dbCommon.ChangePayload) {
	p.mux.Lock()
	defer p.mux.Unlock()

	instance, ok := event.Payload.(params.Instance)
	if !ok {
		slog.ErrorContext(p.ctx, "invalid payload type", "payload_type", fmt.Sprintf("%T", event.Payload))
		return
	}

	switch event.Operation {
	case dbCommon.CreateOperation:
		slog.DebugContext(p.ctx, "got create operation")
		if err := p.handleInstanceAdded(instance); err != nil {
			slog.ErrorContext(p.ctx, "failed to handle instance added", "error", err)
			return
		}
	case dbCommon.UpdateOperation:
		slog.DebugContext(p.ctx, "got update operation")
		existingInstance, ok := p.runners[instance.Name]
		if !ok {
			if err := p.handleInstanceAdded(instance); err != nil {
				slog.ErrorContext(p.ctx, "failed to handle instance added", "error", err)
				return
			}
		} else {
			if err := existingInstance.Update(event); err != nil {
				slog.ErrorContext(p.ctx, "failed to update instance", "error", err)
				return
			}
		}
	case dbCommon.DeleteOperation:
		slog.DebugContext(p.ctx, "got delete operation")
		existingInstance, ok := p.runners[instance.Name]
		if ok {
			if err := existingInstance.Stop(); err != nil {
				slog.ErrorContext(p.ctx, "failed to stop instance", "error", err)
				return
			}
		}
		delete(p.runners, instance.Name)
	default:
		slog.ErrorContext(p.ctx, "invalid operation type", "operation_type", event.Operation)
		return
	}
}
