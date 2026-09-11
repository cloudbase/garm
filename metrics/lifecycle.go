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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Lifecycle outcomes recorded in RunnerLifecycleCount. These describe the
// reason a runner was scheduled for removal.
const (
	OutcomeJobCompleted     = "job_completed"
	OutcomeIdleScaleDown    = "idle_scaledown"
	OutcomeBootstrapTimeout = "bootstrap_timeout"
	OutcomeProviderError    = "provider_error"
	OutcomeOrphaned         = "orphaned"
	OutcomeManualDelete     = "manual_delete"
	OutcomeStartupRecovery  = "startup_recovery"
)

var (
	// runnerDurationBuckets covers runner-scale timings: cloud instances
	// take seconds to tens of minutes to come up or be removed.
	runnerDurationBuckets = []float64{5, 10, 20, 30, 60, 120, 180, 300, 600, 1200, 1800, 3600}

	// jobQueueBuckets covers the time a job waits for a runner. Fast paths
	// are sub-minute (warm idle runners), slow paths are bounded by runner
	// bootstrap times.
	jobQueueBuckets = []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600, 7200}

	// jobExecutionBuckets covers actual job run times, from quick checks to
	// multi-hour builds.
	jobExecutionBuckets = []float64{30, 60, 120, 300, 600, 1200, 1800, 3600, 7200, 14400, 21600}
)

var (
	// RunnerProvisionDuration measures the time from instance record
	// creation until the provider reports the instance as running.
	RunnerProvisionDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsRunnerSubsystem,
		Name:      "provision_duration_seconds",
		Help:      "Time from runner creation until the provider reports it running",
		Buckets:   runnerDurationBuckets,
	}, []string{"provider", "pool_owner", "pool_type"})

	// RunnerReadyDuration measures the time from instance record creation
	// until the runner first reports as idle or active on the forge. This is
	// the end-to-end time until a runner can actually pick up jobs.
	RunnerReadyDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsRunnerSubsystem,
		Name:      "ready_duration_seconds",
		Help:      "Time from runner creation until it first reports idle or active on the forge",
		Buckets:   runnerDurationBuckets,
	}, []string{"provider", "pool_owner", "pool_type"})

	// RunnerDeletionDuration measures the time from a runner being marked
	// for deletion until it is fully removed. Long tails here point at stuck
	// or failing provider deletions (leaked compute).
	RunnerDeletionDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsRunnerSubsystem,
		Name:      "deletion_duration_seconds",
		Help:      "Time from a runner being marked for deletion until it is removed",
		Buckets:   runnerDurationBuckets,
	}, []string{"provider"})

	// RunnerLifecycleCount counts runner removal decisions by reason. A
	// spike in bootstrap_timeout typically means a broken image, network or
	// userdata; provider_error points at the IaaS.
	RunnerLifecycleCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsRunnerSubsystem,
		Name:      "lifecycle_events_total",
		Help:      "Total number of runner removal decisions, by outcome",
	}, []string{"outcome", "provider", "pool_owner", "pool_type", "pool_id", "scaleset_id"})

	// JobQueueDuration measures the time jobs spend queued before a runner
	// picks them up. This is the primary end-user SLO for a CI platform.
	JobQueueDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsJobsSubsystem,
		Name:      "queue_duration_seconds",
		Help:      "Time a workflow job spent queued before it started running",
		Buckets:   jobQueueBuckets,
	}, []string{"owner"})

	// JobExecutionDuration measures how long jobs run once started.
	JobExecutionDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsJobsSubsystem,
		Name:      "execution_duration_seconds",
		Help:      "Time a workflow job spent running, from started to completed",
		Buckets:   jobExecutionBuckets,
	}, []string{"owner"})

	// JobsCompletedCount counts completed jobs by conclusion. This is the
	// controller's job throughput counter.
	JobsCompletedCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsJobsSubsystem,
		Name:      "completed_total",
		Help:      "Total number of workflow jobs completed, by conclusion",
	}, []string{"owner", "repository", "conclusion"})
)
