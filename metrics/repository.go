package metrics

import (
	"log"
	"strconv"

	"github.com/cloudbase/garm/auth"
	"github.com/prometheus/client_golang/prometheus"
)

// CollectOrganizationMetric collects the metrics for the repository objects
func (c *GarmCollector) CollectRepositoryMetric(ch chan<- prometheus.Metric, hostname string, controllerID string) {
	ctx := auth.GetAdminContext()

	repositories, err := c.runner.ListRepositories(ctx)
	if err != nil {
		log.Printf("listing providers: %s", err)
		return
	}

	for _, repository := range repositories {

		repositoryInfo, err := prometheus.NewConstMetric(
			c.repositoryInfo,
			prometheus.GaugeValue,
			1,
			repository.Name,  // label: name
			repository.Owner, // label: owner
			repository.ID,    // label: id
		)
		if err != nil {
			log.Printf("cannot collect repositoryInfo metric: %s", err)
			continue
		}
		ch <- repositoryInfo

		repositoryPoolManagerStatus, err := prometheus.NewConstMetric(
			c.repositoryPoolManagerStatus,
			prometheus.GaugeValue,
			bool2float64(repository.PoolManagerStatus.IsRunning),
			repository.Name, // label: name
			repository.ID,   // label: id
			strconv.FormatBool(repository.PoolManagerStatus.IsRunning), // label: running
		)
		if err != nil {
			log.Printf("cannot collect repositoryPoolManagerStatus metric: %s", err)
			continue
		}
		ch <- repositoryPoolManagerStatus
	}
}
