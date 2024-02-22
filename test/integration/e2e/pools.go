package e2e

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/cloudbase/garm/params"
)

func waitPoolNoInstances(id string, timeout time.Duration) error {
	var timeWaited time.Duration = 0
	var pool *params.Pool
	var err error

	slog.Info("Wait until pool has no instances", "pool_id", id)
	for timeWaited < timeout {
		pool, err = getPool(cli, authToken, id)
		if err != nil {
			return err
		}
		slog.Info("Current pool instances", "instance_count", len(pool.Instances))
		if len(pool.Instances) == 0 {
			return nil
		}
		time.Sleep(5 * time.Second)
		timeWaited += 5 * time.Second
	}

	_ = dumpPoolInstancesDetails(pool.ID)

	return fmt.Errorf("failed to wait for pool %s to have no instances", pool.ID)
}

func dumpPoolInstancesDetails(poolID string) error {
	pool, err := getPool(cli, authToken, poolID)
	if err != nil {
		return err
	}
	if err := printJSONResponse(pool); err != nil {
		return err
	}
	for _, instance := range pool.Instances {
		instanceDetails, err := getInstance(cli, authToken, instance.Name)
		if err != nil {
			return err
		}
		slog.Info("Instance details", "instance_name", instance.Name)
		if err := printJSONResponse(instanceDetails); err != nil {
			return err
		}
	}
	return nil
}
