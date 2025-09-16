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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	garmTesting "github.com/cloudbase/garm/internal/testing" //nolint:typecheck
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	runnerCommonMocks "github.com/cloudbase/garm/runner/common/mocks"
	runnerMocks "github.com/cloudbase/garm/runner/mocks"
)

type EnterpriseTestFixtures struct {
	AdminContext           context.Context
	DBFile                 string
	Store                  dbCommon.Store
	StoreEnterprises       map[string]params.Enterprise
	Providers              map[string]common.Provider
	Credentials            map[string]params.ForgeCredentials
	CreateEnterpriseParams params.CreateEnterpriseParams
	CreatePoolParams       params.CreatePoolParams
	CreateInstanceParams   params.CreateInstanceParams
	UpdateRepoParams       params.UpdateEntityParams
	UpdatePoolParams       params.UpdatePoolParams
	ErrMock                error
	ProviderMock           *runnerCommonMocks.Provider
	PoolMgrMock            *runnerCommonMocks.PoolManager
	PoolMgrCtrlMock        *runnerMocks.PoolManagerController
}

type EnterpriseTestSuite struct {
	suite.Suite
	Fixtures *EnterpriseTestFixtures
	Runner   *Runner

	testCreds          params.ForgeCredentials
	secondaryTestCreds params.ForgeCredentials
	forgeEndpoint      params.ForgeEndpoint
	ghesEndpoint       params.ForgeEndpoint
	ghesCreds          params.ForgeCredentials
}

func (s *EnterpriseTestSuite) SetupTest() {
	// create testing sqlite database
	dbCfg := garmTesting.GetTestSqliteDBConfig(s.T())
	db, err := database.NewDatabase(context.Background(), dbCfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}

	adminCtx := garmTesting.ImpersonateAdminContext(context.Background(), db, s.T())
	s.forgeEndpoint = garmTesting.CreateDefaultGithubEndpoint(adminCtx, db, s.T())
	s.ghesEndpoint = garmTesting.CreateGHESEndpoint(adminCtx, db, s.T())
	s.testCreds = garmTesting.CreateTestGithubCredentials(adminCtx, "new-creds", db, s.T(), s.forgeEndpoint)
	s.secondaryTestCreds = garmTesting.CreateTestGithubCredentials(adminCtx, "secondary-creds", db, s.T(), s.forgeEndpoint)
	s.ghesCreds = garmTesting.CreateTestGithubCredentials(adminCtx, "ghes-creds", db, s.T(), s.ghesEndpoint)

	// create some organization objects in the database, for testing purposes
	enterprises := map[string]params.Enterprise{}
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("test-enterprise-%v", i)
		enterprise, err := db.CreateEnterprise(
			adminCtx,
			name,
			s.testCreds,
			fmt.Sprintf("test-webhook-secret-%v", i),
			params.PoolBalancerTypeRoundRobin,
			false,
		)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to create database object (test-enterprise-%v): %+v", i, err))
		}
		enterprises[name] = enterprise
	}

	// setup test fixtures
	var maxRunners uint = 40
	var minIdleRunners uint = 20
	providerMock := runnerCommonMocks.NewProvider(s.T())
	fixtures := &EnterpriseTestFixtures{
		AdminContext:     adminCtx,
		DBFile:           dbCfg.SQLite.DBFile,
		Store:            db,
		StoreEnterprises: enterprises,
		Providers: map[string]common.Provider{
			"test-provider": providerMock,
		},
		Credentials: map[string]params.ForgeCredentials{
			s.testCreds.Name:          s.testCreds,
			s.secondaryTestCreds.Name: s.secondaryTestCreds,
		},
		CreateEnterpriseParams: params.CreateEnterpriseParams{
			Name:            "test-enterprise-create",
			CredentialsName: s.testCreds.Name,
			WebhookSecret:   "test-create-enterprise-webhook-secret",
		},
		CreatePoolParams: params.CreatePoolParams{
			ProviderName:           "test-provider",
			MaxRunners:             4,
			MinIdleRunners:         2,
			Image:                  "test",
			Flavor:                 "test",
			OSType:                 "linux",
			OSArch:                 "arm64",
			Tags:                   []string{"arm64-linux-runner"},
			RunnerBootstrapTimeout: 0,
		},
		CreateInstanceParams: params.CreateInstanceParams{
			Name:   "test-instance-name",
			OSType: "linux",
		},
		UpdateRepoParams: params.UpdateEntityParams{
			CredentialsName: s.testCreds.Name,
			WebhookSecret:   "test-update-repo-webhook-secret",
		},
		UpdatePoolParams: params.UpdatePoolParams{
			MaxRunners:     &maxRunners,
			MinIdleRunners: &minIdleRunners,
			Image:          "test-images-updated",
			Flavor:         "test-flavor-updated",
		},
		ErrMock:         fmt.Errorf("mock error"),
		ProviderMock:    providerMock,
		PoolMgrMock:     runnerCommonMocks.NewPoolManager(s.T()),
		PoolMgrCtrlMock: runnerMocks.NewPoolManagerController(s.T()),
	}
	s.Fixtures = fixtures

	// setup test runner
	runner := &Runner{
		providers:       fixtures.Providers,
		ctx:             fixtures.AdminContext,
		store:           fixtures.Store,
		poolManagerCtrl: fixtures.PoolMgrCtrlMock,
	}
	s.Runner = runner
}

