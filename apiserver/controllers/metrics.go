package controllers

import (
	"log"

	"garm/auth"
	"garm/runner"

	"github.com/prometheus/client_golang/prometheus"
)

type GarmCollector struct {
	healthMetric   *prometheus.Desc
	instanceMetric *prometheus.Desc
	runner         *runner.Runner
}

func NewGarmCollector(r *runner.Runner) *GarmCollector {
	return &GarmCollector{
		runner: r,
		instanceMetric: prometheus.NewDesc(
			"garm_runner_status",
			"Status of the runner",
			[]string{"name", "status", "runner_status", "pool_owner", "pool_type", "pool_id", "hostname", "controller_id"}, nil,
		),
		healthMetric: prometheus.NewDesc(
			"garm_health",
			"Health of the runner",
			[]string{"hostname", "controller_id"}, nil,
		),
	}
}

func (c *GarmCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.instanceMetric
	ch <- c.healthMetric
}

func (c *GarmCollector) Collect(ch chan<- prometheus.Metric) {
	controllerInfo := c.runner.GetControllerInfo(auth.GetAdminContext())

	c.CollectInstanceMetric(ch, controllerInfo.Hostname, controllerInfo.ControllerID.String())
	c.CollectHealthMetric(ch, controllerInfo.Hostname, controllerInfo.ControllerID.String())
}

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

// CollectInstanceMetric collects the metrics for the runner instances
// reflecting the statuses and the pool they belong to.
func (c *GarmCollector) CollectInstanceMetric(ch chan<- prometheus.Metric, hostname string, controllerID string) {

	ctx := auth.GetAdminContext()

	instances, err := c.runner.ListAllInstances(ctx)
	if err != nil {
		log.Printf("cannot collect metrics, listing instances: %s", err)
		return
	}

	pools, err := c.runner.ListAllPools(ctx)
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
				Type: string(pool.PoolType()),
			}
		} else if pool.OrgName != "" {
			poolNames[pool.ID] = poolInfo{
				Name: pool.OrgName,
				Type: string(pool.PoolType()),
			}
		} else {
			poolNames[pool.ID] = poolInfo{
				Name: pool.RepoName,
				Type: string(pool.PoolType()),
			}
		}
	}

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
			instance.PoolID,
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
