package metrics

import (
	"context"
	"strconv"

	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/runner"
)

func CollectRepositoryMetric(ctx context.Context, r *runner.Runner) error {

	// reset metrics
	metrics.EnterpriseInfo.Reset()
	metrics.EnterprisePoolManagerStatus.Reset()

	repositories, err := r.ListRepositories(ctx)
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
