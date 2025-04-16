package scaleset

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util/github/scalesets"
)

func NewWorker(ctx context.Context, store dbCommon.Store, scaleSet params.ScaleSet, provider common.Provider, ghCli common.GithubClient) (*Worker, error) {
	consumerID := fmt.Sprintf("scaleset-worker-%s-%d", scaleSet.Name, scaleSet.ID)
	controllerInfo, err := store.ControllerInfo()
	if err != nil {
		return nil, fmt.Errorf("getting controller info: %w", err)
	}
	scaleSetCli, err := scalesets.NewClient(ghCli)
	if err != nil {
		return nil, fmt.Errorf("creating scale set client: %w", err)
	}
	return &Worker{
		ctx:            ctx,
		controllerInfo: controllerInfo,
		consumerID:     consumerID,
		store:          store,
		provider:       provider,
		Entity:         scaleSet,
		ghCli:          ghCli,
		scaleSetCli:    scaleSetCli,
	}, nil
}

type Worker struct {
	ctx            context.Context
	consumerID     string
	controllerInfo params.ControllerInfo

	provider common.Provider
	store    dbCommon.Store
	Entity   params.ScaleSet

	ghCli       common.GithubClient
	scaleSetCli *scalesets.ScaleSetClient
	consumer    dbCommon.Consumer

	listener *scaleSetListener

	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func (w *Worker) Stop() error {
	slog.DebugContext(w.ctx, "stopping scale set worker", "scale_set", w.consumerID)
	w.mux.Lock()
	defer w.mux.Unlock()

	if !w.running {
		return nil
	}

	w.consumer.Close()
	w.running = false
	if w.quit != nil {
		close(w.quit)
		w.quit = nil
	}
	w.listener.Stop()
	w.listener = nil
	return nil
}

func (w *Worker) Start() (err error) {
	slog.DebugContext(w.ctx, "starting scale set worker", "scale_set", w.consumerID)
	w.mux.Lock()
	defer w.mux.Unlock()

	if w.running {
		return nil
	}

	consumer, err := watcher.RegisterConsumer(
		w.ctx, w.consumerID,
		watcher.WithAll(
			watcher.WithScaleSetFilter(w.Entity),
			watcher.WithOperationTypeFilter(dbCommon.UpdateOperation),
		),
	)
	if err != nil {
		return fmt.Errorf("error registering consumer: %w", err)
	}
	defer func() {
		if err != nil {
			consumer.Close()
			w.consumer = nil
		}
	}()

	slog.DebugContext(w.ctx, "creating scale set listener")
	listener := newListener(w.ctx, w)

	slog.DebugContext(w.ctx, "starting scale set listener")
	if err := listener.Start(); err != nil {
		return fmt.Errorf("error starting listener: %w", err)
	}

	w.listener = listener
	w.consumer = consumer
	w.running = true
	w.quit = make(chan struct{})

	slog.DebugContext(w.ctx, "starting scale set worker loops", "scale_set", w.consumerID)
	go w.loop()
	go w.keepListenerAlive()
	return nil
}

func (w *Worker) SetGithubClient(client common.GithubClient) error {
	w.mux.Lock()
	defer w.mux.Unlock()

	if err := w.listener.Stop(); err != nil {
		slog.ErrorContext(w.ctx, "error stopping listener", "error", err)
	}

	w.ghCli = client
	scaleSetCli, err := scalesets.NewClient(client)
	if err != nil {
		return fmt.Errorf("error creating scale set client: %w", err)
	}
	w.scaleSetCli = scaleSetCli
	return nil
}

func (w *Worker) handleEvent(event dbCommon.ChangePayload) {
	scaleSet, ok := event.Payload.(params.ScaleSet)
	if !ok {
		slog.ErrorContext(w.ctx, "invalid payload for scale set type", "scale_set_type", event.EntityType, "payload", event.Payload)
		return
	}
	switch event.Operation {
	case dbCommon.UpdateOperation:
		slog.DebugContext(w.ctx, "got update operation")
		w.mux.Lock()
		w.Entity = scaleSet
		w.mux.Unlock()
	default:
		slog.DebugContext(w.ctx, "invalid operation type; ignoring", "operation_type", event.Operation)
	}
}

func (w *Worker) loop() {
	defer w.Stop()

	for {
		select {
		case <-w.quit:
			return
		case event, ok := <-w.consumer.Watch():
			if !ok {
				slog.InfoContext(w.ctx, "consumer channel closed")
				return
			}
			go w.handleEvent(event)
		case <-w.ctx.Done():
			slog.DebugContext(w.ctx, "context done")
			return
		}
	}
}

func (w *Worker) sleepWithCancel(sleepTime time.Duration) (canceled bool) {
	ticker := time.NewTicker(sleepTime)
	defer ticker.Stop()

	select {
	case <-ticker.C:
		return false
	case <-w.quit:
		return true
	case <-w.ctx.Done():
		return true
	}
}

func (w *Worker) keepListenerAlive() {
	var backoff time.Duration
	for {
		select {
		case <-w.quit:
			return
		case <-w.ctx.Done():
			return
		case <-w.listener.Wait():
			slog.DebugContext(w.ctx, "listener is stopped; attempting to restart")
			for {
				w.mux.Lock()
				w.listener.Stop() //cleanup
				slog.DebugContext(w.ctx, "attempting to restart")
				if err := w.listener.Start(); err != nil {
					w.mux.Unlock()
					slog.ErrorContext(w.ctx, "error restarting listener", "error", err)
					if backoff > 60*time.Second {
						backoff = 60 * time.Second
					} else if backoff == 0 {
						backoff = 5 * time.Second
						slog.InfoContext(w.ctx, "backing off restart attempt", "backoff", backoff)
					} else {
						backoff *= 2
					}
					slog.ErrorContext(w.ctx, "error restarting listener", "error", err, "backoff", backoff)
					if canceled := w.sleepWithCancel(backoff); canceled {
						slog.DebugContext(w.ctx, "listener restart canceled")
						return
					}
					continue
				}
				w.mux.Unlock()
				break
			}
		}
	}
}
