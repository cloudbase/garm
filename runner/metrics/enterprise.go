package metrics

import (
	"context"
	"strconv"

	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/runner"
)

// CollectOrganizationMetric collects the metrics for the enterprise objects
func CollectEnterpriseMetric(ctx context.Context, r *runner.Runner) error {
	// reset metrics
	metrics.EnterpriseInfo.Reset()
	metrics.EnterprisePoolManagerStatus.Reset()

	enterprises, err := r.ListEnterprises(ctx)
	if err != nil {
		return err
	}

	for _, enterprise := range enterprises {
		metrics.EnterpriseInfo.WithLabelValues(
			enterprise.Name, // label: name
			enterprise.ID,   // label: id
		).Set(1)

		metrics.EnterprisePoolManagerStatus.WithLabelValues(
			enterprise.Name, // label: name
			enterprise.ID,   // label: id
			strconv.FormatBool(enterprise.PoolManagerStatus.IsRunning), // label: running
		).Set(metrics.Bool2float64(enterprise.PoolManagerStatus.IsRunning))
	}
	return nil
}
