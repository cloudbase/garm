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

	commonParams "github.com/cloudbase/garm-provider-common/params"

	dbCommon "github.com/cloudbase/garm/database/common"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"

	"gopkg.in/DATA-DOG/go-sqlmock.v1"

	"github.com/stretchr/testify/suite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

func (s *InstancesTestSuite) SetupTest() {
	// create testing sqlite database
	db, err := NewSQLDatabase(context.Background(), garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.Store = db

	// create an organization for testing purposes
	org, err := s.Store.CreateOrganization(context.Background(), "test-org", "test-creds", "test-webhookSecret")
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
		Tags:           []string{"self-hosted", "amd64", "linux"},
	}
	pool, err := s.Store.CreateOrganizationPool(context.Background(), org.ID, createPoolParams)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create org pool: %s", err))
	}

	// create some instance objects in the database, for testing purposes
	instances := []params.Instance{}
	for i := 1; i <= 3; i++ {
		instance, err := db.CreateInstance(
			context.Background(),
			pool.ID,
			params.CreateInstanceParams{
				Name:         fmt.Sprintf("test-instance-%d", i),
				OSType:       "linux",
				OSArch:       "amd64",
				CallbackURL:  "https://garm.example.com/",
				Status:       commonParams.InstanceRunning,
				RunnerStatus: params.RunnerIdle,
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
	if flag.Lookup("test.v").Value.String() == "false" {
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
	instance, err := s.Store.CreateInstance(context.Background(), s.Fixtures.Pool.ID, s.Fixtures.CreateInstanceParams)

	// assertions
	s.Require().Nil(err)
	storeInstance, err := s.Store.GetInstanceByName(context.Background(), s.Fixtures.CreateInstanceParams.Name)
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
	_, err := s.Store.CreateInstance(context.Background(), "dummy-pool-id", params.CreateInstanceParams{})

	s.Require().Equal("fetching pool: parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestCreateInstanceDBCreateErr() {
	pool := s.Fixtures.Pool

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE id = ? AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT 1")).
		WithArgs(pool.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(pool.ID))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec("INSERT INTO `pools`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.Fixtures.SQLMock.
		ExpectExec("INSERT INTO `instances`").
		WillReturnError(fmt.Errorf("mocked insert instance error"))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.CreateInstance(context.Background(), pool.ID, s.Fixtures.CreateInstanceParams)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("creating instance: mocked insert instance error", err.Error())
}

func (s *InstancesTestSuite) TestGetPoolInstanceByName() {
	storeInstance := s.Fixtures.Instances[0] // this is already created in `SetupTest()`

	instance, err := s.Store.GetPoolInstanceByName(context.Background(), s.Fixtures.Pool.ID, storeInstance.Name)

	s.Require().Nil(err)
	s.Require().Equal(storeInstance.Name, instance.Name)
	s.Require().Equal(storeInstance.PoolID, instance.PoolID)
	s.Require().Equal(storeInstance.OSArch, instance.OSArch)
	s.Require().Equal(storeInstance.OSType, instance.OSType)
	s.Require().Equal(storeInstance.CallbackURL, instance.CallbackURL)
}

func (s *InstancesTestSuite) TestGetPoolInstanceByNameNotFound() {
	_, err := s.Store.GetPoolInstanceByName(context.Background(), s.Fixtures.Pool.ID, "not-existent-instance-name")

	s.Require().Equal("fetching instance: fetching pool instance by name: not found", err.Error())
}

func (s *InstancesTestSuite) TestGetInstanceByName() {
	storeInstance := s.Fixtures.Instances[1]

	instance, err := s.Store.GetInstanceByName(context.Background(), storeInstance.Name)

	s.Require().Nil(err)
	s.Require().Equal(storeInstance.Name, instance.Name)
	s.Require().Equal(storeInstance.PoolID, instance.PoolID)
	s.Require().Equal(storeInstance.OSArch, instance.OSArch)
	s.Require().Equal(storeInstance.OSType, instance.OSType)
	s.Require().Equal(storeInstance.CallbackURL, instance.CallbackURL)
}

func (s *InstancesTestSuite) TestGetInstanceByNameFetchInstanceFailed() {
	_, err := s.Store.GetInstanceByName(context.Background(), "not-existent-instance-name")

	s.Require().Equal("fetching instance: fetching instance by name: not found", err.Error())
}

func (s *InstancesTestSuite) TestDeleteInstance() {
	storeInstance := s.Fixtures.Instances[0]

	err := s.Store.DeleteInstance(context.Background(), s.Fixtures.Pool.ID, storeInstance.Name)

	s.Require().Nil(err)

	_, err = s.Store.GetPoolInstanceByName(context.Background(), s.Fixtures.Pool.ID, storeInstance.Name)
	s.Require().Equal("fetching instance: fetching pool instance by name: not found", err.Error())
}

func (s *InstancesTestSuite) TestDeleteInstanceInvalidPoolID() {
	err := s.Store.DeleteInstance(context.Background(), "dummy-pool-id", "dummy-instance-name")

	s.Require().Equal("deleting instance: fetching pool: parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestDeleteInstanceDBRecordNotFoundErr() {
	pool := s.Fixtures.Pool
	instance := s.Fixtures.Instances[0]

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE id = ? AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT 1")).
		WithArgs(pool.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(pool.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instances` WHERE (name = ? and pool_id = ?) AND `instances`.`deleted_at` IS NULL ORDER BY `instances`.`id` LIMIT 1")).
		WithArgs(instance.Name, pool.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `addresses` WHERE `addresses`.`instance_id` = ? AND `addresses`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"address", "type", "instance_id"}).AddRow("10.10.1.10", "private", instance.ID))
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

	err := s.StoreSQLMocked.DeleteInstance(context.Background(), pool.ID, instance.Name)

	s.assertSQLMockExpectations()
	s.Require().Nil(err)
}

func (s *InstancesTestSuite) TestDeleteInstanceDBDeleteErr() {
	pool := s.Fixtures.Pool
	instance := s.Fixtures.Instances[0]

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE id = ? AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT 1")).
		WithArgs(pool.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(pool.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instances` WHERE (name = ? and pool_id = ?) AND `instances`.`deleted_at` IS NULL ORDER BY `instances`.`id` LIMIT 1")).
		WithArgs(instance.Name, pool.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `addresses` WHERE `addresses`.`instance_id` = ? AND `addresses`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"address", "type", "instance_id"}).AddRow("12.10.12.13", "public", instance.ID))
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

	err := s.StoreSQLMocked.DeleteInstance(context.Background(), pool.ID, instance.Name)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("deleting instance: mocked delete instance error", err.Error())
}

func (s *InstancesTestSuite) TestAddInstanceEvent() {
	storeInstance := s.Fixtures.Instances[0]
	statusMsg := "test-status-message"

	err := s.Store.AddInstanceEvent(context.Background(), storeInstance.ID, params.StatusEvent, params.EventInfo, statusMsg)

	s.Require().Nil(err)
	instance, err := s.Store.GetInstanceByName(context.Background(), storeInstance.Name)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get db instance: %s", err))
	}
	s.Require().Equal(1, len(instance.StatusMessages))
	s.Require().Equal(statusMsg, instance.StatusMessages[0].Message)
}

func (s *InstancesTestSuite) TestAddInstanceEventInvalidPoolID() {
	err := s.Store.AddInstanceEvent(context.Background(), "dummy-id", params.StatusEvent, params.EventInfo, "dummy-message")

	s.Require().Equal("updating instance: parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestAddInstanceEventDBUpdateErr() {
	instance := s.Fixtures.Instances[0]
	statusMsg := "test-status-message"

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instances` WHERE id = ? AND `instances`.`deleted_at` IS NULL ORDER BY `instances`.`id` LIMIT 1")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `addresses` WHERE `addresses`.`instance_id` = ? AND `addresses`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"address", "type", "instance_id"}).AddRow("10.10.1.10", "private", instance.ID))
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

	err := s.StoreSQLMocked.AddInstanceEvent(context.Background(), instance.ID, params.StatusEvent, params.EventInfo, statusMsg)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("adding status message: mocked add status message error", err.Error())
}

func (s *InstancesTestSuite) TestUpdateInstance() {
	instance, err := s.Store.UpdateInstance(context.Background(), s.Fixtures.Instances[0].ID, s.Fixtures.UpdateInstanceParams)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.UpdateInstanceParams.ProviderID, instance.ProviderID)
	s.Require().Equal(s.Fixtures.UpdateInstanceParams.OSName, instance.OSName)
	s.Require().Equal(s.Fixtures.UpdateInstanceParams.OSVersion, instance.OSVersion)
	s.Require().Equal(s.Fixtures.UpdateInstanceParams.Status, instance.Status)
	s.Require().Equal(s.Fixtures.UpdateInstanceParams.RunnerStatus, instance.RunnerStatus)
	s.Require().Equal(s.Fixtures.UpdateInstanceParams.AgentID, instance.AgentID)
	s.Require().Equal(s.Fixtures.UpdateInstanceParams.CreateAttempt, instance.CreateAttempt)
}

func (s *InstancesTestSuite) TestUpdateInstanceInvalidPoolID() {
	_, err := s.Store.UpdateInstance(context.Background(), "dummy-id", params.UpdateInstanceParams{})

	s.Require().Equal("updating instance: parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestUpdateInstanceDBUpdateInstanceErr() {
	instance := s.Fixtures.Instances[0]

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instances` WHERE id = ? AND `instances`.`deleted_at` IS NULL ORDER BY `instances`.`id` LIMIT 1")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `addresses` WHERE `addresses`.`instance_id` = ? AND `addresses`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"address", "type", "instance_id"}).AddRow("10.10.1.10", "private", instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instance_status_updates` WHERE `instance_status_updates`.`instance_id` = ? AND `instance_status_updates`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"message", "instance_id"}).AddRow("instance sample message", instance.ID))
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectExec(("UPDATE `instances`")).
		WillReturnError(fmt.Errorf("mocked update instance error"))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.UpdateInstance(context.Background(), instance.ID, s.Fixtures.UpdateInstanceParams)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("updating instance: mocked update instance error", err.Error())
}

func (s *InstancesTestSuite) TestUpdateInstanceDBUpdateAddressErr() {
	instance := s.Fixtures.Instances[0]

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instances` WHERE id = ? AND `instances`.`deleted_at` IS NULL ORDER BY `instances`.`id` LIMIT 1")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(instance.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `addresses` WHERE `addresses`.`instance_id` = ? AND `addresses`.`deleted_at` IS NULL")).
		WithArgs(instance.ID).
		WillReturnRows(sqlmock.NewRows([]string{"address", "type", "instance_id"}).AddRow("10.10.1.10", "private", instance.ID))
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

	_, err := s.StoreSQLMocked.UpdateInstance(context.Background(), instance.ID, s.Fixtures.UpdateInstanceParams)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("updating addresses: update addresses mock error", err.Error())
}

func (s *InstancesTestSuite) TestListPoolInstances() {
	instances, err := s.Store.ListPoolInstances(context.Background(), s.Fixtures.Pool.ID)

	s.Require().Nil(err)
	s.equalInstancesByName(s.Fixtures.Instances, instances)
}

func (s *InstancesTestSuite) TestListPoolInstancesInvalidPoolID() {
	_, err := s.Store.ListPoolInstances(context.Background(), "dummy-pool-id")

	s.Require().Equal("parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestListAllInstances() {
	instances, err := s.Store.ListAllInstances(context.Background())

	s.Require().Nil(err)
	s.equalInstancesByName(s.Fixtures.Instances, instances)
}

func (s *InstancesTestSuite) TestListAllInstancesDBFetchErr() {
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `instances` WHERE `instances`.`deleted_at` IS NULL")).
		WillReturnError(fmt.Errorf("fetch instances mock error"))

	_, err := s.StoreSQLMocked.ListAllInstances(context.Background())

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("fetching instances: fetch instances mock error", err.Error())
}

func (s *InstancesTestSuite) TestPoolInstanceCount() {
	instancesCount, err := s.Store.PoolInstanceCount(context.Background(), s.Fixtures.Pool.ID)

	s.Require().Nil(err)
	s.Require().Equal(int64(len(s.Fixtures.Instances)), instancesCount)
}

func (s *InstancesTestSuite) TestPoolInstanceCountInvalidPoolID() {
	_, err := s.Store.PoolInstanceCount(context.Background(), "dummy-pool-id")

	s.Require().Equal("fetching pool: parsing id: invalid request", err.Error())
}

func (s *InstancesTestSuite) TestPoolInstanceCountDBCountErr() {
	pool := s.Fixtures.Pool

	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pools` WHERE id = ? AND `pools`.`deleted_at` IS NULL ORDER BY `pools`.`id` LIMIT 1")).
		WithArgs(pool.ID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(pool.ID))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `instances` WHERE pool_id = ? AND `instances`.`deleted_at` IS NULL")).
		WithArgs(pool.ID).
		WillReturnError(fmt.Errorf("count mock error"))

	_, err := s.StoreSQLMocked.PoolInstanceCount(context.Background(), pool.ID)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("fetching instance count: count mock error", err.Error())
}

func TestInstTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(InstancesTestSuite))
}
