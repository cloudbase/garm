package metrics

import (
	"context"
	"strconv"

	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/runner"
)

// CollectOrganizationMetric collects the metrics for the organization objects
func CollectOrganizationMetric(ctx context.Context, r *runner.Runner) error {
	// reset metrics
	metrics.OrganizationInfo.Reset()
	metrics.OrganizationPoolManagerStatus.Reset()

	organizations, err := r.ListOrganizations(ctx)
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
