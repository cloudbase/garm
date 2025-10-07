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
	"sync"
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

type InstancesTestFixtures struct {
	Org                  params.Organization
	Pool                 params.Pool
	Instances            []params.Instance
	CreateInstanceParams params.CreateInstanceParams
	UpdateInstanceParams params.UpdateInstanceParams
	SQLMock              sqlmock.Sqlmock
}

type InstancesTestSuite struct {
	suite.Suite
	Store          dbCommon.Store
	StoreSQLMocked *sqlDatabase
	Fixtures       *InstancesTestFixtures
	adminCtx       context.Context
}

func (s *InstancesTestSuite) equalInstancesByName(expected, actual []params.Instance) {
	s.Require().Equal(len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].Name > expected[j].Name })
	sort.Slice(actual, func(i, j int) bool { return actual[i].Name > actual[j].Name })

	for i := 0; i < len(expected); i++ {
		s.Require().Equal(expected[i].Name, actual[i].Name)
	}
}

func (s *InstancesTestSuite) assertSQLMockExpectations() {
	err := s.Fixtures.SQLMock.ExpectationsWereMet()
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to meet sqlmock expectations, got error: %v", err))
	}
}

func (s *InstancesTestSuite) TearDownTest() {
	watcher.CloseWatcher()
}

func (s *InstancesTestSuite) SetupTest() {
	ctx := context.Background()
	watcher.InitWatcher(ctx)
	// create testing sqlite database
	db, err := NewSQLDatabase(ctx, garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.Store = db

	adminCtx := garmTesting.ImpersonateAdminContext(ctx, db, s.T())
	s.adminCtx = adminCtx

	githubEndpoint := garmTesting.CreateDefaultGithubEndpoint(adminCtx, db, s.T())
	creds := garmTesting.CreateTestGithubCredentials(adminCtx, "new-creds", db, s.T(), githubEndpoint)

	// create an organization for testing purposes
	org, err := s.Store.CreateOrganization(s.adminCtx, "test-org", creds, "test-webhookSecret", params.PoolBalancerTypeRoundRobin)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create org: %s", err))
	}

	// create an organization pool for testing purposes
	createPoolParams := params.CreatePoolParams{
		ProviderName:   "test-provider",
		MaxRunners:     4,
		MinIdleRunners: 2,
		Image:          "test-image",
		Flavor:         "test-flavor",
		OSType:         "linux",
		Tags:           []string{"amd64", "linux"},
	}
	entity, err := org.GetEntity()
	s.Require().Nil(err)
	pool, err := s.Store.CreateEntityPool(s.adminCtx, entity, createPoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create org pool: %s", err))
	}

	// create some instance objects in the database, for testing purposes
	instances := []params.Instance{}
	for i := 1; i <= 3; i++ {
		instance, err := db.CreateInstance(
			s.adminCtx,
			pool.ID,
			params.CreateInstanceParams{
				Name:         fmt.Sprintf("test-instance-%d", i),
				OSType:       "linux",
				OSArch:       "amd64",
				CallbackURL:  "https://garm.example.com/",
				Status:       commonParams.InstanceRunning,
				RunnerStatus: params.RunnerIdle,
				JitConfiguration: map[string]string{
					"secret": fmt.Sprintf("secret-%d", i),
				},
				AditionalLabels: []string{
					fmt.Sprintf("label-%d", i),
				},
			},
		)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to create instance object (test-instance-%v)", i))
		}
		instances = append(instances, instance)
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
	fixtures := &InstancesTestFixtures{
		Org:       org,
		Pool:      pool,
		Instances: instances,
		CreateInstanceParams: params.CreateInstanceParams{
			Name:        "test-create-instance",
			OSType:      "linux",
			OSArch:      "amd64",
			CallbackURL: "https://garm.example.com/",
		},
		UpdateInstanceParams: params.UpdateInstanceParams{
			ProviderID:    "update-provider-test",
			OSName:        "ubuntu",
			OSVersion:     "focal",
			Status:        commonParams.InstancePendingDelete,
			RunnerStatus:  params.RunnerActive,
			AgentID:       4,
			CreateAttempt: 3,
			Addresses: []commonParams.Address{
				{
					Address: "12.10.12.10",
					Type:    commonParams.PublicAddress,
				},
				{
					Address: "10.1.1.2",
					Type:    commonParams.PrivateAddress,
				},
			},
		},
		SQLMock: sqlMock,
	}
	s.Fixtures = fixtures
}