func (s *EnterpriseTestSuite) TestCreateEnterprise() {
	// setup mocks expectations
	s.Fixtures.PoolMgrMock.On("Start").Return(nil)
	s.Fixtures.PoolMgrCtrlMock.On("CreateEnterprisePoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Enterprise"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, nil)

	// call tested function
	enterprise, err := s.Runner.CreateEnterprise(s.Fixtures.AdminContext, s.Fixtures.CreateEnterpriseParams)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.CreateEnterpriseParams.Name, enterprise.Name)
	s.Require().Equal(s.Fixtures.Credentials[s.Fixtures.CreateEnterpriseParams.CredentialsName].Name, enterprise.Credentials.Name)
	s.Require().Equal(params.PoolBalancerTypeRoundRobin, enterprise.PoolBalancerType)
	// assertions
	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
}

func (s *EnterpriseTestSuite) TestCreateEnterpriseErrUnauthorized() {
	_, err := s.Runner.CreateEnterprise(context.Background(), s.Fixtures.CreateEnterpriseParams)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *EnterpriseTestSuite) TestCreateEnterpriseEmptyParams() {
	_, err := s.Runner.CreateEnterprise(s.Fixtures.AdminContext, params.CreateEnterpriseParams{})

	s.Require().Regexp("validating params: missing enterprise name", err.Error())
}

func (s *EnterpriseTestSuite) TestCreateEnterpriseMissingCredentials() {
	s.Fixtures.CreateEnterpriseParams.CredentialsName = notExistingCredentialsName

	_, err := s.Runner.CreateEnterprise(s.Fixtures.AdminContext, s.Fixtures.CreateEnterpriseParams)

	s.Require().Equal(runnerErrors.NewBadRequestError("credentials %s not defined", s.Fixtures.CreateEnterpriseParams.CredentialsName), err)
}

func (s *EnterpriseTestSuite) TestCreateEnterpriseAlreadyExists() {
	s.Fixtures.CreateEnterpriseParams.Name = "test-enterprise-1" // this is already created in `SetupTest()`

	_, err := s.Runner.CreateEnterprise(s.Fixtures.AdminContext, s.Fixtures.CreateEnterpriseParams)

	s.Require().Equal(runnerErrors.NewConflictError("enterprise %s already exists", s.Fixtures.CreateEnterpriseParams.Name), err)
}

