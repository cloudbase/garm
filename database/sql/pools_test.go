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
	"encoding/json"
	"flag"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
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
	Store dbCommon.Store
	ctx   context.Context

	StoreSQLMocked *sqlDatabase
	Fixtures       *PoolsTestFixtures
	adminCtx       context.Context
}

func (s *PoolsTestSuite) assertSQLMockExpectations() {
	err := s.Fixtures.SQLMock.ExpectationsWereMet()
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to meet sqlmock expectations, got error: %v", err))
	}
}

func (s *PoolsTestSuite) TearDownTest() {
	watcher.CloseWatcher()
}

func (s *PoolsTestSuite) SetupTest() {
	// create testing sqlite database
	ctx := context.Background()
	watcher.InitWatcher(ctx)

	db, err := NewSQLDatabase(context.Background(), garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.Store = db
	s.ctx = garmTesting.ImpersonateAdminContext(ctx, s.Store, s.T())

	adminCtx := garmTesting.ImpersonateAdminContext(context.Background(), db, s.T())
	s.adminCtx = adminCtx

	githubEndpoint := garmTesting.CreateDefaultGithubEndpoint(adminCtx, db, s.T())
	creds := garmTesting.CreateTestGithubCredentials(adminCtx, "new-creds", db, s.T(), githubEndpoint)

	// create an organization for testing purposes
	org, err := s.Store.CreateOrganization(s.adminCtx, "test-org", creds, "test-webhookSecret", params.PoolBalancerTypeRoundRobin, false)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create org: %s", err))
	}

	entity, err := org.GetEntity()
	s.Require().Nil(err)
	// create some pool objects in the database, for testing purposes
	orgPools := []params.Pool{}
	for i := 1; i <= 3; i++ {
		pool, err := db.CreateEntityPool(
			s.adminCtx,
			entity,
			params.CreatePoolParams{
				ProviderName:   "test-provider",
				MaxRunners:     4,
				MinIdleRunners: 2,
				Image:          fmt.Sprintf("test-image-%d", i),
				Flavor:         "test-flavor",
				OSType:         "linux",
				Tags:           []string{"amd64-linux-runner"},
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
	pools, err := s.Store.ListAllPools(s.adminCtx)

	s.Require().Nil(err)
	garmTesting.EqualDBEntityID(s.T(), s.Fixtures.Pools, pools)
}

func (s *PoolsTestSuite) TestListAllPoolsDBFetchErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT `pools`.`id`,`pools`.`created_at`,`pools`.`updated_at`,`pools`.`deleted_at`,`pools`.`provider_name`,`pools`.`runner_prefix`,`pools`.`max_runners`,`pools`.`min_idle_runners`,`pools`.`runner_bootstrap_timeout`,`pools`.`image`,`pools`.`flavor`,`pools`.`os_type`,`pools`.`os_arch`,`pools`.`enabled`,`pools`.`git_hub_runner_group`,`pools`.`repo_id`,`pools`.`org_id`,`pools`.`enterprise_id`,`pools`.`template_id`,`pools`.`priority` FROM `pools` WHERE `pools`.`deleted_at` IS NULL")).
		WillReturnError(fmt.Errorf("mocked fetching all pools error"))

	_, err := s.StoreSQLMocked.ListAllPools(s.adminCtx)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("error fetching all pools: mocked fetching all pools error", err.Error())
}

func (s *PoolsTestSuite) TestGetPoolByID() {
	pool, err := s.Store.GetPoolByID(s.adminCtx, s.Fixtures.Pools[0].ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.Pools[0].ID, pool.ID)
}

func (s *PoolsTestSuite) TestGetPoolByIDInvalidPoolID() {
	_, err := s.Store.GetPoolByID(s.adminCtx, "dummy-pool-id")

	s.Require().NotNil(err)
	s.Require().Equal("error fetching pool by ID: error parsing id: invalid request", err.Error())
}

func (s *PoolsTestSuite) TestDeletePoolByID() {
	err := s.Store.DeletePoolByID(s.adminCtx, s.Fixtures.Pools[0].ID)

	s.Require().Nil(err)
	_, err = s.Store.GetPoolByID(s.adminCtx, s.Fixtures.Pools[0].ID)
	s.Require().Equal("error fetching pool by ID: not found", err.Error())
}

func (s *PoolsTestSuite) TestDeletePoolByIDInvalidPoolID() {
	err := s.Store.DeletePoolByID(s.adminCtx, "dummy-pool-id")

	s.Require().NotNil(err)
	s.Require().Equal("error fetching pool by ID: error parsing id: invalid request", err.Error())
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

	err := s.StoreSQLMocked.DeletePoolByID(s.adminCtx, s.Fixtures.Pools[0].ID)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("error removing pool: mocked removing pool error", err.Error())
}

func (s *PoolsTestSuite) TestEntityPoolOperations() {
	ep := garmTesting.CreateDefaultGithubEndpoint(s.ctx, s.Store, s.T())
	creds := garmTesting.CreateTestGithubCredentials(s.ctx, "test-creds", s.Store, s.T(), ep)
	s.T().Cleanup(func() { s.Store.DeleteGithubCredentials(s.ctx, creds.ID) })
	repo, err := s.Store.CreateRepository(s.ctx, "test-owner", "test-repo", creds, "test-secret", params.PoolBalancerTypeRoundRobin, false)
	s.Require().NoError(err)
	s.Require().NotEmpty(repo.ID)
	s.T().Cleanup(func() { s.Store.DeleteRepository(s.ctx, repo.ID) })

	entity, err := repo.GetEntity()
	s.Require().NoError(err)

	createPoolParams := params.CreatePoolParams{
		ProviderName: "test-provider",
		MaxRunners:   5,
		Image:        "test-image",
		Flavor:       "test-flavor",
		OSType:       commonParams.Linux,
		OSArch:       commonParams.Amd64,
		Tags:         []string{"test-tag"},
	}

	pool, err := s.Store.CreateEntityPool(s.ctx, entity, createPoolParams)
	s.Require().NoError(err)
	s.Require().NotEmpty(pool.ID)
	s.T().Cleanup(func() { s.Store.DeleteEntityPool(s.ctx, entity, pool.ID) })

	entityPool, err := s.Store.GetEntityPool(s.ctx, entity, pool.ID)
	s.Require().NoError(err)
	s.Require().Equal(pool.ID, entityPool.ID)
	s.Require().Equal(pool.ProviderName, entityPool.ProviderName)

	updatePoolParams := params.UpdatePoolParams{
		Enabled: garmTesting.Ptr(true),
		Flavor:  "new-flavor",
		Image:   "new-image",
		RunnerPrefix: params.RunnerPrefix{
			Prefix: "new-prefix",
		},
		MaxRunners:             garmTesting.Ptr(uint(100)),
		MinIdleRunners:         garmTesting.Ptr(uint(50)),
		OSType:                 commonParams.Windows,
		OSArch:                 commonParams.Amd64,
		Tags:                   []string{"new-tag"},
		RunnerBootstrapTimeout: garmTesting.Ptr(uint(10)),
		ExtraSpecs:             json.RawMessage(`{"extra": "specs"}`),
		GitHubRunnerGroup:      garmTesting.Ptr("new-group"),
		Priority:               garmTesting.Ptr(uint(1)),
	}
	pool, err = s.Store.UpdateEntityPool(s.ctx, entity, pool.ID, updatePoolParams)
	s.Require().NoError(err)
	s.Require().Equal(*updatePoolParams.Enabled, pool.Enabled)
	s.Require().Equal(updatePoolParams.Flavor, pool.Flavor)
	s.Require().Equal(updatePoolParams.Image, pool.Image)
	s.Require().Equal(updatePoolParams.Prefix, pool.Prefix)
	s.Require().Equal(*updatePoolParams.MaxRunners, pool.MaxRunners)
	s.Require().Equal(*updatePoolParams.MinIdleRunners, pool.MinIdleRunners)
	s.Require().Equal(updatePoolParams.OSType, pool.OSType)
	s.Require().Equal(updatePoolParams.OSArch, pool.OSArch)
	s.Require().Equal(*updatePoolParams.RunnerBootstrapTimeout, pool.RunnerBootstrapTimeout)
	s.Require().Equal(updatePoolParams.ExtraSpecs, pool.ExtraSpecs)
	s.Require().Equal(*updatePoolParams.GitHubRunnerGroup, pool.GitHubRunnerGroup)
	s.Require().Equal(*updatePoolParams.Priority, pool.Priority)

	entityPools, err := s.Store.ListEntityPools(s.ctx, entity)
	s.Require().NoError(err)
	s.Require().Len(entityPools, 1)
	s.Require().Equal(pool.ID, entityPools[0].ID)

	tagsToMatch := []string{"new-tag"}
	pools, err := s.Store.FindPoolsMatchingAllTags(s.ctx, entity.EntityType, entity.ID, tagsToMatch)
	s.Require().NoError(err)
	s.Require().Len(pools, 1)
	s.Require().Equal(pool.ID, pools[0].ID)

	invalidTagsToMatch := []string{"invalid-tag"}
	pools, err = s.Store.FindPoolsMatchingAllTags(s.ctx, entity.EntityType, entity.ID, invalidTagsToMatch)
	s.Require().NoError(err)
	s.Require().Len(pools, 0)
}

func (s *PoolsTestSuite) TestListEntityInstances() {
	ep := garmTesting.CreateDefaultGithubEndpoint(s.ctx, s.Store, s.T())
	creds := garmTesting.CreateTestGithubCredentials(s.ctx, "test-creds", s.Store, s.T(), ep)
	s.T().Cleanup(func() { s.Store.DeleteGithubCredentials(s.ctx, creds.ID) })
	repo, err := s.Store.CreateRepository(s.ctx, "test-owner", "test-repo", creds, "test-secret", params.PoolBalancerTypeRoundRobin, false)
	s.Require().NoError(err)
	s.Require().NotEmpty(repo.ID)
	s.T().Cleanup(func() { s.Store.DeleteRepository(s.ctx, repo.ID) })

	entity, err := repo.GetEntity()
	s.Require().NoError(err)

	createPoolParams := params.CreatePoolParams{
		ProviderName: "test-provider",
		MaxRunners:   5,
		Image:        "test-image",
		Flavor:       "test-flavor",
		OSType:       commonParams.Linux,
		OSArch:       commonParams.Amd64,
		Tags:         []string{"test-tag"},
	}

	pool, err := s.Store.CreateEntityPool(s.ctx, entity, createPoolParams)
	s.Require().NoError(err)
	s.Require().NotEmpty(pool.ID)
	s.T().Cleanup(func() { s.Store.DeleteEntityPool(s.ctx, entity, pool.ID) })

	createInstanceParams := params.CreateInstanceParams{
		Name:   "test-instance",
		OSType: commonParams.Linux,
		OSArch: commonParams.Amd64,
		Status: commonParams.InstanceCreating,
	}
	instance, err := s.Store.CreateInstance(s.ctx, pool.ID, createInstanceParams)
	s.Require().NoError(err)
	s.Require().NotEmpty(instance.ID)

	s.T().Cleanup(func() { s.Store.DeleteInstance(s.ctx, pool.ID, instance.ID) })

	instances, err := s.Store.ListEntityInstances(s.ctx, entity)
	s.Require().NoError(err)
	s.Require().Len(instances, 1)
	s.Require().Equal(instance.ID, instances[0].ID)
	s.Require().Equal(instance.Name, instances[0].Name)
	s.Require().Equal(instance.ProviderName, pool.ProviderName)
}

func TestPoolsTestSuite(t *testing.T) {
	suite.Run(t, new(PoolsTestSuite))
}
