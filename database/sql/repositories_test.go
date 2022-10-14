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
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"
)

type RepoTestFixtures struct {
	Repos                []params.Repository
	CreateRepoParams     params.CreateRepoParams
	CreatePoolParams     params.CreatePoolParams
	CreateInstanceParams params.CreateInstanceParams
	UpdateRepoParams     params.UpdateRepositoryParams
	UpdatePoolParams     params.UpdatePoolParams
}

type RepoTestSuite struct {
	suite.Suite
	Store    dbCommon.Store
	Fixtures *RepoTestFixtures
}

func (s *RepoTestSuite) equalReposByName(expected, actual []params.Repository) {
	s.Require().Equal(len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].Name > expected[j].Name })
	sort.Slice(actual, func(i, j int) bool { return actual[i].Name > actual[j].Name })

	for i := 0; i < len(expected); i++ {
		s.Require().Equal(expected[i].Name, actual[i].Name)
	}
}

func (s *RepoTestSuite) equalPoolsByID(expected, actual []params.Pool) {
	s.Require().Equal(len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].ID > expected[j].ID })
	sort.Slice(actual, func(i, j int) bool { return actual[i].ID > actual[j].ID })

	for i := 0; i < len(expected); i++ {
		s.Require().Equal(expected[i].ID, actual[i].ID)
	}
}

func (s *RepoTestSuite) equalInstancesByID(expected, actual []params.Instance) {
	s.Require().Equal(len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].ID > expected[j].ID })
	sort.Slice(actual, func(i, j int) bool { return actual[i].ID > actual[j].ID })

	for i := 0; i < len(expected); i++ {
		s.Require().Equal(expected[i].ID, actual[i].ID)
	}
}

func (s *RepoTestSuite) SetupTest() {
	// create testing sqlite database
	db, err := NewSQLDatabase(context.Background(), garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.Store = db

	// create some repository objects in the database, for testing purposes
	repos := []params.Repository{}
	for i := 1; i <= 3; i++ {
		repo, err := db.CreateRepository(
			context.Background(),
			fmt.Sprintf("test-owner-%d", i),
			fmt.Sprintf("test-repo-%d", i),
			fmt.Sprintf("test-creds-%d", i),
			fmt.Sprintf("test-webhook-secret-%d", i),
		)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to create database object (test-repo-%d)", i))
		}

		repos = append(repos, repo)
	}

	// setup test fixtures
	var maxRunners uint = 40
	var minIdleRunners uint = 20
	fixtures := &RepoTestFixtures{
		Repos: repos,
		CreateRepoParams: params.CreateRepoParams{
			Owner:           "test-owner-repo",
			Name:            "test-repo",
			CredentialsName: "test-creds-repo",
			WebhookSecret:   "test-webhook-secret",
		},
		CreatePoolParams: params.CreatePoolParams{
			ProviderName:   "test-provider",
			MaxRunners:     4,
			MinIdleRunners: 2,
			Image:          "test-image",
			Flavor:         "test-flavor",
			OSType:         "windows",
			OSArch:         "amd64",
			Tags:           []string{"self-hosted", "arm64", "windows"},
		},
		CreateInstanceParams: params.CreateInstanceParams{
			Name:   "test-instance",
			OSType: "linux",
		},
		UpdateRepoParams: params.UpdateRepositoryParams{
			CredentialsName: "test-update-creds",
			WebhookSecret:   "test-update-webhook-secret",
		},
		UpdatePoolParams: params.UpdatePoolParams{
			MaxRunners:     &maxRunners,
			MinIdleRunners: &minIdleRunners,
			Image:          "test-update-image",
			Flavor:         "test-update-flavor",
		},
	}
	s.Fixtures = fixtures
}

func (s *RepoTestSuite) TestCreateRepository() {
	// call tested function
	repo, err := s.Store.CreateRepository(
		context.Background(),
		s.Fixtures.CreateRepoParams.Owner,
		s.Fixtures.CreateRepoParams.Name,
		s.Fixtures.CreateRepoParams.CredentialsName,
		s.Fixtures.CreateRepoParams.WebhookSecret,
	)

	// assertions
	s.Require().Nil(err)
	storeRepo, err := s.Store.GetRepositoryByID(context.Background(), repo.ID)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get repository by id: %v", err))
	}
	s.Require().Equal(storeRepo.Owner, repo.Owner)
	s.Require().Equal(storeRepo.Name, repo.Name)
	s.Require().Equal(storeRepo.CredentialsName, repo.CredentialsName)
	s.Require().Equal(storeRepo.WebhookSecret, repo.WebhookSecret)
}

