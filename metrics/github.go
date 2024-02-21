package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	GithubOperationCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsGithubSubsystem,
		Name:      "operation_total",
		Help:      "Total number of github operation attempts",
	}, []string{"operation", "scope"})

	GithubOperationFailedCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsGithubSubsystem,
		Name:      "operation_failed_total",
		Help:      "Total number of failed github operation attempts",
	}, []string{"operation", "scope"})
)
