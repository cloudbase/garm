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

var JobStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: metricsNamespace,
	Subsystem: metricsRunnerSubsystem,
	Name:      "jobs_status",
	Help:      "List of jobs and their status",
}, []string{"job_id", "name", "status", "conclusion", "runner_name", "repository", "requested_labels"})
