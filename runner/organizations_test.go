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
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	runnerCommonMocks "github.com/cloudbase/garm/runner/common/mocks"
	runnerMocks "github.com/cloudbase/garm/runner/mocks"
)

type OrgTestFixtures struct {
	AdminContext         context.Context
	DBFile               string
	Store                dbCommon.Store
	StoreOrgs            map[string]params.Organization
	Providers            map[string]common.Provider
	Credentials          map[string]params.ForgeCredentials
	CreateOrgParams      params.CreateOrgParams
	CreatePoolParams     params.CreatePoolParams
	CreateInstanceParams params.CreateInstanceParams
	UpdateRepoParams     params.UpdateEntityParams
	UpdatePoolParams     params.UpdatePoolParams
	ErrMock              error
	ProviderMock         *runnerCommonMocks.Provider
	PoolMgrMock          *runnerCommonMocks.PoolManager
	PoolMgrCtrlMock      *runnerMocks.PoolManagerController
}

type OrgTestSuite struct {
	suite.Suite
	Fixtures *OrgTestFixtures
	Runner   *Runner

	testCreds          params.ForgeCredentials
	secondaryTestCreds params.ForgeCredentials
	giteaTestCreds     params.ForgeCredentials
	githubEndpoint     params.ForgeEndpoint
	giteaEndpoint      params.ForgeEndpoint
}

