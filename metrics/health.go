package metrics

import (
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
)

func (c *GarmCollector) CollectHealthMetric(ch chan<- prometheus.Metric, hostname string, controllerID string) {
	m, err := prometheus.NewConstMetric(
		c.healthMetric,
		prometheus.GaugeValue,
		1,
		hostname,
		controllerID,
	)
	if err != nil {
		slog.With(slog.Any("error", err)).Error("error on creating health metric")
		return
	}
	ch <- m
}
