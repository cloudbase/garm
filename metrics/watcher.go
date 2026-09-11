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

var (
	// WatcherEventsCount counts database change events published to the
	// watcher. This is the change-feed heartbeat of the controller.
	WatcherEventsCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsWatcherSubsystem,
		Name:      "events_total",
		Help:      "Total number of database change events published to the watcher",
	}, []string{"entity_type", "operation"})

	// WatcherNotifyTimeoutsCount counts database change notifications that
	// were dropped because the watcher could not accept them within the
	// notify timeout. The database write itself succeeded, but downstream
	// consumers (caches, workers) did not see the event. Any nonzero value
	// means internal state may be stale.
	WatcherNotifyTimeoutsCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsWatcherSubsystem,
		Name:      "notify_timeouts_total",
		Help:      "Total number of database change notifications dropped on watcher backpressure",
	})
)