func (s *OrgTestSuite) SetupTest() {
	// create testing sqlite database
	dbCfg := garmTesting.GetTestSqliteDBConfig(s.T())
	db, err := database.NewDatabase(context.Background(), dbCfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}

	adminCtx := garmTesting.ImpersonateAdminContext(context.Background(), db, s.T())

	s.githubEndpoint = garmTesting.CreateDefaultGithubEndpoint(adminCtx, db, s.T())
	s.giteaEndpoint = garmTesting.CreateDefaultGiteaEndpoint(adminCtx, db, s.T())
	s.testCreds = garmTesting.CreateTestGithubCredentials(adminCtx, "new-creds", db, s.T(), s.githubEndpoint)
	s.giteaTestCreds = garmTesting.CreateTestGiteaCredentials(adminCtx, "gitea-creds", db, s.T(), s.giteaEndpoint)
	s.secondaryTestCreds = garmTesting.CreateTestGithubCredentials(adminCtx, "secondary-creds", db, s.T(), s.githubEndpoint)

	// create some organization objects in the database, for testing purposes
	orgs := map[string]params.Organization{}
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("test-org-%v", i)
		org, err := db.CreateOrganization(
			adminCtx,
			name,
			s.testCreds,
			fmt.Sprintf("test-webhook-secret-%v", i),
			params.PoolBalancerTypeRoundRobin,
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
		Credentials: map[string]params.ForgeCredentials{
			s.testCreds.Name:          s.testCreds,
			s.secondaryTestCreds.Name: s.secondaryTestCreds,
		},
		CreateOrgParams: params.CreateOrgParams{
			Name:            "test-org-create",
			CredentialsName: s.testCreds.Name,
			WebhookSecret:   "test-create-org-webhook-secret",
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
	s.Require().Equal(s.Fixtures.Credentials[s.Fixtures.CreateOrgParams.CredentialsName].Name, org.Credentials.Name)
	s.Require().Equal(params.PoolBalancerTypeRoundRobin, org.PoolBalancerType)
}

func (s *OrgTestSuite) TestCreateOrganizationPoolBalancerTypePack() {
	s.Fixtures.CreateOrgParams.PoolBalancerType = params.PoolBalancerTypePack
	s.Fixtures.PoolMgrMock.On("Start").Return(nil)
	s.Fixtures.PoolMgrCtrlMock.On("CreateOrgPoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Organization"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, nil)

	org, err := s.Runner.CreateOrganization(s.Fixtures.AdminContext, s.Fixtures.CreateOrgParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(params.PoolBalancerTypePack, org.PoolBalancerType)
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
	s.Fixtures.CreateOrgParams.CredentialsName = notExistingCredentialsName

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
	s.Require().Equal(fmt.Sprintf("error creating org pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *OrgTestSuite) TestCreateOrganizationStartPoolMgrFailed() {
	s.Fixtures.PoolMgrMock.On("Start").Return(s.Fixtures.ErrMock)
	s.Fixtures.PoolMgrCtrlMock.On("CreateOrgPoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Organization"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrCtrlMock.On("DeleteOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.ErrMock)

	_, err := s.Runner.CreateOrganization(s.Fixtures.AdminContext, s.Fixtures.CreateOrgParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("error starting org pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *OrgTestSuite) TestListOrganizations() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)
	orgs, err := s.Runner.ListOrganizations(s.Fixtures.AdminContext, params.OrganizationFilter{})

	s.Require().Nil(err)
	garmTesting.EqualDBEntityByName(s.T(), garmTesting.DBEntityMapToSlice(s.Fixtures.StoreOrgs), orgs)
}

func (s *OrgTestSuite) TestListOrganizationsWithFilter() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)

	org, err := s.Fixtures.Store.CreateOrganization(
		s.Fixtures.AdminContext,
		"test-org",
		s.testCreds,
		"super-secret",
		params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)

	org2, err := s.Fixtures.Store.CreateOrganization(
		s.Fixtures.AdminContext,
		"test-org",
		s.giteaTestCreds,
		"super-secret",
		params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)

	org3, err := s.Fixtures.Store.CreateOrganization(
		s.Fixtures.AdminContext,
		"test-org2",
		s.giteaTestCreds,
		"super-secret",
		params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)

	orgs, err := s.Runner.ListOrganizations(
		s.Fixtures.AdminContext,
		params.OrganizationFilter{
			Name: "test-org",
		},
	)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityByName(s.T(), []params.Organization{org, org2}, orgs)

	orgs, err = s.Runner.ListOrganizations(
		s.Fixtures.AdminContext,
		params.OrganizationFilter{
			Name:     "test-org",
			Endpoint: s.giteaEndpoint.Name,
		},
	)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityByName(s.T(), []params.Organization{org2}, orgs)

	orgs, err = s.Runner.ListOrganizations(
		s.Fixtures.AdminContext,
		params.OrganizationFilter{
			Name: "test-org2",
		},
	)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityByName(s.T(), []params.Organization{org3}, orgs)
}

func (s *OrgTestSuite) TestListOrganizationsErrUnauthorized() {
	_, err := s.Runner.ListOrganizations(context.Background(), params.OrganizationFilter{})

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestGetOrganizationByID() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)
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

	err := s.Runner.DeleteOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-3"].ID, true)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)

	_, err = s.Fixtures.Store.GetOrganizationByID(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-3"].ID)
	s.Require().Equal("error fetching org: not found", err.Error())
}

func (s *OrgTestSuite) TestDeleteOrganizationErrUnauthorized() {
	err := s.Runner.DeleteOrganization(context.Background(), "dummy-org-id", true)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestDeleteOrganizationPoolDefinedFailed() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreOrgs["test-org-1"].ID,
		EntityType: params.ForgeEntityTypeOrganization,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create store organizations pool: %v", err))
	}

	err = s.Runner.DeleteOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, true)

	s.Require().Equal(runnerErrors.NewBadRequestError("org has pools defined (%s)", pool.ID), err)
}

func (s *OrgTestSuite) TestDeleteOrganizationPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("DeleteOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.ErrMock)

	err := s.Runner.DeleteOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, true)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("error deleting org pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *OrgTestSuite) TestUpdateOrganization() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)

	org, err := s.Runner.UpdateOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.CredentialsName, org.Credentials.Name)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.WebhookSecret, org.WebhookSecret)
}

func (s *OrgTestSuite) TestUpdateRepositoryBalancingType() {
	s.Fixtures.UpdateRepoParams.PoolBalancerType = params.PoolBalancerTypePack
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)

	param := s.Fixtures.UpdateRepoParams
	param.PoolBalancerType = params.PoolBalancerTypePack
	org, err := s.Runner.UpdateOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, param)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(params.PoolBalancerTypePack, org.PoolBalancerType)
}

func (s *OrgTestSuite) TestUpdateOrganizationErrUnauthorized() {
	_, err := s.Runner.UpdateOrganization(context.Background(), "dummy-org-id", s.Fixtures.UpdateRepoParams)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestUpdateOrganizationInvalidCreds() {
	s.Fixtures.UpdateRepoParams.CredentialsName = invalidCredentialsName

	_, err := s.Runner.UpdateOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.UpdateRepoParams)
	if !errors.Is(err, runnerErrors.ErrNotFound) {
		s.FailNow(fmt.Sprintf("expected error: %v", runnerErrors.ErrNotFound))
	}
}

func (s *OrgTestSuite) TestUpdateOrganizationPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.UpdateOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("failed to get org pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *OrgTestSuite) TestUpdateOrganizationCreateOrgPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.UpdateOrganization(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("failed to get org pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *OrgTestSuite) TestCreateOrgPool() {
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

func (s *OrgTestSuite) TestCreateOrgPoolFetchPoolParamsFailed() {
	s.Fixtures.CreatePoolParams.ProviderName = notExistingProviderName
	_, err := s.Runner.CreateOrgPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, s.Fixtures.CreatePoolParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Regexp("fetching pool params: no such provider", err.Error())
}

func (s *OrgTestSuite) TestGetOrgPoolByID() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreOrgs["test-org-1"].ID,
		EntityType: params.ForgeEntityTypeOrganization,
	}
	orgPool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
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
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreOrgs["test-org-1"].ID,
		EntityType: params.ForgeEntityTypeOrganization,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %s", err))
	}

	err = s.Runner.DeleteOrgPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, pool.ID)

	s.Require().Nil(err)

	_, err = s.Fixtures.Store.GetEntityPool(s.Fixtures.AdminContext, entity, pool.ID)
	s.Require().Equal("fetching pool: error finding pool: not found", err.Error())
}

