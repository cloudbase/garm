//go:build integration
// +build integration

package integration

import (
	"fmt"
	"time"

	"github.com/go-openapi/runtime"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/client"
	clientInstances "github.com/cloudbase/garm/client/instances"
	"github.com/cloudbase/garm/params"
)

func (suite *GarmSuite) TestExternalProvider() {
	t := suite.T()
	t.Log("Testing external provider")
	repoPoolParams2 := params.CreatePoolParams{
		MaxRunners:     2,
		MinIdleRunners: 0,
		Flavor:         "default",
		Image:          "ubuntu:24.04",
		OSType:         commonParams.Linux,
		OSArch:         commonParams.Amd64,
		ProviderName:   "test_external",
		Tags:           []string{"repo-runner-2"},
		Enabled:        true,
	}
	repoPool2 := suite.CreateRepoPool(suite.repo.ID, repoPoolParams2)
	newParams := suite.UpdateRepoPool(suite.repo.ID, repoPool2.ID, repoPoolParams2.MaxRunners, 1)
	t.Logf("Updated repo pool with pool_id %s with new_params %+v", repoPool2.ID, newParams)

	err := suite.WaitPoolInstances(repoPool2.ID, commonParams.InstanceRunning, params.RunnerPending, 1*time.Minute)
	suite.NoError(err, "error waiting for pool instances to be running")
	repoPool2 = suite.GetRepoPool(suite.repo.ID, repoPool2.ID)
	suite.DisableRepoPool(suite.repo.ID, repoPool2.ID)
	suite.DeleteInstance(repoPool2.Instances[0].Name, false, false)
	err = suite.WaitPoolInstances(repoPool2.ID, commonParams.InstancePendingDelete, params.RunnerPending, 1*time.Minute)
	suite.NoError(err, "error waiting for pool instances to be pending delete")
	suite.DeleteInstance(repoPool2.Instances[0].Name, true, false) // delete instance with forceRemove
	err = suite.WaitInstanceToBeRemoved(repoPool2.Instances[0].Name, 1*time.Minute)
	suite.NoError(err, "error waiting for instance to be removed")
	suite.DeleteRepoPool(suite.repo.ID, repoPool2.ID)
}

func (suite *GarmSuite) WaitPoolInstances(poolID string, status commonParams.InstanceStatus, runnerStatus params.RunnerStatus, timeout time.Duration) error {
	t := suite.T()
	var timeWaited time.Duration // default is 0

	pool, err := getPool(suite.cli, suite.authToken, poolID)
	if err != nil {
		return err
	}

	t.Logf("Waiting for pool instances with pool_id %s to reach desired status %v and desired_runner_status %v", poolID, status, runnerStatus)
	for timeWaited < timeout {
		poolInstances, err := listPoolInstances(suite.cli, suite.authToken, poolID)
		if err != nil {
			return err
		}

		instancesCount := 0
		for _, instance := range poolInstances {
			if instance.Status == status && instance.RunnerStatus == runnerStatus {
				instancesCount++
			}
		}

		t.Logf(
			"Pool instance with pool_id %s reached status %v and runner_status %v, desired_instance_count %d, pool_instance_count %d",
			poolID, status, runnerStatus, instancesCount,
			len(poolInstances))
		if pool.MinIdleRunnersAsInt() == instancesCount {
			return nil
		}
		time.Sleep(5 * time.Second)
		timeWaited += 5 * time.Second
	}

	err = suite.dumpPoolInstancesDetails(pool.ID)
	suite.NoError(err, "error dumping pool instances details")

	return fmt.Errorf("timeout waiting for pool %s instances to reach status: %s and runner status: %s", poolID, status, runnerStatus)
}

func (suite *GarmSuite) dumpPoolInstancesDetails(poolID string) error {
	t := suite.T()
	pool, err := getPool(suite.cli, suite.authToken, poolID)
	if err != nil {
		return err
	}
	if err := printJSONResponse(pool); err != nil {
		return err
	}
	for _, instance := range pool.Instances {
		instanceDetails, err := getInstance(suite.cli, suite.authToken, instance.Name)
		if err != nil {
			return err
		}
		t.Logf("Instance details: instance_name %s", instance.Name)
		if err := printJSONResponse(instanceDetails); err != nil {
			return err
		}
	}
	return nil
}

func (suite *GarmSuite) DisableRepoPool(repoID, repoPoolID string) {
	t := suite.T()
	t.Logf("Disable repo pool with repo_id %s and pool_id %s", repoID, repoPoolID)
	enabled := false
	poolParams := params.UpdatePoolParams{Enabled: &enabled}
	_, err := updateRepoPool(suite.cli, suite.authToken, repoID, repoPoolID, poolParams)
	suite.NoError(err, "error disabling repository pool")
}

func (suite *GarmSuite) DeleteInstance(name string, forceRemove, bypassGHUnauthorized bool) {
	t := suite.T()
	t.Logf("Delete instance %s with force_remove %t", name, forceRemove)
	err := deleteInstance(suite.cli, suite.authToken, name, forceRemove, bypassGHUnauthorized)
	suite.NoError(err, "error deleting instance", name)
	t.Logf("Instance deletion initiated for instance %s", name)
}

func (suite *GarmSuite) WaitInstanceToBeRemoved(name string, timeout time.Duration) error {
	t := suite.T()
	var timeWaited time.Duration // default is 0
	var instance *params.Instance

	t.Logf("Waiting for instance %s to be removed", name)
	for timeWaited < timeout {
		instances, err := listInstances(suite.cli, suite.authToken)
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

func listPoolInstances(apiCli *client.GarmAPI, apiAuthToken runtime.ClientAuthInfoWriter, poolID string) (params.Instances, error) {
	listPoolInstancesResponse, err := apiCli.Instances.ListPoolInstances(
		clientInstances.NewListPoolInstancesParams().WithPoolID(poolID),
		apiAuthToken)
	if err != nil {
		return nil, err
	}
	return listPoolInstancesResponse.Payload, nil
}
