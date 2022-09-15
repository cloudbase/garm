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
	"garm/auth"
	"garm/config"
	"garm/database"
	dbCommon "garm/database/common"
	runnerErrors "garm/errors"
	garmTesting "garm/internal/testing"
	"garm/params"
	"garm/runner/common"
	runnerCommonMocks "garm/runner/common/mocks"
	runnerMocks "garm/runner/mocks"
	"sort"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type OrgTestFixtures struct {
	AdminContext          context.Context
	DBFile                string
	Store                 dbCommon.Store
	StoreOrgs             map[string]params.Organization
	Providers             map[string]common.Provider
	Credentials           map[string]config.Github
	CreateOrgParams       params.CreateOrgParams
	CreatePoolParams      params.CreatePoolParams
	CreateInstanceParams  params.CreateInstanceParams
	UpdateRepoParams      params.UpdateRepositoryParams
	UpdatePoolParams      params.UpdatePoolParams
	UpdatePoolStateParams params.UpdatePoolStateParams
	ErrMock               error
	ProviderMock          *runnerCommonMocks.Provider
	PoolMgrMock           *runnerCommonMocks.PoolManager
	PoolMgrCtrlMock       *runnerMocks.PoolManagerController
}

type OrgTestSuite struct {
	suite.Suite
	Fixtures *OrgTestFixtures
	Runner   *Runner
}

func (s *OrgTestSuite) orgsMapValues(orgs map[string]params.Organization) []params.Organization {
	orgsSlice := []params.Organization{}
	for _, value := range orgs {
		orgsSlice = append(orgsSlice, value)
	}
	return orgsSlice
}

func (s *OrgTestSuite) equalOrgsByName(expected, actual []params.Organization) {
	s.Require().Equal(len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].Name > expected[j].Name })
	sort.Slice(actual, func(i, j int) bool { return actual[i].Name > actual[j].Name })

	for i := 0; i < len(expected); i++ {
		s.Require().Equal(expected[i].Name, actual[i].Name)
	}
}

func (s *OrgTestSuite) equalPoolsByID(expected, actual []params.Pool) {
	s.Require().Equal(len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].ID > expected[j].ID })
	sort.Slice(actual, func(i, j int) bool { return actual[i].ID > actual[j].ID })

	for i := 0; i < len(expected); i++ {
		s.Require().Equal(expected[i].ID, actual[i].ID)
	}
}

func (s *OrgTestSuite) equalInstancesByName(expected, actual []params.Instance) {
	s.Require().Equal(len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].Name > expected[j].Name })
	sort.Slice(actual, func(i, j int) bool { return actual[i].Name > actual[j].Name })

	for i := 0; i < len(expected); i++ {
		s.Require().Equal(expected[i].Name, actual[i].Name)
	}
}

