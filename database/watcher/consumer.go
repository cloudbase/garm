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
