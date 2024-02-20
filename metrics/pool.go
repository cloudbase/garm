package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	PoolInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsPoolSubsystem,
		Name:      "info",
		Help:      "Info of the pool",
	}, []string{"id", "image", "flavor", "prefix", "os_type", "os_arch", "tags", "provider", "pool_owner", "pool_type"})

	PoolStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsPoolSubsystem,
		Name:      "status",
		Help:      "Status of the pool",
	}, []string{"id", "enabled"})

	PoolMaxRunners = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsPoolSubsystem,
		Name:      "max_runners",
		Help:      "Maximum number of runners in the pool",
	}, []string{"id"})

	PoolMinIdleRunners = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsPoolSubsystem,
		Name:      "min_idle_runners",
		Help:      "Minimum number of idle runners in the pool",
	}, []string{"id"})

	PoolBootstrapTimeout = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsPoolSubsystem,
		Name:      "bootstrap_timeout",
		Help:      "Runner bootstrap timeout in the pool",
	}, []string{"id"})
)
