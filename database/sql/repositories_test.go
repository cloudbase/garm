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
	"flag"
	"fmt"
	"regexp"
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	dbCommon "github.com/cloudbase/garm/database/common"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type RepoTestFixtures struct {
	Repos                []params.Repository
	CreateRepoParams     params.CreateRepoParams
	CreatePoolParams     params.CreatePoolParams
	CreateInstanceParams params.CreateInstanceParams
	UpdateRepoParams     params.UpdateEntityParams
	UpdatePoolParams     params.UpdatePoolParams
	SQLMock              sqlmock.Sqlmock
}

type RepoTestSuite struct {
	suite.Suite
	Store          dbCommon.Store
	StoreSQLMocked *sqlDatabase
	Fixtures       *RepoTestFixtures

	adminCtx           context.Context
	testCreds          params.GithubCredentials
	secondaryTestCreds params.GithubCredentials
	githubEndpoint     params.GithubEndpoint
}

func (s *RepoTestSuite) equalReposByName(expected, actual []params.Repository) {
	s.Require().Equal(len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].Name > expected[j].Name })
	sort.Slice(actual, func(i, j int) bool { return actual[i].Name > actual[j].Name })

	for i := 0; i < len(expected); i++ {
		s.Require().Equal(expected[i].Name, actual[i].Name)
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

func (s *RepoTestSuite) assertSQLMockExpectations() {
	err := s.Fixtures.SQLMock.ExpectationsWereMet()
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to meet sqlmock expectations, got error: %v", err))
	}
}

func (s *RepoTestSuite) SetupTest() {
	// create testing sqlite database
	db, err := NewSQLDatabase(context.Background(), garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.Store = db

	adminCtx := garmTesting.ImpersonateAdminContext(context.Background(), db, s.T())
	s.adminCtx = adminCtx

	s.githubEndpoint = garmTesting.CreateDefaultGithubEndpoint(adminCtx, db, s.T())
	s.testCreds = garmTesting.CreateTestGithubCredentials(adminCtx, "new-creds", db, s.T(), s.githubEndpoint)
	s.secondaryTestCreds = garmTesting.CreateTestGithubCredentials(adminCtx, "secondary-creds", db, s.T(), s.githubEndpoint)

	// create some repository objects in the database, for testing purposes
	repos := []params.Repository{}
	for i := 1; i <= 3; i++ {
		repo, err := db.CreateRepository(
			adminCtx,
			fmt.Sprintf("test-owner-%d", i),
			fmt.Sprintf("test-repo-%d", i),
			s.testCreds.Name,
			fmt.Sprintf("test-webhook-secret-%d", i),
			params.PoolBalancerTypeRoundRobin,
		)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to create database object (test-repo-%d): %v", i, err))
		}

		repos = append(repos, repo)
	}

	// create store with mocked sql connection
	sqlDB, sqlMock, err := sqlmock.New()
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to run 'sqlmock.New()', got error: %v", err))
	}
	s.T().Cleanup(func() { sqlDB.Close() })
	mysqlConfig := mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}
	gormConfig := &gorm.Config{}
	if flag.Lookup("test.v").Value.String() == falseString {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	}
	gormConn, err := gorm.Open(mysql.New(mysqlConfig), gormConfig)
	if err != nil {
		s.FailNow(fmt.Sprintf("fail to open gorm connection: %v", err))
	}
	s.StoreSQLMocked = &sqlDatabase{
		conn: gormConn,
		cfg:  garmTesting.GetTestSqliteDBConfig(s.T()),
	}

	// setup test fixtures
	var maxRunners uint = 40
	var minIdleRunners uint = 20
	fixtures := &RepoTestFixtures{
		Repos: repos,
		CreateRepoParams: params.CreateRepoParams{
			Owner:           "test-owner-repo",
			Name:            "test-repo",
			CredentialsName: s.testCreds.Name,
			WebhookSecret:   "test-webhook-secret",
		},
		CreatePoolParams: params.CreatePoolParams{
			ProviderName:   "test-provider",
			MaxRunners:     4,
			MinIdleRunners: 2,
			Image:          "test-image",
			Enabled:        true,
			Flavor:         "test-flavor",
			OSType:         "windows",
			OSArch:         "amd64",
			Tags:           []string{"self-hosted", "arm64", "windows"},
		},
		CreateInstanceParams: params.CreateInstanceParams{
			Name:   "test-instance",
			OSType: "linux",
		},
		UpdateRepoParams: params.UpdateEntityParams{
			CredentialsName: s.secondaryTestCreds.Name,
			WebhookSecret:   "test-update-webhook-secret",
		},
		UpdatePoolParams: params.UpdatePoolParams{
			MaxRunners:     &maxRunners,
			MinIdleRunners: &minIdleRunners,
			Image:          "test-update-image",
			Flavor:         "test-update-flavor",
		},
		SQLMock: sqlMock,
	}
	s.Fixtures = fixtures
}

