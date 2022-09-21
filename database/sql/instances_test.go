// Copyright 2022 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package sql

import (
	"context"
	"fmt"
	dbCommon "garm/database/common"
	garmTesting "garm/internal/testing"
	"garm/params"
	"garm/runner/providers/common"
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"
)

type InstancesTestFixtures struct {
	Org       params.Organization
	Pool      params.Pool
	Instances []params.Instance
}

type InstancesTestSuite struct {
	suite.Suite
	Store    dbCommon.Store
	Fixtures *InstancesTestFixtures
}

func (s *InstancesTestSuite) equalInstancesByName(expected, actual []params.Instance) {
	s.Require().Equal(len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].Name > expected[j].Name })
	sort.Slice(actual, func(i, j int) bool { return actual[i].Name > actual[j].Name })

	for i := 0; i < len(expected); i++ {
		s.Require().Equal(expected[i].Name, actual[i].Name)
	}
}

func (s *InstancesTestSuite) SetupTest() {
	// create testing sqlite database
	db, err := NewSQLDatabase(context.Background(), garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.Store = db

	// create an organization for testing purposes
	org, err := s.Store.CreateOrganization(context.Background(), "test-org", "test-creds", "test-webhookSecret")
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create org: %s", err))
	}

	// create an organization pool for testing purposes
	createPoolParams := params.CreatePoolParams{
		ProviderName:   "test-provider",
		MaxRunners:     4,
		MinIdleRunners: 2,
		Image:          "test-image",
		Flavor:         "test-flavor",
		OSType:         "linux",
		Tags:           []string{"self-hosted", "amd64", "linux"},
	}
	pool, err := s.Store.CreateOrganizationPool(context.Background(), org.ID, createPoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create org pool: %s", err))
	}

	// create some instance objects in the database, for testing purposes
	instances := []params.Instance{}
	for i := 1; i <= 3; i++ {
		instance, err := db.CreateInstance(
			context.Background(),
			pool.ID,
			params.CreateInstanceParams{
				Name:         fmt.Sprintf("test-instance-%d", i),
				OSType:       "linux",
				OSArch:       "amd64",
				CallbackURL:  "https://garm.example.com/",
				Status:       common.InstanceRunning,
				RunnerStatus: common.RunnerIdle,
			},
		)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to create instance object (test-instance-%v)", i))
		}
		instances = append(instances, instance)
	}

	// setup test fixtures
	fixtures := &InstancesTestFixtures{
		Org:       org,
		Pool:      pool,
		Instances: instances,
	}
	s.Fixtures = fixtures
}

func (s *InstancesTestSuite) TestCreateInstance() {
	// setup enviroment for this test
	instanceName := "test-create-instance"
	createInstanceParams := params.CreateInstanceParams{
		Name:        instanceName,
		OSType:      "linux",
		OSArch:      "amd64",
		CallbackURL: "https://garm.example.com/",
	}

	// call tested function
	instance, err := s.Store.CreateInstance(context.Background(), s.Fixtures.Pool.ID, createInstanceParams)

	// assertions
	s.Require().Nil(err)
	storeInstance, err := s.Store.GetInstanceByName(context.Background(), instanceName)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get instance: %v", err))
	}
	s.Require().Equal(storeInstance.Name, instance.Name)
	s.Require().Equal(storeInstance.PoolID, instance.PoolID)
	s.Require().Equal(storeInstance.OSArch, instance.OSArch)
	s.Require().Equal(storeInstance.OSType, instance.OSType)
	s.Require().Equal(storeInstance.CallbackURL, instance.CallbackURL)
}

