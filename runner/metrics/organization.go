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
	"strconv"

	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner"
)

// CollectOrganizationMetric collects the metrics for the organization objects
func CollectOrganizationMetric(ctx context.Context, r *runner.Runner) error {
	// reset metrics
	metrics.OrganizationInfo.Reset()
	metrics.OrganizationPoolManagerStatus.Reset()

	organizations, err := r.ListOrganizations(ctx, params.OrganizationFilter{})
	if err != nil {
		return err
	}

	for _, organization := range organizations {
		metrics.OrganizationInfo.WithLabelValues(
			organization.Name, // label: name
			organization.ID,   // label: id
		).Set(1)

		metrics.OrganizationPoolManagerStatus.WithLabelValues(
			organization.Name, // label: name
			organization.ID,   // label: id
			strconv.FormatBool(organization.PoolManagerStatus.IsRunning), // label: running
		).Set(metrics.Bool2float64(organization.PoolManagerStatus.IsRunning))
	}
	return nil
}
