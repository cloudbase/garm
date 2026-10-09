// Copyright 2026 Cloudbase Solutions SRL
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

package jobs

import (
	"context"
	"log/slog"
	"sync"
	"time"

	dbCommon "github.com/cloudbase/garm/database/common"
	garmUtil "github.com/cloudbase/garm/util"
)

// Job records are informational once they are no longer actionable. Keep
// them around for a day so users can look at recent history, then clean
// them up.
const (
	retentionInterval = 1 * time.Hour
	retentionAge      = 24 * time.Hour
)

// Worker periodically prunes old webhook and scale set job records.
type Worker struct {
	ctx   context.Context
	store dbCommon.Store

	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func NewWorker(ctx context.Context, store dbCommon.Store) *Worker {
	ctx = garmUtil.WithSlogContext(ctx, slog.Any("worker", "job_retention"))
	return &Worker{
		ctx:   ctx,
		store: store,
		quit:  make(chan struct{}),
	}
}

func (w *Worker) Start() error {
	w.mux.Lock()
	defer w.mux.Unlock()

	if w.running {
		return nil
	}
	w.running = true
	go w.loop()
	return nil
}

func (w *Worker) Stop() error {
	w.mux.Lock()
	defer w.mux.Unlock()

	if !w.running {
		return nil
	}
	w.running = false
	close(w.quit)
	return nil
}

func (w *Worker) loop() {
	// The ticker deliberately does not fire at startup. After a long
	// outage, jobs past the retention age may still be live. The scale set
	// listeners refresh them once sessions resync, so give them a full
	// interval before pruning by age.
	ticker := time.NewTicker(retentionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.quit:
			return
		case <-ticker.C:
			w.prune(retentionAge)
		}
	}
}

func (w *Worker) prune(age time.Duration) {
	if err := w.store.DeleteInactionableJobs(w.ctx, age); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(w.ctx, "failed to prune workflow jobs")
	}
	if err := w.store.DeleteOldScaleSetJobs(w.ctx, age); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(w.ctx, "failed to prune scale set jobs")
	}
}
