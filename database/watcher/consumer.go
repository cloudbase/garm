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

// queueWarnThreshold is the consumer queue depth at which we start warning
// that a consumer is not keeping up. Events are never dropped. The queue
// grows until the consumer drains it, so sustained growth indicates a wedged
// consumer and deserves a loud signal.
const queueWarnThreshold = 1024

type consumer struct {
	// messages is the channel returned by Watch(). It is fed exclusively by
	// the dispatch goroutine, which also closes it on exit.
	messages chan common.ChangePayload
	// in hands accepted payloads from Send to the dispatch goroutine. It is
	// buffered so the producer completes with a plain copy in the common
	// case, without waiting for the dispatch goroutine to be scheduled. The
	// dispatch loop drains it into an unbounded queue, so a full buffer only
	// ever means a brief wait for the dispatcher and never a drop.
	in      chan common.ChangePayload
	filters []common.PayloadFilterFunc
	id      string

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
	w.closed = true
	// The dispatch goroutine closes w.messages on its way out. Closing it
	// here would race a concurrent dispatch send.
	close(w.quit)
}

func (w *consumer) IsClosed() bool {
	w.mux.Lock()
	defer w.mux.Unlock()
	return w.closed
}

// Send delivers a payload to this consumer. Delivery is in call order, never
// blocks beyond a goroutine handoff and never drops. The payload is either
// filtered out, accepted into the dispatch queue, or the consumer is closed.
func (w *consumer) Send(payload common.ChangePayload) {
	w.mux.Lock()
	if w.closed {
		w.mux.Unlock()
		return
	}
	filters := w.filters
	w.mux.Unlock()

	if len(filters) > 0 {
		for _, filter := range filters {
			if !filter(payload) {
				return
			}
		}
	}

	select {
	case w.in <- payload:
	case <-w.quit:
	case <-w.ctx.Done():
	}
}

// dispatch owns delivery to the messages channel. It accepts payloads from
// Send into an in-memory FIFO queue and feeds them to messages one at a
// time, so a slow consumer only grows its own queue and can never block a
// producer or another consumer. It closes messages on exit.
func (w *consumer) dispatch() {
	defer close(w.messages)

	var queue []common.ChangePayload
	for {
		if len(queue) == 0 {
			select {
			case payload := <-w.in:
				queue = append(queue, payload)
			case <-w.quit:
				return
			case <-w.ctx.Done():
				return
			}
			continue
		}

		select {
		case payload := <-w.in:
			queue = append(queue, payload)
			if len(queue)%queueWarnThreshold == 0 {
				slog.WarnContext(
					w.ctx, "consumer is not keeping up with events",
					"consumer_id", w.id, "queue_depth", len(queue))
			}
		case w.messages <- queue[0]:
			queue = queue[1:]
		case <-w.quit:
			return
		case <-w.ctx.Done():
			return
		}
	}
}
