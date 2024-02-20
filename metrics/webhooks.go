package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	WebhooksReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsWebhookSubsystem,
		Name:      "received",
		Help:      "The total number of webhooks received",
	}, []string{"valid", "reason"})
)