func (s *InstancesTestSuite) TestCreateInstance() {
	// call tested function
	instance, err := s.Store.CreateInstance(s.adminCtx, s.Fixtures.Pool.ID, s.Fixtures.CreateInstanceParams)

	// assertions
	s.Require().Nil(err)
	storeInstance, err := s.Store.GetInstance(s.adminCtx, s.Fixtures.CreateInstanceParams.Name)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get instance: %v", err))
	}
	s.Require().Equal(storeInstance.Name, instance.Name)
	s.Require().Equal(storeInstance.PoolID, instance.PoolID)
	s.Require().Equal(storeInstance.OSArch, instance.OSArch)
	s.Require().Equal(storeInstance.OSType, instance.OSType)
	s.Require().Equal(storeInstance.CallbackURL, instance.CallbackURL)
}

func (s *InstancesTestSuite) TestCreateInstanceInvalidPoolID() {
	_, err := s.Store.CreateInstance(s.adminCtx, "dummy-pool-id", params.CreateInstanceParams{})

	s.Require().Equal("error creating instance: error fetching pool: error parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestCreateInstanceMaxRunnersReached() {
	// Create a fourth instance (pool has max 4 runners, already has 3)
	fourthInstanceParams := params.CreateInstanceParams{
		Name:        "test-instance-4",
		OSType:      "linux",
		OSArch:      "amd64",
		CallbackURL: "https://garm.example.com/",
	}
	_, err := s.Store.CreateInstance(s.adminCtx, s.Fixtures.Pool.ID, fourthInstanceParams)
	s.Require().Nil(err)

	// Try to create a fifth instance, which should fail due to max runners limit
	fifthInstanceParams := params.CreateInstanceParams{
		Name:        "test-instance-5",
		OSType:      "linux",
		OSArch:      "amd64",
		CallbackURL: "https://garm.example.com/",
	}
	_, err = s.Store.CreateInstance(s.adminCtx, s.Fixtures.Pool.ID, fifthInstanceParams)
	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "max runners reached for pool")
}

func (s *InstancesTestSuite) TestCreateInstanceMaxRunnersReachedSpecificPool() {
	// Create a new pool with max runners set to 3
	createPoolParams := params.CreatePoolParams{
		ProviderName:   "test-provider",
		MaxRunners:     3,
		MinIdleRunners: 1,
		Image:          "test-image",
		Flavor:         "test-flavor",
		OSType:         "linux",
		Tags:           []string{"amd64", "linux"},
	}
	entity, err := s.Fixtures.Org.GetEntity()
	s.Require().Nil(err)
	testPool, err := s.Store.CreateEntityPool(s.adminCtx, entity, createPoolParams)
	s.Require().Nil(err)

	// Create exactly 3 instances (max limit)
	for i := 1; i <= 3; i++ {
		instanceParams := params.CreateInstanceParams{
			Name:        fmt.Sprintf("max-test-instance-%d", i),
			OSType:      "linux",
			OSArch:      "amd64",
			CallbackURL: "https://garm.example.com/",
		}
		_, err := s.Store.CreateInstance(s.adminCtx, testPool.ID, instanceParams)
		s.Require().Nil(err)
	}

	// Try to create a fourth instance, which should fail
	fourthInstanceParams := params.CreateInstanceParams{
		Name:        "max-test-instance-4",
		OSType:      "linux",
		OSArch:      "amd64",
		CallbackURL: "https://garm.example.com/",
	}
	_, err = s.Store.CreateInstance(s.adminCtx, testPool.ID, fourthInstanceParams)
	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "max runners reached for pool")

	// Verify instance count is still 3
	count, err := s.Store.PoolInstanceCount(s.adminCtx, testPool.ID)
	s.Require().Nil(err)
	s.Require().Equal(int64(3), count)
}

