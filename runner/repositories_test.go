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
	"garm/params"
	"garm/runner/common"
	runnerCommonMocks "garm/runner/common/mocks"
	runnerMocks "garm/runner/mocks"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type RepoTestFixtures struct {
	AdminContext     context.Context
	Store            dbCommon.Store
	StoreRepos       map[string]params.Repository
	Providers        map[string]common.Provider
	Credentials      map[string]config.Github
	CreateRepoParams params.CreateRepoParams
	ErrMock          error
	ProviderMock     *runnerCommonMocks.Provider
	PoolMgrMock      *runnerCommonMocks.PoolManager
	PoolMgrCtrlMock  *runnerMocks.PoolManagerController
}

type RepoTestSuite struct {
	suite.Suite
	Fixtures *RepoTestFixtures
	Runner   *Runner
}

func (s *RepoTestSuite) SetupTest() {
	adminCtx := auth.GetAdminContext()

	// create testing sqlite database
	dbCfg := getTestSqliteDBConfig(s.T())
	db, err := database.NewDatabase(adminCtx, dbCfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}

	// create some repository objects in the database, for testing purposes
	repos := map[string]params.Repository{}
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("test-repo-%v", i)
		repo, err := db.CreateRepository(
			adminCtx,
			fmt.Sprintf("test-owner-%v", i),
			name,
			fmt.Sprintf("test-creds-%v", i),
			fmt.Sprintf("test-webhook-secret-%v", i),
		)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to create database object (test-repo-%v)", i))
		}
		repos[name] = repo
	}

	// setup test fixtures
	providerMock := runnerCommonMocks.NewProvider(s.T())
	fixtures := &RepoTestFixtures{
		AdminContext: auth.GetAdminContext(),
		Store:        db,
		StoreRepos:   repos,
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
		CreateRepoParams: params.CreateRepoParams{
			Owner:           "test-owner-create",
			Name:            "test-repo-create",
			CredentialsName: "test-creds",
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
	s.Require().Equal(s.Fixtures.Credentials[s.Fixtures.CreateRepoParams.CredentialsName].Name, repo.CredentialsName)
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
	s.Fixtures.CreateRepoParams.CredentialsName = "not-existent-creds-name"

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

func TestRepoTestSuite(t *testing.T) {
	suite.Run(t, new(RepoTestSuite))
}
