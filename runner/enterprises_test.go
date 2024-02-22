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

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	garmTesting "github.com/cloudbase/garm/internal/testing" //nolint:typecheck
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	runnerCommonMocks "github.com/cloudbase/garm/runner/common/mocks"
	runnerMocks "github.com/cloudbase/garm/runner/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type EnterpriseTestFixtures struct {
	AdminContext           context.Context
	DBFile                 string
	Store                  dbCommon.Store
	StoreEnterprises       map[string]params.Enterprise
	Providers              map[string]common.Provider
	Credentials            map[string]config.Github
	CreateEnterpriseParams params.CreateEnterpriseParams
	CreatePoolParams       params.CreatePoolParams
	CreateInstanceParams   params.CreateInstanceParams
	UpdateRepoParams       params.UpdateEntityParams
	UpdatePoolParams       params.UpdatePoolParams
	UpdatePoolStateParams  params.UpdatePoolStateParams
	ErrMock                error
	ProviderMock           *runnerCommonMocks.Provider
	PoolMgrMock            *runnerCommonMocks.PoolManager
	PoolMgrCtrlMock        *runnerMocks.PoolManagerController
}

type EnterpriseTestSuite struct {
	suite.Suite
	Fixtures *EnterpriseTestFixtures
	Runner   *Runner
}