func (s *EnterpriseTestSuite) TestCreateEnterprisePoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("CreateEnterprisePoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Enterprise"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.CreateEnterprise(s.Fixtures.AdminContext, s.Fixtures.CreateEnterpriseParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("error creating enterprise pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *EnterpriseTestSuite) TestCreateEnterpriseStartPoolMgrFailed() {
	s.Fixtures.PoolMgrMock.On("Start").Return(s.Fixtures.ErrMock)
	s.Fixtures.PoolMgrCtrlMock.On("CreateEnterprisePoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Enterprise"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrCtrlMock.On("DeleteEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.ErrMock)

	_, err := s.Runner.CreateEnterprise(s.Fixtures.AdminContext, s.Fixtures.CreateEnterpriseParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("error starting enterprise pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *EnterpriseTestSuite) TestListEnterprises() {
	s.Fixtures.PoolMgrCtrlMock.On("GetEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)
	orgs, err := s.Runner.ListEnterprises(s.Fixtures.AdminContext, params.EnterpriseFilter{})

	s.Require().Nil(err)
	garmTesting.EqualDBEntityByName(s.T(), garmTesting.DBEntityMapToSlice(s.Fixtures.StoreEnterprises), orgs)
}

func (s *EnterpriseTestSuite) TestListEnterprisesWithFilters() {
	s.Fixtures.PoolMgrCtrlMock.On("GetEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)

	enterprise, err := s.Fixtures.Store.CreateEnterprise(
		s.Fixtures.AdminContext,
		"test-enterprise",
		s.testCreds,
		"super secret",
		params.PoolBalancerTypeRoundRobin,
		false,
	)
	s.Require().NoError(err)
	enterprise2, err := s.Fixtures.Store.CreateEnterprise(
		s.Fixtures.AdminContext,
		"test-enterprise2",
		s.testCreds,
		"super secret",
		params.PoolBalancerTypeRoundRobin,
		false,
	)
	s.Require().NoError(err)
	enterprise3, err := s.Fixtures.Store.CreateEnterprise(
		s.Fixtures.AdminContext,
		"test-enterprise",
		s.ghesCreds,
		"super secret",
		params.PoolBalancerTypeRoundRobin,
		false,
	)
	s.Require().NoError(err)
	orgs, err := s.Runner.ListEnterprises(
		s.Fixtures.AdminContext,
		params.EnterpriseFilter{
			Name: "test-enterprise",
		},
	)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityByName(s.T(), []params.Enterprise{enterprise, enterprise3}, orgs)

	orgs, err = s.Runner.ListEnterprises(
		s.Fixtures.AdminContext,
		params.EnterpriseFilter{
			Name:     "test-enterprise",
			Endpoint: s.ghesEndpoint.Name,
		},
	)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityByName(s.T(), []params.Enterprise{enterprise3}, orgs)

	orgs, err = s.Runner.ListEnterprises(
		s.Fixtures.AdminContext,
		params.EnterpriseFilter{
			Name: "test-enterprise2",
		},
	)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityByName(s.T(), []params.Enterprise{enterprise2}, orgs)
}

func (s *EnterpriseTestSuite) TestListEnterprisesErrUnauthorized() {
	_, err := s.Runner.ListEnterprises(context.Background(), params.EnterpriseFilter{})

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *EnterpriseTestSuite) TestGetEnterpriseByID() {
	s.Fixtures.PoolMgrCtrlMock.On("GetEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)
	org, err := s.Runner.GetEnterpriseByID(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, org.ID)
}

func (s *EnterpriseTestSuite) TestGetEnterpriseByIDErrUnauthorized() {
	_, err := s.Runner.GetEnterpriseByID(context.Background(), "dummy-org-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *EnterpriseTestSuite) TestDeleteEnterprise() {
	s.Fixtures.PoolMgrCtrlMock.On("DeleteEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(nil)

	err := s.Runner.DeleteEnterprise(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-3"].ID)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)

	_, err = s.Fixtures.Store.GetEnterpriseByID(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-3"].ID)
	s.Require().Equal("error fetching enterprise: not found", err.Error())
}

func (s *EnterpriseTestSuite) TestDeleteEnterpriseErrUnauthorized() {
	err := s.Runner.DeleteEnterprise(context.Background(), "dummy-enterprise-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *EnterpriseTestSuite) TestDeleteEnterprisePoolDefinedFailed() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreEnterprises["test-enterprise-1"].ID,
		EntityType: params.ForgeEntityTypeEnterprise,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create store enterprises pool: %v", err))
	}

	err = s.Runner.DeleteEnterprise(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID)

	s.Require().Equal(runnerErrors.NewBadRequestError("enterprise has pools defined (%s)", pool.ID), err)
}

func (s *EnterpriseTestSuite) TestDeleteEnterprisePoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("DeleteEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.ErrMock)

	err := s.Runner.DeleteEnterprise(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("error deleting enterprise pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *EnterpriseTestSuite) TestUpdateEnterprise() {
	s.Fixtures.PoolMgrCtrlMock.On("GetEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)

	param := s.Fixtures.UpdateRepoParams
	param.PoolBalancerType = params.PoolBalancerTypePack
	org, err := s.Runner.UpdateEnterprise(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, param)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.CredentialsName, org.Credentials.Name)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.WebhookSecret, org.WebhookSecret)
	s.Require().Equal(params.PoolBalancerTypePack, org.PoolBalancerType)
}

func (s *EnterpriseTestSuite) TestUpdateEnterpriseErrUnauthorized() {
	_, err := s.Runner.UpdateEnterprise(context.Background(), "dummy-enterprise-id", s.Fixtures.UpdateRepoParams)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *EnterpriseTestSuite) TestUpdateEnterpriseInvalidCreds() {
	s.Fixtures.UpdateRepoParams.CredentialsName = invalidCredentialsName

	_, err := s.Runner.UpdateEnterprise(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.UpdateRepoParams)

	if !errors.Is(err, runnerErrors.ErrNotFound) {
		s.FailNow(fmt.Sprintf("expected error: %v", runnerErrors.ErrNotFound))
	}
}

func (s *EnterpriseTestSuite) TestUpdateEnterprisePoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("GetEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.UpdateEnterprise(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("failed to get enterprise pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *EnterpriseTestSuite) TestUpdateEnterpriseCreateEnterprisePoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("GetEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.UpdateEnterprise(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("failed to get enterprise pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *EnterpriseTestSuite) TestCreateEnterprisePool() {
	pool, err := s.Runner.CreateEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.CreatePoolParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)

	enterprise, err := s.Fixtures.Store.GetEnterpriseByID(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot get enterprise by ID: %v", err))
	}
	s.Require().Equal(1, len(enterprise.Pools))
	s.Require().Equal(pool.ID, enterprise.Pools[0].ID)
	s.Require().Equal(s.Fixtures.CreatePoolParams.ProviderName, enterprise.Pools[0].ProviderName)
	s.Require().Equal(s.Fixtures.CreatePoolParams.MaxRunners, enterprise.Pools[0].MaxRunners)
	s.Require().Equal(s.Fixtures.CreatePoolParams.MinIdleRunners, enterprise.Pools[0].MinIdleRunners)
}

func (s *EnterpriseTestSuite) TestCreateEnterprisePoolErrUnauthorized() {
	_, err := s.Runner.CreateEnterprisePool(context.Background(), "dummy-enterprise-id", s.Fixtures.CreatePoolParams)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *EnterpriseTestSuite) TestCreateEnterprisePoolFetchPoolParamsFailed() {
	s.Fixtures.CreatePoolParams.ProviderName = notExistingProviderName
	_, err := s.Runner.CreateEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.CreatePoolParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Regexp("failed to append tags to create pool params: no such provider not-existent-provider-name", err.Error())
}

func (s *EnterpriseTestSuite) TestGetEnterprisePoolByID() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreEnterprises["test-enterprise-1"].ID,
		EntityType: params.ForgeEntityTypeEnterprise,
	}
	enterprisePool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create enterprise pool: %s", err))
	}

	pool, err := s.Runner.GetEnterprisePoolByID(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, enterprisePool.ID)

	s.Require().Nil(err)
	s.Require().Equal(enterprisePool.ID, pool.ID)
}

func (s *EnterpriseTestSuite) TestGetEnterprisePoolByIDErrUnauthorized() {
	_, err := s.Runner.GetEnterprisePoolByID(context.Background(), "dummy-enterprise-id", "dummy-pool-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *EnterpriseTestSuite) TestDeleteEnterprisePool() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreEnterprises["test-enterprise-1"].ID,
		EntityType: params.ForgeEntityTypeEnterprise,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create enterprise pool: %s", err))
	}

	err = s.Runner.DeleteEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, pool.ID)

	s.Require().Nil(err)

	_, err = s.Fixtures.Store.GetEntityPool(s.Fixtures.AdminContext, entity, pool.ID)
	s.Require().Equal("fetching pool: error finding pool: not found", err.Error())
}

func (s *EnterpriseTestSuite) TestDeleteEnterprisePoolErrUnauthorized() {
	err := s.Runner.DeleteEnterprisePool(context.Background(), "dummy-enterprise-id", "dummy-pool-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *EnterpriseTestSuite) TestDeleteEnterprisePoolRunnersFailed() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreEnterprises["test-enterprise-1"].ID,
		EntityType: params.ForgeEntityTypeEnterprise,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create enterprise pool: %v", err))
	}
	instance, err := s.Fixtures.Store.CreateInstance(s.Fixtures.AdminContext, pool.ID, s.Fixtures.CreateInstanceParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create instance: %s", err))
	}

	err = s.Runner.DeleteEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, pool.ID)

	s.Require().Equal(runnerErrors.NewBadRequestError("pool has runners: %s", instance.ID), err)
}

func (s *EnterpriseTestSuite) TestListEnterprisePools() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreEnterprises["test-enterprise-1"].ID,
		EntityType: params.ForgeEntityTypeEnterprise,
	}
	enterprisePools := []params.Pool{}
	for i := 1; i <= 2; i++ {
		s.Fixtures.CreatePoolParams.Image = fmt.Sprintf("test-enterprise-%v", i)
		pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
		}
		enterprisePools = append(enterprisePools, pool)
	}

	pools, err := s.Runner.ListEnterprisePools(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityID(s.T(), enterprisePools, pools)
}

func (s *EnterpriseTestSuite) TestListOrgPoolsErrUnauthorized() {
	_, err := s.Runner.ListOrgPools(context.Background(), "dummy-org-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *EnterpriseTestSuite) TestUpdateEnterprisePool() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreEnterprises["test-enterprise-1"].ID,
		EntityType: params.ForgeEntityTypeEnterprise,
	}
	enterprisePool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create enterprise pool: %s", err))
	}

	pool, err := s.Runner.UpdateEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, enterprisePool.ID, s.Fixtures.UpdatePoolParams)

	s.Require().Nil(err)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MaxRunners, pool.MaxRunners)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MinIdleRunners, pool.MinIdleRunners)
}

func (s *EnterpriseTestSuite) TestUpdateEnterprisePoolErrUnauthorized() {
	_, err := s.Runner.UpdateEnterprisePool(context.Background(), "dummy-enterprise-id", "dummy-pool-id", s.Fixtures.UpdatePoolParams)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *EnterpriseTestSuite) TestUpdateEnterprisePoolMinIdleGreaterThanMax() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreEnterprises["test-enterprise-1"].ID,
		EntityType: params.ForgeEntityTypeEnterprise,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create enterprise pool: %s", err))
	}
	var maxRunners uint = 10
	var minIdleRunners uint = 11
	s.Fixtures.UpdatePoolParams.MaxRunners = &maxRunners
	s.Fixtures.UpdatePoolParams.MinIdleRunners = &minIdleRunners

	_, err = s.Runner.UpdateEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, pool.ID, s.Fixtures.UpdatePoolParams)

	s.Require().Equal(runnerErrors.NewBadRequestError("min_idle_runners cannot be larger than max_runners"), err)
}

