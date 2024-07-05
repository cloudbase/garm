package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonUtil "github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/websocket"
)

func NewHandler(ctx context.Context, client *websocket.Client) (*EventHandler, error) {
	if client == nil {
		return nil, runnerErrors.ErrUnauthorized
	}

	newID := commonUtil.NewID()
	userID := auth.UserID(ctx)
	if userID == "" {
		return nil, runnerErrors.ErrUnauthorized
	}
	consumerID := fmt.Sprintf("ws-event-watcher-%s-%s", userID, newID)
	consumer, err := watcher.RegisterConsumer(
		// Filter everything by default. Users should set up filters
		// after registration.
		ctx, consumerID, watcher.WithNone())
	if err != nil {
		return nil, err
	}

	handler := &EventHandler{
		client:   client,
		ctx:      ctx,
		consumer: consumer,
		done:     make(chan struct{}),
	}
	client.SetMessageHandler(handler.HandleClientMessages)

	return handler, nil
}

type EventHandler struct {
	client   *websocket.Client
	consumer common.Consumer

	ctx     context.Context
	done    chan struct{}
	running bool

	mux sync.Mutex
}

func (e *EventHandler) loop() {
	defer e.Stop()

	for {
		select {
		case <-e.ctx.Done():
			slog.DebugContext(e.ctx, "context done, stopping event handler")
			return
		case <-e.client.Done():
			slog.DebugContext(e.ctx, "client done, stopping event handler")
			return
		case <-e.Done():
			slog.DebugContext(e.ctx, "done channel closed, stopping event handler")
		case event, ok := <-e.consumer.Watch():
			if !ok {
				slog.DebugContext(e.ctx, "watcher closed, stopping event handler")
				return
			}
			asJs, err := json.Marshal(event)
			if err != nil {
				slog.ErrorContext(e.ctx, "failed to marshal event", "error", err)
				continue
			}
			if _, err := e.client.Write(asJs); err != nil {
				slog.ErrorContext(e.ctx, "failed to write event", "error", err)
			}
		}
	}
}

func (e *EventHandler) Start() error {
	e.mux.Lock()
	defer e.mux.Unlock()

	if e.running {
		return nil
	}

	if err := e.client.Start(); err != nil {
		return err
	}
	e.running = true
	go e.loop()
	return nil
}

func (e *EventHandler) Stop() {
	e.mux.Lock()
	defer e.mux.Unlock()

	if !e.running {
		return
	}
	e.running = false
	e.consumer.Close()
	e.client.Stop()
	close(e.done)
}

func (e *EventHandler) Done() <-chan struct{} {
	return e.done
}

// optionsToWatcherFilters converts the Options struct to a PayloadFilterFunc.
// The client will send an array of filters that indicates which entities and which
// operations the client is interested in. The behavior is that of "any" filter.
// Which means that if any of the elements in the array match an event, it will be
// sent to the websocket.
// Alternatively, clients can choose to get everything.
func (e *EventHandler) optionsToWatcherFilters(opt Options) common.PayloadFilterFunc {
	if opt.SendEverything {
		return watcher.WithEverything()
	}

	var funcs []common.PayloadFilterFunc
	for _, filter := range opt.Filters {
		var filterFunc []common.PayloadFilterFunc
		if filter.EntityType == "" {
			return watcher.WithNone()
		}
		filterFunc = append(filterFunc, watcher.WithEntityTypeFilter(filter.EntityType))
		if len(filter.Operations) > 0 {
			var opFunc []common.PayloadFilterFunc
			for _, op := range filter.Operations {
				opFunc = append(opFunc, watcher.WithOperationTypeFilter(op))
			}
			filterFunc = append(filterFunc, watcher.WithAny(opFunc...))
		}
		funcs = append(funcs, watcher.WithAll(filterFunc...))
	}
	return watcher.WithAny(funcs...)
}

func (e *EventHandler) HandleClientMessages(message []byte) error {
	if e.consumer == nil {
		return fmt.Errorf("consumer not initialized")
	}

	var opt Options
	if err := json.Unmarshal(message, &opt); err != nil {
		slog.ErrorContext(e.ctx, "failed to unmarshal message from client", "error", err, "message", string(message))
		// Client is in error. Disconnect.
		e.client.Write([]byte("failed to unmarshal filter"))
		e.Stop()
		return nil
	}

	if err := opt.Validate(); err != nil {
		if errors.Is(err, common.ErrNoFiltersProvided) {
			slog.DebugContext(e.ctx, "no filters provided; ignoring")
			return nil
		}
		slog.ErrorContext(e.ctx, "invalid filter", "error", err)
		e.client.Write([]byte("invalid filter"))
		e.Stop()
		return nil
	}

	watcherFilters := e.optionsToWatcherFilters(opt)
	e.consumer.SetFilters(watcherFilters)
	return nil
}
