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
	runnerErrors "garm/errors"
	"garm/params"
	"garm/util"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

func (s *sqlDatabase) getUserByUsernameOrEmail(user string) (User, error) {
	field := "username"
	if util.IsValidEmail(user) {
		field = "email"
	}
	query := fmt.Sprintf("%s = ?", field)

	var dbUser User
	q := s.conn.Model(&User{}).Where(query, user).First(&dbUser)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return User{}, runnerErrors.ErrNotFound
		}
		return User{}, errors.Wrap(q.Error, "fetching user")
	}
	return dbUser, nil
}

func (s *sqlDatabase) getUserByID(userID string) (User, error) {
	var dbUser User
	q := s.conn.Model(&User{}).Where("id = ?", userID).First(&dbUser)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return User{}, runnerErrors.ErrNotFound
		}
		return User{}, errors.Wrap(q.Error, "fetching user")
	}
	return dbUser, nil
}

func (s *sqlDatabase) CreateUser(ctx context.Context, user params.NewUserParams) (params.User, error) {
	if user.Username == "" || user.Email == "" {
		return params.User{}, runnerErrors.NewBadRequestError("missing username or email")
	}
	if _, err := s.getUserByUsernameOrEmail(user.Username); err == nil || !errors.Is(err, runnerErrors.ErrNotFound) {
		return params.User{}, runnerErrors.NewConflictError("username already exists")
	}
	if _, err := s.getUserByUsernameOrEmail(user.Email); err == nil || !errors.Is(err, runnerErrors.ErrNotFound) {
		return params.User{}, runnerErrors.NewConflictError("email already exists")
	}

	newUser := User{
		Username: user.Username,
		Password: user.Password,
		FullName: user.FullName,
		Enabled:  user.Enabled,
		Email:    user.Email,
		IsAdmin:  user.IsAdmin,
	}

	q := s.conn.Save(&newUser)
	if q.Error != nil {
		return params.User{}, errors.Wrap(q.Error, "creating user")
	}
	return s.sqlToParamsUser(newUser), nil
}

func (s *sqlDatabase) HasAdminUser(ctx context.Context) bool {
	var user User
	q := s.conn.Model(&User{}).Where("is_admin = ?", true).First(&user)
	return q.Error == nil
}

func (s *sqlDatabase) GetUser(ctx context.Context, user string) (params.User, error) {
	dbUser, err := s.getUserByUsernameOrEmail(user)
	if err != nil {
		return params.User{}, errors.Wrap(err, "fetching user")
	}
	return s.sqlToParamsUser(dbUser), nil
}

func (s *sqlDatabase) GetUserByID(ctx context.Context, userID string) (params.User, error) {
	dbUser, err := s.getUserByID(userID)
	if err != nil {
		return params.User{}, errors.Wrap(err, "fetching user")
	}
	return s.sqlToParamsUser(dbUser), nil
}

func (s *sqlDatabase) UpdateUser(ctx context.Context, user string, param params.UpdateUserParams) (params.User, error) {
	dbUser, err := s.getUserByUsernameOrEmail(user)
	if err != nil {
		return params.User{}, errors.Wrap(err, "fetching user")
	}

	if param.FullName != "" {
		dbUser.FullName = param.FullName
	}

	if param.Enabled != nil {
		dbUser.Enabled = *param.Enabled
	}

	if param.Password != "" {
		dbUser.Password = param.Password
	}

	if q := s.conn.Save(&dbUser); q.Error != nil {
		return params.User{}, errors.Wrap(q.Error, "saving user")
	}

	return s.sqlToParamsUser(dbUser), nil
}
