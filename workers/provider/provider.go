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

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	garmUtil "github.com/cloudbase/garm/util"
)

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
		scaleSets:   make(map[uint]params.ScaleSet),
		runners:     make(map[string]*instanceManager),
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
	scaleSets map[uint]params.ScaleSet
	runners   map[string]*instanceManager

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
		p.scaleSets[scaleSet.ID] = scaleSet
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
		instanceManager, err := newInstanceManager(
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

func (p *Provider) Start() error {
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

func (p *Provider) Stop() error {
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

func (p *Provider) loop() {
	defer p.Stop()
	for {
		select {
		case payload, ok := <-p.consumer.Watch():
			if !ok {
				slog.ErrorContext(p.ctx, "watcher channel closed")
				return
			}
			slog.InfoContext(p.ctx, "received payload", "operation", payload.Operation, "entity_type", payload.EntityType)
			go p.handleWatcherEvent(payload)
		case <-p.ctx.Done():
			return
		case <-p.quit:
			return
		}
	}
}

func (p *Provider) handleWatcherEvent(payload dbCommon.ChangePayload) {
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

func (p *Provider) handleInstanceAdded(instance params.Instance) error {
	scaleSet, ok := p.scaleSets[instance.ScaleSetID]
	if !ok {
		return fmt.Errorf("scale set not found for instance %s", instance.Name)
	}
	instanceManager, err := newInstanceManager(
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

func (p *Provider) stopAndDeleteInstance(instance params.Instance) error {
	if instance.Status != commonParams.InstanceDeleted {
		return nil
	}
	existingInstance, ok := p.runners[instance.Name]
	if ok {
		if err := existingInstance.Stop(); err != nil {
			return fmt.Errorf("failed to stop instance manager: %w", err)
		}
		delete(p.runners, instance.Name)
	}
	return nil
}

func (p *Provider) handleInstanceEvent(event dbCommon.ChangePayload) {
	p.mux.Lock()
	defer p.mux.Unlock()

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
		if err := p.handleInstanceAdded(instance); err != nil {
			slog.ErrorContext(p.ctx, "failed to handle instance added", "error", err)
			return
		}
	case dbCommon.UpdateOperation:
		slog.DebugContext(p.ctx, "got update operation")
		existingInstance, ok := p.runners[instance.Name]
		if !ok {
			slog.DebugContext(p.ctx, "instance not found, creating new instance", "instance_name", instance.Name)
			if err := p.handleInstanceAdded(instance); err != nil {
				slog.ErrorContext(p.ctx, "failed to handle instance added", "error", err)
				return
			}
		} else {
			slog.DebugContext(p.ctx, "updating instance", "instance_name", instance.Name)
			if instance.Status == commonParams.InstanceDeleted {
				if err := p.stopAndDeleteInstance(instance); err != nil {
					slog.ErrorContext(p.ctx, "failed to clean up instance manager", "error", err)
					return
				}
				return
			}
			if err := existingInstance.Update(event); err != nil {
				slog.ErrorContext(p.ctx, "failed to update instance", "error", err, "instance_name", instance.Name, "payload", event.Payload)
				return
			}
		}
	case dbCommon.DeleteOperation:
		slog.DebugContext(p.ctx, "got delete operation", "instance_name", instance.Name)
		if err := p.stopAndDeleteInstance(instance); err != nil {
			slog.ErrorContext(p.ctx, "failed to clean up instance manager", "error", err)
			return
		}
	default:
		slog.ErrorContext(p.ctx, "invalid operation type", "operation_type", event.Operation)
		return
	}
}
