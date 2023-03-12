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

	dbCommon "github.com/cloudbase/garm/database/common"
	runnerErrors "github.com/cloudbase/garm/errors"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"

	"github.com/stretchr/testify/suite"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type OrgTestFixtures struct {
	Orgs                 []params.Organization
	CreateOrgParams      params.CreateOrgParams
	CreatePoolParams     params.CreatePoolParams
	CreateInstanceParams params.CreateInstanceParams
	UpdateRepoParams     params.UpdateRepositoryParams
	UpdatePoolParams     params.UpdatePoolParams
	SQLMock              sqlmock.Sqlmock
}

type OrgTestSuite struct {
	suite.Suite
	Store          dbCommon.Store
	StoreSQLMocked *sqlDatabase
	Fixtures       *OrgTestFixtures
}

func (s *OrgTestSuite) equalInstancesByName(expected, actual []params.Instance) {
	s.Require().Equal(len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].Name > expected[j].Name })
	sort.Slice(actual, func(i, j int) bool { return actual[i].Name > actual[j].Name })

	for i := 0; i < len(expected); i++ {
		s.Require().Equal(expected[i].Name, actual[i].Name)
	}
}

func (s *OrgTestSuite) assertSQLMockExpectations() {
	err := s.Fixtures.SQLMock.ExpectationsWereMet()
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to meet sqlmock expectations, got error: %v", err))
	}
}

func (s *OrgTestSuite) SetupTest() {
	// create testing sqlite database
	db, err := NewSQLDatabase(context.Background(), garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.Store = db

	// create some organization objects in the database, for testing purposes
	orgs := []params.Organization{}
	for i := 1; i <= 3; i++ {
		org, err := db.CreateOrganization(
			context.Background(),
			fmt.Sprintf("test-org-%d", i),
			fmt.Sprintf("test-creds-%d", i),
			fmt.Sprintf("test-webhook-secret-%d", i),
		)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to create database object (test-org-%d)", i))
		}

		orgs = append(orgs, org)
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
	if flag.Lookup("test.v").Value.String() == "false" {
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
	var maxRunners uint = 30
	var minIdleRunners uint = 10
	fixtures := &OrgTestFixtures{
		Orgs: orgs,
		CreateOrgParams: params.CreateOrgParams{
			Name:            "new-test-org",
			CredentialsName: "new-creds",
			WebhookSecret:   "new-webhook-secret",
		},
		CreatePoolParams: params.CreatePoolParams{
			ProviderName:   "test-provider",
			MaxRunners:     3,
			MinIdleRunners: 1,
			Image:          "test-image",
			Flavor:         "test-flavor",
			OSType:         "linux",
			OSArch:         "amd64",
			Tags:           []string{"self-hosted", "arm64", "linux"},
		},
		CreateInstanceParams: params.CreateInstanceParams{
			Name:   "test-instance-name",
			OSType: "linux",
		},
		UpdateRepoParams: params.UpdateRepositoryParams{
			CredentialsName: "test-update-creds",
			WebhookSecret:   "test-update-repo-webhook-secret",
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

func (s *OrgTestSuite) TestCreateOrganization() {
	// call tested function
	org, err := s.Store.CreateOrganization(
		context.Background(),
		s.Fixtures.CreateOrgParams.Name,
		s.Fixtures.CreateOrgParams.CredentialsName,
		s.Fixtures.CreateOrgParams.WebhookSecret)

	// assertions
	s.Require().Nil(err)
	storeOrg, err := s.Store.GetOrganizationByID(context.Background(), org.ID)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get organization by id: %v", err))
	}
	s.Require().Equal(storeOrg.Name, org.Name)
	s.Require().Equal(storeOrg.CredentialsName, org.CredentialsName)
	s.Require().Equal(storeOrg.WebhookSecret, org.WebhookSecret)
}

func (s *OrgTestSuite) TestCreateOrganizationInvalidDBPassphrase() {
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

	_, err = sqlDB.CreateOrganization(
		context.Background(),
		s.Fixtures.CreateOrgParams.Name,
		s.Fixtures.CreateOrgParams.CredentialsName,
		s.Fixtures.CreateOrgParams.WebhookSecret)

	s.Require().NotNil(err)
	s.Require().Equal("failed to encrypt string", err.Error())
}

func (s *OrgTestSuite) TestCreateOrganizationDBCreateErr() {
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `organizations`")).
		WillReturnError(fmt.Errorf("creating org mock error"))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.CreateOrganization(
		context.Background(),
		s.Fixtures.CreateOrgParams.Name,
		s.Fixtures.CreateOrgParams.CredentialsName,
		s.Fixtures.CreateOrgParams.WebhookSecret)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("creating org: creating org mock error", err.Error())
}

func (s *OrgTestSuite) TestGetOrganization() {
	org, err := s.Store.GetOrganization(context.Background(), s.Fixtures.Orgs[0].Name)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.Orgs[0].Name, org.Name)
	s.Require().Equal(s.Fixtures.Orgs[0].ID, org.ID)
}

func (s *OrgTestSuite) TestGetOrganizationCaseInsensitive() {
	org, err := s.Store.GetOrganization(context.Background(), "TeSt-oRg-1")

	s.Require().Nil(err)
	s.Require().Equal("test-org-1", org.Name)
}

func (s *OrgTestSuite) TestGetOrganizationNotFound() {
	_, err := s.Store.GetOrganization(context.Background(), "dummy-name")

	s.Require().NotNil(err)
	s.Require().Equal("fetching org: not found", err.Error())
}

func (s *OrgTestSuite) TestGetOrganizationDBDecryptingErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE name = ? COLLATE NOCASE AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].Name).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow(s.Fixtures.Orgs[0].Name))

	_, err := s.StoreSQLMocked.GetOrganization(context.Background(), s.Fixtures.Orgs[0].Name)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("fetching org: missing secret", err.Error())
}

