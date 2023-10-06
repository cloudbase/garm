package metrics

import (
	"log"

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
		log.Printf("error on creating health metric: %s", err)
		return
	}
	ch <- m
}