func (s *RepoTestSuite) TestCreateRepositoryInvalidDBPassphrase() {
	cfg := garmTesting.GetTestSqliteDBConfig(s.T())
	conn, err := newDBConn(cfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	// make sure we use a 'sqlDatabase' struct with a wrong 'cfg.Passphrase'
	cfg.Passphrase = "wrong-passphrase" // it must have a size different than 32
	sqlDB := &sqlDatabase{
		conn: conn,
		cfg:  cfg,
	}

	_, err = sqlDB.CreateRepository(
		context.Background(),
		s.Fixtures.CreateRepoParams.Owner,
		s.Fixtures.CreateRepoParams.Name,
		s.Fixtures.CreateRepoParams.CredentialsName,
		s.Fixtures.CreateRepoParams.WebhookSecret,
	)

	s.Require().NotNil(err)
	s.Require().Equal("failed to encrypt string", err.Error())
}

func (s *RepoTestSuite) TestGetRepository() {
	repo, err := s.Store.GetRepository(context.Background(), s.Fixtures.Repos[0].Owner, s.Fixtures.Repos[0].Name)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.Repos[0].Owner, repo.Owner)
	s.Require().Equal(s.Fixtures.Repos[0].Name, repo.Name)
	s.Require().Equal(s.Fixtures.Repos[0].ID, repo.ID)
}

func (s *RepoTestSuite) TestGetRepositoryCaseInsensitive() {
	repo, err := s.Store.GetRepository(context.Background(), "TeSt-oWnEr-1", "TeSt-rEpO-1")

	s.Require().Nil(err)
	s.Require().Equal("test-owner-1", repo.Owner)
	s.Require().Equal("test-repo-1", repo.Name)
}

func (s *RepoTestSuite) TestGetRepositoryNotFound() {
	_, err := s.Store.GetRepository(context.Background(), "dummy-owner", "dummy-name")

	s.Require().NotNil(err)
	s.Require().Equal("fetching repo: not found", err.Error())
}

func (s *RepoTestSuite) TestListRepositories() {
	repos, err := s.Store.ListRepositories((context.Background()))

	s.Require().Nil(err)
	s.equalReposByName(s.Fixtures.Repos, repos)
}

func (s *RepoTestSuite) TestDeleteRepository() {
	err := s.Store.DeleteRepository(context.Background(), s.Fixtures.Repos[0].ID)

	s.Require().Nil(err)
	_, err = s.Store.GetRepositoryByID(context.Background(), s.Fixtures.Repos[0].ID)
	s.Require().NotNil(err)
	s.Require().Equal("fetching repo: not found", err.Error())
}

func (s *RepoTestSuite) TestDeleteRepositoryInvalidRepoID() {
	err := s.Store.DeleteRepository(context.Background(), "dummy-repo-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching repo: parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestUpdateRepository() {
	repo, err := s.Store.UpdateRepository(context.Background(), s.Fixtures.Repos[0].ID, s.Fixtures.UpdateRepoParams)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.CredentialsName, repo.CredentialsName)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.WebhookSecret, repo.WebhookSecret)
}

