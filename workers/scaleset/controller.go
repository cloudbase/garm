package scaleset

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"golang.org/x/sync/errgroup"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	garmUtil "github.com/cloudbase/garm/util"
)

func NewController(ctx context.Context, store dbCommon.Store, entity params.ForgeEntity, providers map[string]common.Provider) (*Controller, error) {
	consumerID := fmt.Sprintf("scaleset-controller-%s", entity.String())

	ctx = garmUtil.WithSlogContext(
		ctx,
		slog.Any("worker", consumerID))

	return &Controller{
		ctx:        ctx,
		consumerID: consumerID,
		ScaleSets:  make(map[uint]*scaleSet),
		Entity:     entity,
		providers:  providers,
		store:      store,
	}, nil
}

type scaleSet struct {
	scaleSet params.ScaleSet
	worker   *Worker

	mux sync.Mutex
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

	Entity params.ForgeEntity

	consumer  dbCommon.Consumer
	store     dbCommon.Store
	providers map[string]common.Provider

	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func (c *Controller) loadAllScaleSets() error {
	scaleSets, err := c.store.ListEntityScaleSets(c.ctx, c.Entity)
	if err != nil {
		return fmt.Errorf("listing scale sets: %w", err)
	}

	for _, sSet := range scaleSets {
		slog.DebugContext(c.ctx, "loading scale set", "scale_set", sSet.ID)
		if err := c.handleScaleSetCreateOperation(sSet); err != nil {
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

	slog.DebugContext(c.ctx, "loaging scale sets", "entity", c.Entity.String())
	if err := c.loadAllScaleSets(); err != nil {
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
	c.consumer.Close()

	return nil
}

// ConsolidateRunnerState will send a list of existing github runners to each scale set worker.
// The scale set worker will then need to cross check the existing runners in Github with the sate
// in the database. Any inconsistencies will b reconciliated. This cleans up any manually removed
// runners in either github or the providers.
func (c *Controller) ConsolidateRunnerState(byScaleSetID map[int][]params.RunnerReference) error {
	g, ctx := errgroup.WithContext(c.ctx)
	for _, scaleSet := range c.ScaleSets {
		runners := byScaleSetID[scaleSet.scaleSet.ScaleSetID]
		g.Go(func() error {
			slog.DebugContext(ctx, "consolidating runners for scale set", "scale_set_id", scaleSet.scaleSet.ScaleSetID, "runners", runners)
			if err := scaleSet.worker.consolidateRunnerState(runners); err != nil {
				return fmt.Errorf("consolidating runners for scale set %d: %w", scaleSet.scaleSet.ScaleSetID, err)
			}
			return nil
		})
	}
	if err := c.waitForErrorGroupOrContextCancelled(g); err != nil {
		return fmt.Errorf("waiting for error group: %w", err)
	}
	return nil
}

func (c *Controller) waitForErrorGroupOrContextCancelled(g *errgroup.Group) error {
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
	case <-c.ctx.Done():
		return c.ctx.Err()
	case <-c.quit:
		return nil
	}
}

func (c *Controller) loop() {
	defer c.Stop()

	for {
		select {
		case payload, ok := <-c.consumer.Watch():
			if !ok {
				slog.InfoContext(c.ctx, "consumer channel closed")
				return
			}
			slog.InfoContext(c.ctx, "received payload")
			c.handleWatcherEvent(payload)
		case <-c.ctx.Done():
			return
		case <-c.quit:
			return
		}
	}
}
