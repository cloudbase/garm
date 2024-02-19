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
	}, []string{"name", "status", "runner_status", "pool_owner", "pool_type", "pool_id", "hostname", "controller_id", "provider"})
)
