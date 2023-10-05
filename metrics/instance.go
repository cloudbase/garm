package metrics

import (
	"log"

	"github.com/cloudbase/garm/auth"
	"github.com/prometheus/client_golang/prometheus"
)

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
		return
	}

	type poolInfo struct {
		Name         string
		Type         string
		ProviderName string
	}

	poolNames := make(map[string]poolInfo)
	for _, pool := range pools {
		if pool.EnterpriseName != "" {
			poolNames[pool.ID] = poolInfo{
				Name:         pool.EnterpriseName,
				Type:         string(pool.PoolType()),
				ProviderName: pool.ProviderName,
			}
		} else if pool.OrgName != "" {
			poolNames[pool.ID] = poolInfo{
				Name:         pool.OrgName,
				Type:         string(pool.PoolType()),
				ProviderName: pool.ProviderName,
			}
		} else {
			poolNames[pool.ID] = poolInfo{
				Name:         pool.RepoName,
				Type:         string(pool.PoolType()),
				ProviderName: pool.ProviderName,
			}
		}
	}

	for _, instance := range instances {

		m, err := prometheus.NewConstMetric(
			c.instanceMetric,
			prometheus.GaugeValue,
			1,
			instance.Name,                           // label: name
			string(instance.Status),                 // label: status
			string(instance.RunnerStatus),           // label: runner_status
			poolNames[instance.PoolID].Name,         // label: pool_owner
			poolNames[instance.PoolID].Type,         // label: pool_type
			instance.PoolID,                         // label: pool_id
			hostname,                                // label: hostname
			controllerID,                            // label: controller_id
			poolNames[instance.PoolID].ProviderName, // label: provider
		)

		if err != nil {
			log.Printf("cannot collect runner metric: %s", err)
			continue
		}
		ch <- m
	}
}
