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

	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	runnerCommonMocks "github.com/cloudbase/garm/runner/common/mocks"
	runnerMocks "github.com/cloudbase/garm/runner/mocks"
)

type RepoTestFixtures struct {
	AdminContext         context.Context
	Store                dbCommon.Store
	StoreRepos           map[string]params.Repository
	Providers            map[string]common.Provider
	Credentials          map[string]params.ForgeCredentials
	CreateRepoParams     params.CreateRepoParams
	CreatePoolParams     params.CreatePoolParams
	CreateInstanceParams params.CreateInstanceParams
	UpdateRepoParams     params.UpdateEntityParams
	UpdatePoolParams     params.UpdatePoolParams
	ErrMock              error
	ProviderMock         *runnerCommonMocks.Provider
	PoolMgrMock          *runnerCommonMocks.PoolManager
	PoolMgrCtrlMock      *runnerMocks.PoolManagerController
}

func init() {
	watcher.SetWatcher(&garmTesting.MockWatcher{})
}

type RepoTestSuite struct {
	suite.Suite
	Fixtures *RepoTestFixtures
	Runner   *Runner

	testCreds          params.ForgeCredentials
	secondaryTestCreds params.ForgeCredentials
	githubEndpoint     params.ForgeEndpoint
}

func (s *RepoTestSuite) SetupTest() {
	// create testing sqlite database
	dbCfg := garmTesting.GetTestSqliteDBConfig(s.T())
	db, err := database.NewDatabase(context.Background(), dbCfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}

	adminCtx := garmTesting.ImpersonateAdminContext(context.Background(), db, s.T())
	s.githubEndpoint = garmTesting.CreateDefaultGithubEndpoint(adminCtx, db, s.T())
	s.testCreds = garmTesting.CreateTestGithubCredentials(adminCtx, "new-creds", db, s.T(), s.githubEndpoint)
	s.secondaryTestCreds = garmTesting.CreateTestGithubCredentials(adminCtx, "secondary-creds", db, s.T(), s.githubEndpoint)

	// create some repository objects in the database, for testing purposes
	repos := map[string]params.Repository{}
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("test-repo-%v", i)
		repo, err := db.CreateRepository(
			adminCtx,
			fmt.Sprintf("test-owner-%v", i),
			name,
			s.testCreds,
			fmt.Sprintf("test-webhook-secret-%v", i),
			params.PoolBalancerTypeRoundRobin,
		)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to create database object (test-repo-%v): %q", i, err))
		}
		repos[name] = repo
	}

	// setup test fixtures
	var maxRunners uint = 40
	var minIdleRunners uint = 20
	providerMock := runnerCommonMocks.NewProvider(s.T())
	fixtures := &RepoTestFixtures{
		AdminContext: adminCtx,
		Store:        db,
		StoreRepos:   repos,
		Providers: map[string]common.Provider{
			"test-provider": providerMock,
		},
		Credentials: map[string]params.ForgeCredentials{
			s.testCreds.Name:          s.testCreds,
			s.secondaryTestCreds.Name: s.secondaryTestCreds,
		},
		CreateRepoParams: params.CreateRepoParams{
			Owner:           "test-owner-create",
			Name:            "test-repo-create",
			CredentialsName: s.testCreds.Name,
			WebhookSecret:   "test-create-repo-webhook-secret",
			ForgeType:       params.GithubEndpointType,
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

func (s *RepoTestSuite) TestCreateRepository() {
	// setup mocks expectations
	s.Fixtures.PoolMgrMock.On("Start").Return(nil)
	s.Fixtures.PoolMgrCtrlMock.On("CreateRepoPoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Repository"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, nil)

	// call tested function
	repo, err := s.Runner.CreateRepository(s.Fixtures.AdminContext, s.Fixtures.CreateRepoParams)

	// assertions
	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.CreateRepoParams.Owner, repo.Owner)
	s.Require().Equal(s.Fixtures.CreateRepoParams.Name, repo.Name)
	s.Require().Equal(s.Fixtures.Credentials[s.Fixtures.CreateRepoParams.CredentialsName].Name, repo.Credentials.Name)
	s.Require().Equal(params.PoolBalancerTypeRoundRobin, repo.PoolBalancerType)
}

func (s *RepoTestSuite) TestCreateRepositoryPoolBalancerTypePack() {
	// setup mocks expectations
	s.Fixtures.PoolMgrMock.On("Start").Return(nil)
	s.Fixtures.PoolMgrCtrlMock.On("CreateRepoPoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Repository"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, nil)

	// call tested function
	param := s.Fixtures.CreateRepoParams
	param.PoolBalancerType = params.PoolBalancerTypePack
	repo, err := s.Runner.CreateRepository(s.Fixtures.AdminContext, param)

	// assertions
	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(param.Owner, repo.Owner)
	s.Require().Equal(param.Name, repo.Name)
	s.Require().Equal(s.Fixtures.Credentials[s.Fixtures.CreateRepoParams.CredentialsName].Name, repo.Credentials.Name)
	s.Require().Equal(params.PoolBalancerTypePack, repo.PoolBalancerType)
}

func (s *RepoTestSuite) TestCreateRepositoryErrUnauthorized() {
	_, err := s.Runner.CreateRepository(context.Background(), s.Fixtures.CreateRepoParams)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *RepoTestSuite) TestCreateRepositoryEmptyParams() {
	_, err := s.Runner.CreateRepository(s.Fixtures.AdminContext, params.CreateRepoParams{})

	s.Require().Regexp("validating params: missing owner", err.Error())
}

func (s *RepoTestSuite) TestCreateRepositoryMissingCredentials() {
	s.Fixtures.CreateRepoParams.CredentialsName = notExistingCredentialsName

	_, err := s.Runner.CreateRepository(s.Fixtures.AdminContext, s.Fixtures.CreateRepoParams)

	s.Require().Equal(runnerErrors.NewBadRequestError("credentials %s not defined", s.Fixtures.CreateRepoParams.CredentialsName), err)
}

func (s *RepoTestSuite) TestCreateRepositoryAlreadyExists() {
	// this is already created in `SetupTest()`
	s.Fixtures.CreateRepoParams.Owner = "test-owner-1"
	s.Fixtures.CreateRepoParams.Name = "test-repo-1"

	_, err := s.Runner.CreateRepository(s.Fixtures.AdminContext, s.Fixtures.CreateRepoParams)

	s.Require().Equal(runnerErrors.NewConflictError("repository %s/%s already exists", s.Fixtures.CreateRepoParams.Owner, s.Fixtures.CreateRepoParams.Name), err)
}

func (s *RepoTestSuite) TestCreateRepositoryPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("CreateRepoPoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Repository"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.CreateRepository(s.Fixtures.AdminContext, s.Fixtures.CreateRepoParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("creating repo pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *RepoTestSuite) TestCreateRepositoryStartPoolMgrFailed() {
	s.Fixtures.PoolMgrMock.On("Start").Return(s.Fixtures.ErrMock)
	s.Fixtures.PoolMgrCtrlMock.On("CreateRepoPoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.Repository"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrCtrlMock.On("DeleteRepoPoolManager", mock.AnythingOfType("params.Repository")).Return(s.Fixtures.ErrMock)

	_, err := s.Runner.CreateRepository(s.Fixtures.AdminContext, s.Fixtures.CreateRepoParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("starting repo pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *RepoTestSuite) TestListRepositories() {
	s.Fixtures.PoolMgrCtrlMock.On("GetRepoPoolManager", mock.AnythingOfType("params.Repository")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)
	repos, err := s.Runner.ListRepositories(s.Fixtures.AdminContext)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityByName(s.T(), garmTesting.DBEntityMapToSlice(s.Fixtures.StoreRepos), repos)
}

func (s *RepoTestSuite) TestListRepositoriesErrUnauthorized() {
	_, err := s.Runner.ListRepositories(context.Background())

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *RepoTestSuite) TestGetRepositoryByID() {
	s.Fixtures.PoolMgrCtrlMock.On("GetRepoPoolManager", mock.AnythingOfType("params.Repository")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)
	repo, err := s.Runner.GetRepositoryByID(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.StoreRepos["test-repo-1"].ID, repo.ID)
}

func (s *RepoTestSuite) TestGetRepositoryByIDErrUnauthorized() {
	_, err := s.Runner.GetRepositoryByID(context.Background(), "dummy-repo-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *RepoTestSuite) TestDeleteRepository() {
	s.Fixtures.PoolMgrCtrlMock.On("DeleteRepoPoolManager", mock.AnythingOfType("params.Repository")).Return(nil)

	err := s.Runner.DeleteRepository(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, true)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)

	_, err = s.Fixtures.Store.GetRepositoryByID(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID)
	s.Require().Equal("fetching repo: not found", err.Error())
}

func (s *RepoTestSuite) TestDeleteRepositoryErrUnauthorized() {
	err := s.Runner.DeleteRepository(context.Background(), "dummy-repo-id", true)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *RepoTestSuite) TestDeleteRepositoryPoolDefinedFailed() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreRepos["test-repo-1"].ID,
		EntityType: params.ForgeEntityTypeRepository,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create store repositories pool: %v", err))
	}

	err = s.Runner.DeleteRepository(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, true)

	s.Require().Equal(runnerErrors.NewBadRequestError("repo has pools defined (%s)", pool.ID), err)
}

func (s *RepoTestSuite) TestDeleteRepositoryPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("DeleteRepoPoolManager", mock.AnythingOfType("params.Repository")).Return(s.Fixtures.ErrMock)

	err := s.Runner.DeleteRepository(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, true)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("deleting repo pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *RepoTestSuite) TestUpdateRepository() {
	s.Fixtures.PoolMgrCtrlMock.On("GetRepoPoolManager", mock.AnythingOfType("params.Repository")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)

	repo, err := s.Runner.UpdateRepository(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.CredentialsName, repo.Credentials.Name)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.WebhookSecret, repo.WebhookSecret)
	s.Require().Equal(params.PoolBalancerTypeRoundRobin, repo.PoolBalancerType)
}

func (s *RepoTestSuite) TestUpdateRepositoryBalancingType() {
	s.Fixtures.PoolMgrCtrlMock.On("GetRepoPoolManager", mock.AnythingOfType("params.Repository")).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrMock.On("Status").Return(params.PoolManagerStatus{IsRunning: true}, nil)

	updateRepoParams := s.Fixtures.UpdateRepoParams
	updateRepoParams.PoolBalancerType = params.PoolBalancerTypePack
	repo, err := s.Runner.UpdateRepository(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, updateRepoParams)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(updateRepoParams.CredentialsName, repo.Credentials.Name)
	s.Require().Equal(updateRepoParams.WebhookSecret, repo.WebhookSecret)
	s.Require().Equal(params.PoolBalancerTypePack, repo.PoolBalancerType)
}

func (s *RepoTestSuite) TestUpdateRepositoryErrUnauthorized() {
	_, err := s.Runner.UpdateRepository(context.Background(), "dummy-repo-id", s.Fixtures.UpdateRepoParams)
	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *RepoTestSuite) TestUpdateRepositoryInvalidCreds() {
	s.Fixtures.UpdateRepoParams.CredentialsName = invalidCredentialsName

	_, err := s.Runner.UpdateRepository(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, s.Fixtures.UpdateRepoParams)

	if !errors.Is(err, runnerErrors.ErrNotFound) {
		s.FailNow(fmt.Sprintf("expected error: %v", runnerErrors.ErrNotFound))
	}
}

func (s *RepoTestSuite) TestUpdateRepositoryPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("GetRepoPoolManager", mock.AnythingOfType("params.Repository")).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.UpdateRepository(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("getting pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *RepoTestSuite) TestUpdateRepositoryCreateRepoPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("GetRepoPoolManager", mock.AnythingOfType("params.Repository")).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.UpdateRepository(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, s.Fixtures.UpdateRepoParams)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Equal(fmt.Sprintf("getting pool manager: %s", s.Fixtures.ErrMock.Error()), err.Error())
}

func (s *RepoTestSuite) TestCreateRepoPool() {
	pool, err := s.Runner.CreateRepoPool(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, s.Fixtures.CreatePoolParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)

	repo, err := s.Fixtures.Store.GetRepositoryByID(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot get repo by ID: %v", err))
	}
	s.Require().Equal(1, len(repo.Pools))
	s.Require().Equal(pool.ID, repo.Pools[0].ID)
	s.Require().Equal(s.Fixtures.CreatePoolParams.ProviderName, repo.Pools[0].ProviderName)
	s.Require().Equal(s.Fixtures.CreatePoolParams.MaxRunners, repo.Pools[0].MaxRunners)
	s.Require().Equal(s.Fixtures.CreatePoolParams.MinIdleRunners, repo.Pools[0].MinIdleRunners)
}

func (s *RepoTestSuite) TestCreateRepoPoolErrUnauthorized() {
	_, err := s.Runner.CreateRepoPool(context.Background(), "dummy-repo-id", s.Fixtures.CreatePoolParams)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *RepoTestSuite) TestCreateRepoPoolFetchPoolParamsFailed() {
	s.Fixtures.CreatePoolParams.ProviderName = notExistingProviderName
	_, err := s.Runner.CreateRepoPool(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, s.Fixtures.CreatePoolParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Regexp("appending tags to create pool params: no such provider not-existent-provider-name", err.Error())
}

func (s *RepoTestSuite) TestGetRepoPoolByID() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreRepos["test-repo-1"].ID,
		EntityType: params.ForgeEntityTypeRepository,
	}
	repoPool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %s", err))
	}

	pool, err := s.Runner.GetRepoPoolByID(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, repoPool.ID)

	s.Require().Nil(err)
	s.Require().Equal(repoPool.ID, pool.ID)
}

func (s *RepoTestSuite) TestGetRepoPoolByIDErrUnauthorized() {
	_, err := s.Runner.GetRepoPoolByID(context.Background(), "dummy-repo-id", "dummy-pool-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *RepoTestSuite) TestDeleteRepoPool() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreRepos["test-repo-1"].ID,
		EntityType: params.ForgeEntityTypeRepository,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %s", err))
	}

	err = s.Runner.DeleteRepoPool(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, pool.ID)

	s.Require().Nil(err)

	_, err = s.Fixtures.Store.GetEntityPool(s.Fixtures.AdminContext, entity, pool.ID)
	s.Require().Equal("fetching pool: finding pool: not found", err.Error())
}

func (s *RepoTestSuite) TestDeleteRepoPoolErrUnauthorized() {
	err := s.Runner.DeleteRepoPool(context.Background(), "dummy-repo-id", "dummy-pool-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *RepoTestSuite) TestDeleteRepoPoolRunnersFailed() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreRepos["test-repo-1"].ID,
		EntityType: params.ForgeEntityTypeRepository,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %s", err))
	}
	instance, err := s.Fixtures.Store.CreateInstance(s.Fixtures.AdminContext, pool.ID, s.Fixtures.CreateInstanceParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create instance: %s", err))
	}

	err = s.Runner.DeleteRepoPool(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, pool.ID)

	s.Require().Equal(runnerErrors.NewBadRequestError("pool has runners: %s", instance.ID), err)
}

func (s *RepoTestSuite) TestListRepoPools() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreRepos["test-repo-1"].ID,
		EntityType: params.ForgeEntityTypeRepository,
	}
	repoPools := []params.Pool{}
	for i := 1; i <= 2; i++ {
		s.Fixtures.CreatePoolParams.Image = fmt.Sprintf("test-repo-%v", i)
		pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
		}
		repoPools = append(repoPools, pool)
	}

	pools, err := s.Runner.ListRepoPools(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityID(s.T(), repoPools, pools)
}

