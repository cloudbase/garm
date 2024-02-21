package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	InstanceStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsRunnerSubsystem,
		Name:      "status",
		Help:      "Status of the instance",
	}, []string{"name", "status", "runner_status", "pool_owner", "pool_type", "pool_id", "provider"})

	InstanceOperationCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsRunnerSubsystem,
		Name:      "operation_total",
		Help:      "Total number of instance operation attempts",
	}, []string{"operation", "provider"})

	InstanceOperationFailedCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsRunnerSubsystem,
		Name:      "operation_failed_total",
		Help:      "Total number of failed instance operation attempts",
	}, []string{"operation", "provider"})
)