func (s *OrgTestSuite) TestListOrganizations() {
	orgs, err := s.Store.ListOrganizations(context.Background())

	s.Require().Nil(err)
	garmTesting.EqualDBEntityByName(s.T(), s.Fixtures.Orgs, orgs)
}

func (s *OrgTestSuite) TestListOrganizationsDBFetchErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE `organizations`.`deleted_at` IS NULL")).
		WillReturnError(fmt.Errorf("fetching user from database mock error"))

	_, err := s.StoreSQLMocked.ListOrganizations(context.Background())

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("fetching org from database: fetching user from database mock error", err.Error())
}

func (s *OrgTestSuite) TestDeleteOrganization() {
	err := s.Store.DeleteOrganization(context.Background(), s.Fixtures.Orgs[0].ID)

	s.Require().Nil(err)
	_, err = s.Store.GetOrganizationByID(context.Background(), s.Fixtures.Orgs[0].ID)
	s.Require().NotNil(err)
	s.Require().Equal("fetching org: not found", err.Error())
}

func (s *OrgTestSuite) TestDeleteOrganizationInvalidOrgID() {
	err := s.Store.DeleteOrganization(context.Background(), "dummy-org-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching org: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestDeleteOrganizationDBDeleteErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("DELETE FROM `organizations`")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnError(fmt.Errorf("mocked delete org error"))
	s.Fixtures.SQLMock.ExpectRollback()

	err := s.StoreSQLMocked.DeleteOrganization(context.Background(), s.Fixtures.Orgs[0].ID)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("deleting org: mocked delete org error", err.Error())
}

func (s *OrgTestSuite) TestUpdateOrganization() {
	org, err := s.Store.UpdateOrganization(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.UpdateRepoParams)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.CredentialsName, org.CredentialsName)
	s.Require().Equal(s.Fixtures.UpdateRepoParams.WebhookSecret, org.WebhookSecret)
}

func (s *OrgTestSuite) TestUpdateOrganizationInvalidOrgID() {
	_, err := s.Store.UpdateOrganization(context.Background(), "dummy-org-id", s.Fixtures.UpdateRepoParams)

	s.Require().NotNil(err)
	s.Require().Equal("fetching org: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestUpdateOrganizationDBEncryptErr() {
	s.StoreSQLMocked.cfg.Passphrase = "wrong-passphrase"

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))

	_, err := s.StoreSQLMocked.UpdateOrganization(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.UpdateRepoParams)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("saving org: failed to encrypt string: invalid passphrase length (expected length 32 characters)", err.Error())
}

func (s *OrgTestSuite) TestUpdateOrganizationDBSaveErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(("UPDATE `organizations` SET")).
		WillReturnError(fmt.Errorf("saving org mock error"))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.UpdateOrganization(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.UpdateRepoParams)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("saving org: saving org mock error", err.Error())
}

