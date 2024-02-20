package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	GarmHealth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "health",
		Help:      "Health of the garm",
	}, []string{"metadata_url", "callback_url", "webhook_url", "controller_webhook_url", "controller_id"})
)