func (s *InstancesTestSuite) TestCreateInstanceInvalidPoolID() {
	_, err := s.Store.CreateInstance(context.Background(), "dummy-pool-id", params.CreateInstanceParams{})

	s.Require().Equal("fetching pool: parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestGetPoolInstanceByName() {
	storeInstance := s.Fixtures.Instances[0] // this is already created in `SetupTest()`

	instance, err := s.Store.GetPoolInstanceByName(context.Background(), s.Fixtures.Pool.ID, storeInstance.Name)

	s.Require().Nil(err)
	s.Require().Equal(storeInstance.Name, instance.Name)
	s.Require().Equal(storeInstance.PoolID, instance.PoolID)
	s.Require().Equal(storeInstance.OSArch, instance.OSArch)
	s.Require().Equal(storeInstance.OSType, instance.OSType)
	s.Require().Equal(storeInstance.CallbackURL, instance.CallbackURL)
}

func (s *InstancesTestSuite) TestGetPoolInstanceByNameNotFound() {
	_, err := s.Store.GetPoolInstanceByName(context.Background(), s.Fixtures.Pool.ID, "not-existent-instance-name")

	s.Require().Equal("fetching instance: fetching pool instance by name: not found", err.Error())
}

func (s *InstancesTestSuite) TestGetInstanceByName() {
	storeInstance := s.Fixtures.Instances[1]

	instance, err := s.Store.GetInstanceByName(context.Background(), storeInstance.Name)

	s.Require().Nil(err)
	s.Require().Equal(storeInstance.Name, instance.Name)
	s.Require().Equal(storeInstance.PoolID, instance.PoolID)
	s.Require().Equal(storeInstance.OSArch, instance.OSArch)
	s.Require().Equal(storeInstance.OSType, instance.OSType)
	s.Require().Equal(storeInstance.CallbackURL, instance.CallbackURL)
}

func (s *InstancesTestSuite) TestGetInstanceByNameFetchInstanceFailed() {
	_, err := s.Store.GetInstanceByName(context.Background(), "not-existent-instance-name")

	s.Require().Equal("fetching instance: fetching instance by name: not found", err.Error())
}

func (s *InstancesTestSuite) TestDeleteInstance() {
	storeInstance := s.Fixtures.Instances[0]
	err := s.Store.DeleteInstance(context.Background(), s.Fixtures.Pool.ID, storeInstance.Name)

	s.Require().Nil(err)

	_, err = s.Store.GetPoolInstanceByName(context.Background(), s.Fixtures.Pool.ID, storeInstance.Name)
	s.Require().Equal("fetching instance: fetching pool instance by name: not found", err.Error())
}

func (s *InstancesTestSuite) TestDeleteInstanceInvalidPoolID() {
	err := s.Store.DeleteInstance(context.Background(), "dummy-pool-id", "dummy-instance-name")

	s.Require().Equal("deleting instance: fetching pool: parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestAddInstanceStatusMessage() {
	storeInstance := s.Fixtures.Instances[0]
	statusMsg := "test-status-message"

	err := s.Store.AddInstanceStatusMessage(context.Background(), storeInstance.ID, statusMsg)

	s.Require().Nil(err)
	instance, err := s.Store.GetInstanceByName(context.Background(), storeInstance.Name)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get db instance: %s", err))
	}
	s.Require().Equal(1, len(instance.StatusMessages))
	s.Require().Equal(statusMsg, instance.StatusMessages[0].Message)
}

func (s *InstancesTestSuite) TestAddInstanceStatusMessageInvalidPoolID() {
	err := s.Store.AddInstanceStatusMessage(context.Background(), "dummy-id", "dummy-message")

	s.Require().Equal("updating instance: parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestUpdateInstance() {
	updateInstanceParams := params.UpdateInstanceParams{
		ProviderID:    "update-provider-test",
		OSName:        "ubuntu",
		OSVersion:     "focal",
		Status:        common.InstancePendingDelete,
		RunnerStatus:  common.RunnerActive,
		AgentID:       4,
		CreateAttempt: 3,
		Addresses: []params.Address{
			{
				Address: "12.10.12.10",
				Type:    params.PublicAddress,
			},
			{
				Address: "10.1.1.2",
				Type:    params.PrivateAddress,
			},
		},
	}

	instance, err := s.Store.UpdateInstance(context.Background(), s.Fixtures.Instances[0].ID, updateInstanceParams)

	s.Require().Nil(err)
	s.Require().Equal(updateInstanceParams.ProviderID, instance.ProviderID)
	s.Require().Equal(updateInstanceParams.OSName, instance.OSName)
	s.Require().Equal(updateInstanceParams.OSVersion, instance.OSVersion)
	s.Require().Equal(updateInstanceParams.Status, instance.Status)
	s.Require().Equal(updateInstanceParams.RunnerStatus, instance.RunnerStatus)
	s.Require().Equal(updateInstanceParams.AgentID, instance.AgentID)
	s.Require().Equal(updateInstanceParams.CreateAttempt, instance.CreateAttempt)
}

func (s *InstancesTestSuite) TestUpdateInstanceInvalidPoolID() {
	_, err := s.Store.UpdateInstance(context.Background(), "dummy-id", params.UpdateInstanceParams{})

	s.Require().Equal("updating instance: parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestListPoolInstances() {
	instances, err := s.Store.ListPoolInstances(context.Background(), s.Fixtures.Pool.ID)

	s.Require().Nil(err)
	s.equalInstancesByName(s.Fixtures.Instances, instances)
}

func (s *InstancesTestSuite) TestListPoolInstancesInvalidPoolID() {
	_, err := s.Store.ListPoolInstances(context.Background(), "dummy-pool-id")

	s.Require().Equal("fetching pool: parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestListAllInstances() {
	instances, err := s.Store.ListAllInstances(context.Background())

	s.Require().Nil(err)
	s.equalInstancesByName(s.Fixtures.Instances, instances)
}

func (s *InstancesTestSuite) TestPoolInstanceCount() {
	instancesCount, err := s.Store.PoolInstanceCount(context.Background(), s.Fixtures.Pool.ID)

	s.Require().Nil(err)
	s.Require().Equal(int64(len(s.Fixtures.Instances)), instancesCount)
}

func (s *InstancesTestSuite) TestPoolInstanceCountInvalidPoolID() {
	_, err := s.Store.PoolInstanceCount(context.Background(), "dummy-pool-id")

	s.Require().Equal("fetching pool: parsing id: invalid request", err.Error())
}

func TestInstTestSuite(t *testing.T) {
	suite.Run(t, new(InstancesTestSuite))
}
