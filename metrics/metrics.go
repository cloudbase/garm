// Copyright 2025 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace             = "garm"
	metricsRunnerSubsystem       = "runner"
	metricsPoolSubsystem         = "pool"
	metricsProviderSubsystem     = "provider"
	metricsOrganizationSubsystem = "organization"
	metricsRepositorySubsystem   = "repository"
	metricsEnterpriseSubsystem   = "enterprise"
	metricsWebhookSubsystem      = "webhook"
	metricsGithubSubsystem       = "github"
)

// RegisterMetrics registers all the metrics
func RegisterMetrics() error {
	var collectors []prometheus.Collector
	collectors = append(collectors,

		// metrics created during the periodically update of the metrics
		//
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

		// metrics used within normal garm operations
		// e.g. count instance creations, count github api calls, ...
		//
		// runner instances
		InstanceOperationCount,
		InstanceOperationFailedCount,
		// github
		GithubOperationCount,
		GithubOperationFailedCount,
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
