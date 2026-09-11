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

// Error kinds recorded in ProviderOperationErrorsCount.
const (
	ProviderErrKindTimeout    = "timeout"
	ProviderErrKindProvider   = "provider_error"
	ProviderErrKindDecode     = "decode_error"
	ProviderErrKindValidation = "validation_error"
)

// providerOpBuckets covers external provider binary execution times.
var providerOpBuckets = []float64{0.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600}

var (
	// ProviderOperationDuration measures the execution time of successful
	// external provider operations.
	ProviderOperationDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsProviderSubsystem,
		Name:      "operation_duration_seconds",
		Help:      "Execution time of successful provider operations",
		Buckets:   providerOpBuckets,
	}, []string{"provider", "operation", "pool_id", "scaleset_id", "entity_type", "entity_id"})

	// ProviderOperationErrorsCount counts failed provider operations by
	// error kind. Complements garm_runner_errors_total with the error kind
	// dimension.
	ProviderOperationErrorsCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsProviderSubsystem,
		Name:      "operation_errors_total",
		Help:      "Total number of failed provider operations, by error kind",
	}, []string{"provider", "operation", "pool_id", "scaleset_id", "entity_type", "entity_id", "error_kind"})
)