func (s *InstancesTestSuite) TestCreateInstanceConcurrentMaxRunnersRaceCondition() {
	// Create a new pool with max runners set to 15, starting from 0
	createPoolParams := params.CreatePoolParams{
		ProviderName:   "test-provider",
		MaxRunners:     15,
		MinIdleRunners: 0,
		Image:          "test-image",
		Flavor:         "test-flavor",
		OSType:         "linux",
		Tags:           []string{"amd64", "linux"},
	}
	entity, err := s.Fixtures.Org.GetEntity()
	s.Require().Nil(err)
	raceTestPool, err := s.Store.CreateEntityPool(s.adminCtx, entity, createPoolParams)
	s.Require().Nil(err)

	// Verify pool starts with 0 instances
	initialCount, err := s.Store.PoolInstanceCount(s.adminCtx, raceTestPool.ID)
	s.Require().Nil(err)
	s.Require().Equal(int64(0), initialCount)

	// Concurrently try to create 150 instances (should only allow 15)
	var wg sync.WaitGroup
	results := make([]error, 150)

	for i := 0; i < 150; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			instanceParams := params.CreateInstanceParams{
				Name:        fmt.Sprintf("race-test-instance-%d", index),
				OSType:      "linux",
				OSArch:      "amd64",
				CallbackURL: "https://garm.example.com/",
			}
			_, err := s.Store.CreateInstance(s.adminCtx, raceTestPool.ID, instanceParams)
			results[index] = err
		}(i)
	}

	wg.Wait()

	// Count successful and failed creations
	successCount := 0
	conflictErrorCount := 0
	databaseLockedCount := 0
	otherErrorCount := 0

	for i, err := range results {
		if err == nil {
			successCount++
			continue
		}

		errStr := fmt.Sprintf("%v", err)
		expectedConflictErr1 := "error creating instance: max runners reached for pool " + raceTestPool.ID
		expectedConflictErr2 := "max runners reached for pool " + raceTestPool.ID
		databaseLockedErr := "error creating instance: error creating instance: database is locked"

		switch errStr {
		case expectedConflictErr1, expectedConflictErr2:
			conflictErrorCount++
		case databaseLockedErr:
			databaseLockedCount++
			s.T().Logf("Got database locked error for goroutine %d: %v", i, err)
		default:
			otherErrorCount++
			s.T().Logf("Got unexpected error for goroutine %d: %v", i, err)
		}
	}

	s.T().Logf("Results: success=%d, conflict=%d, databaseLocked=%d, other=%d",
		successCount, conflictErrorCount, databaseLockedCount, otherErrorCount)

	// Verify final instance count is <= 15 (the main test - no more than max runners)
	finalCount, err := s.Store.PoolInstanceCount(s.adminCtx, raceTestPool.ID)
	s.Require().Nil(err)
	s.Require().LessOrEqual(int64(successCount), int64(15), "Should not create more than max runners")
	s.Require().Equal(int64(successCount), finalCount, "Final count should match successful creations")

	// The key test: verify we never exceeded max runners despite concurrent attempts
	s.Require().True(finalCount <= 15, "Pool should never exceed max runners limit of 15, got %d", finalCount)

	// If there were database lock errors, that's a concurrency issue but not a max runners violation
	if databaseLockedCount > 0 {
		s.T().Logf("WARNING: Got %d database lock errors during concurrent testing - this indicates SQLite concurrency limitations", databaseLockedCount)
	}

	// The critical assertion: total successful attempts + database locked + conflicts should equal 150
	s.Require().Equal(150, successCount+conflictErrorCount+databaseLockedCount+otherErrorCount,
		"All 150 goroutines should have completed with some result")
}

