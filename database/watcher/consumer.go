package watcher

import (
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
		shouldSend := false
		for _, filter := range w.filters {
			if filter(payload) {
				shouldSend = true
				break
			}
		}

		if !shouldSend {
			return
		}
	}

	slog.Info("Sending payload to consumer", "consumer", w.id)
	select {
	case w.messages <- payload:
	case <-time.After(1 * time.Second):
	}
}
