package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	RepositoryInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsRepositorySubsystem,
		Name:      "info",
		Help:      "Info of the enterprise",
	}, []string{"name", "id"})

	RepositoryPoolManagerStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsRepositorySubsystem,
		Name:      "pool_manager_status",
		Help:      "Status of the enterprise pool manager",
	}, []string{"name", "id", "running"})
)
