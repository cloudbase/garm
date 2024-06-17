package watcher

import (
	"context"
	"sync"
	"time"

	"github.com/cloudbase/garm/database/common"
)

type producer struct {
	closed bool
	mux    sync.Mutex
	id     string

	messages chan common.ChangePayload
	quit     chan struct{}
	ctx      context.Context
}

func (w *producer) Notify(payload common.ChangePayload) error {
	w.mux.Lock()
	defer w.mux.Unlock()

	if w.closed {
		return common.ErrProducerClosed
	}

	select {
	case <-w.quit:
		return common.ErrProducerClosed
	case <-w.ctx.Done():
		return common.ErrProducerClosed
	case <-time.After(1 * time.Second):
		return common.ErrProducerTimeoutErr
	case w.messages <- payload:
	}
	return nil
}

func (w *producer) Close() {
	w.mux.Lock()
	defer w.mux.Unlock()
	if w.closed {
		return
	}
	w.closed = true
	close(w.messages)
	close(w.quit)
}

func (w *producer) IsClosed() bool {
	w.mux.Lock()
	defer w.mux.Unlock()
	return w.closed
}