func (s *OrgTestSuite) TestUpdateOrganizationDBDecryptingErr() {
	s.StoreSQLMocked.cfg.Passphrase = "wrong-passphrase"
	s.Fixtures.UpdateRepoParams.WebhookSecret = "webhook-secret"

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))

	_, err := s.StoreSQLMocked.UpdateOrganization(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.UpdateRepoParams)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("saving org: failed to encrypt string: invalid passphrase length (expected length 32 characters)", err.Error())
}

func (s *OrgTestSuite) TestGetOrganizationByID() {
	org, err := s.Store.GetOrganizationByID(context.Background(), s.Fixtures.Orgs[0].ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.Orgs[0].ID, org.ID)
}

func (s *OrgTestSuite) TestGetOrganizationByIDInvalidOrgID() {
	_, err := s.Store.GetOrganizationByID(context.Background(), "dummy-org-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching org: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestGetOrganizationByIDDBDecryptingErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE `pools`.`org_id` = ? AND `pools`.`deleted_at` IS NULL")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"org_id"}).AddRow(s.Fixtures.Orgs[0].ID))

	_, err := s.StoreSQLMocked.GetOrganizationByID(context.Background(), s.Fixtures.Orgs[0].ID)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("fetching enterprise: missing secret", err.Error())
}

func (s *OrgTestSuite) TestCreateOrganizationPool() {
	pool, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)

	s.Require().Nil(err)

	org, err := s.Store.GetOrganizationByID(context.Background(), s.Fixtures.Orgs[0].ID)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot get org by ID: %v", err))
	}
	s.Require().Equal(1, len(org.Pools))
	s.Require().Equal(pool.ID, org.Pools[0].ID)
	s.Require().Equal(s.Fixtures.CreatePoolParams.ProviderName, org.Pools[0].ProviderName)
	s.Require().Equal(s.Fixtures.CreatePoolParams.MaxRunners, org.Pools[0].MaxRunners)
	s.Require().Equal(s.Fixtures.CreatePoolParams.MinIdleRunners, org.Pools[0].MinIdleRunners)
}

func (s *OrgTestSuite) TestCreateOrganizationPoolMissingTags() {
	s.Fixtures.CreatePoolParams.Tags = []string{}

	_, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("no tags specified", err.Error())
}

func (s *OrgTestSuite) TestCreateOrganizationPoolInvalidOrgID() {
	_, err := s.Store.CreateOrganizationPool(context.Background(), "dummy-org-id", s.Fixtures.CreatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("fetching org: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestCreateOrganizationPoolDBCreateErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE `pools`.`org_id` = ? AND (provider_name = ? and image = ? and flavor = ?) AND `pools`.`deleted_at` IS NULL")).
		WillReturnError(fmt.Errorf("mocked creating pool error"))

	_, err := s.StoreSQLMocked.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("creating pool: fetching pool: mocked creating pool error", err.Error())
}

func (s *OrgTestSuite) TestCreateOrganizationDBPoolAlreadyExistErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE `pools`.`org_id` = ? AND (provider_name = ? and image = ? and flavor = ?) AND `pools`.`deleted_at` IS NULL")).
		WithArgs(
			s.Fixtures.Orgs[0].ID,
			s.Fixtures.CreatePoolParams.ProviderName,
			s.Fixtures.CreatePoolParams.Image,
			s.Fixtures.CreatePoolParams.Flavor).
		WillReturnRows(sqlmock.NewRows([]string{"org_id", "provider_name", "image", "flavor"}).
			AddRow(
				s.Fixtures.Orgs[0].ID,
				s.Fixtures.CreatePoolParams.ProviderName,
				s.Fixtures.CreatePoolParams.Image,
				s.Fixtures.CreatePoolParams.Flavor))

	_, err := s.StoreSQLMocked.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal(runnerErrors.NewConflictError("pool with the same image and flavor already exists on this provider"), err)
}

func (s *OrgTestSuite) TestCreateOrganizationPoolDBFetchTagErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE `pools`.`org_id` = ? AND (provider_name = ? and image = ? and flavor = ?) AND `pools`.`deleted_at` IS NULL")).
		WithArgs(
			s.Fixtures.Orgs[0].ID,
			s.Fixtures.CreatePoolParams.ProviderName,
			s.Fixtures.CreatePoolParams.Image,
			s.Fixtures.CreatePoolParams.Flavor).
		WillReturnRows(sqlmock.NewRows([]string{"org_id"}))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `tags` WHERE name = ? AND `tags`.`deleted_at` IS NULL ORDER BY `tags`.`id` LIMIT 1")).
		WillReturnError(fmt.Errorf("mocked fetching tag error"))

	_, err := s.StoreSQLMocked.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("fetching tag: fetching tag from database: mocked fetching tag error", err.Error())
}

