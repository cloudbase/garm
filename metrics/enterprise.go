package metrics

import (
	"log/slog"
	"strconv"

	"github.com/cloudbase/garm/auth"
	"github.com/prometheus/client_golang/prometheus"
)

// CollectOrganizationMetric collects the metrics for the enterprise objects
func (c *GarmCollector) CollectEnterpriseMetric(ch chan<- prometheus.Metric, hostname string, controllerID string) {
	ctx := auth.GetAdminContext()

	enterprises, err := c.runner.ListEnterprises(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing providers")
		return
	}

	for _, enterprise := range enterprises {

		enterpriseInfo, err := prometheus.NewConstMetric(
			c.enterpriseInfo,
			prometheus.GaugeValue,
			1,
			enterprise.Name, // label: name
			enterprise.ID,   // label: id
		)
		if err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot collect enterpriseInfo metric")
			continue
		}
		ch <- enterpriseInfo

		enterprisePoolManagerStatus, err := prometheus.NewConstMetric(
			c.enterprisePoolManagerStatus,
			prometheus.GaugeValue,
			bool2float64(enterprise.PoolManagerStatus.IsRunning),
			enterprise.Name, // label: name
			enterprise.ID,   // label: id
			strconv.FormatBool(enterprise.PoolManagerStatus.IsRunning), // label: running
		)
		if err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot collect enterprisePoolManagerStatus metric")
			continue
		}
		ch <- enterprisePoolManagerStatus
	}
}