func (s *RepoTestSuite) TestListRepoPoolsErrUnauthorized() {
	_, err := s.Runner.ListRepoPools(context.Background(), "dummy-repo-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *RepoTestSuite) TestListPoolInstances() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreRepos["test-repo-1"].ID,
		EntityType: params.ForgeEntityTypeRepository,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
	}
	poolInstances := []params.Instance{}
	for i := 1; i <= 3; i++ {
		s.Fixtures.CreateInstanceParams.Name = fmt.Sprintf("test-repo-%v", i)
		instance, err := s.Fixtures.Store.CreateInstance(s.Fixtures.AdminContext, pool.ID, s.Fixtures.CreateInstanceParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create instance: %s", err))
		}
		poolInstances = append(poolInstances, instance)
	}

	instances, err := s.Runner.ListPoolInstances(s.Fixtures.AdminContext, pool.ID)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityID(s.T(), poolInstances, instances)
}

func (s *RepoTestSuite) TestListPoolInstancesErrUnauthorized() {
	_, err := s.Runner.ListPoolInstances(context.Background(), "dummy-pool-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *RepoTestSuite) TestUpdateRepoPool() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreRepos["test-repo-1"].ID,
		EntityType: params.ForgeEntityTypeRepository,
	}
	repoPool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create store repositories pool: %v", err))
	}

	pool, err := s.Runner.UpdateRepoPool(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, repoPool.ID, s.Fixtures.UpdatePoolParams)

	s.Require().Nil(err)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MaxRunners, pool.MaxRunners)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MinIdleRunners, pool.MinIdleRunners)
}