func (s *InstancesTestSuite) TestCreateInstanceDBCreateErr() {
	pool := s.Fixtures.Pool

	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE id = ? AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT ?")).
		WithArgs(pool.ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "max_runners"}).AddRow(pool.ID, 4))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `instances` WHERE pool_id = ? AND `instances`.`deleted_at` IS NULL")).
		WithArgs(pool.ID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	s.Fixtures.SQLMock.
		ExpectExec("INSERT INTO `pools`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.
		ExpectExec("INSERT INTO `instances`").
		WillReturnError(fmt.Errorf("mocked insert instance error"))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.CreateInstance(s.adminCtx, pool.ID, s.Fixtures.CreateInstanceParams)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("error creating instance: error creating instance: mocked insert instance error", err.Error())
}

func (s *InstancesTestSuite) TestGetInstanceByName() {
	storeInstance := s.Fixtures.Instances[1]

	instance, err := s.Store.GetInstance(s.adminCtx, storeInstance.Name)

	s.Require().Nil(err)
	s.Require().Equal(storeInstance.Name, instance.Name)
	s.Require().Equal(storeInstance.PoolID, instance.PoolID)
	s.Require().Equal(storeInstance.OSArch, instance.OSArch)
	s.Require().Equal(storeInstance.OSType, instance.OSType)
	s.Require().Equal(storeInstance.CallbackURL, instance.CallbackURL)
}

func (s *InstancesTestSuite) TestGetInstanceByNameFetchInstanceFailed() {
	_, err := s.Store.GetInstance(s.adminCtx, "not-existent-instance-name")

	s.Require().Equal("error fetching instance: error fetching instance by name: not found", err.Error())
}

func (s *InstancesTestSuite) TestDeleteInstance() {
	storeInstance := s.Fixtures.Instances[0]

	err := s.Store.DeleteInstance(s.adminCtx, s.Fixtures.Pool.ID, storeInstance.Name)

	s.Require().Nil(err)

	_, err = s.Store.GetInstance(s.adminCtx, storeInstance.Name)
	s.Require().Equal("error fetching instance: error fetching instance by name: not found", err.Error())

	err = s.Store.DeleteInstance(s.adminCtx, s.Fixtures.Pool.ID, storeInstance.Name)
	s.Require().Nil(err)
}

func (s *InstancesTestSuite) TestDeleteInstanceByName() {
	storeInstance := s.Fixtures.Instances[0]

	err := s.Store.DeleteInstanceByName(s.adminCtx, storeInstance.Name)

	s.Require().Nil(err)

	_, err = s.Store.GetInstance(s.adminCtx, storeInstance.Name)
	s.Require().Equal("error fetching instance: error fetching instance by name: not found", err.Error())

	err = s.Store.DeleteInstanceByName(s.adminCtx, storeInstance.Name)
	s.Require().Nil(err)
}

func (s *InstancesTestSuite) TestDeleteInstanceInvalidPoolID() {
	err := s.Store.DeleteInstance(s.adminCtx, "dummy-pool-id", "dummy-instance-name")

	s.Require().Equal("error deleting instance: error fetching pool: error parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestDeleteInstanceDBRecordNotFoundErr() {
	pool := s.Fixtures.Pool
	instance := s.Fixtures.Instances[0]

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE id = ? AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT ?")).
		WithArgs(pool.ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(pool.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instances` WHERE (name = ? and pool_id = ?) AND `instances`.`deleted_at` IS NULL ORDER BY `instances`.`id` LIMIT ?")).
		WithArgs(instance.Name, pool.ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `addresses` WHERE `addresses`.`instance_id` = ? AND `addresses`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"address", "type", "instance_id"}).AddRow("10.10.1.10", "private", instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `workflow_jobs` WHERE `workflow_jobs`.`instance_id` = ? AND `workflow_jobs`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{}))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instance_status_updates` WHERE `instance_status_updates`.`instance_id` = ? AND `instance_status_updates`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"message", "instance_id"}).AddRow("instance sample message", instance.ID))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("DELETE FROM `instances` WHERE `instances`.`id` = ?")).
		WithArgs(instance.ID).
		WillReturnError(gorm.ErrRecordNotFound)
	s.Fixtures.SQLMock.ExpectRollback()

	err := s.StoreSQLMocked.DeleteInstance(s.adminCtx, pool.ID, instance.Name)

	s.assertSQLMockExpectations()
	s.Require().Nil(err)
}

func (s *InstancesTestSuite) TestDeleteInstanceDBDeleteErr() {
	pool := s.Fixtures.Pool
	instance := s.Fixtures.Instances[0]

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE id = ? AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT ?")).
		WithArgs(pool.ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(pool.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instances` WHERE (name = ? and pool_id = ?) AND `instances`.`deleted_at` IS NULL ORDER BY `instances`.`id` LIMIT ?")).
		WithArgs(instance.Name, pool.ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `addresses` WHERE `addresses`.`instance_id` = ? AND `addresses`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"address", "type", "instance_id"}).AddRow("12.10.12.13", "public", instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `workflow_jobs` WHERE `workflow_jobs`.`instance_id` = ? AND `workflow_jobs`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{}))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instance_status_updates` WHERE `instance_status_updates`.`instance_id` = ? AND `instance_status_updates`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"message", "instance_id"}).AddRow("instance sample message", instance.ID))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("DELETE FROM `instances` WHERE `instances`.`id` = ?")).
		WithArgs(instance.ID).
		WillReturnError(fmt.Errorf("mocked delete instance error"))
	s.Fixtures.SQLMock.ExpectRollback()

	err := s.StoreSQLMocked.DeleteInstance(s.adminCtx, pool.ID, instance.Name)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("error deleting instance: mocked delete instance error", err.Error())
}