func (s *OrgTestSuite) TestCreateOrganizationPoolDBAddingPoolErr() {
	s.Fixtures.CreatePoolParams.Tags = []string{"linux"}

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE `pools`.`org_id` = ? AND (provider_name = ? and image = ? and flavor = ?) AND `pools`.`deleted_at` IS NULL")).
		WithArgs(
			s.Fixtures.Orgs[0].ID,
			s.Fixtures.CreatePoolParams.ProviderName,
			s.Fixtures.CreatePoolParams.Image,
			s.Fixtures.CreatePoolParams.Flavor).
		WillReturnRows(sqlmock.NewRows([]string{"org_id"}))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `tags` WHERE name = ? AND `tags`.`deleted_at` IS NULL ORDER BY `tags`.`id` LIMIT 1")).
		WillReturnRows(sqlmock.NewRows([]string{"linux"}))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `tags`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.ExpectCommit()
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `pools`")).
		WillReturnError(fmt.Errorf("mocked adding pool error"))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("adding pool: mocked adding pool error", err.Error())
}

func (s *OrgTestSuite) TestCreateOrganizationPoolDBSaveTagErr() {
	s.Fixtures.CreatePoolParams.Tags = []string{"linux"}

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE `pools`.`org_id` = ? AND (provider_name = ? and image = ? and flavor = ?) AND `pools`.`deleted_at` IS NULL")).
		WithArgs(
			s.Fixtures.Orgs[0].ID,
			s.Fixtures.CreatePoolParams.ProviderName,
			s.Fixtures.CreatePoolParams.Image,
			s.Fixtures.CreatePoolParams.Flavor).
		WillReturnRows(sqlmock.NewRows([]string{"org_id"}))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `tags` WHERE name = ? AND `tags`.`deleted_at` IS NULL ORDER BY `tags`.`id` LIMIT 1")).
		WillReturnRows(sqlmock.NewRows([]string{"linux"}))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `tags`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.ExpectCommit()
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `pools`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.ExpectCommit()
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("UPDATE `pools` SET")).
		WillReturnError(fmt.Errorf("mocked saving tag error"))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("saving tag: mocked saving tag error", err.Error())
}

func (s *OrgTestSuite) TestCreateOrganizationPoolDBFetchPoolErr() {
	s.Fixtures.CreatePoolParams.Tags = []string{"linux"}

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `organizations` WHERE id = ? AND `organizations`.`deleted_at` IS NULL ORDER BY `organizations`.`id` LIMIT 1")).
		WithArgs(s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Orgs[0].ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE `pools`.`org_id` = ? AND (provider_name = ? and image = ? and flavor = ?) AND `pools`.`deleted_at` IS NULL")).
		WithArgs(
			s.Fixtures.Orgs[0].ID,
			s.Fixtures.CreatePoolParams.ProviderName,
			s.Fixtures.CreatePoolParams.Image,
			s.Fixtures.CreatePoolParams.Flavor).
		WillReturnRows(sqlmock.NewRows([]string{"org_id"}))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `tags` WHERE name = ? AND `tags`.`deleted_at` IS NULL ORDER BY `tags`.`id` LIMIT 1")).
		WillReturnRows(sqlmock.NewRows([]string{"linux"}))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `tags`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.ExpectCommit()
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `pools`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.ExpectCommit()
	s.Fixtures.SQLMock.ExpectBegin()
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
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE id = ? AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT 1")).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := s.StoreSQLMocked.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: not found", err.Error())
}

func (s *OrgTestSuite) TestListOrgPools() {
	orgPools := []params.Pool{}
	for i := 1; i <= 2; i++ {
		s.Fixtures.CreatePoolParams.Flavor = fmt.Sprintf("test-flavor-%v", i)
		pool, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
		}
		orgPools = append(orgPools, pool)
	}

	pools, err := s.Store.ListOrgPools(context.Background(), s.Fixtures.Orgs[0].ID)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityID(s.T(), orgPools, pools)
}

func (s *OrgTestSuite) TestListOrgPoolsInvalidOrgID() {
	_, err := s.Store.ListOrgPools(context.Background(), "dummy-org-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching pools: fetching org: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestGetOrganizationPool() {
	pool, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
	}

	orgPool, err := s.Store.GetOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, pool.ID)

	s.Require().Nil(err)
	s.Require().Equal(orgPool.ID, pool.ID)
}