func (s *RepoTestSuite) TestCreateRepository() {
	// call tested function
	repo, err := s.Store.CreateRepository(
		s.adminCtx,
		s.Fixtures.CreateRepoParams.Owner,
		s.Fixtures.CreateRepoParams.Name,
		s.Fixtures.CreateRepoParams.CredentialsName,
		s.Fixtures.CreateRepoParams.WebhookSecret,
		params.PoolBalancerTypeRoundRobin,
	)

	// assertions
	s.Require().Nil(err)
	storeRepo, err := s.Store.GetRepositoryByID(s.adminCtx, repo.ID)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get repository by id: %v", err))
	}
	s.Require().Equal(storeRepo.Owner, repo.Owner)
	s.Require().Equal(storeRepo.Name, repo.Name)
	s.Require().Equal(storeRepo.Credentials.Name, repo.Credentials.Name)
	s.Require().Equal(storeRepo.WebhookSecret, repo.WebhookSecret)
}

func (s *RepoTestSuite) TestCreateRepositoryInvalidDBPassphrase() {
	cfg := garmTesting.GetTestSqliteDBConfig(s.T())
	conn, err := newDBConn(cfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	// make sure we use a 'sqlDatabase' struct with a wrong 'cfg.Passphrase'
	cfg.Passphrase = wrongPassphrase // it must have a size different than 32
	sqlDB := &sqlDatabase{
		conn: conn,
		cfg:  cfg,
	}

	_, err = sqlDB.CreateRepository(
		s.adminCtx,
		s.Fixtures.CreateRepoParams.Owner,
		s.Fixtures.CreateRepoParams.Name,
		s.Fixtures.CreateRepoParams.CredentialsName,
		s.Fixtures.CreateRepoParams.WebhookSecret,
		params.PoolBalancerTypeRoundRobin,
	)

	s.Require().NotNil(err)
	s.Require().Equal("failed to encrypt string", err.Error())
}

func (s *RepoTestSuite) TestCreateRepositoryInvalidDBCreateErr() {
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `github_credentials` WHERE name = ? AND `github_credentials`.`deleted_at` IS NULL ORDER BY `github_credentials`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Repos[0].CredentialsName).
		WillReturnRows(sqlmock.NewRows([]string{"id", "endpoint_name"}).
			AddRow(s.testCreds.ID, s.githubEndpoint.Name))
	s.Fixtures.SQLMock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `github_endpoints` WHERE `github_endpoints`.`name` = ? AND `github_endpoints`.`deleted_at` IS NULL")).
		WithArgs(s.testCreds.Endpoint).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).
			AddRow(s.githubEndpoint.Name))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `repositories`")).
		WillReturnError(fmt.Errorf("creating repo mock error"))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.CreateRepository(
		s.adminCtx,
		s.Fixtures.CreateRepoParams.Owner,
		s.Fixtures.CreateRepoParams.Name,
		s.Fixtures.CreateRepoParams.CredentialsName,
		s.Fixtures.CreateRepoParams.WebhookSecret,
		params.PoolBalancerTypeRoundRobin,
	)

	s.Require().NotNil(err)
	s.Require().Equal("creating repository: creating repository: creating repo mock error", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestGetRepository() {
	repo, err := s.Store.GetRepository(s.adminCtx, s.Fixtures.Repos[0].Owner, s.Fixtures.Repos[0].Name)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.Repos[0].Owner, repo.Owner)
	s.Require().Equal(s.Fixtures.Repos[0].Name, repo.Name)
	s.Require().Equal(s.Fixtures.Repos[0].ID, repo.ID)
}

func (s *RepoTestSuite) TestGetRepositoryCaseInsensitive() {
	repo, err := s.Store.GetRepository(s.adminCtx, "TeSt-oWnEr-1", "TeSt-rEpO-1")

	s.Require().Nil(err)
	s.Require().Equal("test-owner-1", repo.Owner)
	s.Require().Equal("test-repo-1", repo.Name)
}

