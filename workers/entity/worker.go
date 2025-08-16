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

	"golang.org/x/sync/errgroup"

	"github.com/cloudbase/garm/cache"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	garmUtil "github.com/cloudbase/garm/util"
	"github.com/cloudbase/garm/util/github"
	"github.com/cloudbase/garm/util/github/scalesets"
	"github.com/cloudbase/garm/workers/scaleset"
)

func NewWorker(ctx context.Context, store dbCommon.Store, entity params.ForgeEntity, providers map[string]common.Provider) (*Worker, error) {
	consumerID := fmt.Sprintf("entity-worker-%s", entity.ID)

	ctx = garmUtil.WithSlogContext(
		ctx,
		slog.Any("worker", consumerID))

	return &Worker{
		ctx:        ctx,
		consumerID: consumerID,
		store:      store,
		Entity:     entity,
		providers:  providers,
	}, nil
}

type Worker struct {
	ctx        context.Context
	consumerID string

	consumer dbCommon.Consumer
	store    dbCommon.Store
	ghCli    common.GithubClient

	Entity             params.ForgeEntity
	providers          map[string]common.Provider
	scaleSetController *scaleset.Controller

	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func (w *Worker) Stop() error {
	slog.DebugContext(w.ctx, "stopping entity worker", "entity", w.consumerID)
	w.mux.Lock()
	defer w.mux.Unlock()

	if !w.running {
		return nil
	}
	slog.DebugContext(w.ctx, "stopping entity worker")

	if err := w.scaleSetController.Stop(); err != nil {
		return fmt.Errorf("stopping scale set controller: %w", err)
	}

	w.running = false
	close(w.quit)
	w.consumer.Close()
	slog.DebugContext(w.ctx, "entity worker stopped", "entity", w.consumerID)
	return nil
}

func (w *Worker) Start() (err error) {
	slog.DebugContext(w.ctx, "starting entity worker", "entity", w.consumerID)
	w.mux.Lock()
	defer w.mux.Unlock()

	epType, err := w.Entity.GetForgeType()
	if err != nil {
		return fmt.Errorf("failed to get endpoint type: %w", err)
	}
	if epType != params.GithubEndpointType {
		return nil
	}

	ghCli, err := github.Client(w.ctx, w.Entity)
	if err != nil {
		return fmt.Errorf("creating github client: %w", err)
	}
	w.ghCli = ghCli
	cache.SetGithubClient(w.Entity.ID, ghCli)

	scaleSetController, err := scaleset.NewController(w.ctx, w.store, w.Entity, w.providers)
	if err != nil {
		return fmt.Errorf("creating scale set controller: %w", err)
	}

	if err := scaleSetController.Start(); err != nil {
		return fmt.Errorf("starting scale set controller: %w", err)
	}
	w.scaleSetController = scaleSetController

	defer func() {
		if err != nil {
			w.scaleSetController.Stop()
		}
	}()

	consumer, err := watcher.RegisterConsumer(
		w.ctx, w.consumerID,
		composeWorkerWatcherFilters(w.Entity),
	)
	if err != nil {
		return fmt.Errorf("registering consumer: %w", err)
	}
	w.consumer = consumer

	w.running = true
	w.quit = make(chan struct{})

	go w.loop()
	go w.consolidateRunnerLoop()
	return nil
}

func (w *Worker) IsRunning() bool {
	w.mux.Lock()
	defer w.mux.Unlock()
	return w.running
}

// consolidateRunnerState will list all runners on GitHub for this entity, sort by
// pool or scale set and pass those runners to the appropriate controller (pools or scale sets).
// The controller will then pass along to their respective workers the list of runners
// they should be responsible for. The workers will then cross check the current state
// from github with their local state and reconcile any differences. This cleans up
// any runners that have been removed out of band in either the provider or github.
func (w *Worker) consolidateRunnerState() error {
	scaleSetCli, err := scalesets.NewClient(w.ghCli)
	if err != nil {
		return fmt.Errorf("creating scaleset client: %w", err)
	}
	// Client is scoped to the current entity. Only runners in a repo/org/enterprise
	// will be listed.
	runners, err := scaleSetCli.ListAllRunners(w.ctx)
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

	g, ctx := errgroup.WithContext(w.ctx)
	g.Go(func() error {
		slog.DebugContext(ctx, "consolidating scale set runners", "entity", w.Entity.String(), "runners", runners)
		if err := w.scaleSetController.ConsolidateRunnerState(byScaleSetID); err != nil {
			return fmt.Errorf("consolidating runners for scale set: %w", err)
		}
		return nil
	})

	if err := w.waitForErrorGroupOrContextCancelled(g); err != nil {
		return fmt.Errorf("waiting for error group: %w", err)
	}
	return nil
}

func (w *Worker) waitForErrorGroupOrContextCancelled(g *errgroup.Group) error {
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
	case <-w.ctx.Done():
		return w.ctx.Err()
	case <-w.quit:
		return nil
	}
}

func (w *Worker) consolidateRunnerLoop() {
	ticker := time.NewTicker(common.PoolReapTimeoutInterval)
	defer ticker.Stop()

	for {
		select {
		case _, ok := <-ticker.C:
			if !ok {
				slog.InfoContext(w.ctx, "consolidate ticker closed")
				return
			}
			if err := w.consolidateRunnerState(); err != nil {
				w.addStatusEvent(fmt.Sprintf("failed to consolidate runner state: %q", err.Error()), params.EventError)
				slog.With(slog.Any("error", err)).Error("failed to consolidate runner state")
			}
		case <-w.ctx.Done():
			return
		case <-w.quit:
			return
		}
	}
}

func (w *Worker) loop() {
	defer w.Stop()
	for {
		select {
		case payload := <-w.consumer.Watch():
			slog.InfoContext(w.ctx, "received payload")
			w.handleWorkerWatcherEvent(payload)
		case <-w.ctx.Done():
			return
		case <-w.quit:
			return
		}
	}
}

func (w *Worker) addStatusEvent(msg string, level params.EventLevel) {
	if err := w.store.AddEntityEvent(w.ctx, w.Entity, params.StatusEvent, level, msg, 30); err != nil {
		slog.With(slog.Any("error", err)).Error("failed to add entity event")
	}
}