func (s *OrgTestSuite) TestDeleteOrgPoolErrUnauthorized() {
	err := s.Runner.DeleteOrgPool(context.Background(), "dummy-org-id", "dummy-pool-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestDeleteOrgPoolRunnersFailed() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreOrgs["test-org-1"].ID,
		EntityType: params.ForgeEntityTypeOrganization,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
	}
	instance, err := s.Fixtures.Store.CreateInstance(s.Fixtures.AdminContext, pool.ID, s.Fixtures.CreateInstanceParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create instance: %s", err))
	}

	err = s.Runner.DeleteOrgPool(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID, pool.ID)

	s.Require().Equal(runnerErrors.NewBadRequestError("pool has runners: %s", instance.ID), err)
}

func (s *OrgTestSuite) TestListOrgPools() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreOrgs["test-org-1"].ID,
		EntityType: params.ForgeEntityTypeOrganization,
	}
	orgPools := []params.Pool{}
	for i := 1; i <= 2; i++ {
		s.Fixtures.CreatePoolParams.Image = fmt.Sprintf("test-org-%v", i)
		pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
		}
		orgPools = append(orgPools, pool)
	}

	pools, err := s.Runner.ListOrgPools(s.Fixtures.AdminContext, s.Fixtures.StoreOrgs["test-org-1"].ID)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityID(s.T(), orgPools, pools)
}

func (s *OrgTestSuite) TestListOrgPoolsErrUnauthorized() {
	_, err := s.Runner.ListOrgPools(context.Background(), "dummy-org-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestUpdateOrgPool() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreOrgs["test-org-1"].ID,
		EntityType: params.ForgeEntityTypeOrganization,
	}
	orgPool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
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
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreOrgs["test-org-1"].ID,
		EntityType: params.ForgeEntityTypeOrganization,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
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
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreOrgs["test-org-1"].ID,
		EntityType: params.ForgeEntityTypeOrganization,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
	}
	poolInstances := []params.Instance{}
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
	garmTesting.EqualDBEntityID(s.T(), poolInstances, instances)
}

func (s *OrgTestSuite) TestListOrgInstancesErrUnauthorized() {
	_, err := s.Runner.ListOrgInstances(context.Background(), "dummy-org-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *OrgTestSuite) TestFindOrgPoolManager() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, nil)

	poolManager, err := s.Runner.findOrgPoolManager(s.Fixtures.StoreOrgs["test-org-1"].Name, s.Fixtures.StoreOrgs["test-org-1"].Endpoint.Name)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.PoolMgrMock, poolManager)
}

func (s *OrgTestSuite) TestFindOrgPoolManagerFetchPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("GetOrgPoolManager", mock.AnythingOfType("params.Organization")).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.findOrgPoolManager(s.Fixtures.StoreOrgs["test-org-1"].Name, s.Fixtures.StoreOrgs["test-org-1"].Endpoint.Name)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Regexp("fetching pool manager for org", err.Error())
}

func TestOrgTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(OrgTestSuite))
}
