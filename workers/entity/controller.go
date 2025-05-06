package entity

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/cache"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/runner/common"
	garmUtil "github.com/cloudbase/garm/util"
)

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
		Entities:   make(map[string]*Worker),
	}, nil
}

type Controller struct {
	consumerID string
	ctx        context.Context

	consumer dbCommon.Consumer
	store    dbCommon.Store

	providers map[string]common.Provider
	Entities  map[string]*Worker

	running bool
	quit    chan struct{}

	mux sync.Mutex
}

func (c *Controller) loadAllRepositories() error {
	c.mux.Lock()
	defer c.mux.Unlock()
	repos, err := c.store.ListRepositories(c.ctx)
	if err != nil {
		return fmt.Errorf("fetching repositories: %w", err)
	}

	for _, repo := range repos {
		entity, err := repo.GetEntity()
		if err != nil {
			return fmt.Errorf("getting entity: %w", err)
		}
		worker, err := NewWorker(c.ctx, c.store, entity, c.providers)
		if err != nil {
			return fmt.Errorf("creating worker: %w", err)
		}
		if err := worker.Start(); err != nil {
			return fmt.Errorf("starting worker: %w", err)
		}
		c.Entities[entity.ID] = worker
		// take advantage of the fact that we're loading all entities
		// and set the cache.
		cache.SetEntity(entity)
	}
	return nil
}

func (c *Controller) loadAllOrganizations() error {
	c.mux.Lock()
	defer c.mux.Unlock()
	orgs, err := c.store.ListOrganizations(c.ctx)
	if err != nil {
		return fmt.Errorf("fetching organizations: %w", err)
	}
	for _, org := range orgs {
		entity, err := org.GetEntity()
		if err != nil {
			return fmt.Errorf("getting entity: %w", err)
		}
		worker, err := NewWorker(c.ctx, c.store, entity, c.providers)
		if err != nil {
			return fmt.Errorf("creating worker: %w", err)
		}
		if err := worker.Start(); err != nil {
			return fmt.Errorf("starting worker: %w", err)
		}
		c.Entities[entity.ID] = worker
		// take advantage of the fact that we're loading all entities
		// and set the cache.
		cache.SetEntity(entity)
	}
	return nil
}

func (c *Controller) loadAllEnterprises() error {
	c.mux.Lock()
	defer c.mux.Unlock()
	enterprises, err := c.store.ListEnterprises(c.ctx)
	if err != nil {
		return fmt.Errorf("fetching enterprises: %w", err)
	}
	for _, enterprise := range enterprises {
		entity, err := enterprise.GetEntity()
		if err != nil {
			return fmt.Errorf("getting entity: %w", err)
		}
		worker, err := NewWorker(c.ctx, c.store, entity, c.providers)
		if err != nil {
			return fmt.Errorf("creating worker: %w", err)
		}
		if err := worker.Start(); err != nil {
			return fmt.Errorf("starting worker: %w", err)
		}
		c.Entities[entity.ID] = worker
		// take advantage of the fact that we're loading all entities
		// and set the cache.
		cache.SetEntity(entity)
	}
	return nil
}

func (c *Controller) Start() error {
	c.mux.Lock()
	if c.running {
		c.mux.Unlock()
		return nil
	}
	c.mux.Unlock()

	if err := c.loadAllEnterprises(); err != nil {
		return fmt.Errorf("loading enterprises: %w", err)
	}
	if err := c.loadAllOrganizations(); err != nil {
		return fmt.Errorf("loading organizations: %w", err)
	}
	if err := c.loadAllRepositories(); err != nil {
		return fmt.Errorf("loading repositories: %w", err)
	}

	consumer, err := watcher.RegisterConsumer(
		c.ctx, c.consumerID,
		composeControllerWatcherFilters(),
	)
	if err != nil {
		return fmt.Errorf("failed to create consumer for entity controller: %w", err)
	}

	c.mux.Lock()
	c.consumer = consumer
	c.running = true
	c.quit = make(chan struct{})
	c.mux.Unlock()

	go c.loop()

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

	for entityID, worker := range c.Entities {
		if err := worker.Stop(); err != nil {
			slog.ErrorContext(c.ctx, "stopping worker for entity", "entity_id", entityID, "error", err)
		}
	}

	c.running = false
	close(c.quit)
	c.consumer.Close()
	return nil
}

func (c *Controller) loop() {
	defer c.Stop()
	for {
		select {
		case payload := <-c.consumer.Watch():
			slog.InfoContext(c.ctx, "received payload")
			go c.handleWatcherEvent(payload)
		case <-c.ctx.Done():
			return
		case <-c.quit:
			return
		}
	}
}