func (s *RepoTestSuite) TestGetRepositoryNotFound() {
	_, err := s.Store.GetRepository(s.adminCtx, "dummy-owner", "dummy-name")

	s.Require().NotNil(err)
	s.Require().Equal("fetching repo: not found", err.Error())
}

func (s *RepoTestSuite) TestGetRepositoryDBDecryptingErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE (name = ? COLLATE NOCASE and owner = ? COLLATE NOCASE) AND `repositories`.`deleted_at` IS NULL ORDER BY `repositories`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Repos[0].Name, s.Fixtures.Repos[0].Owner, 1).
		WillReturnRows(sqlmock.NewRows([]string{"name", "owner"}).AddRow(s.Fixtures.Repos[0].Name, s.Fixtures.Repos[0].Owner))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE (name = ? COLLATE NOCASE and owner = ? COLLATE NOCASE) AND `repositories`.`deleted_at` IS NULL ORDER BY `repositories`.`id`,`repositories`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Repos[0].Name, s.Fixtures.Repos[0].Owner, 1).
		WillReturnRows(sqlmock.NewRows([]string{"name", "owner"}).AddRow(s.Fixtures.Repos[0].Name, s.Fixtures.Repos[0].Owner))

	_, err := s.StoreSQLMocked.GetRepository(s.adminCtx, s.Fixtures.Repos[0].Owner, s.Fixtures.Repos[0].Name)

	s.Require().NotNil(err)
	s.Require().Equal("fetching repo: missing secret", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestListRepositories() {
	repos, err := s.Store.ListRepositories(s.adminCtx)

	s.Require().Nil(err)
	s.equalReposByName(s.Fixtures.Repos, repos)
}

func (s *RepoTestSuite) TestListRepositoriesDBFetchErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE `repositories`.`deleted_at` IS NULL")).
		WillReturnError(fmt.Errorf("fetching user from database mock error"))

	_, err := s.StoreSQLMocked.ListRepositories(s.adminCtx)

	s.Require().NotNil(err)
	s.Require().Equal("fetching user from database: fetching user from database mock error", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestListRepositoriesDBDecryptingErr() {
	s.StoreSQLMocked.cfg.Passphrase = wrongPassphrase

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE `repositories`.`deleted_at` IS NULL")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "webhook_secret"}).AddRow(s.Fixtures.Repos[0].ID, s.Fixtures.Repos[0].WebhookSecret))

	_, err := s.StoreSQLMocked.ListRepositories(s.adminCtx)

	s.Require().NotNil(err)
	s.Require().Equal("fetching repositories: decrypting secret: invalid passphrase length (expected length 32 characters)", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestDeleteRepository() {
	err := s.Store.DeleteRepository(s.adminCtx, s.Fixtures.Repos[0].ID)

	s.Require().Nil(err)
	_, err = s.Store.GetRepositoryByID(s.adminCtx, s.Fixtures.Repos[0].ID)
	s.Require().NotNil(err)
	s.Require().Equal("fetching repo: not found", err.Error())
}

func (s *RepoTestSuite) TestDeleteRepositoryInvalidRepoID() {
	err := s.Store.DeleteRepository(s.adminCtx, "dummy-repo-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching repo: parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestDeleteRepositoryDBRemoveErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE id = ? AND `repositories`.`deleted_at` IS NULL ORDER BY `repositories`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Repos[0].ID))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("DELETE FROM `repositories`")).
		WithArgs(s.Fixtures.Repos[0].ID).
		WillReturnError(fmt.Errorf("mocked deleting repo error"))
	s.Fixtures.SQLMock.ExpectRollback()

	err := s.StoreSQLMocked.DeleteRepository(s.adminCtx, s.Fixtures.Repos[0].ID)

	s.Require().NotNil(err)
	s.Require().Equal("deleting repo: mocked deleting repo error", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestUpdateRepository() {
	repo, err := s.Store.UpdateRepository(s.adminCtx, s.Fixtures.Repos[0].ID, s.Fixtures.UpdateRepoParams)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.CredentialsName, repo.Credentials.Name)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.WebhookSecret, repo.WebhookSecret)
}

func (s *RepoTestSuite) TestUpdateRepositoryInvalidRepoID() {
	_, err := s.Store.UpdateRepository(s.adminCtx, "dummy-repo-id", s.Fixtures.UpdateRepoParams)

	s.Require().NotNil(err)
	s.Require().Equal("saving repo: fetching repo: parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestUpdateRepositoryDBEncryptErr() {
	s.StoreSQLMocked.cfg.Passphrase = wrongPassphrase
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE id = ? AND `repositories`.`deleted_at` IS NULL ORDER BY `repositories`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "endpoint_name"}).
			AddRow(s.Fixtures.Repos[0].ID, s.githubEndpoint.Name))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `github_credentials` WHERE name = ? AND `github_credentials`.`deleted_at` IS NULL ORDER BY `github_credentials`.`id` LIMIT ?")).
		WithArgs(s.secondaryTestCreds.Name, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "endpoint_name"}).
			AddRow(s.secondaryTestCreds.ID, s.secondaryTestCreds.Endpoint))
	s.Fixtures.SQLMock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `github_endpoints` WHERE `github_endpoints`.`name` = ? AND `github_endpoints`.`deleted_at` IS NULL")).
		WithArgs(s.testCreds.Endpoint).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).
			AddRow(s.secondaryTestCreds.Endpoint))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.UpdateRepository(s.adminCtx, s.Fixtures.Repos[0].ID, s.Fixtures.UpdateRepoParams)

	s.Require().NotNil(err)
	s.Require().Equal("saving repo: saving repo: failed to encrypt string: invalid passphrase length (expected length 32 characters)", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestUpdateRepositoryDBSaveErr() {
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE id = ? AND `repositories`.`deleted_at` IS NULL ORDER BY `repositories`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "endpoint_name"}).
			AddRow(s.Fixtures.Repos[0].ID, s.githubEndpoint.Name))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `github_credentials` WHERE name = ? AND `github_credentials`.`deleted_at` IS NULL ORDER BY `github_credentials`.`id` LIMIT ?")).
		WithArgs(s.secondaryTestCreds.Name, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "endpoint_name"}).
			AddRow(s.secondaryTestCreds.ID, s.secondaryTestCreds.Endpoint))
	s.Fixtures.SQLMock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `github_endpoints` WHERE `github_endpoints`.`name` = ? AND `github_endpoints`.`deleted_at` IS NULL")).
		WithArgs(s.testCreds.Endpoint).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).
			AddRow(s.secondaryTestCreds.Endpoint))
	s.Fixtures.SQLMock.
		ExpectExec(("UPDATE `repositories` SET")).
		WillReturnError(fmt.Errorf("saving repo mock error"))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.UpdateRepository(s.adminCtx, s.Fixtures.Repos[0].ID, s.Fixtures.UpdateRepoParams)

	s.Require().NotNil(err)
	s.Require().Equal("saving repo: saving repo: saving repo mock error", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestUpdateRepositoryDBDecryptingErr() {
	s.StoreSQLMocked.cfg.Passphrase = wrongPassphrase
	s.Fixtures.UpdateRepoParams.WebhookSecret = webhookSecret
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE id = ? AND `repositories`.`deleted_at` IS NULL ORDER BY `repositories`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "endpoint_name"}).
			AddRow(s.Fixtures.Repos[0].ID, s.githubEndpoint.Name))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `github_credentials` WHERE name = ? AND `github_credentials`.`deleted_at` IS NULL ORDER BY `github_credentials`.`id` LIMIT ?")).
		WithArgs(s.secondaryTestCreds.Name, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "endpoint_name"}).
			AddRow(s.secondaryTestCreds.ID, s.secondaryTestCreds.Endpoint))
	s.Fixtures.SQLMock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `github_endpoints` WHERE `github_endpoints`.`name` = ? AND `github_endpoints`.`deleted_at` IS NULL")).
		WithArgs(s.testCreds.Endpoint).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).
			AddRow(s.secondaryTestCreds.Endpoint))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.UpdateRepository(s.adminCtx, s.Fixtures.Repos[0].ID, s.Fixtures.UpdateRepoParams)

	s.Require().NotNil(err)
	s.Require().Equal("saving repo: saving repo: failed to encrypt string: invalid passphrase length (expected length 32 characters)", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestGetRepositoryByID() {
	repo, err := s.Store.GetRepositoryByID(s.adminCtx, s.Fixtures.Repos[0].ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.Repos[0].ID, repo.ID)
}

func (s *RepoTestSuite) TestGetRepositoryByIDInvalidRepoID() {
	_, err := s.Store.GetRepositoryByID(s.adminCtx, "dummy-repo-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching repo: parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestGetRepositoryByIDDBDecryptingErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE id = ? AND `repositories`.`deleted_at` IS NULL ORDER BY `repositories`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Repos[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE `pools`.`repo_id` = ? AND `pools`.`deleted_at` IS NULL")).
		WithArgs(s.Fixtures.Repos[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"repo_id"}).AddRow(s.Fixtures.Repos[0].ID))

	_, err := s.StoreSQLMocked.GetRepositoryByID(s.adminCtx, s.Fixtures.Repos[0].ID)

	s.Require().NotNil(err)
	s.Require().Equal("fetching repo: missing secret", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestCreateRepositoryPool() {
	entity, err := s.Fixtures.Repos[0].GetEntity()
	s.Require().Nil(err)
	pool, err := s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)

	s.Require().Nil(err)
	repo, err := s.Store.GetRepositoryByID(s.adminCtx, s.Fixtures.Repos[0].ID)
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
	entity, err := s.Fixtures.Repos[0].GetEntity()
	s.Require().Nil(err)
	_, err = s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("no tags specified", err.Error())
}