func (s *EnterpriseTestSuite) SetupTest() {
	adminCtx := auth.GetAdminContext(context.Background())

	// create testing sqlite database
	dbCfg := garmTesting.GetTestSqliteDBConfig(s.T())
	db, err := database.NewDatabase(adminCtx, dbCfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}

	// create some organization objects in the database, for testing purposes
	enterprises := map[string]params.Enterprise{}
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("test-enterprise-%v", i)
		enterprise, err := db.CreateEnterprise(
			adminCtx,
			name,
			fmt.Sprintf("test-creds-%v", i),
			fmt.Sprintf("test-webhook-secret-%v", i),
		)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to create database object (test-enterprise-%v)", i))
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
		Credentials: map[string]config.Github{
			"test-creds": {
				Name:        "test-creds-name",
				Description: "test-creds-description",
				OAuth2Token: "test-creds-oauth2-token",
			},
		},
		CreateEnterpriseParams: params.CreateEnterpriseParams{
			Name:            "test-enterprise-create",
			CredentialsName: "test-creds",
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
			Tags:                   []string{"self-hosted", "arm64", "linux"},
			RunnerBootstrapTimeout: 0,
		},
		CreateInstanceParams: params.CreateInstanceParams{
			Name:   "test-instance-name",
			OSType: "linux",
		},
		UpdateRepoParams: params.UpdateEntityParams{
			CredentialsName: "test-creds",
			WebhookSecret:   "test-update-repo-webhook-secret",
		},
		UpdatePoolParams: params.UpdatePoolParams{
			MaxRunners:     &maxRunners,
			MinIdleRunners: &minIdleRunners,
			Image:          "test-images-updated",
			Flavor:         "test-flavor-updated",
		},
		UpdatePoolStateParams: params.UpdatePoolStateParams{
			WebhookSecret: "test-update-repo-webhook-secret",
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
		credentials:     fixtures.Credentials,
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

	// assertions
	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.CreateEnterpriseParams.Name, enterprise.Name)
	s.Require().Equal(s.Fixtures.Credentials[s.Fixtures.CreateEnterpriseParams.CredentialsName].Name, enterprise.CredentialsName)
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
	s.Fixtures.CreateEnterpriseParams.CredentialsName = "not-existent-creds-name"

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
	s.Require().Equal(fmt.Sprintf("creating enterprise pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *EnterpriseTestSuite) TestCreateEnterpriseStartPoolMgrFailed() {
	s.Fixtures.PoolMgrMock.On("Start").Return(s.Fixtures.ErrMock)
	s.Fixtures.PoolMgrCtrlMock.On("CreateEnterprisePoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Enterprise"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrCtrlMock.On("DeleteEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.ErrMock)

	_, err := s.Runner.CreateEnterprise(s.Fixtures.AdminContext, s.Fixtures.CreateEnterpriseParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("starting enterprise pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *EnterpriseTestSuite) TestListEnterprises() {
	s.Fixtures.PoolMgrCtrlMock.On("GetEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)
	orgs, err := s.Runner.ListEnterprises(s.Fixtures.AdminContext)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityByName(s.T(), garmTesting.DBEntityMapToSlice(s.Fixtures.StoreEnterprises), orgs)
}

func (s *EnterpriseTestSuite) TestListEnterprisesErrUnauthorized() {
	_, err := s.Runner.ListEnterprises(context.Background())

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
	s.Require().Equal("fetching enterprise: not found", err.Error())
}

func (s *EnterpriseTestSuite) TestDeleteEnterpriseErrUnauthorized() {
	err := s.Runner.DeleteEnterprise(context.Background(), "dummy-enterprise-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *EnterpriseTestSuite) TestDeleteEnterprisePoolDefinedFailed() {
	pool, err := s.Fixtures.Store.CreateEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.CreatePoolParams)
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
	s.Require().Equal(fmt.Sprintf("deleting enterprise pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *EnterpriseTestSuite) TestUpdateEnterprise() {
	s.Fixtures.PoolMgrCtrlMock.On("UpdateEnterprisePoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)

	org, err := s.Runner.UpdateEnterprise(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.CredentialsName, org.CredentialsName)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.WebhookSecret, org.WebhookSecret)
}

func (s *EnterpriseTestSuite) TestUpdateEnterpriseErrUnauthorized() {
	_, err := s.Runner.UpdateEnterprise(context.Background(), "dummy-enterprise-id", s.Fixtures.UpdateRepoParams)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *EnterpriseTestSuite) TestUpdateEnterpriseInvalidCreds() {
	s.Fixtures.UpdateRepoParams.CredentialsName = "invalid-creds-name"

	_, err := s.Runner.UpdateEnterprise(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Require().Equal(runnerErrors.NewBadRequestError("invalid credentials (%s) for enterprise %s", s.Fixtures.UpdateRepoParams.CredentialsName, s.Fixtures.StoreEnterprises["test-enterprise-1"].Name), err)
}

func (s *EnterpriseTestSuite) TestUpdateEnterprisePoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("UpdateEnterprisePoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.UpdateEnterprise(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("failed to update enterprise pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *EnterpriseTestSuite) TestUpdateEnterpriseCreateEnterprisePoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("UpdateEnterprisePoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.UpdateEnterprise(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("failed to update enterprise pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *EnterpriseTestSuite) TestCreateEnterprisePool() {
	s.Fixtures.PoolMgrCtrlMock.On("GetEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, nil)

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

func (s *EnterpriseTestSuite) TestCreateEnterprisePoolErrNotFound() {
	s.Fixtures.PoolMgrCtrlMock.On("GetEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, runnerErrors.ErrNotFound)

	_, err := s.Runner.CreateEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.CreatePoolParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(runnerErrors.ErrNotFound, err)
}

func (s *EnterpriseTestSuite) TestCreateEnterprisePoolFetchPoolParamsFailed() {
	s.Fixtures.CreatePoolParams.ProviderName = "not-existent-provider-name"

	s.Fixtures.PoolMgrCtrlMock.On("GetEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, nil)

	_, err := s.Runner.CreateEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.CreatePoolParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Regexp("fetching pool params: no such provider", err.Error())
}

func (s *EnterpriseTestSuite) TestGetEnterprisePoolByID() {
	enterprisePool, err := s.Fixtures.Store.CreateEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.CreatePoolParams)
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
	pool, err := s.Fixtures.Store.CreateEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create enterprise pool: %s", err))
	}

	err = s.Runner.DeleteEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, pool.ID)

	s.Require().Nil(err)

	_, err = s.Fixtures.Store.GetEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, pool.ID)
	s.Require().Equal("fetching pool: finding pool: not found", err.Error())
}

func (s *EnterpriseTestSuite) TestDeleteEnterprisePoolErrUnauthorized() {
	err := s.Runner.DeleteEnterprisePool(context.Background(), "dummy-enterprise-id", "dummy-pool-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *EnterpriseTestSuite) TestDeleteEnterprisePoolRunnersFailed() {
	pool, err := s.Fixtures.Store.CreateEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.CreatePoolParams)
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
	enterprisePools := []params.Pool{}
	for i := 1; i <= 2; i++ {
		s.Fixtures.CreatePoolParams.Image = fmt.Sprintf("test-enterprise-%v", i)
		pool, err := s.Fixtures.Store.CreateEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.CreatePoolParams)
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
	enterprisePool, err := s.Fixtures.Store.CreateEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.CreatePoolParams)
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
	pool, err := s.Fixtures.Store.CreateEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.CreatePoolParams)
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
	pool, err := s.Fixtures.Store.CreateEnterprisePool(s.Fixtures.AdminContext, s.Fixtures.StoreEnterprises["test-enterprise-1"].ID, s.Fixtures.CreatePoolParams)
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

	poolManager, err := s.Runner.findEnterprisePoolManager(s.Fixtures.StoreEnterprises["test-enterprise-1"].Name)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.PoolMgrMock, poolManager)
}

func (s *EnterpriseTestSuite) TestFindEnterprisePoolManagerFetchPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("GetEnterprisePoolManager", mock.AnythingOfType("params.Enterprise")).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.findEnterprisePoolManager(s.Fixtures.StoreEnterprises["test-enterprise-1"].Name)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Regexp("fetching pool manager for enterprise", err.Error())
}

func TestEnterpriseTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(EnterpriseTestSuite))
}
