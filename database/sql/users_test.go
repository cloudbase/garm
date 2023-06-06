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
	"testing"

	dbCommon "github.com/cloudbase/garm/database/common"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
	"github.com/stretchr/testify/suite"
)

type UserTestFixtures struct {
	Users         []params.User
	NewUserParams params.NewUserParams
	AdminContext  context.Context
}

type UserTestSuite struct {
	suite.Suite
	Store    dbCommon.Store
	Fixtures *UserTestFixtures
}

func (s *UserTestSuite) SetupTest() {
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

	// setup test fixtures
	fixtures := &UserTestFixtures{
		Users: users,
		NewUserParams: params.NewUserParams{
			Email:    "test@example.com",
			Username: "test-username",
			FullName: "test-fullname",
			Password: "test-password",
		},
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
	s.Require().Equal(("missing username or email"), err.Error())
}

func (s *UserTestSuite) TestCreateUserUsernameAlreadyExist() {
	s.Fixtures.NewUserParams.Username = "test-username-1"

	_, err := s.Store.CreateUser(context.Background(), s.Fixtures.NewUserParams)

	s.Require().NotNil(err)
	s.Require().Equal(("username already exists"), err.Error())
}

func (s *UserTestSuite) TestCreateUserEmailAlreadyExist() {
	s.Fixtures.NewUserParams.Email = "test-1@example.com"

	_, err := s.Store.CreateUser(context.Background(), s.Fixtures.NewUserParams)

	s.Require().NotNil(err)
	s.Require().Equal(("email already exists"), err.Error())
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
	s.Require().Equal("fetching user: not found", err.Error())
}

func (s *UserTestSuite) TestGetUserByID() {
	user, err := s.Store.GetUserByID(context.Background(), s.Fixtures.Users[0].ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.Users[0].ID, user.ID)
}

func (s *UserTestSuite) TestGetUserByIDNotFound() {
	_, err := s.Store.GetUserByID(context.Background(), "dummy-user-id")

	s.Require().NotNil(err)
	s.Require().Equal("fetching user: not found", err.Error())
}

func TestUserTestSuite(t *testing.T) {
	suite.Run(t, new(UserTestSuite))
}