func (s *InstancesTestSuite) TestAddInstanceEvent() {
	storeInstance := s.Fixtures.Instances[0]
	statusMsg := "test-status-message"

	err := s.Store.AddInstanceEvent(s.adminCtx, storeInstance.Name, params.StatusEvent, params.EventInfo, statusMsg)

	s.Require().Nil(err)
	instance, err := s.Store.GetInstance(s.adminCtx, storeInstance.Name)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get db instance: %s", err))
	}
	s.Require().Equal(1, len(instance.StatusMessages))
	s.Require().Equal(statusMsg, instance.StatusMessages[0].Message)
}

func (s *InstancesTestSuite) TestAddInstanceEventDBUpdateErr() {
	instance := s.Fixtures.Instances[0]
	statusMsg := "test-status-message"

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instances` WHERE name = ? AND `instances`.`deleted_at` IS NULL ORDER BY `instances`.`id` LIMIT ?")).
		WithArgs(instance.Name, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `addresses` WHERE `addresses`.`instance_id` = ? AND `addresses`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"address", "type", "instance_id"}).AddRow("10.10.1.10", "private", instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `workflow_jobs` WHERE `workflow_jobs`.`instance_id` = ? AND `workflow_jobs`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{}))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instance_status_updates` WHERE `instance_status_updates`.`instance_id` = ? AND `instance_status_updates`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"message", "instance_id"}).AddRow("instance sample message", instance.ID))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("UPDATE `instances` SET `updated_at`=? WHERE `instances`.`deleted_at` IS NULL AND `id` = ?")).
		WithArgs(sqlmock.AnyArg(), instance.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `instance_status_updates`")).
		WillReturnError(fmt.Errorf("mocked add status message error"))
	s.Fixtures.SQLMock.ExpectRollback()

	err := s.StoreSQLMocked.AddInstanceEvent(s.adminCtx, instance.Name, params.StatusEvent, params.EventInfo, statusMsg)

	s.Require().NotNil(err)
	s.Require().Equal("error adding status message: mocked add status message error", err.Error())
	s.assertSQLMockExpectations()
}

func (s *InstancesTestSuite) TestUpdateInstance() {
	instance, err := s.Store.UpdateInstance(s.adminCtx, s.Fixtures.Instances[0].Name, s.Fixtures.UpdateInstanceParams)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.UpdateInstanceParams.ProviderID, instance.ProviderID)
	s.Require().Equal(s.Fixtures.UpdateInstanceParams.OSName, instance.OSName)
	s.Require().Equal(s.Fixtures.UpdateInstanceParams.OSVersion, instance.OSVersion)
	s.Require().Equal(s.Fixtures.UpdateInstanceParams.Status, instance.Status)
	s.Require().Equal(s.Fixtures.UpdateInstanceParams.RunnerStatus, instance.RunnerStatus)
	s.Require().Equal(s.Fixtures.UpdateInstanceParams.AgentID, instance.AgentID)
	s.Require().Equal(s.Fixtures.UpdateInstanceParams.CreateAttempt, instance.CreateAttempt)
}