func (s *OrgTestSuite) TestGetOrganizationPoolInvalidOrgID() {
	_, err := s.Store.GetOrganizationPool(context.Background(), "dummy-org-id", "dummy-pool-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestDeleteOrganizationPool() {
	pool, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
	}

	err = s.Store.DeleteOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, pool.ID)

	s.Require().Nil(err)
	_, err = s.Store.GetOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, pool.ID)
	s.Require().Equal("fetching pool: finding pool: not found", err.Error())
}

func (s *OrgTestSuite) TestDeleteOrganizationPoolInvalidOrgID() {
	err := s.Store.DeleteOrganizationPool(context.Background(), "dummy-org-id", "dummy-pool-id")

	s.Require().NotNil(err)
	s.Require().Equal("looking up org pool: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestDeleteOrganizationPoolDBDeleteErr() {
	pool, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
	}

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE (id = ? and org_id = ?) AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT 1")).
		WithArgs(pool.ID, s.Fixtures.Orgs[0].ID).
		WillReturnRows(sqlmock.NewRows([]string{"org_id", "id"}).AddRow(s.Fixtures.Orgs[0].ID, pool.ID))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("DELETE FROM `pools` WHERE `pools`.`id` = ?")).
		WithArgs(pool.ID).
		WillReturnError(fmt.Errorf("mocked deleting pool error"))
	s.Fixtures.SQLMock.ExpectRollback()

	err = s.StoreSQLMocked.DeleteOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, pool.ID)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("deleting pool: mocked deleting pool error", err.Error())
}

func (s *OrgTestSuite) TestFindOrganizationPoolByTags() {
	orgPool, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
	}

	pool, err := s.Store.FindOrganizationPoolByTags(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams.Tags)

	s.Require().Nil(err)
	s.Require().Equal(orgPool.ID, pool.ID)
	s.Require().Equal(orgPool.Image, pool.Image)
	s.Require().Equal(orgPool.Flavor, pool.Flavor)
}

func (s *OrgTestSuite) TestFindOrganizationPoolByTagsMissingTags() {
	tags := []string{}

	_, err := s.Store.FindOrganizationPoolByTags(context.Background(), s.Fixtures.Orgs[0].ID, tags)

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: missing tags", err.Error())
}

func (s *OrgTestSuite) TestListOrgInstances() {
	pool, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
	}
	poolInstances := []params.Instance{}
	for i := 1; i <= 3; i++ {
		s.Fixtures.CreateInstanceParams.Name = fmt.Sprintf("test-org-%v", i)
		instance, err := s.Store.CreateInstance(context.Background(), pool.ID, s.Fixtures.CreateInstanceParams)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create instance: %s", err))
		}
		poolInstances = append(poolInstances, instance)
	}

	instances, err := s.Store.ListOrgInstances(context.Background(), s.Fixtures.Orgs[0].ID)

	s.Require().Nil(err)
	s.equalInstancesByName(poolInstances, instances)
}

func (s *OrgTestSuite) TestListOrgInstancesInvalidOrgID() {
	_, err := s.Store.ListOrgInstances(context.Background(), "dummy-org-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching org: fetching org: parsing id: invalid request", err.Error())
}

func (s *OrgTestSuite) TestUpdateOrganizationPool() {
	pool, err := s.Store.CreateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, s.Fixtures.CreatePoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
	}

	pool, err = s.Store.UpdateOrganizationPool(context.Background(), s.Fixtures.Orgs[0].ID, pool.ID, s.Fixtures.UpdatePoolParams)

	s.Require().Nil(err)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MaxRunners, pool.MaxRunners)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MinIdleRunners, pool.MinIdleRunners)
	s.Require().Equal(s.Fixtures.UpdatePoolParams.Image, pool.Image)
	s.Require().Equal(s.Fixtures.UpdatePoolParams.Flavor, pool.Flavor)
}

func (s *OrgTestSuite) TestUpdateOrganizationPoolInvalidOrgID() {
	_, err := s.Store.UpdateOrganizationPool(context.Background(), "dummy-org-id", "dummy-pool-id", s.Fixtures.UpdatePoolParams)

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool: parsing id: invalid request", err.Error())
}

func TestOrgTestSuite(t *testing.T) {
	suite.Run(t, new(OrgTestSuite))
}
