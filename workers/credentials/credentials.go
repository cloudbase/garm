package credentials

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cloudbase/garm/cache"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	garmUtil "github.com/cloudbase/garm/util"
)

func NewWorker(ctx context.Context, store dbCommon.Store) (*Worker, error) {
	consumerID := "credentials-worker"

	ctx = garmUtil.WithSlogContext(
		ctx,
		slog.Any("worker", consumerID))

	return &Worker{
		ctx:         ctx,
		consumerID:  consumerID,
		store:       store,
		running:     false,
		quit:        make(chan struct{}),
		credentials: make(map[uint]params.GithubCredentials),
	}, nil
}

// Worker is responsible for maintaining the credentials cache.
type Worker struct {
	consumerID string
	ctx        context.Context

	consumer dbCommon.Consumer
	store    dbCommon.Store

	credentials map[uint]params.GithubCredentials

	running bool
	quit    chan struct{}

	mux sync.Mutex
}

func (w *Worker) loadAllCredentials() error {
	creds, err := w.store.ListGithubCredentials(w.ctx)
	if err != nil {
		return err
	}

	for _, cred := range creds {
		w.credentials[cred.ID] = cred
		cache.SetGithubCredentials(cred)
	}

	return nil
}

func (w *Worker) Start() error {
	w.mux.Lock()
	defer w.mux.Unlock()

	if w.running {
		return nil
	}
	slog.DebugContext(w.ctx, "starting credentials worker")
	if err := w.loadAllCredentials(); err != nil {
		return fmt.Errorf("loading credentials: %w", err)
	}

	consumer, err := watcher.RegisterConsumer(
		w.ctx, w.consumerID,
		watcher.WithEntityTypeFilter(dbCommon.GithubCredentialsEntityType),
	)
	if err != nil {
		return fmt.Errorf("failed to create consumer for entity controller: %w", err)
	}
	w.consumer = consumer

	w.running = true
	go w.loop()
	return nil
}

func (w *Worker) Stop() error {
	w.mux.Lock()
	defer w.mux.Unlock()

	if !w.running {
		return nil
	}

	close(w.quit)
	w.running = false

	return nil
}

func (w *Worker) loop() {
	defer w.Stop()

	for {
		select {
		case <-w.quit:
			return
		case event, ok := <-w.consumer.Watch():
			if !ok {
				slog.ErrorContext(w.ctx, "consumer channel closed")
				return
			}
			creds, ok := event.Payload.(params.GithubCredentials)
			if !ok {
				slog.ErrorContext(w.ctx, "invalid payload for entity type", "entity_type", event.EntityType, "payload", event.Payload)
				continue
			}
			w.mux.Lock()
			switch event.Operation {
			case dbCommon.DeleteOperation:
				slog.DebugContext(w.ctx, "got delete operation")
				delete(w.credentials, creds.ID)
				cache.DeleteGithubCredentials(creds.ID)
			default:
				w.credentials[creds.ID] = creds
				cache.SetGithubCredentials(creds)
			}
			w.mux.Unlock()
		}
	}
}
