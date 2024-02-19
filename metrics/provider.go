package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	ProviderInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsProviderSubsystem,
		Name:      "info",
		Help:      "Info of the organization",
	}, []string{"name", "type", "description"})
)
