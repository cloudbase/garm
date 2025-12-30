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
	"errors"
	"fmt"

	"gorm.io/gorm"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/params"
)

func (s *sqlDatabase) getUserByUsernameOrEmail(tx *gorm.DB, user string) (User, error) {
	field := "username"
	if util.IsValidEmail(user) {
		field = "email"
	}
	query := fmt.Sprintf("%s = ?", field)

	var dbUser User
	q := tx.Model(&User{}).Where(query, user).First(&dbUser)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return User{}, runnerErrors.ErrNotFound
		}
		return User{}, fmt.Errorf("error fetching user: %w", q.Error)
	}
	return dbUser, nil
}

func (s *sqlDatabase) getUserByID(tx *gorm.DB, userID string) (User, error) {
	var dbUser User
	q := tx.Model(&User{}).Where("id = ?", userID).First(&dbUser)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return User{}, runnerErrors.ErrNotFound
		}
		return User{}, fmt.Errorf("error fetching user: %w", q.Error)
	}
	return dbUser, nil
}

func (s *sqlDatabase) CreateUser(_ context.Context, user params.NewUserParams) (params.User, error) {
	if user.Username == "" || user.Email == "" {
		return params.User{}, runnerErrors.NewBadRequestError("missing username or email")
	}
	// SSO users don't have passwords, but regular users must have one
	if !user.IsSSOUser && user.Password == "" {
		return params.User{}, runnerErrors.NewBadRequestError("missing password for non-SSO user")
	}
	newUser := User{
		Username: user.Username,
		Password: user.Password, // Empty for SSO users
		FullName: user.FullName,
		Enabled:  user.Enabled,
		Email:    user.Email,
		IsAdmin:  user.IsAdmin,
	}
	err := s.conn.Transaction(func(tx *gorm.DB) error {
		if _, err := s.getUserByUsernameOrEmail(tx, user.Username); err == nil || !errors.Is(err, runnerErrors.ErrNotFound) {
			return runnerErrors.NewConflictError("username already exists")
		}
		if _, err := s.getUserByUsernameOrEmail(tx, user.Email); err == nil || !errors.Is(err, runnerErrors.ErrNotFound) {
			return runnerErrors.NewConflictError("email already exists")
		}

		q := tx.Save(&newUser)
		if q.Error != nil {
			return fmt.Errorf("error creating user: %w", q.Error)
		}
		return nil
	})
	if err != nil {
		return params.User{}, fmt.Errorf("error creating user: %w", err)
	}
	return s.sqlToParamsUser(newUser), nil
}

func (s *sqlDatabase) hasAdmin(tx *gorm.DB) bool {
	var user User
	q := tx.Model(&User{}).Where("is_admin = ?", true).First(&user)
	return q.Error == nil
}

func (s *sqlDatabase) HasAdminUser(_ context.Context) bool {
	return s.hasAdmin(s.conn)
}

func (s *sqlDatabase) GetUser(_ context.Context, user string) (params.User, error) {
	dbUser, err := s.getUserByUsernameOrEmail(s.conn, user)
	if err != nil {
		return params.User{}, fmt.Errorf("error fetching user: %w", err)
	}
	return s.sqlToParamsUser(dbUser), nil
}

func (s *sqlDatabase) GetUserByID(_ context.Context, userID string) (params.User, error) {
	dbUser, err := s.getUserByID(s.conn, userID)
	if err != nil {
		return params.User{}, fmt.Errorf("error fetching user: %w", err)
	}
	return s.sqlToParamsUser(dbUser), nil
}

func (s *sqlDatabase) UpdateUser(_ context.Context, user string, param params.UpdateUserParams) (params.User, error) {
	var err error
	var dbUser User
	err = s.conn.Transaction(func(tx *gorm.DB) error {
		dbUser, err = s.getUserByUsernameOrEmail(tx, user)
		if err != nil {
			return fmt.Errorf("error fetching user: %w", err)
		}

		if param.FullName != "" {
			dbUser.FullName = param.FullName
		}

		if param.Enabled != nil {
			dbUser.Enabled = *param.Enabled
		}

		if param.Password != "" {
			dbUser.Password = param.Password
			dbUser.Generation++
		}

		if q := tx.Save(&dbUser); q.Error != nil {
			return fmt.Errorf("error saving user: %w", q.Error)
		}
		return nil
	})
	if err != nil {
		return params.User{}, fmt.Errorf("error updating user: %w", err)
	}
	return s.sqlToParamsUser(dbUser), nil
}

// GetAdminUser returns the system admin user. This is only for internal use.
func (s *sqlDatabase) GetAdminUser(_ context.Context) (params.User, error) {
	var user User
	q := s.conn.Model(&User{}).Where("is_admin = ?", true).First(&user)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return params.User{}, runnerErrors.ErrNotFound
		}
		return params.User{}, fmt.Errorf("error fetching admin user: %w", q.Error)
	}
	return s.sqlToParamsUser(user), nil
}

func (s *sqlDatabase) ListUsers(_ context.Context) ([]params.User, error) {
	var users []User
	q := s.conn.Model(&User{}).Find(&users)
	if q.Error != nil {
		return nil, fmt.Errorf("error fetching users: %w", q.Error)
	}

	ret := make([]params.User, len(users))
	for idx, user := range users {
		ret[idx] = s.sqlToParamsUser(user)
	}
	return ret, nil
}
