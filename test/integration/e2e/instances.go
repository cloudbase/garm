package e2e

import (
	"fmt"
	"log/slog"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

func waitInstanceStatus(name string, status commonParams.InstanceStatus, runnerStatus params.RunnerStatus, timeout time.Duration) (*params.Instance, error) {
	var timeWaited time.Duration // default is 0
	var instance *params.Instance

	slog.Info("Waiting for instance to reach desired status", "instance", name, "desired_status", status, "desired_runner_status", runnerStatus)
	for timeWaited < timeout {
		instance, err := getInstance(cli, authToken, name)
		if err != nil {
			return nil, err
		}
		slog.Info("Instance status", "instance_name", name, "status", instance.Status, "runner_status", instance.RunnerStatus)
		if instance.Status == status && instance.RunnerStatus == runnerStatus {
			return instance, nil
		}
		time.Sleep(5 * time.Second)
		timeWaited += 5 * time.Second
	}

	if err := printJSONResponse(*instance); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("timeout waiting for instance %s status to reach status %s and runner status %s", name, status, runnerStatus)
}

func DeleteInstance(name string, forceRemove bool) {
	if err := deleteInstance(cli, authToken, name, forceRemove); err != nil {
		panic(err)
	}
	slog.Info("Instance deletion initiated", "instance_name", name)
}

func WaitInstanceToBeRemoved(name string, timeout time.Duration) error {
	var timeWaited time.Duration // default is 0
	var instance *params.Instance

	slog.Info("Waiting for instance to be removed", "instance_name", name)
	for timeWaited < timeout {
		instances, err := listInstances(cli, authToken)
		if err != nil {
			return err
		}

		instance = nil
		for k, v := range instances {
			if v.Name == name {
				instance = &instances[k]
				break
			}
		}
		if instance == nil {
			// The instance is not found in the list. We can safely assume
			// that it is removed
			return nil
		}

		time.Sleep(5 * time.Second)
		timeWaited += 5 * time.Second
	}

	if err := printJSONResponse(*instance); err != nil {
		return err
	}
	return fmt.Errorf("instance %s was not removed within the timeout", name)
}

func WaitPoolInstances(poolID string, status commonParams.InstanceStatus, runnerStatus params.RunnerStatus, timeout time.Duration) error {
	var timeWaited time.Duration // default is 0

	pool, err := getPool(cli, authToken, poolID)
	if err != nil {
		return err
	}

	slog.Info("Waiting for pool instances to reach desired status", "pool_id", poolID, "desired_status", status, "desired_runner_status", runnerStatus)
	for timeWaited < timeout {
		poolInstances, err := listPoolInstances(cli, authToken, poolID)
		if err != nil {
			return err
		}

		instancesCount := 0
		for _, instance := range poolInstances {
			if instance.Status == status && instance.RunnerStatus == runnerStatus {
				instancesCount++
			}
		}

		slog.Info(
			"Pool instance reached status",
			"pool_id", poolID,
			"status", status,
			"runner_status", runnerStatus,
			"desired_instance_count", instancesCount,
			"pool_instance_count", len(poolInstances))
		if instancesCount == int(pool.MinIdleRunners) && instancesCount == len(poolInstances) {
			return nil
		}
		time.Sleep(5 * time.Second)
		timeWaited += 5 * time.Second
	}

	_ = dumpPoolInstancesDetails(pool.ID)

	return fmt.Errorf("timeout waiting for pool %s instances to reach status: %s and runner status: %s", poolID, status, runnerStatus)
}
