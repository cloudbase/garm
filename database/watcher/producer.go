package watcher

import (
	"sync"

	"github.com/cloudbase/garm/database/common"
)

type producer struct {
	closed bool
	mux    sync.Mutex
	id     string

	messages chan common.ChangePayload
	quit     chan struct{}
}

func (w *producer) Notify(payload common.ChangePayload) error {
	w.mux.Lock()
	defer w.mux.Unlock()

	if w.closed {
		return common.ErrProducerClosed
	}

	select {
	case w.messages <- payload:
	default:
		return common.ErrProducerTimeoutErr
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
