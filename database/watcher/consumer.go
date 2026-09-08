// Copyright 2025 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package watcher

import (
	"context"
	"log/slog"
	"sync"

	"github.com/cloudbase/garm/database/common"
)

// queueWarnThreshold is the pending-queue depth at which we start warning
// that a consumer is not keeping up with the event stream. Events are never
// dropped; this only makes a slow consumer visible.
const queueWarnThreshold = 512

type consumer struct {
	messages chan common.ChangePayload
	filters  []common.PayloadFilterFunc
	id       string

	mux     sync.Mutex
	cond    *sync.Cond
	pending []common.ChangePayload
	closed  bool
	quit    chan struct{}
	ctx     context.Context
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
	close(w.quit)
	w.closed = true
	// Wake the dispatch loop so it notices the closed state and closes
	// the messages channel.
	w.cond.Broadcast()
}

func (w *consumer) IsClosed() bool {
	w.mux.Lock()
	defer w.mux.Unlock()
	return w.closed
}

// Send enqueues a payload for delivery to this consumer. It never blocks on
// the consumer and never drops events: payloads are appended to an ordered
// queue drained by the dispatch loop. Callers invoking Send sequentially are
// guaranteed in-order delivery, which consumers rely on (e.g. an instance
// update followed by its delete must not be observed in reverse).
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

	w.pending = append(w.pending, payload)
	if len(w.pending) >= queueWarnThreshold && len(w.pending)%queueWarnThreshold == 0 {
		slog.WarnContext(w.ctx, "consumer is falling behind on events", "consumer_id", w.id, "pending_events", len(w.pending))
	}
	w.cond.Signal()
}

// dispatch drains the pending queue in order, delivering each payload to the
// messages channel. It owns closing the messages channel: doing it here (and
// only here) guarantees we never send on a closed channel.
func (w *consumer) dispatch() {
	defer close(w.messages)
	for {
		w.mux.Lock()
		for len(w.pending) == 0 && !w.closed {
			w.cond.Wait()
		}
		if w.closed {
			w.mux.Unlock()
			return
		}
		payload := w.pending[0]
		w.pending = w.pending[1:]
		if len(w.pending) == 0 {
			// Don't pin the backing array of a previously grown queue.
			w.pending = nil
		}
		w.mux.Unlock()

		select {
		case <-w.quit:
			return
		case <-w.ctx.Done():
			return
		case w.messages <- payload:
		}
	}
}
