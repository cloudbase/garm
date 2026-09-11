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

// Package metrics implements a database watcher consumer that derives
// event-driven Prometheus metrics from entity changes: runner lifecycle
// durations and job queue/execution timings. The watcher only surfaces
// writes, which is all this worker needs; read-path metrics (provider
// operations, listener polls) are instrumented at their call sites.
//
// The handler does no I/O and no network calls. It reads the event payload,
// consults the in-memory cache for label resolution and updates Prometheus
// collectors. The watcher consumer machinery guarantees a slow consumer only
// grows its own queue and can never block the producer.
package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/cache"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
)

const consumerID = "metrics"

// sweepInterval is how often stale tracking entries are evicted.
const sweepInterval = 15 * time.Minute

// jobStateTTL is how long a job tracking entry may go without updates before
// it is evicted.
const jobStateTTL = 24 * time.Hour

// instanceState tracks the last observed state of an instance so status
// transitions can be detected and observed exactly once. Durations are only
// recorded for transitions we witnessed or that carry their own timestamps;
// an instance first seen mid-flight is baselined without observations.
type instanceState struct {
	status            commonParams.InstanceStatus
	runnerStatus      params.RunnerStatus
	pendingDeleteAt   time.Time
	provisionObserved bool
	readyObserved     bool
	deletionObserved  bool
	lastSeen          time.Time
}

// jobState tracks the last observed status of a job so repeated update
// events (e.g. lock changes) are not observed as transitions.
type jobState struct {
	status   string
	lastSeen time.Time
}

// Worker consumes database change events and updates event-driven metrics.
type Worker struct {
	ctx      context.Context
	consumer dbCommon.Consumer

	mux       sync.Mutex
	running   bool
	quit      chan struct{}
	instances map[string]*instanceState
	jobs      map[int64]*jobState
}

func NewWorker(ctx context.Context) *Worker {
	return &Worker{
		ctx:       ctx,
		quit:      make(chan struct{}),
		instances: make(map[string]*instanceState),
		jobs:      make(map[int64]*jobState),
	}
}

func (w *Worker) Start() error {
	w.mux.Lock()
	defer w.mux.Unlock()

	if w.running {
		return nil
	}

	consumer, err := watcher.RegisterConsumer(
		w.ctx, consumerID,
		watcher.WithAny(
			watcher.WithEntityTypeFilter(dbCommon.InstanceEntityType),
			watcher.WithEntityTypeFilter(dbCommon.JobEntityType),
		),
	)
	if err != nil {
		return fmt.Errorf("registering metrics consumer: %w", err)
	}
	w.consumer = consumer

	// Baseline instances that already exist so a later status-bearing
	// update (like an agent heartbeat) on a long-running instance is not
	// mistaken for a fresh transition.
	for _, instance := range cache.GetAllInstancesCache() {
		w.instances[instance.Name] = &instanceState{
			status:            instance.Status,
			runnerStatus:      instance.RunnerStatus,
			provisionObserved: true,
			readyObserved:     true,
			lastSeen:          time.Now(),
		}
	}

	w.running = true
	go w.loop()
	go w.sweepLoop()
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
	w.consumer.Close()
	return nil
}

func (w *Worker) loop() {
	for {
		select {
		case <-w.quit:
			return
		case <-w.ctx.Done():
			return
		case event, ok := <-w.consumer.Watch():
			if !ok {
				return
			}
			w.handleEvent(event)
		}
	}
}

func (w *Worker) handleEvent(event dbCommon.ChangePayload) {
	switch event.EntityType {
	case dbCommon.InstanceEntityType:
		instance, ok := event.Payload.(params.Instance)
		if !ok {
			slog.DebugContext(w.ctx, "invalid payload type for instance event", "payload", event.Payload)
			return
		}
		w.handleInstanceEvent(event.Operation, instance)
	case dbCommon.JobEntityType:
		job, ok := event.Payload.(params.Job)
		if !ok {
			slog.DebugContext(w.ctx, "invalid payload type for job event", "payload", event.Payload)
			return
		}
		w.handleJobEvent(event.Operation, job)
	}
}

// instanceLabels resolves the provider, owner and pool type labels for an
// instance from the in-memory cache. The owner is the entity String() form
// (e.g. "owner/repo" for repositories); a bare repo name would be ambiguous
// across owners.
func instanceLabels(instance params.Instance) (provider, owner, poolType string) {
	var entityID string
	switch {
	case instance.PoolID != "":
		if pool, ok := cache.GetPoolByID(instance.PoolID); ok {
			provider = pool.ProviderName
			poolType = string(pool.PoolType())
			if entity, err := pool.GetEntity(); err == nil {
				entityID = entity.ID
			}
		}
	case instance.ScaleSetID != 0:
		if scaleSet, ok := cache.GetScaleSetByID(instance.ScaleSetID); ok {
			provider = scaleSet.ProviderName
			poolType = string(scaleSet.ScaleSetType())
			if entity, err := scaleSet.GetEntity(); err == nil {
				entityID = entity.ID
			}
		}
	}
	if provider == "" {
		provider = instance.ProviderName
	}
	if cachedEntity, ok := cache.GetEntity(entityID); ok {
		owner = cachedEntity.String()
	}
	return provider, owner, poolType
}

func isDeparting(status commonParams.InstanceStatus) bool {
	switch status {
	case commonParams.InstancePendingDelete, commonParams.InstancePendingForceDelete,
		commonParams.InstanceDeleting, commonParams.InstanceDeleted:
		return true
	}
	return false
}

func isRunnerReady(status params.RunnerStatus) bool {
	return status == params.RunnerIdle || status == params.RunnerActive
}

