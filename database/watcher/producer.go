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

	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()
	select {
	case <-w.quit:
		return common.ErrProducerClosed
	case <-w.ctx.Done():
		return common.ErrProducerClosed
	case <-timer.C:
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
