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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// ScaleSetStatus reports the status of each scaleset.
	// The value is 1 if the scaleset is enabled, 0 if disabled.
	ScaleSetStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsScaleSetSubsystem,
		Name:      "status",
		Help:      "Status of each scaleset (1=enabled, 0=disabled)",
	}, []string{"name", "state", "entity_type", "entity_name", "provider"})

	// ScaleSetRunnerCount counts runner instances per scaleset, broken down by
	// instance status and runner status. Use this to track per-scaleset capacity
	// and utilization.
	ScaleSetRunnerCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsScaleSetSubsystem,
		Name:      "runner_count",
		Help:      "Count of runner instances per scaleset by status",
	}, []string{"scaleset_name", "status", "runner_status", "provider"})

	// ScaleSetJobCount counts jobs per scaleset, broken down by job status.
	// Use this to monitor job queue depth per scaleset and decide when to
	// increase max_runners.
	ScaleSetJobCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsScaleSetSubsystem,
		Name:      "job_count",
		Help:      "Count of jobs per scaleset by status",
	}, []string{"scaleset_name", "status"})
)