func (w *Worker) handleInstanceEvent(op dbCommon.OperationType, instance params.Instance) {
	w.mux.Lock()
	defer w.mux.Unlock()

	state, known := w.instances[instance.Name]

	if op == dbCommon.DeleteOperation {
		if known && !state.pendingDeleteAt.IsZero() && !state.deletionObserved {
			provider, _, _ := instanceLabels(instance)
			metrics.RunnerDeletionDuration.WithLabelValues(provider).
				Observe(time.Since(state.pendingDeleteAt).Seconds())
		}
		delete(w.instances, instance.Name)
		return
	}

	if !known {
		// First sight of this instance. Only a create event may be treated
		// as the start of a lifecycle; an update for an unknown instance is
		// baselined without observations, since we cannot tell how long ago
		// its transitions actually happened.
		state = &instanceState{
			status:            instance.Status,
			runnerStatus:      instance.RunnerStatus,
			provisionObserved: op != dbCommon.CreateOperation,
			readyObserved:     op != dbCommon.CreateOperation,
			lastSeen:          time.Now(),
		}
		w.instances[instance.Name] = state
		return
	}

	now := time.Now()
	state.lastSeen = now

	// The database write timestamp is closer to the actual transition than
	// the moment this event got handled.
	transitionedAt := instance.UpdatedAt
	if transitionedAt.IsZero() {
		transitionedAt = now
	}

	if !state.provisionObserved && state.status != commonParams.InstanceRunning && instance.Status == commonParams.InstanceRunning {
		state.provisionObserved = true
		provider, owner, poolType := instanceLabels(instance)
		observeDuration(metrics.RunnerProvisionDuration.WithLabelValues(provider, owner, poolType),
			transitionedAt.Sub(instance.CreatedAt))
	}

	if !state.readyObserved && !isRunnerReady(state.runnerStatus) && isRunnerReady(instance.RunnerStatus) {
		state.readyObserved = true
		provider, owner, poolType := instanceLabels(instance)
		observeDuration(metrics.RunnerReadyDuration.WithLabelValues(provider, owner, poolType),
			transitionedAt.Sub(instance.CreatedAt))
	}

	if !isDeparting(state.status) && isDeparting(instance.Status) {
		state.pendingDeleteAt = now
	}

	if !state.deletionObserved && !state.pendingDeleteAt.IsZero() && instance.Status == commonParams.InstanceDeleted {
		state.deletionObserved = true
		provider, _, _ := instanceLabels(instance)
		metrics.RunnerDeletionDuration.WithLabelValues(provider).
			Observe(now.Sub(state.pendingDeleteAt).Seconds())
	}

	state.status = instance.Status
	state.runnerStatus = instance.RunnerStatus
}

func (w *Worker) handleJobEvent(op dbCommon.OperationType, job params.Job) {
	w.mux.Lock()
	defer w.mux.Unlock()

	state, known := w.jobs[job.ID]

	if op == dbCommon.DeleteOperation {
		delete(w.jobs, job.ID)
		return
	}

	if !known {
		state = &jobState{}
		w.jobs[job.ID] = state
	}
	prevStatus := state.status
	state.lastSeen = time.Now()

	if job.Status == prevStatus {
		return
	}
	state.status = job.Status

	switch job.Status {
	case string(params.JobStatusInProgress):
		// StartedAt and CreatedAt make the queue time computable no matter
		// when we first saw the job. Without StartedAt, the wall clock only
		// works if we witnessed the job while it was still queued.
		switch {
		case !job.StartedAt.IsZero() && !job.CreatedAt.IsZero():
			observeDuration(metrics.JobQueueDuration.WithLabelValues(job.RepositoryOwner),
				job.StartedAt.Sub(job.CreatedAt))
		case prevStatus == string(params.JobStatusQueued):
			observeDuration(metrics.JobQueueDuration.WithLabelValues(job.RepositoryOwner),
				time.Since(job.CreatedAt))
		}
	case string(params.JobStatusCompleted):
		conclusion := job.Conclusion
		if conclusion == "" {
			conclusion = "unknown"
		}
		metrics.JobsCompletedCount.WithLabelValues(
			job.RepositoryOwner, // label: owner
			job.RepositoryName,  // label: repository
			conclusion,          // label: conclusion
		).Inc()

		if !job.CompletedAt.IsZero() && !job.StartedAt.IsZero() {
			observeDuration(metrics.JobExecutionDuration.WithLabelValues(job.RepositoryOwner),
				job.CompletedAt.Sub(job.StartedAt))
		}
	}
}

// observeDuration guards against clock skew between forge and controller
// timestamps. Negative durations carry no signal and would land in the
// lowest bucket, skewing percentiles.
func observeDuration(observer interface{ Observe(float64) }, duration time.Duration) {
	if duration < 0 {
		return
	}
	observer.Observe(duration.Seconds())
}

// sweepLoop evicts tracking entries for entities that no longer exist, in
// case their delete events were missed (e.g. events dropped on watcher
// backpressure).
func (w *Worker) sweepLoop() {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.quit:
			return
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.sweep()
		}
	}
}

func (w *Worker) sweep() {
	w.mux.Lock()
	defer w.mux.Unlock()

	for name, state := range w.instances {
		if _, ok := cache.GetInstanceCache(name); ok {
			continue
		}
		// Grace period: a freshly created instance may not have reached the
		// cache worker yet.
		if time.Since(state.lastSeen) > sweepInterval {
			delete(w.instances, name)
		}
	}

	for id, state := range w.jobs {
		if time.Since(state.lastSeen) > jobStateTTL {
			delete(w.jobs, id)
		}
	}
}
