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

package runner

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

type PoolTestFixtures struct {
	AdminContext         context.Context
	Store                dbCommon.Store
	Pools                []params.Pool
	Providers            map[string]common.Provider
	Credentials          map[string]config.Github
	CreateInstanceParams params.CreateInstanceParams
	UpdatePoolParams     params.UpdatePoolParams
}

type PoolTestSuite struct {
	suite.Suite
	Fixtures *PoolTestFixtures
	Runner   *Runner
}

func (s *PoolTestSuite) SetupTest() {
	adminCtx := auth.GetAdminContext(context.Background())

	// create testing sqlite database
	dbCfg := garmTesting.GetTestSqliteDBConfig(s.T())
	db, err := database.NewDatabase(adminCtx, dbCfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}

	// create an organization for testing purposes
	org, err := db.CreateOrganization(context.Background(), "test-org", "test-creds", "test-webhookSecret")
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create org: %s", err))
	}

	// create some pool objects in the database, for testing purposes
	orgPools := []params.Pool{}
	for i := 1; i <= 3; i++ {
		pool, err := db.CreateOrganizationPool(
			context.Background(),
			org.ID,
			params.CreatePoolParams{
				ProviderName:           "test-provider",
				MaxRunners:             4,
				MinIdleRunners:         2,
				Image:                  fmt.Sprintf("test-image-%d", i),
				Flavor:                 "test-flavor",
				OSType:                 "linux",
				Tags:                   []string{"self-hosted", "amd64", "linux"},
				RunnerBootstrapTimeout: 0,
			},
		)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
		}
		orgPools = append(orgPools, pool)
	}

	// setup test fixtures
	var maxRunners uint = 40
	var minIdleRunners uint = 20
	fixtures := &PoolTestFixtures{
		AdminContext: adminCtx,
		Store:        db,
		Pools:        orgPools,
		UpdatePoolParams: params.UpdatePoolParams{
			MaxRunners:     &maxRunners,
			MinIdleRunners: &minIdleRunners,
			Image:          "test-images-updated",
			Flavor:         "test-flavor-updated",
		},
		CreateInstanceParams: params.CreateInstanceParams{
			Name:   "test-instance-name",
			OSType: "linux",
		},
	}
	s.Fixtures = fixtures

	// setup test runner
	runner := &Runner{
		providers:   fixtures.Providers,
		credentials: fixtures.Credentials,
		store:       fixtures.Store,
		ctx:         fixtures.AdminContext,
	}
	s.Runner = runner
}

func (s *PoolTestSuite) TestListAllPools() {
	// call tested function
	pools, err := s.Runner.ListAllPools(s.Fixtures.AdminContext)

	// assertions
	s.Require().Nil(err)
	garmTesting.EqualDBEntityID(s.T(), s.Fixtures.Pools, pools)
}

func (s *PoolTestSuite) TestListAllPoolsErrUnauthorized() {
	_, err := s.Runner.ListAllPools(context.Background())

	s.Require().NotNil(err)
	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *PoolTestSuite) TestGetPoolByID() {
	pool, err := s.Runner.GetPoolByID(s.Fixtures.AdminContext, s.Fixtures.Pools[0].ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.Pools[0].ID, pool.ID)
}

func (s *PoolTestSuite) TestGetPoolByIDErrUnauthorized() {
	_, err := s.Runner.GetPoolByID(context.Background(), "dummy-pool-id")

	s.Require().NotNil(err)
	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *PoolTestSuite) TestGetPoolByIDNotFound() {
	err := s.Fixtures.Store.DeletePoolByID(context.Background(), s.Fixtures.Pools[0].ID)

	s.Require().Nil(err)
	_, err = s.Runner.GetPoolByID(s.Fixtures.AdminContext, s.Fixtures.Pools[0].ID)
	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: fetching pool by ID: not found", err.Error())
}

func (s *PoolTestSuite) TestDeletePoolByID() {
	err := s.Runner.DeletePoolByID(s.Fixtures.AdminContext, s.Fixtures.Pools[0].ID)

	s.Require().Nil(err)
	_, err = s.Fixtures.Store.GetPoolByID(s.Fixtures.AdminContext, s.Fixtures.Pools[0].ID)
	s.Require().NotNil(err)
	s.Require().Equal("fetching pool by ID: not found", err.Error())
}

func (s *PoolTestSuite) TestDeletePoolByIDErrUnauthorized() {
	err := s.Runner.DeletePoolByID(context.Background(), "dummy-pool-id")

	s.Require().NotNil(err)
	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *PoolTestSuite) TestDeletePoolByIDRunnersFailed() {
	_, err := s.Fixtures.Store.CreateInstance(s.Fixtures.AdminContext, s.Fixtures.Pools[0].ID, s.Fixtures.CreateInstanceParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create instance: %s", err))
	}

	err = s.Runner.DeletePoolByID(s.Fixtures.AdminContext, s.Fixtures.Pools[0].ID)
	s.Require().NotNil(err)
	s.Require().Equal(runnerErrors.NewBadRequestError("pool has runners"), err)
}

func (s *PoolTestSuite) TestUpdatePoolByID() {
	pool, err := s.Runner.UpdatePoolByID(s.Fixtures.AdminContext, s.Fixtures.Pools[0].ID, s.Fixtures.UpdatePoolParams)

	s.Require().Nil(err)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MaxRunners, pool.MaxRunners)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MinIdleRunners, pool.MinIdleRunners)
	s.Require().Equal(s.Fixtures.UpdatePoolParams.Image, pool.Image)
	s.Require().Equal(s.Fixtures.UpdatePoolParams.Flavor, pool.Flavor)
}

func (s *PoolTestSuite) TestUpdatePoolByIDErrUnauthorized() {
	_, err := s.Runner.UpdatePoolByID(context.Background(), "dummy-pool-id", s.Fixtures.UpdatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *PoolTestSuite) TestTestUpdatePoolByIDInvalidPoolID() {
	_, err := s.Runner.UpdatePoolByID(s.Fixtures.AdminContext, "dummy-pool-id", s.Fixtures.UpdatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: fetching pool by ID: parsing id: invalid request", err.Error())
}

func (s *PoolTestSuite) TestTestUpdatePoolByIDRunnerBootstrapTimeoutFailed() {
	// this is already created in `SetupTest()`
	var RunnerBootstrapTimeout uint = 0
	s.Fixtures.UpdatePoolParams.RunnerBootstrapTimeout = &RunnerBootstrapTimeout

	_, err := s.Runner.UpdatePoolByID(s.Fixtures.AdminContext, s.Fixtures.Pools[0].ID, s.Fixtures.UpdatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal(runnerErrors.NewBadRequestError("runner_bootstrap_timeout cannot be 0"), err)
}

func (s *PoolTestSuite) TestTestUpdatePoolByIDMinIdleGreaterThanMax() {
	var maxRunners uint = 10
	var minIdleRunners uint = 11
	s.Fixtures.UpdatePoolParams.MaxRunners = &maxRunners
	s.Fixtures.UpdatePoolParams.MinIdleRunners = &minIdleRunners

	_, err := s.Runner.UpdatePoolByID(s.Fixtures.AdminContext, s.Fixtures.Pools[0].ID, s.Fixtures.UpdatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal(runnerErrors.NewBadRequestError("min_idle_runners cannot be larger than max_runners"), err)
}

func TestPoolTestSuite(t *testing.T) {
	suite.Run(t, new(PoolTestSuite))
}
