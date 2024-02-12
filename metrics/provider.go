package metrics

import (
	"log/slog"

	"github.com/cloudbase/garm/auth"
	"github.com/prometheus/client_golang/prometheus"
)

// CollectPoolMetric collects the metrics for the pool objects
func (c *GarmCollector) CollectProviderMetric(ch chan<- prometheus.Metric, hostname string, controllerID string) {
	ctx := auth.GetAdminContext()

	providers, err := c.runner.ListProviders(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing providers")
		return
	}

	for _, provider := range providers {

		providerInfo, err := prometheus.NewConstMetric(
			c.providerInfo,
			prometheus.GaugeValue,
			1,
			provider.Name,                 // label: name
			string(provider.ProviderType), // label: type
			provider.Description,          // label: description
		)
		if err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot collect providerInfo metric")
			continue
		}
		ch <- providerInfo
	}
}
