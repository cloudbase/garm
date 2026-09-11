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
	// ScaleSetMessagesCount counts scale set job messages received from the
	// forge, by message type (JobAvailable, JobAssigned, JobStarted,
	// JobCompleted). This is the demand signal feed for scale sets.
	ScaleSetMessagesCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsScaleSetSubsystem,
		Name:      "messages_total",
		Help:      "Total number of scale set job messages received, by message type",
	}, []string{"id", "message_type"})

	// ScaleSetListenerLastSuccess records the unix timestamp of the last
	// successful long-poll cycle for a scale set listener, including empty
	// polls. A stale timestamp means the scale set has silently stopped
	// receiving scaling signals.
	ScaleSetListenerLastSuccess = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsScaleSetSubsystem,
		Name:      "listener_last_success_timestamp",
		Help:      "Unix timestamp of the last successful scale set listener poll cycle",
	}, []string{"id"})

	// ScaleSetListenerRestartsCount counts scale set listener restarts.
	// Frequent restarts point at credential or network trouble with the
	// message session.
	ScaleSetListenerRestartsCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsScaleSetSubsystem,
		Name:      "listener_restarts_total",
		Help:      "Total number of scale set listener restarts",
	}, []string{"id"})
)
