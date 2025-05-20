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
	PoolInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsPoolSubsystem,
		Name:      "info",
		Help:      "Info of the pool",
	}, []string{"id", "image", "flavor", "prefix", "os_type", "os_arch", "tags", "provider", "pool_owner", "pool_type"})

	PoolStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsPoolSubsystem,
		Name:      "status",
		Help:      "Status of the pool",
	}, []string{"id", "enabled"})

	PoolMaxRunners = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsPoolSubsystem,
		Name:      "max_runners",
		Help:      "Maximum number of runners in the pool",
	}, []string{"id"})

	PoolMinIdleRunners = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsPoolSubsystem,
		Name:      "min_idle_runners",
		Help:      "Minimum number of idle runners in the pool",
	}, []string{"id"})

	PoolBootstrapTimeout = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsPoolSubsystem,
		Name:      "bootstrap_timeout",
		Help:      "Runner bootstrap timeout in the pool",
	}, []string{"id"})
)
