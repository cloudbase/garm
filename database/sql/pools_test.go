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

type PoolsTestFixtures struct {
	Org     params.Organization
	Pools   []params.Pool
	SQLMock sqlmock.Sqlmock
}

type PoolsTestSuite struct {
	suite.Suite
	Store          dbCommon.Store
	StoreSQLMocked *sqlDatabase
	Fixtures       *PoolsTestFixtures
}

func (s *PoolsTestSuite) assertSQLMockExpectations() {
	err := s.Fixtures.SQLMock.ExpectationsWereMet()
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to meet sqlmock expectations, got error: %v", err))
	}
}

func (s *PoolsTestSuite) SetupTest() {
	// create testing sqlite database
	db, err := NewSQLDatabase(context.Background(), garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.Store = db

	// create an organization for testing purposes
	org, err := s.Store.CreateOrganization(context.Background(), "test-org", "test-creds", "test-webhookSecret", params.PoolBalancerTypeRoundRobin)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create org: %s", err))
	}

	entity, err := org.GetEntity()
	s.Require().Nil(err)
	// create some pool objects in the database, for testing purposes
	orgPools := []params.Pool{}
	for i := 1; i <= 3; i++ {
		pool, err := db.CreateEntityPool(
			context.Background(),
			entity,
			params.CreatePoolParams{
				ProviderName:   "test-provider",
				MaxRunners:     4,
				MinIdleRunners: 2,
				Image:          fmt.Sprintf("test-image-%d", i),
				Flavor:         "test-flavor",
				OSType:         "linux",
				Tags:           []string{"self-hosted", "amd64", "linux"},
			},
		)
		if err != nil {
			s.FailNow(fmt.Sprintf("cannot create org pool: %v", err))
		}
		orgPools = append(orgPools, pool)
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
	}

	// setup test fixtures
	fixtures := &PoolsTestFixtures{
		Org:     org,
		Pools:   orgPools,
		SQLMock: sqlMock,
	}
	s.Fixtures = fixtures
}

func (s *PoolsTestSuite) TestListAllPools() {
	pools, err := s.Store.ListAllPools(context.Background())

	s.Require().Nil(err)
	garmTesting.EqualDBEntityID(s.T(), s.Fixtures.Pools, pools)
}

func (s *PoolsTestSuite) TestListAllPoolsDBFetchErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT `pools`.`id`,`pools`.`created_at`,`pools`.`updated_at`,`pools`.`deleted_at`,`pools`.`provider_name`,`pools`.`runner_prefix`,`pools`.`max_runners`,`pools`.`min_idle_runners`,`pools`.`runner_bootstrap_timeout`,`pools`.`image`,`pools`.`flavor`,`pools`.`os_type`,`pools`.`os_arch`,`pools`.`enabled`,`pools`.`git_hub_runner_group`,`pools`.`repo_id`,`pools`.`org_id`,`pools`.`enterprise_id`,`pools`.`priority` FROM `pools` WHERE `pools`.`deleted_at` IS NULL")).
		WillReturnError(fmt.Errorf("mocked fetching all pools error"))

	_, err := s.StoreSQLMocked.ListAllPools(context.Background())

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("fetching all pools: mocked fetching all pools error", err.Error())
}

func (s *PoolsTestSuite) TestGetPoolByID() {
	pool, err := s.Store.GetPoolByID(context.Background(), s.Fixtures.Pools[0].ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.Pools[0].ID, pool.ID)
}

func (s *PoolsTestSuite) TestGetPoolByIDInvalidPoolID() {
	_, err := s.Store.GetPoolByID(context.Background(), "dummy-pool-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool by ID: parsing id: invalid request", err.Error())
}

func (s *PoolsTestSuite) TestDeletePoolByID() {
	err := s.Store.DeletePoolByID(context.Background(), s.Fixtures.Pools[0].ID)

	s.Require().Nil(err)
	_, err = s.Store.GetPoolByID(context.Background(), s.Fixtures.Pools[0].ID)
	s.Require().Equal("fetching pool by ID: not found", err.Error())
}

func (s *PoolsTestSuite) TestDeletePoolByIDInvalidPoolID() {
	err := s.Store.DeletePoolByID(context.Background(), "dummy-pool-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching pool by ID: parsing id: invalid request", err.Error())
}

func (s *PoolsTestSuite) TestDeletePoolByIDDBRemoveErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE id = ? AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Pools[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Pools[0].ID))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("DELETE FROM `pools` WHERE `pools`.`id` = ?")).
		WillReturnError(fmt.Errorf("mocked removing pool error"))
	s.Fixtures.SQLMock.ExpectRollback()

	err := s.StoreSQLMocked.DeletePoolByID(context.Background(), s.Fixtures.Pools[0].ID)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("removing pool: mocked removing pool error", err.Error())
}

func TestPoolsTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(PoolsTestSuite))
}
