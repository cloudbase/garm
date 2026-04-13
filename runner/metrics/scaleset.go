// Copyright 2026 Cloudbase Solutions SRL
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
	"strings"

	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/runner"
)

// CollectScaleSetMetric collects the metrics for the scale set objects
func CollectScaleSetMetric(ctx context.Context, r *runner.Runner) error {
	// reset metrics
	metrics.ScaleSetInfo.Reset()
	metrics.ScaleSetStatus.Reset()
	metrics.ScaleSetMaxRunners.Reset()
	metrics.ScaleSetMinIdleRunners.Reset()
	metrics.ScaleSetDesiredRunnerCount.Reset()
	metrics.ScaleSetBootstrapTimeout.Reset()

	scaleSets, err := r.ListAllScaleSets(ctx)
	if err != nil {
		return err
	}

	for _, scaleSet := range scaleSets {
		var ownerName string
		switch {
		case scaleSet.OrgName != "":
			ownerName = scaleSet.OrgName
		case scaleSet.EnterpriseName != "":
			ownerName = scaleSet.EnterpriseName
		default:
			ownerName = scaleSet.RepoName
		}

		var tagNames []string
		for _, tag := range scaleSet.Tags {
			tagNames = append(tagNames, tag.Name)
		}

		id := strconv.FormatUint(uint64(scaleSet.ID), 10)

		metrics.ScaleSetInfo.WithLabelValues(
			id,                                // label: id
			strconv.Itoa(scaleSet.ScaleSetID), // label: scaleset_id
			scaleSet.Name,                     // label: name
			scaleSet.Image,                    // label: image
			scaleSet.Flavor,                   // label: flavor
			scaleSet.Prefix,                   // label: prefix
			string(scaleSet.OSType),           // label: os_type
			string(scaleSet.OSArch),           // label: os_arch
			strings.Join(tagNames, ","),       // label: tags
			scaleSet.ProviderName,             // label: provider
			scaleSet.GitHubRunnerGroup,        // label: runner_group
			ownerName,                         // label: scaleset_owner
			string(scaleSet.ScaleSetType()),   // label: scaleset_type
		).Set(1)

		metrics.ScaleSetStatus.WithLabelValues(
			id,                                   // label: id
			strconv.FormatBool(scaleSet.Enabled), // label: enabled
			string(scaleSet.State),               // label: state
		).Set(metrics.Bool2float64(scaleSet.Enabled))

		metrics.ScaleSetMaxRunners.WithLabelValues(
			id,
		).Set(float64(scaleSet.MaxRunners))

		metrics.ScaleSetMinIdleRunners.WithLabelValues(
			id,
		).Set(float64(scaleSet.MinIdleRunners))

		metrics.ScaleSetDesiredRunnerCount.WithLabelValues(
			id,
		).Set(float64(scaleSet.DesiredRunnerCount))

		metrics.ScaleSetBootstrapTimeout.WithLabelValues(
			id,
		).Set(float64(scaleSet.RunnerBootstrapTimeout))
	}
	return nil
}
