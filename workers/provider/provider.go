package provider

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

func NewWorker(ctx context.Context, store dbCommon.Store, providers map[string]common.Provider) (*provider, error) {
	consumerID := "provider-worker"
	return &provider{
		ctx:        context.Background(),
		store:      store,
		consumerID: consumerID,
		providers:  providers,
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
	store dbCommon.Store

	providers map[string]common.Provider
	// A cache of all scale sets kept updated by the watcher.
	// This helps us avoid a bunch of queries to the database.
	scaleSets map[uint]params.ScaleSet
	runners   map[string]params.Instance

	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func (p *provider) loadAllScaleSets() error {
	p.mux.Lock()
	defer p.mux.Unlock()

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
	p.mux.Lock()
	defer p.mux.Unlock()

	runners, err := p.store.ListAllInstances(p.ctx)
	if err != nil {
		return fmt.Errorf("fetching runners: %w", err)
	}

	for _, runner := range runners {
		p.runners[runner.Name] = runner
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
		case payload := <-p.consumer.Watch():
			slog.InfoContext(p.ctx, "received payload", slog.Any("payload", payload))
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

func (p *provider) handleInstanceEvent(event dbCommon.ChangePayload) {
	p.mux.Lock()
	defer p.mux.Unlock()

	instance, ok := event.Payload.(params.Instance)
	if !ok {
		slog.ErrorContext(p.ctx, "invalid payload type", "payload_type", fmt.Sprintf("%T", event.Payload))
		return
	}

	switch event.Operation {
	case dbCommon.CreateOperation, dbCommon.UpdateOperation:
		slog.DebugContext(p.ctx, "got create/update operation")
		p.runners[instance.Name] = instance
	case dbCommon.DeleteOperation:
		slog.DebugContext(p.ctx, "got delete operation")
		delete(p.runners, instance.Name)
	default:
		slog.ErrorContext(p.ctx, "invalid operation type", "operation_type", event.Operation)
		return
	}
}
