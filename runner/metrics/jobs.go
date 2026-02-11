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
	"fmt"
	"strings"

	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/runner"
)

// CollectJobMetric collects the metrics for the jobs recorded by GARM
func CollectJobMetric(ctx context.Context, r *runner.Runner) error {
	// reset metrics
	metrics.JobStatus.Reset()

	jobs, err := r.ListAllJobs(ctx)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		metrics.JobStatus.WithLabelValues(
			fmt.Sprintf("%d", job.ID),     // label: job_id
			job.Name,                      // label: name
			job.Status,                    // label: status
			job.Conclusion,                // label: conclusion
			job.RunnerName,                // label: runner_name
			job.RepositoryName,            // label: repository
			strings.Join(job.Labels, " "), // label: requested_labels
		).Set(1)
	}
	return nil
}
