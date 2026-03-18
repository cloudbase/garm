// Copyright 2026 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package metrics

// MetricsSnapshot is the payload sent to dashboard clients every 5 seconds.
// It contains pre-aggregated data from the in-memory cache, allowing the
// frontend to derive all dashboard stats without making individual API calls.
type MetricsSnapshot struct {
	Entities  []MetricsEntity   `json:"entities"`
	Pools     []MetricsPool     `json:"pools"`
	ScaleSets []MetricsScaleSet `json:"scale_sets"`
}

// MetricsEntity represents a repository, organization, or enterprise
// with enough info for the dashboard entity list.
type MetricsEntity struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Endpoint      string `json:"endpoint"`
	PoolCount     int    `json:"pool_count"`
	ScaleSetCount int    `json:"scale_set_count"`
	Healthy       bool   `json:"healthy"`
}

// MetricsPool represents a pool with runner counts grouped by instance status.
type MetricsPool struct {
	ID                 string         `json:"id"`
	ProviderName       string         `json:"provider_name"`
	OSType             string         `json:"os_type"`
	MaxRunners         uint           `json:"max_runners"`
	Enabled            bool           `json:"enabled"`
	RepoName           string         `json:"repo_name,omitempty"`
	OrgName            string         `json:"org_name,omitempty"`
	EnterpriseName     string         `json:"enterprise_name,omitempty"`
	RunnerCounts       map[string]int `json:"runner_counts"`
	RunnerStatusCounts map[string]int `json:"runner_status_counts"`
}

// MetricsScaleSet represents a scale set with runner counts grouped by instance status.
type MetricsScaleSet struct {
	ID                 uint           `json:"id"`
	Name               string         `json:"name"`
	ProviderName       string         `json:"provider_name"`
	OSType             string         `json:"os_type"`
	MaxRunners         uint           `json:"max_runners"`
	Enabled            bool           `json:"enabled"`
	RepoName           string         `json:"repo_name,omitempty"`
	OrgName            string         `json:"org_name,omitempty"`
	EnterpriseName     string         `json:"enterprise_name,omitempty"`
	RunnerCounts       map[string]int `json:"runner_counts"`
	RunnerStatusCounts map[string]int `json:"runner_status_counts"`
}
