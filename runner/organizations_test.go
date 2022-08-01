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

package runner

import (
	"context"
	"fmt"
	"garm/auth"
	"garm/config"
	dbMocks "garm/database/common/mocks"
	runnerErrors "garm/errors"
	"garm/params"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateOrganizationErrUnauthorized(t *testing.T) {
	ctx := context.Background()
	createOrgParams := params.CreateOrgParams{}
	runner := Runner{}

	org, err := runner.CreateOrganization(ctx, createOrgParams)
	var expectedOrg params.Organization
	require.Equal(t, expectedOrg, org)
	require.Equal(t, runnerErrors.ErrUnauthorized, err)
}

func TestCreateOrganizationInvalidParams(t *testing.T) {
	adminCtx := auth.GetAdminContext()
	createOrgParams := params.CreateOrgParams{}
	runner := Runner{}

	org, err := runner.CreateOrganization(adminCtx, createOrgParams)
	require.NotNil(t, err)
	require.Equal(t, params.Organization{}, org)
	require.Regexp(t, "validating params: missing repo name", err.Error())
}

func TestCreateOrganizationMissingCredentials(t *testing.T) {
	adminCtx := auth.GetAdminContext()
	createOrgParams := params.CreateOrgParams{
		Name:            "test",
		CredentialsName: "test",
		WebhookSecret:   "test",
	}
	runner := Runner{}

	org, err := runner.CreateOrganization(adminCtx, createOrgParams)
	require.Equal(t, params.Organization{}, org)
	require.Equal(t, runnerErrors.NewBadRequestError("credentials %s not defined", createOrgParams.CredentialsName), err)
}

func TestCreateOrganizationOrgFetchFailed(t *testing.T) {
	adminCtx := auth.GetAdminContext()
	createOrgParams := params.CreateOrgParams{
		Name:            "test",
		CredentialsName: "test",
		WebhookSecret:   "test",
	}
	storeMock := dbMocks.NewStore(t)
	errMock := fmt.Errorf("mock error")
	storeMock.On("GetOrganization", adminCtx, createOrgParams.Name).Return(params.Organization{}, errMock)
	runner := Runner{
		credentials: map[string]config.Github{
			"test": {
				Name:        "test",
				Description: "test",
				OAuth2Token: "test-token",
			},
		},
		store: storeMock,
	}

	org, err := runner.CreateOrganization(adminCtx, createOrgParams)
	storeMock.AssertExpectations(t)
	require.Equal(t, params.Organization{}, org)
	require.Equal(t, "fetching repo: mock error", err.Error())
}

func TestCreateOrganizationAlreadyExists(t *testing.T) {
	adminCtx := auth.GetAdminContext()
	createOrgParams := params.CreateOrgParams{
		Name:            "test",
		CredentialsName: "test",
		WebhookSecret:   "test",
	}
	storeMock := dbMocks.NewStore(t)
	storeMock.On("GetOrganization", adminCtx, createOrgParams.Name).Return(params.Organization{}, nil)
	runner := Runner{
		credentials: map[string]config.Github{
			"test": {
				Name:        "test",
				Description: "test",
				OAuth2Token: "test-token",
			},
		},
		store: storeMock,
	}

	org, err := runner.CreateOrganization(adminCtx, createOrgParams)
	storeMock.AssertExpectations(t)
	require.Equal(t, params.Organization{}, org)
	require.Equal(t, runnerErrors.NewConflictError("organization %s already exists", createOrgParams.Name), err)
}

func TestCreateOrganizationOrgFailed(t *testing.T) {
	adminCtx := auth.GetAdminContext()
	createOrgParams := params.CreateOrgParams{
		Name:            "test",
		CredentialsName: "test",
		WebhookSecret:   "test",
	}

	testCreds := config.Github{
		Name:        "test",
		Description: "test",
		OAuth2Token: "test-token",
	}

	storeMock := dbMocks.NewStore(t)
	errMock := fmt.Errorf("mock error")
	storeMock.On("GetOrganization", adminCtx, createOrgParams.Name).Return(params.Organization{}, runnerErrors.ErrNotFound)
	storeMock.On("CreateOrganization", adminCtx, createOrgParams.Name, testCreds.Name, createOrgParams.WebhookSecret).Return(params.Organization{}, errMock)
	runner := Runner{
		credentials: map[string]config.Github{
			"test": testCreds,
		},
		store: storeMock,
	}

	org, err := runner.CreateOrganization(adminCtx, createOrgParams)
	storeMock.AssertExpectations(t)
	require.Equal(t, params.Organization{}, org)
	require.Equal(t, "creating organization: mock error", err.Error())
}

func TestListOrganizations(t *testing.T) {
	adminCtx := auth.GetAdminContext()
	storeMock := dbMocks.NewStore(t)
	var orgs []params.Organization
	storeMock.On("ListOrganizations", adminCtx).Return(orgs, nil)
	runner := Runner{
		store: storeMock,
	}

	org, err := runner.ListOrganizations(adminCtx)
	storeMock.AssertExpectations(t)
	require.Nil(t, err)
	var exceptOrgs []params.Organization
	require.Equal(t, exceptOrgs, org)
}

func TestListOrganizationsErrUnauthorized(t *testing.T) {
	ctx := context.Background()
	runner := Runner{}

	org, err := runner.ListOrganizations(ctx)
	var expectedOrg []params.Organization
	require.Equal(t, expectedOrg, org)
	require.Equal(t, runnerErrors.ErrUnauthorized, err)
}

func TestListOrganizationsOrgMissing(t *testing.T) {
	adminCtx := auth.GetAdminContext()
	storeMock := dbMocks.NewStore(t)

	errMock := fmt.Errorf("mock error")
	storeMock.On("ListOrganizations", adminCtx).Return(nil, errMock)
	runner := Runner{
		store: storeMock,
	}

	org, err := runner.ListOrganizations(adminCtx)
	storeMock.AssertExpectations(t)
	var exceptOrg []params.Organization
	require.Equal(t, exceptOrg, org)
	require.Regexp(t, "listing organizations: mock error", err.Error())
}
