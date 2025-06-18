// Copyright 2025 Cloudbase Solutions SRL
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

import (
	"context"
	"strconv"

	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner"
)

func CollectRepositoryMetric(ctx context.Context, r *runner.Runner) error {
	// reset metrics
	metrics.EnterpriseInfo.Reset()
	metrics.EnterprisePoolManagerStatus.Reset()

	repositories, err := r.ListRepositories(ctx, params.RepositoryFilter{})
	if err != nil {
		return err
	}

	for _, repository := range repositories {
		metrics.EnterpriseInfo.WithLabelValues(
			repository.Name, // label: name
			repository.ID,   // label: id
		).Set(1)

		metrics.EnterprisePoolManagerStatus.WithLabelValues(
			repository.Name, // label: name
			repository.ID,   // label: id
			strconv.FormatBool(repository.PoolManagerStatus.IsRunning), // label: running
		).Set(metrics.Bool2float64(repository.PoolManagerStatus.IsRunning))
	}
	return nil
}
