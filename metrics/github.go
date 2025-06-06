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

import "github.com/prometheus/client_golang/prometheus"

var (
	GithubOperationCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsGithubSubsystem,
		Name:      "operations_total",
		Help:      "Total number of github operation attempts",
	}, []string{"operation", "scope"})

	GithubOperationFailedCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsGithubSubsystem,
		Name:      "errors_total",
		Help:      "Total number of failed github operation attempts",
	}, []string{"operation", "scope"})
)
