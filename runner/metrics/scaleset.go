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
	"context"

	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/runner"
)

const unknownEntityValue = "unknown"

// CollectScaleSetMetric collects the metrics for scalesets
// reporting their state and enabled status.
func CollectScaleSetMetric(ctx context.Context, r *runner.Runner) error {
	// reset metrics
	metrics.ScaleSetStatus.Reset()

	scalesets, err := r.ListAllScaleSets(ctx)
	if err != nil {
		return err
	}

	for _, ss := range scalesets {
		// Determine entity type and name
		var entityType, entityName string
		switch {
		case ss.OrgName != "":
			entityType = "organization"
			entityName = ss.OrgName
		case ss.RepoName != "":
			entityType = "repository"
			entityName = ss.RepoName
		case ss.EnterpriseName != "":
			entityType = "enterprise"
			entityName = ss.EnterpriseName
		default:
			entityType = unknownEntityValue
			entityName = unknownEntityValue
		}

		// Value is 1 if enabled, 0 if disabled
		var value float64
		if ss.Enabled {
			value = 1
		}

		metrics.ScaleSetStatus.WithLabelValues(
			ss.Name,          // label: name
			string(ss.State), // label: state
			entityType,       // label: entity_type
			entityName,       // label: entity_name
			ss.ProviderName,  // label: provider
		).Set(value)
	}

	return nil
}
