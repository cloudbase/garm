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
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type UserTestFixtures struct {
	Users            []params.User
	NewUserParams    params.NewUserParams
	UpdateUserParams params.UpdateUserParams
	SQLMock          sqlmock.Sqlmock
}

type UserTestSuite struct {
	suite.Suite
	Store          dbCommon.Store
	StoreSQLMocked *sqlDatabase
	Fixtures       *UserTestFixtures
}

func (s *UserTestSuite) assertSQLMockExpectations() {
	err := s.Fixtures.SQLMock.ExpectationsWereMet()
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to meet sqlmock expectations, got error: %v", err))
	}
}

func (s *UserTestSuite) TearDownTest() {
	watcher.CloseWatcher()
}

func (s *UserTestSuite) SetupTest() {
	ctx := context.Background()
	watcher.InitWatcher(ctx)
	// create testing sqlite database
	db, err := NewSQLDatabase(context.Background(), garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.Store = db

	// create some user objects in the database, for testing purposes
	users := []params.User{}
	for i := 1; i <= 3; i++ {
		user, err := db.CreateUser(
			context.Background(),
			params.NewUserParams{
				Email:    fmt.Sprintf("test-%d@example.com", i),
				Username: fmt.Sprintf("test-username-%d", i),
				FullName: fmt.Sprintf("test-fullname-%d", i),
				Password: fmt.Sprintf("test-password-%d", i),
			},
		)
		if err != nil {
			s.FailNow(fmt.Sprintf("failed to create database object (test-%d@example.com)", i))
		}

		users = append(users, user)
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
	var enabled bool
	fixtures := &UserTestFixtures{
		Users: users,
		NewUserParams: params.NewUserParams{
			Email:    "test@example.com",
			Username: "test-username",
			FullName: "test-fullname",
			Password: "test-password",
		},
		UpdateUserParams: params.UpdateUserParams{
			FullName: "test-update-fullname",
			Password: "test-update-password",
			Enabled:  &enabled,
		},
		SQLMock: sqlMock,
	}
	s.Fixtures = fixtures
}

func (s *UserTestSuite) TestCreateUser() {
	// call tested function
	user, err := s.Store.CreateUser(context.Background(), s.Fixtures.NewUserParams)

	// assertions
	s.Require().Nil(err)
	storeUser, err := s.Store.GetUserByID(context.Background(), user.ID)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get user by id: %v", err))
	}
	s.Require().Equal(storeUser.Email, user.Email)
	s.Require().Equal(storeUser.Username, user.Username)
	s.Require().Equal(storeUser.FullName, user.FullName)
}

func (s *UserTestSuite) TestCreateUserMissingUsernameEmail() {
	// this is already created in `SetupTest()`
	s.Fixtures.NewUserParams.Username = ""

	_, err := s.Store.CreateUser(context.Background(), s.Fixtures.NewUserParams)

	s.Require().NotNil(err)
	s.Require().Equal(("missing username, password or email"), err.Error())
}

func (s *UserTestSuite) TestCreateUserUsernameAlreadyExist() {
	s.Fixtures.NewUserParams.Username = "test-username-1"

	_, err := s.Store.CreateUser(context.Background(), s.Fixtures.NewUserParams)

	s.Require().NotNil(err)
	s.Require().Equal(("error creating user: username already exists"), err.Error())
}

func (s *UserTestSuite) TestCreateUserEmailAlreadyExist() {
	s.Fixtures.NewUserParams.Email = "test-1@example.com"

	_, err := s.Store.CreateUser(context.Background(), s.Fixtures.NewUserParams)

	s.Require().NotNil(err)
	s.Require().Equal(("error creating user: email already exists"), err.Error())
}

func (s *UserTestSuite) TestCreateUserDBCreateErr() {
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE username = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.NewUserParams.Username, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE email = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.NewUserParams.Email, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	s.Fixtures.SQLMock.
		ExpectExec("INSERT INTO `users`").
		WillReturnError(fmt.Errorf("creating user mock error"))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.CreateUser(context.Background(), s.Fixtures.NewUserParams)

	s.Require().NotNil(err)
	s.Require().Equal("error creating user: error creating user: creating user mock error", err.Error())
	s.assertSQLMockExpectations()
}

func (s *UserTestSuite) TestHasAdminUserNoAdmin() {
	hasAdmin := s.Store.HasAdminUser(context.Background())

	// initially, we don't have any admin users in the store
	s.Require().False(hasAdmin)
}

func (s *UserTestSuite) TestHasAdminUser() {
	// create an admin user
	s.Fixtures.NewUserParams.IsAdmin = true
	_, err := s.Store.CreateUser(context.Background(), s.Fixtures.NewUserParams)
	s.Require().Nil(err)

	// check again if the store has any admin users
	hasAdmin := s.Store.HasAdminUser(context.Background())
	s.Require().True(hasAdmin)
}

func (s *UserTestSuite) TestGetUser() {
	user, err := s.Store.GetUser(context.Background(), s.Fixtures.Users[0].Username)

	s.Require().Nil(err)
	storeUser, err := s.Store.GetUserByID(context.Background(), user.ID)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to get user by id: %v", err))
	}
	s.Require().Equal(storeUser.Email, user.Email)
	s.Require().Equal(storeUser.Username, user.Username)
	s.Require().Equal(storeUser.FullName, user.FullName)
}