func (s *RepoTestSuite) TestUpdateRepoPoolErrUnauthorized() {
	_, err := s.Runner.UpdateRepoPool(context.Background(), "dummy-repo-id", "dummy-pool-id", s.Fixtures.UpdatePoolParams)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *RepoTestSuite) TestUpdateRepoPoolMinIdleGreaterThanMax() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreRepos["test-repo-1"].ID,
		EntityType: params.ForgeEntityTypeRepository,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %s", err))
	}
	var maxRunners uint = 10
	var minIdleRunners uint = 11
	s.Fixtures.UpdatePoolParams.MaxRunners = &maxRunners
	s.Fixtures.UpdatePoolParams.MinIdleRunners = &minIdleRunners

	_, err = s.Runner.UpdateRepoPool(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID, pool.ID, s.Fixtures.UpdatePoolParams)

	s.Require().Equal(runnerErrors.NewBadRequestError("min_idle_runners cannot be larger than max_runners"), err)
}

func (s *RepoTestSuite) TestListRepoInstances() {
	entity := params.ForgeEntity{
		ID:         s.Fixtures.StoreRepos["test-repo-1"].ID,
		EntityType: params.ForgeEntityTypeRepository,
	}
	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
	}
	poolInstances := []params.Instance{}
	for i := 1; i <= 3; i++ {
		s.Fixtures.CreateInstanceParams.Name = fmt.Sprintf("test-repo-%v", i)
		instance, err := s.Fixtures.Store.CreateInstance(s.Fixtures.AdminContext, pool.ID, s.Fixtures.CreateInstanceParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create instance: %s", err))
		}
		poolInstances = append(poolInstances, instance)
	}

	instances, err := s.Runner.ListRepoInstances(s.Fixtures.AdminContext, s.Fixtures.StoreRepos["test-repo-1"].ID)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityID(s.T(), poolInstances, instances)
}

