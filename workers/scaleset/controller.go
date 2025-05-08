package scaleset

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	garmUtil "github.com/cloudbase/garm/util"
	"github.com/cloudbase/garm/util/github"
	"github.com/cloudbase/garm/util/github/scalesets"
)

const (
	// These are duplicated until we decide if we move the pool manager to the new
	// worker flow.
	poolIDLabelprefix     = "runner-pool-id:"
	controllerLabelPrefix = "runner-controller-id:"
)

func NewController(ctx context.Context, store dbCommon.Store, entity params.GithubEntity, providers map[string]common.Provider) (*Controller, error) {
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

	Entity params.GithubEntity

	consumer  dbCommon.Consumer
	store     dbCommon.Store
	providers map[string]common.Provider

	ghCli common.GithubClient

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
	c.consumer.Close()

	return nil
}

// consolidateRunnerState will list all runners on GitHub for this entity, sort by
// pool or scale set and pass those runners to the appropriate worker. The worker will
// then have the responsibility to cross check the runners from github with what it
// knows should be true from the database. Any inconsistency needs to be handled.
// If we have an offline runner in github but no database entry for it, we remove the
// runner from github. If we have a runner that is active in the provider but does not
// exist in github, we remove it from the provider and the database.
func (c *Controller) consolidateRunnerState() error {
	scaleSetCli, err := scalesets.NewClient(c.ghCli)
	if err != nil {
		return fmt.Errorf("creating scaleset client: %w", err)
	}
	// Client is scoped to the current entity. Only runners in a repo/org/enterprise
	// will be listed.
	runners, err := scaleSetCli.ListAllRunners(c.ctx)
	if err != nil {
		return fmt.Errorf("listing runners: %w", err)
	}

	byPoolID := make(map[string][]params.RunnerReference)
	byScaleSetID := make(map[int][]params.RunnerReference)
	for _, runner := range runners.RunnerReferences {
		if runner.RunnerScaleSetID != 0 {
			byScaleSetID[runner.RunnerScaleSetID] = append(byScaleSetID[runner.RunnerScaleSetID], runner)
		} else {
			poolID := poolIDFromLabels(runner)
			if poolID == "" {
				continue
			}
			byPoolID[poolID] = append(byPoolID[poolID], runner)
		}
	}

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

	consolidateTicker := time.NewTicker(common.PoolReapTimeoutInterval)
	defer consolidateTicker.Stop()

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
		case _, ok := <-consolidateTicker.C:
			if !ok {
				slog.InfoContext(c.ctx, "consolidate ticker closed")
				return
			}
			if err := c.consolidateRunnerState(); err != nil {
				if err := c.store.AddEntityEvent(c.ctx, c.Entity, params.StatusEvent, params.EventError, fmt.Sprintf("failed to consolidate runner state: %q", err.Error()), 30); err != nil {
					slog.With(slog.Any("error", err)).Error("failed to add entity event")
				}
				slog.With(slog.Any("error", err)).Error("failed to consolidate runner state")
			}
		case <-c.quit:
			return
		}
	}
}