func (s *UserTestSuite) TestGetUserNotFound() {
	_, err := s.Store.GetUser(context.Background(), "dummy-user")

	s.Require().NotNil(err)
	s.Require().Equal("error fetching user: not found", err.Error())
}

func (s *UserTestSuite) TestGetUserByID() {
	user, err := s.Store.GetUserByID(context.Background(), s.Fixtures.Users[0].ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.Users[0].ID, user.ID)
}

func (s *UserTestSuite) TestGetUserByIDNotFound() {
	_, err := s.Store.GetUserByID(context.Background(), "dummy-user-id")

	s.Require().NotNil(err)
	s.Require().Equal("error fetching user: not found", err.Error())
}

func (s *UserTestSuite) TestUpdateUser() {
	user, err := s.Store.UpdateUser(context.Background(), s.Fixtures.Users[0].Username, s.Fixtures.UpdateUserParams)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.UpdateUserParams.FullName, user.FullName)
	s.Require().Equal(s.Fixtures.UpdateUserParams.Password, user.Password)
	s.Require().Equal(*s.Fixtures.UpdateUserParams.Enabled, user.Enabled)
}

func (s *UserTestSuite) TestUpdateUserNotFound() {
	_, err := s.Store.UpdateUser(context.Background(), "dummy-user", s.Fixtures.UpdateUserParams)

	s.Require().NotNil(err)
	s.Require().Equal("error updating user: error fetching user: not found", err.Error())
}

func (s *UserTestSuite) TestUpdateUserDBSaveErr() {
	s.Fixtures.SQLMock.ExpectBegin()
	s.Fixtures.SQLMock.
		ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE username = ? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT ?")).
		WithArgs(s.Fixtures.Users[0].ID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(s.Fixtures.Users[0].ID))
	s.Fixtures.SQLMock.
		ExpectExec(("UPDATE `users` SET")).
		WillReturnError(fmt.Errorf("saving user mock error"))
	s.Fixtures.SQLMock.ExpectRollback()

	_, err := s.StoreSQLMocked.UpdateUser(context.Background(), s.Fixtures.Users[0].ID, s.Fixtures.UpdateUserParams)

	s.assertSQLMockExpectations()
	s.Require().NotNil(err)
	s.Require().Equal("error updating user: error saving user: saving user mock error", err.Error())
}

func TestUserTestSuite(t *testing.T) {
	suite.Run(t, new(UserTestSuite))
}