func (s *InstancesTestSuite) TestUpdateInstanceDBUpdateInstanceErr() {
	instance := s.Fixtures.Instances[0]

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instances` WHERE name = ? AND `instances`.`deleted_at` IS NULL ORDER BY `instances`.`id` LIMIT ?")).
		WithArgs(instance.Name, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `addresses` WHERE `addresses`.`instance_id` = ? AND `addresses`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"address", "type", "instance_id"}).AddRow("10.10.1.10", "private", instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `workflow_jobs` WHERE `workflow_jobs`.`instance_id` = ? AND `workflow_jobs`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{}))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instance_status_updates` WHERE `instance_status_updates`.`instance_id` = ? AND `instance_status_updates`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"message", "instance_id"}).AddRow("instance sample message", instance.ID))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(("UPDATE `instances`")).
		WillReturnError(fmt.Errorf("mocked update instance error"))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.UpdateInstance(s.adminCtx, instance.Name, s.Fixtures.UpdateInstanceParams)

	s.Require().NotNil(err)
	s.Require().Equal("error updating instance: mocked update instance error", err.Error())
	s.assertSQLMockExpectations()
}

func (s *InstancesTestSuite) TestUpdateInstanceDBUpdateAddressErr() {
	instance := s.Fixtures.Instances[0]

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instances` WHERE name = ? AND `instances`.`deleted_at` IS NULL ORDER BY `instances`.`id` LIMIT ?")).
		WithArgs(instance.Name, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `addresses` WHERE `addresses`.`instance_id` = ? AND `addresses`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"address", "type", "instance_id"}).AddRow("10.10.1.10", "private", instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `workflow_jobs` WHERE `workflow_jobs`.`instance_id` = ? AND `workflow_jobs`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{}))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instance_status_updates` WHERE `instance_status_updates`.`instance_id` = ? AND `instance_status_updates`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"message", "instance_id"}).AddRow("instance sample message", instance.ID))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("UPDATE `instances` SET")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `addresses`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `instance_status_updates`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.ExpectCommit()
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("UPDATE `instances` SET")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.
		ExpectExec(regexp.QuoteMeta("INSERT INTO `addresses`")).
		WillReturnError(fmt.Errorf("update addresses mock error"))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.UpdateInstance(s.adminCtx, instance.Name, s.Fixtures.UpdateInstanceParams)

	s.Require().NotNil(err)
	s.Require().Equal("error updating addresses: update addresses mock error", err.Error())
	s.assertSQLMockExpectations()
}

func (s *InstancesTestSuite) TestListPoolInstances() {
	instances, err := s.Store.ListPoolInstances(s.adminCtx, s.Fixtures.Pool.ID)

	s.Require().Nil(err)
	s.equalInstancesByName(s.Fixtures.Instances, instances)
}

func (s *InstancesTestSuite) TestListPoolInstancesInvalidPoolID() {
	_, err := s.Store.ListPoolInstances(s.adminCtx, "dummy-pool-id")

	s.Require().Equal("error parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestListAllInstances() {
	instances, err := s.Store.ListAllInstances(s.adminCtx)

	s.Require().Nil(err)
	s.equalInstancesByName(s.Fixtures.Instances, instances)
}

func (s *InstancesTestSuite) TestListAllInstancesDBFetchErr() {
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instances` WHERE `instances`.`deleted_at` IS NULL LIMIT ?")).
		WithArgs(1000).
		WillReturnError(fmt.Errorf("fetch instances mock error"))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.ListAllInstances(s.adminCtx)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("failed to list all instances: error fetching instances: fetch instances mock error", err.Error())
}

func (s *InstancesTestSuite) TestPoolInstanceCount() {
	instancesCount, err := s.Store.PoolInstanceCount(s.adminCtx, s.Fixtures.Pool.ID)

	s.Require().Nil(err)
	s.Require().Equal(int64(len(s.Fixtures.Instances)), instancesCount)
}

func (s *InstancesTestSuite) TestPoolInstanceCountInvalidPoolID() {
	_, err := s.Store.PoolInstanceCount(s.adminCtx, "dummy-pool-id")

	s.Require().Equal("error fetching pool: error parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestPoolInstanceCountDBCountErr() {
	pool := s.Fixtures.Pool

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE id = ? AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT ?")).
		WithArgs(pool.ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(pool.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `instances` WHERE pool_id = ? AND `instances`.`deleted_at` IS NULL")).
		WithArgs(pool.ID).
		WillReturnError(fmt.Errorf("count mock error"))

	_, err := s.StoreSQLMocked.PoolInstanceCount(s.adminCtx, pool.ID)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("error fetching instance count: count mock error", err.Error())
}

func TestInstTestSuite(t *testing.T) {
	suite.Run(t, new(InstancesTestSuite))
}
