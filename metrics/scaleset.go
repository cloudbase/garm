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
	ScaleSetInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsScaleSetSubsystem,
		Name:      "info",
		Help:      "Info of the scale set",
	}, []string{"id", "scaleset_id", "name", "image", "flavor", "prefix", "os_type", "os_arch", "tags", "provider", "runner_group", "scaleset_owner", "scaleset_type"})

	ScaleSetStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsScaleSetSubsystem,
		Name:      "status",
		Help:      "Status of the scale set",
	}, []string{"id", "enabled", "state"})

	ScaleSetMaxRunners = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsScaleSetSubsystem,
		Name:      "max_runners",
		Help:      "Maximum number of runners in the scale set",
	}, []string{"id"})

	ScaleSetMinIdleRunners = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsScaleSetSubsystem,
		Name:      "min_idle_runners",
		Help:      "Minimum number of idle runners in the scale set",
	}, []string{"id"})

	ScaleSetDesiredRunnerCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsScaleSetSubsystem,
		Name:      "desired_runner_count",
		Help:      "Desired runner count requested by GitHub for the scale set",
	}, []string{"id"})

	ScaleSetBootstrapTimeout = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsScaleSetSubsystem,
		Name:      "bootstrap_timeout",
		Help:      "Runner bootstrap timeout in the scale set",
	}, []string{"id"})
)