func (s *RepoTestSuite) TestUpdateRepositoryInvalidRepoID() {
	_, err := s.Store.UpdateRepository(context.Background(), "dummy-repo-id", s.Fixtures.UpdateRepoParams)

	s.Require().NotNil(err)
	s.Require().Equal("fetching repo: parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestGetRepositoryByID() {
	repo, err := s.Store.GetRepositoryByID(context.Background(), s.Fixtures.Repos[0].ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.Repos[0].ID, repo.ID)
}

func (s *RepoTestSuite) TestGetRepositoryByIDInvalidRepoID() {
	_, err := s.Store.GetRepositoryByID(context.Background(), "dummy-repo-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching repo: parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestCreateRepositoryPool() {
	pool, err := s.Store.CreateRepositoryPool(context.Background(), s.Fixtures.Repos[0].ID, s.Fixtures.CreatePoolParams)

	s.Require().Nil(err)
	repo, err := s.Store.GetRepositoryByID(context.Background(), s.Fixtures.Repos[0].ID)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot get repo by ID: %v", err))
	}
	s.Require().Equal(1, len(repo.Pools))
	s.Require().Equal(pool.ID, repo.Pools[0].ID)
	s.Require().Equal(s.Fixtures.CreatePoolParams.ProviderName, repo.Pools[0].ProviderName)
	s.Require().Equal(s.Fixtures.CreatePoolParams.MaxRunners, repo.Pools[0].MaxRunners)
	s.Require().Equal(s.Fixtures.CreatePoolParams.MinIdleRunners, repo.Pools[0].MinIdleRunners)
}

func (s *RepoTestSuite) TestCreateRepositoryPoolMissingTags() {
	s.Fixtures.CreatePoolParams.Tags = []string{}

	_, err := s.Store.CreateRepositoryPool(context.Background(), s.Fixtures.Repos[0].ID, s.Fixtures.CreatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("no tags specified", err.Error())
}

func (s *RepoTestSuite) TestCreateRepositoryPoolInvalidRepoID() {
	_, err := s.Store.CreateRepositoryPool(context.Background(), "dummy-repo-id", s.Fixtures.CreatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("fetching repo: parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestListRepoPools() {
	repoPools := []params.Pool{}
	for i := 1; i <= 2; i++ {
		s.Fixtures.CreatePoolParams.Flavor = fmt.Sprintf("test-flavor-%d", i)
		pool, err := s.Store.CreateRepositoryPool(context.Background(), s.Fixtures.Repos[0].ID, s.Fixtures.CreatePoolParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
		}
		repoPools = append(repoPools, pool)
	}

	pools, err := s.Store.ListRepoPools(context.Background(), s.Fixtures.Repos[0].ID)

	s.Require().Nil(err)
	s.equalPoolsByID(repoPools, pools)
}

func (s *RepoTestSuite) TestListRepoPoolsInvalidRepoID() {
	_, err := s.Store.ListRepoPools(context.Background(), "dummy-repo-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching pools: fetching repo: parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestGetRepositoryPool() {
	pool, err := s.Store.CreateRepositoryPool(context.Background(), s.Fixtures.Repos[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
	}

	repoPool, err := s.Store.GetRepositoryPool(context.Background(), s.Fixtures.Repos[0].ID, pool.ID)

	s.Require().Nil(err)
	s.Require().Equal(repoPool.ID, pool.ID)
}

func (s *RepoTestSuite) TestGetRepositoryPoolInvalidRepoID() {
	_, err := s.Store.GetRepositoryPool(context.Background(), "dummy-repo-id", "dummy-pool-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: fetching repo: parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestDeleteRepositoryPool() {
	pool, err := s.Store.CreateRepositoryPool(context.Background(), s.Fixtures.Repos[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
	}

	err = s.Store.DeleteRepositoryPool(context.Background(), s.Fixtures.Repos[0].ID, pool.ID)

	s.Require().Nil(err)
	_, err = s.Store.GetOrganizationPool(context.Background(), s.Fixtures.Repos[0].ID, pool.ID)
	s.Require().Equal("fetching pool: fetching org: not found", err.Error())
}

func (s *RepoTestSuite) TestDeleteRepositoryPoolInvalidRepoID() {
	err := s.Store.DeleteRepositoryPool(context.Background(), "dummy-repo-id", "dummy-pool-id")

	s.Require().NotNil(err)
	s.Require().Equal("looking up repo pool: fetching repo: parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestFindRepositoryPoolByTags() {
	repoPool, err := s.Store.CreateRepositoryPool(context.Background(), s.Fixtures.Repos[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
	}

	pool, err := s.Store.FindRepositoryPoolByTags(context.Background(), s.Fixtures.Repos[0].ID, s.Fixtures.CreatePoolParams.Tags)
	s.Require().Nil(err)
	s.Require().Equal(repoPool.ID, pool.ID)
	s.Require().Equal(repoPool.Image, pool.Image)
	s.Require().Equal(repoPool.Flavor, pool.Flavor)
}

func (s *RepoTestSuite) TestFindRepositoryPoolByTagsMissingTags() {
	tags := []string{}

	_, err := s.Store.FindRepositoryPoolByTags(context.Background(), s.Fixtures.Repos[0].ID, tags)

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: missing tags", err.Error())
}

func (s *RepoTestSuite) TestListRepoInstances() {
	pool, err := s.Store.CreateRepositoryPool(context.Background(), s.Fixtures.Repos[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
	}
	poolInstances := []params.Instance{}
	for i := 1; i <= 3; i++ {
		s.Fixtures.CreateInstanceParams.Name = fmt.Sprintf("test-repo-%d", i)
		instance, err := s.Store.CreateInstance(context.Background(), pool.ID, s.Fixtures.CreateInstanceParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create instance: %s", err))
		}
		poolInstances = append(poolInstances, instance)
	}

	instances, err := s.Store.ListRepoInstances(context.Background(), s.Fixtures.Repos[0].ID)

	s.Require().Nil(err)
	s.equalInstancesByID(poolInstances, instances)
}

func (s *RepoTestSuite) TestUpdateRepositoryPool() {
	repoPool, err := s.Store.CreateRepositoryPool(context.Background(), s.Fixtures.Repos[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
	}

	pool, err := s.Store.UpdateRepositoryPool(context.Background(), s.Fixtures.Repos[0].ID, repoPool.ID, s.Fixtures.UpdatePoolParams)

	s.Require().Nil(err)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MaxRunners, pool.MaxRunners)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MinIdleRunners, pool.MinIdleRunners)
	s.Require().Equal(s.Fixtures.UpdatePoolParams.Image, pool.Image)
	s.Require().Equal(s.Fixtures.UpdatePoolParams.Flavor, pool.Flavor)
}

func (s *RepoTestSuite) TestUpdateRepositoryPoolInvalidRepoID() {
	_, err := s.Store.UpdateRepositoryPool(context.Background(), "dummy-org-id", "dummy-repo-id", s.Fixtures.UpdatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: fetching repo: parsing id: invalid request", err.Error())
}

func TestRepoTestSuite(t *testing.T) {
	suite.Run(t, new(RepoTestSuite))
}
