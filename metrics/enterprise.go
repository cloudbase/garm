package metrics

import (
	"log"
	"strconv"

	"github.com/cloudbase/garm/auth"
	"github.com/prometheus/client_golang/prometheus"
)

// CollectOrganizationMetric collects the metrics for the enterprise objects
func (c *GarmCollector) CollectEnterpriseMetric(ch chan<- prometheus.Metric, hostname string, controllerID string) {
	ctx := auth.GetAdminContext()

	enterprises, err := c.runner.ListEnterprises(ctx)
	if err != nil {
		log.Printf("listing providers: %s", err)
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
			log.Printf("cannot collect enterpriseInfo metric: %s", err)
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
			log.Printf("cannot collect enterprisePoolManagerStatus metric: %s", err)
			continue
		}
		ch <- enterprisePoolManagerStatus
	}
}
