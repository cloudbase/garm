package metrics

import (
	"log"
	"strconv"

	"github.com/cloudbase/garm/auth"
	"github.com/prometheus/client_golang/prometheus"
)

// CollectOrganizationMetric collects the metrics for the organization objects
func (c *GarmCollector) CollectOrganizationMetric(ch chan<- prometheus.Metric, hostname string, controllerID string) {
	ctx := auth.GetAdminContext()

	organizations, err := c.runner.ListOrganizations(ctx)
	if err != nil {
		log.Printf("listing providers: %s", err)
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
			log.Printf("cannot collect organizationInfo metric: %s", err)
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
			log.Printf("cannot collect organizationPoolManagerStatus metric: %s", err)
			continue
		}
		ch <- organizationPoolManagerStatus
	}
}