func (s *RepoTestSuite) TestListRepoInstancesErrUnauthorized() {
	_, err := s.Runner.ListRepoInstances(context.Background(), "dummy-repo-id")

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *RepoTestSuite) TestFindRepoPoolManager() {
	s.Fixtures.PoolMgrCtrlMock.On("GetRepoPoolManager", mock.AnythingOfType("params.Repository")).Return(s.Fixtures.PoolMgrMock, nil)

	poolManager, err := s.Runner.findRepoPoolManager(s.Fixtures.StoreRepos["test-repo-1"].Owner, s.Fixtures.StoreRepos["test-repo-1"].Name, s.Fixtures.StoreRepos["test-repo-1"].Endpoint.Name)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.PoolMgrMock, poolManager)
}

func (s *RepoTestSuite) TestFindRepoPoolManagerFetchPoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("GetRepoPoolManager", mock.AnythingOfType("params.Repository")).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	_, err := s.Runner.findRepoPoolManager(s.Fixtures.StoreRepos["test-repo-1"].Owner, s.Fixtures.StoreRepos["test-repo-1"].Name, s.Fixtures.StoreRepos["test-repo-1"].Endpoint.Name)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Regexp("fetching pool manager for repo", err.Error())
}

func TestRepoTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(RepoTestSuite))
}
