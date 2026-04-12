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
package entity

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudbase/garm/auth"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	garmUtil "github.com/cloudbase/garm/util"
	workersCommon "github.com/cloudbase/garm/workers/common"
)

const retryLoopInterval = 5 * time.Second

func NewController(ctx context.Context, store dbCommon.Store, providers map[string]common.Provider) (*Controller, error) {
	consumerID := "entity-controller"
	ctx = garmUtil.WithSlogContext(
		ctx,
		slog.Any("worker", consumerID))
	ctx = auth.GetAdminContext(ctx)

	return &Controller{
		consumerID: consumerID,
		ctx:        ctx,
		store:      store,
		providers:  providers,
		backoff:    workersCommon.NewBackoff(workersCommon.DefaultBackoffConfig()),
	}, nil
}

type Controller struct {
	consumerID string
	ctx        context.Context

	consumer dbCommon.Consumer
	store    dbCommon.Store

	providers map[string]common.Provider
	// sync.Map[string]*Worker
	Entities sync.Map

	eventQueue *workersCommon.UnboundedChan[dbCommon.ChangePayload]
	backoff    *workersCommon.Backoff

	running bool
	quit    chan struct{}

	mux sync.Mutex
}

func (c *Controller) loadAllRepositories() {
	repos, err := c.store.ListRepositories(c.ctx, params.RepositoryFilter{})
	if err != nil {
		slog.ErrorContext(c.ctx, "fetching repositories", "error", err)
		return
	}

	for _, repo := range repos {
		entity, err := repo.GetEntity()
		if err != nil {
			slog.ErrorContext(c.ctx, "getting entity from repository", "error", err)
			continue
		}
		c.storeEntityWorker(entity)
	}
}

func (c *Controller) loadAllOrganizations() {
	orgs, err := c.store.ListOrganizations(c.ctx, params.OrganizationFilter{})
	if err != nil {
		slog.ErrorContext(c.ctx, "fetching organizations", "error", err)
		return
	}

	for _, org := range orgs {
		entity, err := org.GetEntity()
		if err != nil {
			slog.ErrorContext(c.ctx, "getting entity from organization", "error", err)
			continue
		}
		c.storeEntityWorker(entity)
	}
}

func (c *Controller) loadAllEnterprises() {
	enterprises, err := c.store.ListEnterprises(c.ctx, params.EnterpriseFilter{})
	if err != nil {
		slog.ErrorContext(c.ctx, "fetching enterprises", "error", err)
		return
	}

	for _, enterprise := range enterprises {
		entity, err := enterprise.GetEntity()
		if err != nil {
			slog.ErrorContext(c.ctx, "getting entity from enterprise", "error", err)
			continue
		}
		c.storeEntityWorker(entity)
	}
}

// storeEntityWorker creates a worker for the entity and stores it.
// The retry loop will start it.
func (c *Controller) storeEntityWorker(entity params.ForgeEntity) {
	worker, err := NewWorker(c.ctx, c.store, entity, c.providers)
	if err != nil {
		slog.ErrorContext(c.ctx, "creating worker", "entity_id", entity.ID, "error", err)
		return
	}
	c.Entities.Store(entity.ID, worker)
}

func (c *Controller) Start() error {
	c.mux.Lock()
	defer c.mux.Unlock()

	if c.running {
		return nil
	}
	c.quit = nil

	c.loadAllEnterprises()
	c.loadAllOrganizations()
	c.loadAllRepositories()

	consumer, err := watcher.RegisterConsumer(
		c.ctx, c.consumerID,
		composeControllerWatcherFilters(),
	)
	if err != nil {
		return fmt.Errorf("failed to create consumer for entity controller: %w", err)
	}

	c.consumer = consumer
	c.running = true
	c.quit = make(chan struct{})
	c.eventQueue = workersCommon.NewUnboundedChan[dbCommon.ChangePayload](c.ctx, c.quit)

	go c.loop()
	go c.eventQueue.Process(c.handleWatcherEvent)
	go workersCommon.RetryLoop(c.ctx, c.quit, retryLoopInterval, c.retryFailedWorkers)

	return nil
}

func (c *Controller) Stop() error {
	slog.DebugContext(c.ctx, "stopping entity controller", "entity", c.consumerID)
	c.mux.Lock()
	defer c.mux.Unlock()
	if !c.running {
		return nil
	}
	slog.DebugContext(c.ctx, "stopping entity controller")

	c.Entities.Range(func(key, value any) bool {
		entityID := key.(string)
		worker := value.(*Worker)
		if err := worker.Stop(); err != nil {
			slog.ErrorContext(c.ctx, "stopping worker for entity", "entity_id", entityID, "error", err)
		}
		return true
	})

	c.running = false
	close(c.quit)
	c.consumer.Close()
	slog.DebugContext(c.ctx, "stopped entity controller", "entity", c.consumerID)
	return nil
}

func (c *Controller) retryFailedWorkers() {
	c.mux.Lock()
	defer c.mux.Unlock()

	if !c.running {
		return
	}

	c.Entities.Range(func(key, value any) bool {
		entityID := key.(string)
		worker := value.(*Worker)

		if worker.IsRunning() {
			return true
		}

		if !c.backoff.ShouldRetry(entityID) {
			return true
		}

		slog.InfoContext(c.ctx, "retrying failed worker", "entity_id", entityID)
		if err := worker.Start(); err != nil {
			slog.ErrorContext(c.ctx, "retry failed for worker", "entity_id", entityID, "error", err)
			worker.addStatusEvent(fmt.Sprintf("retry failed for worker %s (%s): %s", entityID, worker.Entity.ForgeURL(), err.Error()), params.EventError)
			c.backoff.RecordFailure(entityID)
			return true
		}

		slog.InfoContext(c.ctx, "worker successfully started after retry", "entity_id", entityID)
		worker.addStatusEvent(fmt.Sprintf("worker successfully started after retry for entity: %s (%s)", entityID, worker.Entity.ForgeURL()), params.EventInfo)
		c.backoff.RecordSuccess(entityID)
		return true
	})
}

// loop drains the watcher consumer channel as fast as possible.
// It exits when the context is cancelled or quit is closed, triggering
// a controller stop.
func (c *Controller) loop() {
	defer c.Stop()
	for {
		select {
		case payload := <-c.consumer.Watch():
			slog.DebugContext(c.ctx, "received payload, queuing for processing")
			select {
			case c.eventQueue.In() <- payload:
			case <-c.ctx.Done():
				return
			case <-c.quit:
				return
			}
		case <-c.ctx.Done():
			return
		case <-c.quit:
			return
		}
	}
}
