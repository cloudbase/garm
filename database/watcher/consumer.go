package watcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudbase/garm/database/common"
)

type consumer struct {
	messages chan common.ChangePayload
	filters  []common.PayloadFilterFunc
	id       string

	mux    sync.Mutex
	closed bool
	quit   chan struct{}
	ctx    context.Context
}

func (w *consumer) SetFilters(filters ...common.PayloadFilterFunc) {
	w.mux.Lock()
	defer w.mux.Unlock()
	w.filters = filters
}

func (w *consumer) Watch() <-chan common.ChangePayload {
	return w.messages
}

func (w *consumer) Close() {
	w.mux.Lock()
	defer w.mux.Unlock()
	if w.closed {
		return
	}
	close(w.messages)
	close(w.quit)
	w.closed = true
}

func (w *consumer) IsClosed() bool {
	w.mux.Lock()
	defer w.mux.Unlock()
	return w.closed
}

func (w *consumer) Send(payload common.ChangePayload) {
	w.mux.Lock()
	defer w.mux.Unlock()

	if w.closed {
		return
	}

	if len(w.filters) > 0 {
		shouldSend := true
		for _, filter := range w.filters {
			if !filter(payload) {
				shouldSend = false
				break
			}
		}

		if !shouldSend {
			return
		}
	}

	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()
	slog.DebugContext(w.ctx, "sending payload")
	select {
	case <-w.quit:
		slog.DebugContext(w.ctx, "consumer is closed")
	case <-w.ctx.Done():
		slog.DebugContext(w.ctx, "consumer is closed")
	case <-timer.C:
		slog.DebugContext(w.ctx, "timeout trying to send payload", "payload", payload)
	case w.messages <- payload:
	}
}
