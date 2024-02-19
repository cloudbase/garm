package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	EnterpriseInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsEnterpriseSubsystem,
		Name:      "info",
		Help:      "Info of the enterprise",
	}, []string{"name", "id"})

	EnterprisePoolManagerStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsEnterpriseSubsystem,
		Name:      "pool_manager_status",
		Help:      "Status of the enterprise pool manager",
	}, []string{"name", "id", "running"})
)
