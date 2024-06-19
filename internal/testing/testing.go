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

//go:build testing
// +build testing

package testing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/appdefaults"
)

//nolint:golangci-lint,gosec
var encryptionPassphrase = "bocyasicgatEtenOubwonIbsudNutDom"

func ImpersonateAdminContext(ctx context.Context, db common.Store, s *testing.T) context.Context {
	adminUser, err := db.GetAdminUser(ctx)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			s.Fatalf("failed to get admin user: %v", err)
		}
		newUserParams := params.NewUserParams{
			Email:    "admin@localhost",
			Username: "admin",
			Password: "superSecretAdminPassword@123",
			IsAdmin:  true,
			Enabled:  true,
		}
		adminUser, err = db.CreateUser(ctx, newUserParams)
		if err != nil {
			s.Fatalf("failed to create admin user: %v", err)
		}
	}
	ctx = auth.PopulateContext(ctx, adminUser)
	return ctx
}

func CreateGARMTestUser(ctx context.Context, username string, db common.Store, s *testing.T) params.User {
	newUserParams := params.NewUserParams{
		Email:    fmt.Sprintf("%s@localhost", username),
		Username: username,
		Password: "superSecretPassword@123",
		IsAdmin:  false,
		Enabled:  true,
	}

	user, err := db.CreateUser(ctx, newUserParams)
	if err != nil {
		if errors.Is(err, runnerErrors.ErrDuplicateEntity) {
			user, err = db.GetUser(ctx, newUserParams.Username)
			if err != nil {
				s.Fatalf("failed to get user by email: %v", err)
			}
			return user
		}
		s.Fatalf("failed to create user: %v", err)
	}

	return user
}

func CreateDefaultGithubEndpoint(ctx context.Context, db common.Store, s *testing.T) params.GithubEndpoint {
	endpointParams := params.CreateGithubEndpointParams{
		Name:          "github.com",
		Description:   "github endpoint",
		APIBaseURL:    appdefaults.GithubDefaultBaseURL,
		UploadBaseURL: appdefaults.GithubDefaultUploadBaseURL,
		BaseURL:       appdefaults.DefaultGithubURL,
	}

	ep, err := db.GetGithubEndpoint(ctx, endpointParams.Name)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			s.Fatalf("failed to get database object (github.com): %v", err)
		}
		ep, err = db.CreateGithubEndpoint(ctx, endpointParams)
		if err != nil {
			if !errors.Is(err, runnerErrors.ErrDuplicateEntity) {
				s.Fatalf("failed to create database object (github.com): %v", err)
			}
		}
	}

	return ep
}

func CreateTestGithubCredentials(ctx context.Context, credsName string, db common.Store, s *testing.T, endpoint params.GithubEndpoint) params.GithubCredentials {
	newCredsParams := params.CreateGithubCredentialsParams{
		Name:        credsName,
		Description: "Test creds",
		AuthType:    params.GithubAuthTypePAT,
		Endpoint:    endpoint.Name,
		PAT: params.GithubPAT{
			OAuth2Token: "test-token",
		},
	}
	newCreds, err := db.CreateGithubCredentials(ctx, newCredsParams)
	if err != nil {
		s.Fatalf("failed to create database object (%s): %v", credsName, err)
	}
	return newCreds
}

func GetTestSqliteDBConfig(t *testing.T) config.Database {
	dir, err := os.MkdirTemp("", "garm-config-test")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	return config.Database{
		Debug:      false,
		DbBackend:  config.SQLiteBackend,
		Passphrase: encryptionPassphrase,
		SQLite: config.SQLite{
			DBFile: filepath.Join(dir, "garm.db"),
		},
	}
}

type IDDBEntity interface {
	GetID() string
}

type NameAndIDDBEntity interface {
	IDDBEntity
	GetName() string
}

func EqualDBEntityByName[T NameAndIDDBEntity](t *testing.T, expected, actual []T) {
	require.Equal(t, len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].GetName() > expected[j].GetName() })
	sort.Slice(actual, func(i, j int) bool { return actual[i].GetName() > actual[j].GetName() })

	for i := 0; i < len(expected); i++ {
		require.Equal(t, expected[i].GetName(), actual[i].GetName())
	}
}

func EqualDBEntityID[T IDDBEntity](t *testing.T, expected, actual []T) {
	require.Equal(t, len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].GetID() > expected[j].GetID() })
	sort.Slice(actual, func(i, j int) bool { return actual[i].GetID() > actual[j].GetID() })

	for i := 0; i < len(expected); i++ {
		require.Equal(t, expected[i].GetID(), actual[i].GetID())
	}
}

func DBEntityMapToSlice[T NameAndIDDBEntity](orgs map[string]T) []T {
	orgsSlice := []T{}
	for _, value := range orgs {
		orgsSlice = append(orgsSlice, value)
	}
	return orgsSlice
}
