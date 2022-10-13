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
	Repos            []params.Repository
	CreateRepoParams params.CreateRepoParams
	UpdateRepoParams params.UpdateRepositoryParams
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
	fixtures := &RepoTestFixtures{
		Repos: repos,
		CreateRepoParams: params.CreateRepoParams{
			Owner:           "test-owner-repo",
			Name:            "test-repo",
			CredentialsName: "test-creds-repo",
			WebhookSecret:   "test-webhook-secret",
		},
		UpdateRepoParams: params.UpdateRepositoryParams{
			CredentialsName: "test-update-creds",
			WebhookSecret:   "test-update-webhook-secret",
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

func TestRepoTestSuite(t *testing.T) {
	suite.Run(t, new(RepoTestSuite))
}
