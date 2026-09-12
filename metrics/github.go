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

import "github.com/prometheus/client_golang/prometheus"

var (
	GithubOperationCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsGithubSubsystem,
		Name:      "operations_total",
		Help:      "Total number of forge (github, gitea) operation attempts",
	}, []string{"operation", "scope", "endpoint"})

	GithubOperationFailedCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsGithubSubsystem,
		Name:      "errors_total",
		Help:      "Total number of failed forge (github, gitea) operation attempts",
	}, []string{"operation", "scope", "endpoint"})

	// GitHub rate limit metrics
	GithubRateLimitLimit = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsGithubSubsystem,
		Name:      "rate_limit_limit",
		Help:      "The maximum number of requests allowed per hour for GitHub API",
	}, []string{"credential_name", "credential_id", "endpoint"})

	GithubRateLimitRemaining = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsGithubSubsystem,
		Name:      "rate_limit_remaining",
		Help:      "The number of requests remaining in the current rate limit window",
	}, []string{"credential_name", "credential_id", "endpoint"})

	GithubRateLimitUsed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsGithubSubsystem,
		Name:      "rate_limit_used",
		Help:      "The number of requests used in the current rate limit window",
	}, []string{"credential_name", "credential_id", "endpoint"})

	GithubRateLimitResetTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsGithubSubsystem,
		Name:      "rate_limit_reset_timestamp",
		Help:      "Unix timestamp when the rate limit resets",
	}, []string{"credential_name", "credential_id", "endpoint"})

	// GithubTokenExpirationTimestamp records when a credential's token
	// expires, as reported by the forge. PATs created without an expiration
	// and forges that do not report one emit no series; app credentials are
	// excluded since their tokens rotate automatically. Alert on this before
	// the expiry turns into a wall of API errors.
	GithubTokenExpirationTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsGithubSubsystem,
		Name:      "token_expiration_timestamp",
		Help:      "Unix timestamp when the credential's token expires, if reported by the forge",
	}, []string{"credential_name", "credential_id", "endpoint"})
)
