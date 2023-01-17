package controllers

import (
	"log"

	"garm/auth"
	"github.com/prometheus/client_golang/prometheus"
)

type GarmCollector struct {
	instanceMetric *prometheus.Desc
	apiController  *APIController
}

func NewGarmCollector(a *APIController) *GarmCollector {
	return &GarmCollector{
		apiController: a,
		instanceMetric: prometheus.NewDesc(
			"garm_runner_status",
			"Status of the runner",
			[]string{"name", "status", "runner_status", "pool", "pool_type", "hostname", "controller_id"}, nil,
		)}
}

func (c *GarmCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.instanceMetric
}

func (c *GarmCollector) Collect(ch chan<- prometheus.Metric) {
	c.CollectInstanceMetric(ch)
}

// CollectInstanceMetric collects the metrics for the runner instances
// reflecting the statuses and the pool they belong to.
func (c *GarmCollector) CollectInstanceMetric(ch chan<- prometheus.Metric) {

	ctx := auth.GetAdminContext()

	instances, err := c.apiController.r.ListAllInstances(ctx)
	if err != nil {
		log.Printf("cannot collect metrics, listing instances: %s", err)
		return
	}

	pools, err := c.apiController.r.ListAllPools(ctx)
	if err != nil {
		log.Printf("listing pools: %s", err)
		// continue anyway
	}

	type poolInfo struct {
		Name string
		Type string
	}

	poolNames := make(map[string]poolInfo)
	for _, pool := range pools {
		if pool.EnterpriseName != "" {
			poolNames[pool.ID] = poolInfo{
				Name: pool.EnterpriseName,
				Type: "enterprise",
			}
		} else if pool.OrgName != "" {
			poolNames[pool.ID] = poolInfo{
				Name: pool.OrgName,
				Type: "organization",
			}
		} else {
			poolNames[pool.ID] = poolInfo{
				Name: pool.RepoName,
				Type: "repository",
			}
		}
	}

	hostname, controllerID := c.apiController.GetControllerInfo()

	for _, instance := range instances {

		m, err := prometheus.NewConstMetric(
			c.instanceMetric,
			prometheus.GaugeValue,
			1,
			instance.Name,
			string(instance.Status),
			string(instance.RunnerStatus),
			poolNames[instance.PoolID].Name,
			poolNames[instance.PoolID].Type,
			hostname,
			controllerID,
		)

		if err != nil {
			log.Printf("cannot collect metrics, creating metric: %s", err)
			continue
		}
		ch <- m
	}
}
