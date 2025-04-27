package entity

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	garmUtil "github.com/cloudbase/garm/util"
	"github.com/cloudbase/garm/workers/scaleset"
)

func NewWorker(ctx context.Context, store dbCommon.Store, entity params.GithubEntity, providers map[string]common.Provider) (*Worker, error) {
	consumerID := fmt.Sprintf("entity-worker-%s", entity.String())

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

	Entity             params.GithubEntity
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
	w.scaleSetController = nil

	w.running = false
	close(w.quit)
	w.consumer.Close()
	return nil
}

func (w *Worker) Start() (err error) {
	slog.DebugContext(w.ctx, "starting entity worker", "entity", w.consumerID)
	w.mux.Lock()
	defer w.mux.Unlock()

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
			w.scaleSetController = nil
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
	return nil
}

func (w *Worker) loop() {
	defer w.Stop()
	for {
		select {
		case payload := <-w.consumer.Watch():
			slog.InfoContext(w.ctx, "received payload")
			go w.handleWorkerWatcherEvent(payload)
		case <-w.ctx.Done():
			return
		case <-w.quit:
			return
		}
	}
}