func (s *RepoTestSuite) TestCreateRepositoryPoolInvalidRepoID() {
	entity := params.GithubEntity{
		ID:         "dummy-repo-id",
		EntityType: params.GithubEntityTypeRepository,
	}
	_, err := s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestCreateRepositoryPoolDBCreateErr() {
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE id = ? AND `repositories`.`deleted_at` IS NULL ORDER BY `repositories`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Repos[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE (provider_name = ? and image = ? and flavor = ? and repo_id = ?) AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT ?")).
		WillReturnError(fmt.Errorf("mocked creating pool error"))

	entity, err := s.Fixtures.Repos[0].GetEntity()
	s.Require().Nil(err)
	_, err = s.StoreSQLMocked.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("checking pool existence: mocked creating pool error", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestCreateRepositoryPoolDBPoolAlreadyExistErr() {
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE id = ? AND `repositories`.`deleted_at` IS NULL ORDER BY `repositories`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Repos[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE (provider_name = ? and image = ? and flavor = ? and repo_id = ?) AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT ?")).
		WithArgs(
			s.Fixtures.CreatePoolParams.ProviderName,
			s.Fixtures.CreatePoolParams.Image,
			s.Fixtures.CreatePoolParams.Flavor,
			s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"repo_id", "provider_name", "image", "flavor"}).
			AddRow(
				s.Fixtures.Repos[0].ID,
				s.Fixtures.CreatePoolParams.ProviderName,
				s.Fixtures.CreatePoolParams.Image,
				s.Fixtures.CreatePoolParams.Flavor))

	entity, err := s.Fixtures.Repos[0].GetEntity()
	s.Require().Nil(err)

	_, err = s.StoreSQLMocked.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("pool with the same image and flavor already exists on this provider", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestCreateRepositoryPoolDBFetchTagErr() {
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE id = ? AND `repositories`.`deleted_at` IS NULL ORDER BY `repositories`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Repos[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE (provider_name = ? and image = ? and flavor = ? and repo_id = ?) AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT ?")).
		WithArgs(
			s.Fixtures.CreatePoolParams.ProviderName,
			s.Fixtures.CreatePoolParams.Image,
			s.Fixtures.CreatePoolParams.Flavor,
			s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"repo_id"}))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `tags` WHERE name = ? AND `tags`.`deleted_at` IS NULL ORDER BY `tags`.`id` LIMIT ?")).
		WillReturnError(fmt.Errorf("mocked fetching tag error"))

	entity, err := s.Fixtures.Repos[0].GetEntity()
	s.Require().Nil(err)

	_, err = s.StoreSQLMocked.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("creating tag: fetching tag from database: mocked fetching tag error", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestCreateRepositoryPoolDBAddingPoolErr() {
	s.Fixtures.CreatePoolParams.Tags = []string{"linux"}

	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE id = ? AND `repositories`.`deleted_at` IS NULL ORDER BY `repositories`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Repos[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE (provider_name = ? and image = ? and flavor = ? and repo_id = ?) AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT ?")).
		WithArgs(
			s.Fixtures.CreatePoolParams.ProviderName,
			s.Fixtures.CreatePoolParams.Image,
			s.Fixtures.CreatePoolParams.Flavor,
			s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"repo_id"}))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `tags` WHERE name = ? AND `tags`.`deleted_at` IS NULL ORDER BY `tags`.`id` LIMIT ?")).
		WillReturnRows(sqlmock.NewRows([]string{"linux"}))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `tags`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `pools`")).
		WillReturnError(fmt.Errorf("mocked adding pool error"))
	s.Fixtures.SQLMock.ExpectRollback()

	entity, err := s.Fixtures.Repos[0].GetEntity()
	s.Require().Nil(err)

	_, err = s.StoreSQLMocked.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("creating pool: mocked adding pool error", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestCreateRepositoryPoolDBSaveTagErr() {
	s.Fixtures.CreatePoolParams.Tags = []string{"linux"}

	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE id = ? AND `repositories`.`deleted_at` IS NULL ORDER BY `repositories`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Repos[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE (provider_name = ? and image = ? and flavor = ? and repo_id = ?) AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT ?")).
		WithArgs(
			s.Fixtures.CreatePoolParams.ProviderName,
			s.Fixtures.CreatePoolParams.Image,
			s.Fixtures.CreatePoolParams.Flavor,
			s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"repo_id"}))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `tags` WHERE name = ? AND `tags`.`deleted_at` IS NULL ORDER BY `tags`.`id` LIMIT ?")).
		WillReturnRows(sqlmock.NewRows([]string{"linux"}))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `tags`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `pools`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("UPDATE `pools` SET")).
		WillReturnError(fmt.Errorf("mocked saving tag error"))
	s.Fixtures.SQLMock.ExpectRollback()

	entity, err := s.Fixtures.Repos[0].GetEntity()
	s.Require().Nil(err)

	_, err = s.StoreSQLMocked.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)
	s.Require().NotNil(err)
	s.Require().Equal("associating tags: mocked saving tag error", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestCreateRepositoryPoolDBFetchPoolErr() {
	s.Fixtures.CreatePoolParams.Tags = []string{"linux"}

	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `repositories` WHERE id = ? AND `repositories`.`deleted_at` IS NULL ORDER BY `repositories`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Repos[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE (provider_name = ? and image = ? and flavor = ? and repo_id = ?) AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT ?")).
		WithArgs(
			s.Fixtures.CreatePoolParams.ProviderName,
			s.Fixtures.CreatePoolParams.Image,
			s.Fixtures.CreatePoolParams.Flavor,
			s.Fixtures.Repos[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"repo_id"}))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `tags` WHERE name = ? AND `tags`.`deleted_at` IS NULL ORDER BY `tags`.`id` LIMIT ?")).
		WillReturnRows(sqlmock.NewRows([]string{"linux"}))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `tags`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `pools`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("UPDATE `pools` SET")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `tags`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `pool_tags`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.ExpectCommit()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE id = ? AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT ?")).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	entity, err := s.Fixtures.Repos[0].GetEntity()
	s.Require().Nil(err)

	_, err = s.StoreSQLMocked.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: not found", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestListRepoPools() {
	entity, err := s.Fixtures.Repos[0].GetEntity()
	s.Require().Nil(err)
	repoPools := []params.Pool{}
	for i := 1; i <= 2; i++ {
		s.Fixtures.CreatePoolParams.Flavor = fmt.Sprintf("test-flavor-%d", i)
		pool, err := s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
		}
		repoPools = append(repoPools, pool)
	}

	pools, err := s.Store.ListEntityPools(s.adminCtx, entity)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityID(s.T(), repoPools, pools)
}

func (s *RepoTestSuite) TestListRepoPoolsInvalidRepoID() {
	entity := params.GithubEntity{
		ID:         "dummy-repo-id",
		EntityType: params.GithubEntityTypeRepository,
	}
	_, err := s.Store.ListEntityPools(s.adminCtx, entity)

	s.Require().NotNil(err)
	s.Require().Equal("fetching pools: parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestGetRepositoryPool() {
	entity, err := s.Fixtures.Repos[0].GetEntity()
	s.Require().Nil(err)
	pool, err := s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
	}

	repoPool, err := s.Store.GetEntityPool(s.adminCtx, entity, pool.ID)

	s.Require().Nil(err)
	s.Require().Equal(repoPool.ID, pool.ID)
}

func (s *RepoTestSuite) TestGetRepositoryPoolInvalidRepoID() {
	entity := params.GithubEntity{
		ID:         "dummy-repo-id",
		EntityType: params.GithubEntityTypeRepository,
	}
	_, err := s.Store.GetEntityPool(s.adminCtx, entity, "dummy-pool-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestDeleteRepositoryPool() {
	entity, err := s.Fixtures.Repos[0].GetEntity()
	s.Require().Nil(err)
	pool, err := s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
	}

	err = s.Store.DeleteEntityPool(s.adminCtx, entity, pool.ID)

	s.Require().Nil(err)
	_, err = s.Store.GetEntityPool(s.adminCtx, entity, pool.ID)
	s.Require().Equal("fetching pool: finding pool: not found", err.Error())
}

func (s *RepoTestSuite) TestDeleteRepositoryPoolInvalidRepoID() {
	entity := params.GithubEntity{
		ID:         "dummy-repo-id",
		EntityType: params.GithubEntityTypeRepository,
	}
	err := s.Store.DeleteEntityPool(s.adminCtx, entity, "dummy-pool-id")

	s.Require().NotNil(err)
	s.Require().Equal("parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestDeleteRepositoryPoolDBDeleteErr() {
	entity, err := s.Fixtures.Repos[0].GetEntity()
	s.Require().Nil(err)

	pool, err := s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
	}

	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("DELETE FROM `pools` WHERE id = ? and repo_id = ?")).
		WithArgs(pool.ID, s.Fixtures.Repos[0].ID).
		WillReturnError(fmt.Errorf("mocked deleting pool error"))
	s.Fixtures.SQLMock.ExpectRollback()

	err = s.StoreSQLMocked.DeleteEntityPool(s.adminCtx, entity, pool.ID)
	s.Require().NotNil(err)
	s.Require().Equal("removing pool: mocked deleting pool error", err.Error())
	s.assertSQLMockExpectations()
}

func (s *RepoTestSuite) TestListRepoInstances() {
	entity, err := s.Fixtures.Repos[0].GetEntity()
	s.Require().Nil(err)
	pool, err := s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
	}
	poolInstances := []params.Instance{}
	for i := 1; i <= 3; i++ {
		s.Fixtures.CreateInstanceParams.Name = fmt.Sprintf("test-repo-%d", i)
		instance, err := s.Store.CreateInstance(s.adminCtx, pool.ID, s.Fixtures.CreateInstanceParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create instance: %s", err))
		}
		poolInstances = append(poolInstances, instance)
	}

	instances, err := s.Store.ListEntityInstances(s.adminCtx, entity)

	s.Require().Nil(err)
	s.equalInstancesByID(poolInstances, instances)
}

func (s *RepoTestSuite) TestListRepoInstancesInvalidRepoID() {
	entity := params.GithubEntity{
		ID:         "dummy-repo-id",
		EntityType: params.GithubEntityTypeRepository,
	}
	_, err := s.Store.ListEntityInstances(s.adminCtx, entity)

	s.Require().NotNil(err)
	s.Require().Equal("fetching entity: parsing id: invalid request", err.Error())
}

func (s *RepoTestSuite) TestUpdateRepositoryPool() {
	entity, err := s.Fixtures.Repos[0].GetEntity()
	s.Require().Nil(err)
	repoPool, err := s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create repo pool: %v", err))
	}

	pool, err := s.Store.UpdateEntityPool(s.adminCtx, entity, repoPool.ID, s.Fixtures.UpdatePoolParams)

	s.Require().Nil(err)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MaxRunners, pool.MaxRunners)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MinIdleRunners, pool.MinIdleRunners)
	s.Require().Equal(s.Fixtures.UpdatePoolParams.Image, pool.Image)
	s.Require().Equal(s.Fixtures.UpdatePoolParams.Flavor, pool.Flavor)
}

func (s *RepoTestSuite) TestUpdateRepositoryPoolInvalidRepoID() {
	entity := params.GithubEntity{
		ID:         "dummy-repo-id",
		EntityType: params.GithubEntityTypeRepository,
	}
	_, err := s.Store.UpdateEntityPool(s.adminCtx, entity, "dummy-repo-id", s.Fixtures.UpdatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: parsing id: invalid request", err.Error())
}

func TestRepoTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(RepoTestSuite))
}
