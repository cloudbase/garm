package metrics

import (
	"context"

	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner"
)

// CollectInstanceMetric collects the metrics for the runner instances
// reflecting the statuses and the pool they belong to.
func CollectInstanceMetric(ctx context.Context, r *runner.Runner, controllerInfo params.ControllerInfo) error {

	// reset metrics
	metrics.InstanceStatus.Reset()

	instances, err := r.ListAllInstances(ctx)
	if err != nil {
		return err
	}

	pools, err := r.ListAllPools(ctx)
	if err != nil {
		return err
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

		metrics.InstanceStatus.WithLabelValues(
			instance.Name,                           // label: name
			string(instance.Status),                 // label: status
			string(instance.RunnerStatus),           // label: runner_status
			poolNames[instance.PoolID].Name,         // label: pool_owner
			poolNames[instance.PoolID].Type,         // label: pool_type
			instance.PoolID,                         // label: pool_id
			controllerInfo.Hostname,                 // label: hostname
			controllerInfo.ControllerID.String(),    // label: controller_id
			poolNames[instance.PoolID].ProviderName, // label: provider

		).Set(1)
	}
	return nil
}