func (s *EnterpriseTestSuite) TestListEnterpriseInstances() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreEnterprises["test-enterprise-1"].ID,
		EntityType: params.ForgeEntityTypeEnterprise,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create enterprise pool: %v", err))
	}
	poolInstances := []params.Instance{}
	for i := 1; i <= 3; i++ {
		s.Fixtures.CreateInstanceParams.Name = fmt.Sprintf("test-enterprise-%v", i)
		instance, err := s.Fixtures.Store.CreateInstance(s.Fixtures.AdminContext, pool.ID, s.Fixtures.CreateInstanceParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create instance: %s", err))
		}
		poolInstances = append(poolInstances, instance)
	}

	instances, err := s.Runner.ListEnterpriseInstances(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityID(s.T(), poolInstances, instances)
}

func (s *EnterpriseTestSuite) TestListEnterpriseInstancesErrUnauthorized() {
	_, err := s.Runner.ListEnterpriseInstances(context.Background(), "dummy-enterprise-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *EnterpriseTestSuite) TestFindEnterprisePoolManager() {
	s.Fixtures.PoolMgrCtrlMock.On("GetEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, nil)

	poolManager, err := s.Runner.findEnterprisePoolManager(s.Fixtures.StoreEnterprises["test-enterprise-1"].Name, s.Fixtures.StoreEnterprises["test-enterprise-1"].Endpoint.Name)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.PoolMgrMock, poolManager)
}

func (s *EnterpriseTestSuite) TestFindEnterprisePoolManagerFetchPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("GetEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.findEnterprisePoolManager(s.Fixtures.StoreEnterprises["test-enterprise-1"].Name, s.Fixtures.StoreEnterprises["test-enterprise-1"].Endpoint.Name)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Regexp("fetching pool manager for enterprise", err.Error())
}

func TestEnterpriseTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(EnterpriseTestSuite))
}
