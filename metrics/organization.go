package metrics

import (
	"log/slog"
	"strconv"

	"github.com/cloudbase/garm/auth"
	"github.com/prometheus/client_golang/prometheus"
)

// CollectOrganizationMetric collects the metrics for the organization objects
func (c *GarmCollector) CollectOrganizationMetric(ch chan<- prometheus.Metric, hostname string, controllerID string) {
	ctx := auth.GetAdminContext()

	organizations, err := c.runner.ListOrganizations(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing providers")
		return
	}

	for _, organization := range organizations {

		organizationInfo, err := prometheus.NewConstMetric(
			c.organizationInfo,
			prometheus.GaugeValue,
			1,
			organization.Name, // label: name
			organization.ID,   // label: id
		)
		if err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot collect organizationInfo metric")
			continue
		}
		ch <- organizationInfo

		organizationPoolManagerStatus, err := prometheus.NewConstMetric(
			c.organizationPoolManagerStatus,
			prometheus.GaugeValue,
			bool2float64(organization.PoolManagerStatus.IsRunning),
			organization.Name,                                            // label: name
			organization.ID,                                              // label: id
			strconv.FormatBool(organization.PoolManagerStatus.IsRunning), // label: running
		)
		if err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot collect organizationPoolManagerStatus metric")
			continue
		}
		ch <- organizationPoolManagerStatus
	}
}
