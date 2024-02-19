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

func init() {
	// runner metrics
	prometheus.MustRegister(InstanceStatus)
	// organization metrics
	prometheus.MustRegister(OrganizationInfo)
	prometheus.MustRegister(OrganizationPoolManagerStatus)
	// enterprise metrics
	prometheus.MustRegister(EnterpriseInfo)
	prometheus.MustRegister(EnterprisePoolManagerStatus)
	// repository metrics
	prometheus.MustRegister(RepositoryInfo)
	prometheus.MustRegister(RepositoryPoolManagerStatus)
	// provider metrics
	prometheus.MustRegister(ProviderInfo)
	// pool metrics
	prometheus.MustRegister(PoolInfo)
	prometheus.MustRegister(PoolStatus)
	prometheus.MustRegister(PoolMaxRunners)
	prometheus.MustRegister(PoolMinIdleRunners)
	prometheus.MustRegister(PoolBootstrapTimeout)
	// health metrics
	prometheus.MustRegister(GarmHealth)
	// webhook metrics
	prometheus.MustRegister(WebhooksReceived)

}
