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
	workersCommon "github.com/cloudbase/garm/workers/common"
)

const scaleSetRetryLoopInterval = 5 * time.Second

func NewController(ctx context.Context, store dbCommon.Store, entity params.ForgeEntity, providers map[string]common.Provider) (*Controller, error) {
	consumerID := fmt.Sprintf("scaleset-controller-%s", entity.ID)

	ctx = garmUtil.WithSlogContext(
		ctx,
		slog.Any("worker", consumerID),
		slog.Any("entity", entity.String()),
		slog.Any("endpoint", entity.Credentials.Endpoint.Name),
	)

	return &Controller{
		ctx:        ctx,
		consumerID: consumerID,
		Entity:     entity,
		providers:  providers,
		store:      store,
		backoff:    workersCommon.NewBackoff(workersCommon.DefaultBackoffConfig()),
	}, nil
}

type scaleSet struct {
	scaleSet params.ScaleSet
	worker   *Worker

	mux sync.Mutex
}

func (s *scaleSet) SetWorker(worker *Worker) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.worker = worker
}

func (s *scaleSet) SetScaleSet(sSet params.ScaleSet) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.scaleSet = sSet
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

	// sync.Map[uint]*scaleSet
	ScaleSets sync.Map

	Entity params.ForgeEntity

	consumer  dbCommon.Consumer
	store     dbCommon.Store
	providers map[string]common.Provider
	backoff   *workersCommon.Backoff

	eventQueue *workersCommon.UnboundedChan[dbCommon.ChangePayload]

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

	forgeType, err := c.Entity.GetForgeType()
	if err != nil {
		return fmt.Errorf("getting forge type: %w", err)
	}
	if forgeType == params.GithubEndpointType {
		// scale sets are only available in Github
		slog.DebugContext(c.ctx, "loaging scale sets", "entity", c.Entity.String())
		if err := c.loadAllScaleSets(); err != nil {
			return fmt.Errorf("loading all scale sets: %w", err)
		}
	}

	consumer, err := watcher.RegisterConsumer(
		c.ctx, c.consumerID,
		composeControllerWatcherFilters(c.Entity),
	)
	if err != nil {
		return fmt.Errorf("registering consumer %q: %w", c.consumerID, err)
	}

	c.mux.Lock()
	c.consumer = consumer
	c.running = true
	c.quit = make(chan struct{})
	c.eventQueue = workersCommon.NewUnboundedChan[dbCommon.ChangePayload](c.ctx, c.quit)
	c.mux.Unlock()

	go c.loop()
	go c.eventQueue.Process(c.handleWatcherEvent)
	go workersCommon.RetryLoop(c.ctx, c.quit, scaleSetRetryLoopInterval, c.retryFailedScaleSets)
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

	c.ScaleSets.Range(func(key, value any) bool {
		scaleSetID := key.(uint)
		set := value.(*scaleSet)
		if err := set.Stop(); err != nil {
			slog.ErrorContext(c.ctx, "stopping worker for scale set", "scale_set_id", scaleSetID, "error", err)
			return true
		}
		c.ScaleSets.Delete(scaleSetID)
		return true
	})

	c.running = false
	close(c.quit)
	c.consumer.Close()
	slog.DebugContext(c.ctx, "stopped scale set controller", "entity", c.Entity.String())
	return nil
}

// ConsolidateRunnerState will send a list of existing github runners to each scale set worker.
// The scale set worker will then need to cross check the existing runners in Github with the sate
// in the database. Any inconsistencies will be reconciliated. This cleans up any manually removed
// runners in either github or the providers.
func (c *Controller) ConsolidateRunnerState(byScaleSetID map[int][]params.RunnerReference) error {
	g, ctx := errgroup.WithContext(c.ctx)
	c.ScaleSets.Range(func(_, value any) bool {
		set := value.(*scaleSet)
		if set.worker == nil || !set.worker.IsRunning() {
			return true
		}
		runners := byScaleSetID[set.scaleSet.ScaleSetID]
		g.Go(func() error {
			slog.DebugContext(ctx, "consolidating runners for scale set", "scale_set_id", set.scaleSet.ScaleSetID, "runners", runners)
			if err := set.worker.consolidateRunnerState(runners); err != nil {
				return fmt.Errorf("consolidating runners for scale set %d: %w", set.scaleSet.ScaleSetID, err)
			}
			return nil
		})
		return true
	})
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

func (c *Controller) retryFailedScaleSets() {
	c.mux.Lock()
	defer c.mux.Unlock()

	if !c.running {
		return
	}

	c.ScaleSets.Range(func(key, value any) bool {
		scaleSetID := key.(uint)
		set := value.(*scaleSet)
		backoffKey := scaleSetBackoffKey(set.scaleSet)

		if set.worker != nil && set.worker.IsRunning() {
			return true
		}

		if !c.backoff.ShouldRetry(backoffKey) {
			return true
		}

		slog.InfoContext(c.ctx, "retrying failed scale set", "scale_set_id", scaleSetID)

		set.mux.Lock()
		worker := set.worker
		set.mux.Unlock()

		if worker == nil {
			var err error
			worker, err = c.createScaleSetWorker(set.scaleSet)
			if err != nil {
				slog.ErrorContext(c.ctx, "retry failed: creating scale set worker", "scale_set_id", scaleSetID, "error", err)
				c.backoff.RecordFailure(backoffKey)
				return true
			}
			set.SetWorker(worker)
		}

		if err := worker.Start(); err != nil {
			slog.ErrorContext(c.ctx, "retry failed: starting scale set worker", "scale_set_id", scaleSetID, "error", err)
			c.backoff.RecordFailure(backoffKey)
			return true
		}

		slog.InfoContext(c.ctx, "scale set worker successfully started after retry", "scale_set_id", scaleSetID)
		c.backoff.RecordSuccess(backoffKey)
		return true
	})
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
