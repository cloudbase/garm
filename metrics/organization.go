package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	OrganizationInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsOrganizationSubsystem,
		Name:      "info",
		Help:      "Info of the organization",
	}, []string{"name", "id"})

	OrganizationPoolManagerStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsOrganizationSubsystem,
		Name:      "pool_manager_status",
		Help:      "Status of the organization pool manager",
	}, []string{"name", "id", "running"})
)
