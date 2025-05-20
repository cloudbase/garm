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
	"strings"

	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/runner"
)

// CollectPoolMetric collects the metrics for the pool objects
func CollectPoolMetric(ctx context.Context, r *runner.Runner) error {
	// reset metrics
	metrics.PoolInfo.Reset()
	metrics.PoolStatus.Reset()
	metrics.PoolMaxRunners.Reset()
	metrics.PoolMinIdleRunners.Reset()
	metrics.PoolBootstrapTimeout.Reset()

	pools, err := r.ListAllPools(ctx)
	if err != nil {
		return err
	}

	type poolInfo struct {
		Name string
		Type string
	}

	poolNames := make(map[string]poolInfo)
	for _, pool := range pools {
		switch {
		case pool.OrgName != "":
			poolNames[pool.ID] = poolInfo{
				Name: pool.OrgName,
				Type: string(pool.PoolType()),
			}
		case pool.EnterpriseName != "":
			poolNames[pool.ID] = poolInfo{
				Name: pool.EnterpriseName,
				Type: string(pool.PoolType()),
			}
		default:
			poolNames[pool.ID] = poolInfo{
				Name: pool.RepoName,
				Type: string(pool.PoolType()),
			}
		}

		var poolTags []string
		for _, tag := range pool.Tags {
			poolTags = append(poolTags, tag.Name)
		}

		metrics.PoolInfo.WithLabelValues(
			pool.ID,                     // label: id
			pool.Image,                  // label: image
			pool.Flavor,                 // label: flavor
			pool.Prefix,                 // label: prefix
			string(pool.OSType),         // label: os_type
			string(pool.OSArch),         // label: os_arch
			strings.Join(poolTags, ","), // label: tags
			pool.ProviderName,           // label: provider
			poolNames[pool.ID].Name,     // label: pool_owner
			poolNames[pool.ID].Type,     // label: pool_type
		).Set(1)

		metrics.PoolStatus.WithLabelValues(
			pool.ID,                          // label: id
			strconv.FormatBool(pool.Enabled), // label: enabled
		).Set(metrics.Bool2float64(pool.Enabled))

		metrics.PoolMaxRunners.WithLabelValues(
			pool.ID, // label: id
		).Set(float64(pool.MaxRunners))

		metrics.PoolMinIdleRunners.WithLabelValues(
			pool.ID, // label: id
		).Set(float64(pool.MinIdleRunners))

		metrics.PoolBootstrapTimeout.WithLabelValues(
			pool.ID, // label: id
		).Set(float64(pool.RunnerBootstrapTimeout))
	}
	return nil
}
