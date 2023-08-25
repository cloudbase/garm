package e2e

import (
	"fmt"
	"log"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

func waitInstanceStatus(name string, status commonParams.InstanceStatus, runnerStatus params.RunnerStatus, timeout time.Duration) (*params.Instance, error) {
	var timeWaited time.Duration = 0
	var instance *params.Instance

	log.Printf("Waiting for instance %s status to reach status %s and runner status %s", name, status, runnerStatus)
	for timeWaited < timeout {
		instance, err := getInstance(cli, authToken, name)
		if err != nil {
			return nil, err
		}
		log.Printf("Instance %s status %s and runner status %s", name, instance.Status, instance.RunnerStatus)
		if instance.Status == status && instance.RunnerStatus == runnerStatus {
			return instance, nil
		}
		time.Sleep(5 * time.Second)
		timeWaited += 5 * time.Second
	}

	if err := printJsonResponse(*instance); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("timeout waiting for instance %s status to reach status %s and runner status %s", name, status, runnerStatus)
}

func waitInstanceToBeRemoved(name string, timeout time.Duration) error {
	var timeWaited time.Duration = 0
	var instance *params.Instance

	log.Printf("Waiting for instance %s to be removed", name)
	for timeWaited < timeout {
		instances, err := listInstances(cli, authToken)
		if err != nil {
			return err
		}

		instance = nil
		for _, i := range instances {
			if i.Name == name {
				instance = &i
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

	if err := printJsonResponse(*instance); err != nil {
		return err
	}
	return fmt.Errorf("instance %s was not removed within the timeout", name)
}

func waitPoolRunningIdleInstances(poolID string, timeout time.Duration) error {
	var timeWaited time.Duration = 0

	pool, err := getPool(cli, authToken, poolID)
	if err != nil {
		return err
	}

	log.Printf("Waiting for pool %s to have all instances as idle running", poolID)
	for timeWaited < timeout {
		poolInstances, err := listPoolInstances(cli, authToken, poolID)
		if err != nil {
			return err
		}

		runningIdleCount := 0
		for _, instance := range poolInstances {
			if instance.Status == commonParams.InstanceRunning && instance.RunnerStatus == params.RunnerIdle {
				runningIdleCount++
			}
		}

		log.Printf("Pool min idle runners: %d, pool instances: %d, current pool running idle instances: %d", pool.MinIdleRunners, len(poolInstances), runningIdleCount)
		if runningIdleCount == int(pool.MinIdleRunners) && runningIdleCount == len(poolInstances) {
			return nil
		}
		time.Sleep(5 * time.Second)
		timeWaited += 5 * time.Second
	}

	_ = dumpPoolInstancesDetails(pool.ID)

	return fmt.Errorf("timeout waiting for pool %s to have all idle instances running", poolID)
}
