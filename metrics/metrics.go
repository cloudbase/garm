package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const metricsNamespace = "garm"
const metricsRunnerSubsystem = "runner"
const metricsPoolSubsystem = "pool"
const metricsProviderSubsystem = "provider"
const metricsOrganizationSubsystem = "organization"
const metricsRepositorySubsystem = "repository"
const metricsEnterpriseSubsystem = "enterprise"
const metricsWebhookSubsystem = "webhook"

// RegisterMetrics registers all the metrics
func RegisterMetrics() error {

	var collectors []prometheus.Collector
	collectors = append(collectors,
		// runner metrics
		InstanceStatus,
		// organization metrics
		OrganizationInfo,
		OrganizationPoolManagerStatus,
		// enterprise metrics
		EnterpriseInfo,
		EnterprisePoolManagerStatus,
		// repository metrics
		RepositoryInfo,
		RepositoryPoolManagerStatus,
		// provider metrics
		ProviderInfo,
		// pool metrics
		PoolInfo,
		PoolStatus,
		PoolMaxRunners,
		PoolMinIdleRunners,
		PoolBootstrapTimeout,
		// health metrics
		GarmHealth,
		// webhook metrics
		WebhooksReceived,
	)

	for _, c := range collectors {
		if err := prometheus.Register(c); err != nil {
			return err
		}
	}

	return nil
}
