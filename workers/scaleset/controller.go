package scaleset

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"

	"github.com/cloudbase/garm/cache"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	garmUtil "github.com/cloudbase/garm/util"
	"github.com/cloudbase/garm/util/github"
)

func NewController(ctx context.Context, store dbCommon.Store, entity params.GithubEntity, providers map[string]common.Provider) (*Controller, error) {

	consumerID := fmt.Sprintf("scaleset-worker-%s", entity.String())

	ctx = garmUtil.WithSlogContext(
		ctx,
		slog.Any("worker", consumerID))

	return &Controller{
		ctx:           ctx,
		consumerID:    consumerID,
		ScaleSets:     make(map[uint]*scaleSet),
		Entity:        entity,
		providers:     providers,
		store:         store,
		statusUpdates: make(chan scaleSetStatus, 10),
	}, nil
}

type scaleSet struct {
	scaleSet params.ScaleSet
	status   scaleSetStatus
	worker   *Worker

	mux sync.Mutex
}

func (s *scaleSet) updateStatus(status scaleSetStatus) {
	s.mux.Lock()
	defer s.mux.Unlock()

	s.status = status
}

func (s *scaleSet) Stop() error {
	s.mux.Lock()
	defer s.mux.Unlock()

	if s.worker == nil {
		return nil
	}

	return s.worker.Stop()
}

// Controller is responsible for managing scale sets for one github entity.
type Controller struct {
	ctx        context.Context
	consumerID string

	ScaleSets map[uint]*scaleSet

	Entity params.GithubEntity

	consumer  dbCommon.Consumer
	store     dbCommon.Store
	providers map[string]common.Provider

	ghCli              common.GithubClient
	forgeCredsAreValid bool

	statusUpdates chan scaleSetStatus

	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func (c *Controller) loadAllScaleSets(cli common.GithubClient) error {
	scaleSets, err := c.store.ListEntityScaleSets(c.ctx, c.Entity)
	if err != nil {
		return fmt.Errorf("listing scale sets: %w", err)
	}

	for _, sSet := range scaleSets {
		slog.DebugContext(c.ctx, "loading scale set", "scale_set", sSet.ID)
		if err := c.handleScaleSetCreateOperation(sSet, cli); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(c.ctx, "failed to handle scale set create operation")
			continue
		}
	}
	return nil
}

func (c *Controller) Start() (err error) {
	slog.DebugContext(c.ctx, "starting scale set controller", "scale_set", c.consumerID)
	c.mux.Lock()
	if c.running {
		c.mux.Unlock()
		return nil
	}
	c.mux.Unlock()

	ghCli, err := github.Client(c.ctx, c.Entity)
	if err != nil {
		return fmt.Errorf("creating github client: %w", err)
	}

	slog.DebugContext(c.ctx, "loaging scale sets", "entity", c.Entity.String())
	if err := c.loadAllScaleSets(ghCli); err != nil {
		return fmt.Errorf("loading all scale sets: %w", err)
	}

	consumer, err := watcher.RegisterConsumer(
		c.ctx, c.consumerID,
		composeControllerWatcherFilters(c.Entity),
	)
	if err != nil {
		return fmt.Errorf("registering consumer: %w", err)
	}

	c.mux.Lock()
	c.ghCli = ghCli
	c.consumer = consumer
	c.running = true
	c.quit = make(chan struct{})
	c.mux.Unlock()

	go c.loop()
	return nil
}

func (c *Controller) Stop() error {
	slog.DebugContext(c.ctx, "stopping scale set controller", "scale_set", c.consumerID)
	c.mux.Lock()
	defer c.mux.Unlock()

	if !c.running {
		return nil
	}
	slog.DebugContext(c.ctx, "stopping scaleset controller", "entity", c.Entity.String())

	for scaleSetID, scaleSet := range c.ScaleSets {
		if err := scaleSet.Stop(); err != nil {
			slog.ErrorContext(c.ctx, "stopping worker for scale set", "scale_set_id", scaleSetID, "error", err)
			continue
		}
		delete(c.ScaleSets, scaleSetID)
	}

	c.running = false
	close(c.quit)
	c.quit = nil
	close(c.statusUpdates)
	c.statusUpdates = nil
	c.consumer.Close()

	return nil
}

func (c *Controller) updateTools() error {
	c.mux.Lock()
	defer c.mux.Unlock()

	slog.DebugContext(c.ctx, "updating tools for entity", "entity", c.Entity.String())

	tools, err := garmUtil.FetchTools(c.ctx, c.ghCli)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			c.ctx, "failed to update tools for entity", "entity", c.Entity.String())
		if errors.Is(err, runnerErrors.ErrUnauthorized) {
			// TODO: block all scale sets
			c.forgeCredsAreValid = false
		}
		return fmt.Errorf("failed to update tools for entity %s: %w", c.Entity.String(), err)
	}
	slog.DebugContext(c.ctx, "tools successfully updated for entity", "entity", c.Entity.String())
	c.forgeCredsAreValid = true
	cache.SetGithubToolsCache(c.Entity, tools)
	return nil
}

func (c *Controller) handleScaleSetStatusUpdates(status scaleSetStatus) {
	if status.scaleSet.ID == 0 {
		slog.DebugContext(c.ctx, "invalid scale set ID; ignoring")
		return
	}

	scaleSet, ok := c.ScaleSets[status.scaleSet.ID]
	if !ok {
		slog.DebugContext(c.ctx, "scale set not found; ignoring")
		return
	}

	scaleSet.updateStatus(status)
}

func (c *Controller) loop() {
	defer c.Stop()
	updateToolsTicker := time.NewTicker(common.PoolToolUpdateInterval)
	initialToolUpdate := make(chan struct{}, 1)
	defer close(initialToolUpdate)
	go func() {
		slog.InfoContext(c.ctx, "running initial tool update")
		if err := c.updateTools(); err != nil {
			slog.With(slog.Any("error", err)).Error("failed to update tools")
		}
		initialToolUpdate <- struct{}{}
	}()

	for {
		select {
		case payload, ok := <-c.consumer.Watch():
			if !ok {
				slog.InfoContext(c.ctx, "consumer channel closed")
				return
			}
			slog.InfoContext(c.ctx, "received payload")
			go c.handleWatcherEvent(payload)
		case <-c.ctx.Done():
			return
		case <-initialToolUpdate:
		case update, ok := <-c.statusUpdates:
			if !ok {
				return
			}
			go c.handleScaleSetStatusUpdates(update)
		case _, ok := <-updateToolsTicker.C:
			if !ok {
				slog.InfoContext(c.ctx, "update tools ticker closed")
				return
			}
			if err := c.updateTools(); err != nil {
				slog.With(slog.Any("error", err)).Error("failed to update tools")
			}
		case <-c.quit:
			return
		}
	}
}
