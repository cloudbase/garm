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
				Name:        fmt.Sprintf("test-instance-%v", i),
				OSType:      "linux",
				OSArch:      "amd64",
				CallbackURL: "https://garm.example.com/",
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

func (s *InstancesTestSuite) TestCreateInstanceFetchPoolFailed() {
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

func (s *InstancesTestSuite) TestGetPoolInstanceByNameFetchInstanceFailed() {
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

func TestInstTestSuite(t *testing.T) {
	suite.Run(t, new(InstancesTestSuite))
}