func (s *OrgTestSuite) SetupTest() {
	adminCtx := auth.GetAdminContext()

	// create testing sqlite database
	dbCfg := garmTesting.GetTestSqliteDBConfig(s.T())
	db, err := database.NewDatabase(adminCtx, dbCfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}

	// create some organization objects in the database, for testing purposes
	orgs := map[string]params.Organization{}
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("test-org-%v", i)
		org, err := db.CreateOrganization(
			adminCtx,
			name,
			fmt.Sprintf("test-creds-%v", i),
			fmt.Sprintf("test-webhook-secret-%v", i),
		)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to create database object (test-org-%v)", i))
		}
		orgs[name] = org
	}

	// setup test fixtures
	var maxRunners uint = 40
	var minIdleRunners uint = 20
	providerMock := runnerCommonMocks.NewProvider(s.T())
	fixtures := &OrgTestFixtures{
		AdminContext: adminCtx,
		DBFile:       dbCfg.SQLite.DBFile,
		Store:        db,
		StoreOrgs:    orgs,
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
		CreateOrgParams: params.CreateOrgParams{
			Name:            "test-org-create",
			CredentialsName: "test-creds",
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
		UpdateRepoParams: params.UpdateRepositoryParams{
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

func (s *OrgTestSuite) TestCreateOrganization() {
	// setup mocks expectations
	s.Fixtures.PoolMgrMock.On("Start").Return(nil)
	s.Fixtures.PoolMgrCtrlMock.On("CreateOrgPoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Organization"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, nil)

	// call tested function
	org, err := s.Runner.CreateOrganization(s.Fixtures.AdminContext, s.Fixtures.CreateOrgParams)

	// assertions
	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.CreateOrgParams.Name, org.Name)
	s.Require().Equal(s.Fixtures.Credentials[s.Fixtures.CreateOrgParams.CredentialsName].Name, org.CredentialsName)
}

func (s *OrgTestSuite) TestCreateOrganizationErrUnauthorized() {
	_, err := s.Runner.CreateOrganization(context.Background(), s.Fixtures.CreateOrgParams)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestCreateOrganizationEmptyParams() {
	_, err := s.Runner.CreateOrganization(s.Fixtures.AdminContext, params.CreateOrgParams{})

	s.Require().Regexp("validating params: missing org name", err.Error())
}

func (s *OrgTestSuite) TestCreateOrganizationMissingCredentials() {
	s.Fixtures.CreateOrgParams.CredentialsName = "not-existent-creds-name"

	_, err := s.Runner.CreateOrganization(s.Fixtures.AdminContext, s.Fixtures.CreateOrgParams)

	s.Require().Equal(runnerErrors.NewBadRequestError("credentials %s not defined", s.Fixtures.CreateOrgParams.CredentialsName), err)
}

func (s *OrgTestSuite) TestCreateOrganizationAlreadyExists() {
	s.Fixtures.CreateOrgParams.Name = "test-org-1" // this is already created in `SetupTest()`

	_, err := s.Runner.CreateOrganization(s.Fixtures.AdminContext, s.Fixtures.CreateOrgParams)

	s.Require().Equal(runnerErrors.NewConflictError("organization %s already exists", s.Fixtures.CreateOrgParams.Name), err)
}

func (s *OrgTestSuite) TestCreateOrganizationPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("CreateOrgPoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Organization"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.CreateOrganization(s.Fixtures.AdminContext, s.Fixtures.CreateOrgParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("creating org pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *OrgTestSuite) TestCreateOrganizationStartPoolMgrFailed() {
	s.Fixtures.PoolMgrMock.On("Start").Return(s.Fixtures.ErrMock)
	s.Fixtures.PoolMgrCtrlMock.On("CreateOrgPoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Organization"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrCtrlMock.On("DeleteOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.ErrMock)

	_, err := s.Runner.CreateOrganization(s.Fixtures.AdminContext, s.Fixtures.CreateOrgParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("starting org pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *OrgTestSuite) TestListOrganizations() {
	orgs, err := s.Runner.ListOrganizations(s.Fixtures.AdminContext)

	s.Require().Nil(err)
	s.equalOrgsByName(s.orgsMapValues(s.Fixtures.StoreOrgs), orgs)
}

func (s *OrgTestSuite) TestListOrganizationsErrUnauthorized() {
	_, err := s.Runner.ListOrganizations(context.Background())

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestGetOrganizationByID() {
	org, err := s.Runner.GetOrganizationByID(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.StoreOrgs["test-org-1"].ID, org.ID)
}

func (s *OrgTestSuite) TestGetOrganizationByIDErrUnauthorized() {
	_, err := s.Runner.GetOrganizationByID(context.Background(), "dummy-org-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestDeleteOrganization() {
	s.Fixtures.PoolMgrCtrlMock.On("DeleteOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(nil)

	err := s.Runner.DeleteOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-3"].ID)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)

	_, err = s.Fixtures.Store.GetOrganizationByID(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-3"].ID)
	s.Require().Equal("fetching org: not found", err.Error())
}

func (s *OrgTestSuite) TestDeleteOrganizationErrUnauthorized() {
	err := s.Runner.DeleteOrganization(context.Background(), "dummy-org-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestDeleteOrganizationPoolDefinedFailed() {
	pool, err := s.Fixtures.Store.CreateOrganizationPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create store organizations pool: %v", err))
	}

	err = s.Runner.DeleteOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID)

	s.Require().Equal(runnerErrors.NewBadRequestError("org has pools defined (%s)", pool.ID), err)
}

func (s *OrgTestSuite) TestDeleteOrganizationPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("DeleteOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.ErrMock)

	err := s.Runner.DeleteOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("deleting org pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *OrgTestSuite) TestUpdateOrganization() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrCtrlMock.On("CreateOrgPoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Organization"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, nil)

	org, err := s.Runner.UpdateOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.CredentialsName, org.CredentialsName)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.WebhookSecret, org.WebhookSecret)
}

func (s *OrgTestSuite) TestUpdateOrganizationErrUnauthorized() {
	_, err := s.Runner.UpdateOrganization(context.Background(), "dummy-org-id", s.Fixtures.UpdateRepoParams)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestUpdateOrganizationInvalidCreds() {
	s.Fixtures.UpdateRepoParams.CredentialsName = "invalid-creds-name"

	_, err := s.Runner.UpdateOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Require().Equal(runnerErrors.NewBadRequestError("invalid credentials (%s) for org %s", s.Fixtures.UpdateRepoParams.CredentialsName, s.Fixtures.StoreOrgs["test-org-1"].Name), err)
}

func (s *OrgTestSuite) TestUpdateOrganizationPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)
	s.Fixtures.PoolMgrMock.On("RefreshState", s.Fixtures.UpdatePoolStateParams).Return(s.Fixtures.ErrMock)

	_, err := s.Runner.UpdateOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("updating org pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *OrgTestSuite) TestUpdateOrganizationCreateOrgPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrCtrlMock.On("CreateOrgPoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Organization"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.UpdateOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("creating org pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *OrgTestSuite) TestCreateOrgPool() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, nil)

	pool, err := s.Runner.CreateOrgPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.CreatePoolParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)

	org, err := s.Fixtures.Store.GetOrganizationByID(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot get org by ID: %v", err))
	}
	s.Require().Equal(1, len(org.Pools))
	s.Require().Equal(pool.ID, org.Pools[0].ID)
	s.Require().Equal(s.Fixtures.CreatePoolParams.ProviderName, org.Pools[0].ProviderName)
	s.Require().Equal(s.Fixtures.CreatePoolParams.MaxRunners, org.Pools[0].MaxRunners)
	s.Require().Equal(s.Fixtures.CreatePoolParams.MinIdleRunners, org.Pools[0].MinIdleRunners)
}

func (s *OrgTestSuite) TestCreateOrgPoolErrUnauthorized() {
	_, err := s.Runner.CreateOrgPool(context.Background(), "dummy-org-id", s.Fixtures.CreatePoolParams)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestCreateOrgPoolErrNotFound() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, runnerErrors.ErrNotFound)

	_, err := s.Runner.CreateOrgPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.CreatePoolParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(runnerErrors.ErrNotFound, err)
}

func (s *OrgTestSuite) TestCreateOrgPoolFetchPoolParamsFailed() {
	s.Fixtures.CreatePoolParams.ProviderName = "not-existent-provider-name"

	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, nil)

	_, err := s.Runner.CreateOrgPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.CreatePoolParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Regexp("fetching pool params: no such provider", err.Error())
}

func (s *OrgTestSuite) TestGetOrgPoolByID() {
	orgPool, err := s.Fixtures.Store.CreateOrganizationPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %s", err))
	}

	pool, err := s.Runner.GetOrgPoolByID(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, orgPool.ID)

	s.Require().Nil(err)
	s.Require().Equal(orgPool.ID, pool.ID)
}

func (s *OrgTestSuite) TestGetOrgPoolByIDErrUnauthorized() {
	_, err := s.Runner.GetOrgPoolByID(context.Background(), "dummy-org-id", "dummy-pool-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestDeleteOrgPool() {
	pool, err := s.Fixtures.Store.CreateOrganizationPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %s", err))
	}

	err = s.Runner.DeleteOrgPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, pool.ID)

	s.Require().Nil(err)

	_, err = s.Fixtures.Store.GetOrganizationPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, pool.ID)
	s.Require().Equal("fetching pool: not found", err.Error())
}

func (s *OrgTestSuite) TestDeleteOrgPoolErrUnauthorized() {
	err := s.Runner.DeleteOrgPool(context.Background(), "dummy-org-id", "dummy-pool-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestDeleteOrgPoolRunnersFailed() {
	pool, err := s.Fixtures.Store.CreateOrganizationPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
	}
	s.Fixtures.CreateInstanceParams.Pool = pool.ID
	instance, err := s.Fixtures.Store.CreateInstance(s.Fixtures.AdminContext, pool.ID, s.Fixtures.CreateInstanceParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create instance: %s", err))
	}

	err = s.Runner.DeleteOrgPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, pool.ID)

	s.Require().Equal(runnerErrors.NewBadRequestError("pool has runners: %s", instance.ID), err)
}

func (s *OrgTestSuite) TestListOrgPools() {
	orgPools := []params.Pool{}
	for i := 1; i <= 2; i++ {
		s.Fixtures.CreatePoolParams.Image = fmt.Sprintf("test-org-%v", i)
		pool, err := s.Fixtures.Store.CreateOrganizationPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.CreatePoolParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
		}
		orgPools = append(orgPools, pool)
	}

	pools, err := s.Runner.ListOrgPools(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID)

	s.Require().Nil(err)
	s.equalPoolsByID(orgPools, pools)
}

func (s *OrgTestSuite) TestListOrgPoolsErrUnauthorized() {
	_, err := s.Runner.ListOrgPools(context.Background(), "dummy-org-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestUpdateOrgPool() {
	orgPool, err := s.Fixtures.Store.CreateOrganizationPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %s", err))
	}

	pool, err := s.Runner.UpdateOrgPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, orgPool.ID, s.Fixtures.UpdatePoolParams)

	s.Require().Nil(err)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MaxRunners, pool.MaxRunners)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MinIdleRunners, pool.MinIdleRunners)
}

func (s *OrgTestSuite) TestUpdateOrgPoolErrUnauthorized() {
	_, err := s.Runner.UpdateOrgPool(context.Background(), "dummy-org-id", "dummy-pool-id", s.Fixtures.UpdatePoolParams)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestUpdateOrgPoolMinIdleGreaterThanMax() {
	pool, err := s.Fixtures.Store.CreateOrganizationPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %s", err))
	}
	var maxRunners uint = 10
	var minIdleRunners uint = 11
	s.Fixtures.UpdatePoolParams.MaxRunners = &maxRunners
	s.Fixtures.UpdatePoolParams.MinIdleRunners = &minIdleRunners

	_, err = s.Runner.UpdateOrgPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, pool.ID, s.Fixtures.UpdatePoolParams)

	s.Require().Equal(runnerErrors.NewBadRequestError("min_idle_runners cannot be larger than max_runners"), err)
}

func (s *OrgTestSuite) TestListOrgInstances() {
	pool, err := s.Fixtures.Store.CreateOrganizationPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
	}
	poolInstances := []params.Instance{}
	s.Fixtures.CreateInstanceParams.Pool = pool.ID
	for i := 1; i <= 3; i++ {
		s.Fixtures.CreateInstanceParams.Name = fmt.Sprintf("test-org-%v", i)
		instance, err := s.Fixtures.Store.CreateInstance(s.Fixtures.AdminContext, pool.ID, s.Fixtures.CreateInstanceParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create instance: %s", err))
		}
		poolInstances = append(poolInstances, instance)
	}

	instances, err := s.Runner.ListOrgInstances(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID)

	s.Require().Nil(err)
	s.equalInstancesByName(poolInstances, instances)
}

func (s *OrgTestSuite) TestListOrgInstancesErrUnauthorized() {
	_, err := s.Runner.ListOrgInstances(context.Background(), "dummy-org-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestFindOrgPoolManager() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, nil)

	poolManager, err := s.Runner.findOrgPoolManager(s.Fixtures.StoreOrgs["test-org-1"].Name)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.PoolMgrMock, poolManager)
}

func (s *OrgTestSuite) TestFindOrgPoolManagerFetchPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.findOrgPoolManager(s.Fixtures.StoreOrgs["test-org-1"].Name)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Regexp("fetching pool manager for org", err.Error())
}

func TestOrgTestSuite(t *testing.T) {
	suite.Run(t, new(OrgTestSuite))
}
