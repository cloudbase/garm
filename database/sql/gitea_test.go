// Copyright 2024 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.

package sql

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type GiteaTestSuite struct {
	suite.Suite

	giteaEndpoint params.ForgeEndpoint
	db            common.Store
}

func (s *GiteaTestSuite) SetupTest() {
	ctx := context.Background()
	watcher.InitWatcher(ctx)
	db, err := NewSQLDatabase(ctx, garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}

	s.db = db

	createEpParams := params.CreateGiteaEndpointParams{
		Name:        testEndpointName,
		Description: testEndpointDescription,
		APIBaseURL:  testAPIBaseURL,
		BaseURL:     testBaseURL,
	}
	endpoint, err := s.db.CreateGiteaEndpoint(ctx, createEpParams)
	s.Require().NoError(err)
	s.Require().NotNil(endpoint)
	s.Require().Equal(testEndpointName, endpoint.Name)
	s.giteaEndpoint = endpoint
}

func (s *GiteaTestSuite) TearDownTest() {
	watcher.CloseWatcher()
}

func (s *GiteaTestSuite) TestCreatingEndpoint() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	createEpParams := params.CreateGiteaEndpointParams{
		Name:        alternetTestEndpointName,
		Description: testEndpointDescription,
		APIBaseURL:  testAPIBaseURL,
		BaseURL:     testBaseURL,
	}

	endpoint, err := s.db.CreateGiteaEndpoint(ctx, createEpParams)
	s.Require().NoError(err)
	s.Require().NotNil(endpoint)
	s.Require().Equal(alternetTestEndpointName, endpoint.Name)
}

func (s *GiteaTestSuite) TestCreatingDuplicateEndpointFails() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	createEpParams := params.CreateGiteaEndpointParams{
		Name:        alternetTestEndpointName,
		Description: testEndpointDescription,
		APIBaseURL:  testAPIBaseURL,
		BaseURL:     testBaseURL,
	}

	_, err := s.db.CreateGiteaEndpoint(ctx, createEpParams)
	s.Require().NoError(err)

	_, err = s.db.CreateGiteaEndpoint(ctx, createEpParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrDuplicateEntity)
}

func (s *GiteaTestSuite) TestGetEndpoint() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	createEpParams := params.CreateGiteaEndpointParams{
		Name:        alternetTestEndpointName,
		Description: testEndpointDescription,
		APIBaseURL:  testAPIBaseURL,
		BaseURL:     testBaseURL,
	}

	newEndpoint, err := s.db.CreateGiteaEndpoint(ctx, createEpParams)
	s.Require().NoError(err)

	endpoint, err := s.db.GetGiteaEndpoint(ctx, createEpParams.Name)
	s.Require().NoError(err)
	s.Require().NotNil(endpoint)
	s.Require().Equal(newEndpoint.Name, endpoint.Name)
}

func (s *GiteaTestSuite) TestGetNonExistingEndpointFailsWithNotFoundError() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	_, err := s.db.GetGiteaEndpoint(ctx, "non-existing")
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GiteaTestSuite) TestDeletingNonExistingEndpointIsANoop() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	err := s.db.DeleteGiteaEndpoint(ctx, "non-existing")
	s.Require().NoError(err)
}

func (s *GiteaTestSuite) TestDeletingEndpoint() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	createEpParams := params.CreateGiteaEndpointParams{
		Name:        alternetTestEndpointName,
		Description: testEndpointDescription,
		APIBaseURL:  testAPIBaseURL,
		BaseURL:     testBaseURL,
	}

	endpoint, err := s.db.CreateGiteaEndpoint(ctx, createEpParams)
	s.Require().NoError(err)
	s.Require().NotNil(endpoint)

	err = s.db.DeleteGiteaEndpoint(ctx, alternetTestEndpointName)
	s.Require().NoError(err)

	_, err = s.db.GetGiteaEndpoint(ctx, alternetTestEndpointName)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GiteaTestSuite) TestUpdateEndpoint() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	createEpParams := params.CreateGiteaEndpointParams{
		Name:        "deleteme",
		Description: testEndpointDescription,
		APIBaseURL:  testAPIBaseURL,
		BaseURL:     testBaseURL,
	}

	endpoint, err := s.db.CreateGiteaEndpoint(ctx, createEpParams)
	s.Require().NoError(err)
	s.Require().NotNil(endpoint)

	newDescription := "another description"
	newAPIBaseURL := "https://updated.example.com"
	newBaseURL := "https://updated.example.com"
	caCertBundle, err := os.ReadFile("../../testdata/certs/srv-pub.pem")
	s.Require().NoError(err)
	updateEpParams := params.UpdateGiteaEndpointParams{
		Description:  &newDescription,
		APIBaseURL:   &newAPIBaseURL,
		BaseURL:      &newBaseURL,
		CACertBundle: caCertBundle,
	}

	updatedEndpoint, err := s.db.UpdateGiteaEndpoint(ctx, testEndpointName, updateEpParams)
	s.Require().NoError(err)
	s.Require().NotNil(updatedEndpoint)
	s.Require().Equal(newDescription, updatedEndpoint.Description)
	s.Require().Equal(newAPIBaseURL, updatedEndpoint.APIBaseURL)
	s.Require().Equal(newBaseURL, updatedEndpoint.BaseURL)
	s.Require().Equal(caCertBundle, updatedEndpoint.CACertBundle)
}

func (s *GiteaTestSuite) TestUpdatingNonExistingEndpointReturnsNotFoundError() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	newDescription := "test desc"
	updateEpParams := params.UpdateGiteaEndpointParams{
		Description: &newDescription,
	}

	_, err := s.db.UpdateGiteaEndpoint(ctx, "non-existing", updateEpParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GiteaTestSuite) TestListEndpoints() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	createEpParams := params.CreateGiteaEndpointParams{
		Name:        alternetTestEndpointName,
		Description: testEndpointDescription,
		APIBaseURL:  testAPIBaseURL,
		BaseURL:     testBaseURL,
	}

	_, err := s.db.CreateGiteaEndpoint(ctx, createEpParams)
	s.Require().NoError(err)

	endpoints, err := s.db.ListGiteaEndpoints(ctx)
	s.Require().NoError(err)
	s.Require().Len(endpoints, 2)
}

func (s *GiteaTestSuite) TestCreateCredentialsFailsWithUnauthorizedForAnonUser() {
	ctx := context.Background()

	_, err := s.db.CreateGiteaCredentials(ctx, params.CreateGiteaCredentialsParams{})
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *GiteaTestSuite) TestCreateCredentialsFailsWhenEndpointNameIsEmpty() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	_, err := s.db.CreateGiteaCredentials(ctx, params.CreateGiteaCredentialsParams{})
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)
	s.Require().Regexp("endpoint name is required", err.Error())
}

func (s *GiteaTestSuite) TestCreateCredentialsFailsWhenEndpointDoesNotExist() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	_, err := s.db.CreateGiteaCredentials(ctx, params.CreateGiteaCredentialsParams{Endpoint: "non-existing"})
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
	s.Require().Regexp("error creating gitea credentials: gitea endpoint \"non-existing\" not found", err.Error())
}

func (s *GiteaTestSuite) TestCreateCredentialsFailsWhenAuthTypeIsInvalid() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	_, err := s.db.CreateGiteaCredentials(ctx, params.CreateGiteaCredentialsParams{Endpoint: s.giteaEndpoint.Name, AuthType: "invalid"})
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)
	s.Require().Regexp("invalid auth type", err.Error())
}

func (s *GiteaTestSuite) TestCreateCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	credParams := params.CreateGiteaCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    s.giteaEndpoint.Name,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGiteaCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)
	s.Require().Equal(credParams.Name, creds.Name)
	s.Require().Equal(credParams.Description, creds.Description)
	s.Require().Equal(credParams.Endpoint, creds.Endpoint.Name)
	s.Require().Equal(credParams.AuthType, creds.AuthType)
}

func (s *GiteaTestSuite) TestCreateCredentialsFailsOnDuplicateCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())
	testUser := garmTesting.CreateGARMTestUser(ctx, "testuser", s.db, s.T())
	testUserCtx := auth.PopulateContext(context.Background(), testUser, nil)

	credParams := params.CreateGiteaCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    s.giteaEndpoint.Name,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	_, err := s.db.CreateGiteaCredentials(ctx, credParams)
	s.Require().NoError(err)

	// Creating creds with the same parameters should fail for the same user.
	_, err = s.db.CreateGiteaCredentials(ctx, credParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrDuplicateEntity)

	// Creating creds with the same parameters should work for different users.
	_, err = s.db.CreateGiteaCredentials(testUserCtx, credParams)
	s.Require().NoError(err)
}

func (s *GiteaTestSuite) TestNormalUsersCanOnlySeeTheirOwnCredentialsAdminCanSeeAll() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())
	testUser := garmTesting.CreateGARMTestUser(ctx, "testuser1", s.db, s.T())
	testUser2 := garmTesting.CreateGARMTestUser(ctx, "testuser2", s.db, s.T())
	testUserCtx := auth.PopulateContext(context.Background(), testUser, nil)
	testUser2Ctx := auth.PopulateContext(context.Background(), testUser2, nil)

	credParams := params.CreateGiteaCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    s.giteaEndpoint.Name,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGiteaCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	credParams.Name = "test-creds2"
	creds2, err := s.db.CreateGiteaCredentials(testUserCtx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds2)

	credParams.Name = "test-creds3"
	creds3, err := s.db.CreateGiteaCredentials(testUser2Ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds3)

	credsList, err := s.db.ListGiteaCredentials(ctx)
	s.Require().NoError(err)
	s.Require().Len(credsList, 3)

	credsList, err = s.db.ListGiteaCredentials(testUserCtx)
	s.Require().NoError(err)
	s.Require().Len(credsList, 1)
	s.Require().Equal("test-creds2", credsList[0].Name)

	credsList, err = s.db.ListGiteaCredentials(testUser2Ctx)
	s.Require().NoError(err)
	s.Require().Len(credsList, 1)
	s.Require().Equal("test-creds3", credsList[0].Name)
}

func (s *GiteaTestSuite) TestGetGiteaCredentialsFailsWhenCredentialsDontExist() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	_, err := s.db.GetGiteaCredentials(ctx, 1, true)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)

	_, err = s.db.GetGiteaCredentialsByName(ctx, "non-existing", true)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GiteaTestSuite) TestGetGithubCredentialsByNameReturnsOnlyCurrentUserCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())
	testUser := garmTesting.CreateGARMTestUser(ctx, "test-user1", s.db, s.T())
	testUserCtx := auth.PopulateContext(context.Background(), testUser, nil)

	credParams := params.CreateGiteaCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    s.giteaEndpoint.Name,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGiteaCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	creds2, err := s.db.CreateGiteaCredentials(testUserCtx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds2)

	creds2Get, err := s.db.GetGiteaCredentialsByName(testUserCtx, testCredsName, true)
	s.Require().NoError(err)
	s.Require().NotNil(creds2)
	s.Require().Equal(testCredsName, creds2Get.Name)
	s.Require().Equal(creds2.ID, creds2Get.ID)

	credsGet, err := s.db.GetGiteaCredentialsByName(ctx, testCredsName, true)
	s.Require().NoError(err)
	s.Require().NotNil(creds)
	s.Require().Equal(testCredsName, credsGet.Name)
	s.Require().Equal(creds.ID, credsGet.ID)

	// Admin can get any creds by ID
	credsGet, err = s.db.GetGiteaCredentials(ctx, creds2.ID, true)
	s.Require().NoError(err)
	s.Require().NotNil(creds2)
	s.Require().Equal(creds2.ID, credsGet.ID)

	// Normal user cannot get other user creds by ID
	_, err = s.db.GetGiteaCredentials(testUserCtx, creds.ID, true)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GiteaTestSuite) TestGetGithubCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	credParams := params.CreateGiteaCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    s.giteaEndpoint.Name,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGiteaCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	creds2, err := s.db.GetGiteaCredentialsByName(ctx, testCredsName, true)
	s.Require().NoError(err)
	s.Require().NotNil(creds2)
	s.Require().Equal(creds.Name, creds2.Name)
	s.Require().Equal(creds.ID, creds2.ID)

	creds2, err = s.db.GetGiteaCredentials(ctx, creds.ID, true)
	s.Require().NoError(err)
	s.Require().NotNil(creds2)
	s.Require().Equal(creds.Name, creds2.Name)
	s.Require().Equal(creds.ID, creds2.ID)
}

func (s *GiteaTestSuite) TestDeleteGiteaCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	credParams := params.CreateGiteaCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    s.giteaEndpoint.Name,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGiteaCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	err = s.db.DeleteGiteaCredentials(ctx, creds.ID)
	s.Require().NoError(err)

	_, err = s.db.GetGiteaCredentials(ctx, creds.ID, true)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GiteaTestSuite) TestDeleteGiteaCredentialsByNonAdminUser() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())
	testUser := garmTesting.CreateGARMTestUser(ctx, "test-user4", s.db, s.T())
	testUserCtx := auth.PopulateContext(context.Background(), testUser, nil)

	credParams := params.CreateGiteaCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    s.giteaEndpoint.Name,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test-creds4",
		},
	}

	// Create creds as admin
	creds, err := s.db.CreateGiteaCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	// Deleting non existent creds will return a nil error. For the test user
	// the creds created by the admin should not be visible, which leads to not found
	// which in turn returns no error.
	err = s.db.DeleteGiteaCredentials(testUserCtx, creds.ID)
	s.Require().NoError(err)

	// Check that the creds created by the admin are still there.
	credsGet, err := s.db.GetGiteaCredentials(ctx, creds.ID, true)
	s.Require().NoError(err)
	s.Require().NotNil(credsGet)
	s.Require().Equal(creds.ID, credsGet.ID)

	// Create the same creds with the test user.
	creds2, err := s.db.CreateGiteaCredentials(testUserCtx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds2)

	// Remove creds created by test user.
	err = s.db.DeleteGiteaCredentials(testUserCtx, creds2.ID)
	s.Require().NoError(err)

	// The creds created by the test user should be gone.
	_, err = s.db.GetGiteaCredentials(testUserCtx, creds2.ID, true)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GiteaTestSuite) TestDeleteCredentialsFailsIfReposOrgsOrEntitiesUseIt() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	credParams := params.CreateGiteaCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    s.giteaEndpoint.Name,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGiteaCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	repo, err := s.db.CreateRepository(ctx, "test-owner", "test-repo", creds, "superSecret@123BlaBla", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotNil(repo)

	err = s.db.DeleteGiteaCredentials(ctx, creds.ID)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)

	err = s.db.DeleteRepository(ctx, repo.ID)
	s.Require().NoError(err)

	org, err := s.db.CreateOrganization(ctx, "test-org", creds, "superSecret@123BlaBla", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotNil(org)

	err = s.db.DeleteGiteaCredentials(ctx, creds.ID)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)

	err = s.db.DeleteOrganization(ctx, org.ID)
	s.Require().NoError(err)

	enterprise, err := s.db.CreateEnterprise(ctx, "test-enterprise", creds, "superSecret@123BlaBla", params.PoolBalancerTypeRoundRobin)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)
	s.Require().Equal(params.Enterprise{}, enterprise)

	err = s.db.DeleteGiteaCredentials(ctx, creds.ID)
	s.Require().NoError(err)

	_, err = s.db.GetGiteaCredentials(ctx, creds.ID, true)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GiteaTestSuite) TestUpdateCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	credParams := params.CreateGiteaCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    s.giteaEndpoint.Name,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGiteaCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	newDescription := "just a description"
	newName := "new-name"
	newToken := "new-token"
	updateCredParams := params.UpdateGiteaCredentialsParams{
		Description: &newDescription,
		Name:        &newName,
		PAT: &params.GithubPAT{
			OAuth2Token: newToken,
		},
	}

	updatedCreds, err := s.db.UpdateGiteaCredentials(ctx, creds.ID, updateCredParams)
	s.Require().NoError(err)
	s.Require().NotNil(updatedCreds)
	s.Require().Equal(newDescription, updatedCreds.Description)
	s.Require().Equal(newName, updatedCreds.Name)
}

func (s *GiteaTestSuite) TestUpdateCredentialsFailsForNonExistingCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	updateCredParams := params.UpdateGiteaCredentialsParams{
		Description: nil,
	}

	_, err := s.db.UpdateGiteaCredentials(ctx, 1, updateCredParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GiteaTestSuite) TestUpdateCredentialsFailsIfCredentialsAreOwnedByNonAdminUser() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())
	testUser := garmTesting.CreateGARMTestUser(ctx, "test-user5", s.db, s.T())
	testUserCtx := auth.PopulateContext(context.Background(), testUser, nil)

	credParams := params.CreateGiteaCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    s.giteaEndpoint.Name,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test-creds5",
		},
	}

	creds, err := s.db.CreateGiteaCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	newDescription := "new params desc"
	updateCredParams := params.UpdateGiteaCredentialsParams{
		Description: &newDescription,
	}

	_, err = s.db.UpdateGiteaCredentials(testUserCtx, creds.ID, updateCredParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GiteaTestSuite) TestAdminUserCanUpdateAnyGiteaCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())
	testUser := garmTesting.CreateGARMTestUser(ctx, "test-user5", s.db, s.T())
	testUserCtx := auth.PopulateContext(context.Background(), testUser, nil)

	credParams := params.CreateGiteaCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    s.giteaEndpoint.Name,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test-creds5",
		},
	}

	creds, err := s.db.CreateGiteaCredentials(testUserCtx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	newDescription := "another new description"
	updateCredParams := params.UpdateGiteaCredentialsParams{
		Description: &newDescription,
	}

	newCreds, err := s.db.UpdateGiteaCredentials(ctx, creds.ID, updateCredParams)
	s.Require().NoError(err)
	s.Require().Equal(newDescription, newCreds.Description)
}

func (s *GiteaTestSuite) TestDeleteCredentialsWithOrgsOrReposFails() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	credParams := params.CreateGiteaCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    s.giteaEndpoint.Name,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test-creds5",
		},
	}

	creds, err := s.db.CreateGiteaCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	repo, err := s.db.CreateRepository(ctx, "test-owner", "test-repo", creds, "superSecret@123BlaBla", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotNil(repo)

	err = s.db.DeleteGiteaCredentials(ctx, creds.ID)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)

	err = s.db.DeleteRepository(ctx, repo.ID)
	s.Require().NoError(err)

	org, err := s.db.CreateOrganization(ctx, "test-org", creds, "superSecret@123BlaBla", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotNil(org)

	err = s.db.DeleteGiteaCredentials(ctx, creds.ID)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)

	err = s.db.DeleteOrganization(ctx, org.ID)
	s.Require().NoError(err)

	err = s.db.DeleteGiteaCredentials(ctx, creds.ID)
	s.Require().NoError(err)

	_, err = s.db.GetGiteaCredentials(ctx, creds.ID, true)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GiteaTestSuite) TestDeleteGiteaEndpointFailsWithOrgsReposOrCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	endpointParams := params.CreateGiteaEndpointParams{
		Name:        "deleteme",
		Description: testEndpointDescription,
		APIBaseURL:  testAPIBaseURL,
		BaseURL:     testBaseURL,
	}

	ep, err := s.db.CreateGiteaEndpoint(ctx, endpointParams)
	s.Require().NoError(err)
	s.Require().NotNil(ep)

	credParams := params.CreateGiteaCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    ep.Name,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test-creds5",
		},
	}

	creds, err := s.db.CreateGiteaCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	repo, err := s.db.CreateRepository(ctx, "test-owner", "test-repo", creds, "superSecret@123BlaBla", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotNil(repo)

	badRequest := &runnerErrors.BadRequestError{}
	err = s.db.DeleteGiteaEndpoint(ctx, ep.Name)
	s.Require().Error(err)
	s.Require().ErrorAs(err, &badRequest)

	err = s.db.DeleteRepository(ctx, repo.ID)
	s.Require().NoError(err)

	org, err := s.db.CreateOrganization(ctx, "test-org", creds, "superSecret@123BlaBla", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotNil(org)

	err = s.db.DeleteGiteaEndpoint(ctx, ep.Name)
	s.Require().Error(err)
	s.Require().ErrorAs(err, &badRequest)

	err = s.db.DeleteOrganization(ctx, org.ID)
	s.Require().NoError(err)

	err = s.db.DeleteGiteaCredentials(ctx, creds.ID)
	s.Require().NoError(err)

	err = s.db.DeleteGiteaEndpoint(ctx, ep.Name)
	s.Require().NoError(err)

	_, err = s.db.GetGiteaEndpoint(ctx, ep.Name)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GiteaTestSuite) TestUpdateEndpointURLsFailsIfCredentialsAreAssociated() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	createEpParams := params.CreateGiteaEndpointParams{
		Name:        "deleteme",
		Description: testEndpointDescription,
		APIBaseURL:  testAPIBaseURL,
		BaseURL:     testBaseURL,
	}

	endpoint, err := s.db.CreateGiteaEndpoint(ctx, createEpParams)
	s.Require().NoError(err)
	s.Require().NotNil(endpoint)

	credParams := params.CreateGiteaCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    testEndpointName,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	_, err = s.db.CreateGiteaCredentials(ctx, credParams)
	s.Require().NoError(err)

	newDescription := "new gitea description"
	newBaseURL := "https://new-gitea.example.com"
	newAPIBaseURL := "https://new-gotea.example.com"
	updateEpParams := params.UpdateGiteaEndpointParams{
		BaseURL: &newBaseURL,
	}

	_, err = s.db.UpdateGiteaEndpoint(ctx, testEndpointName, updateEpParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)
	s.Require().EqualError(err, "error updating gitea endpoint: cannot update endpoint URLs with existing credentials")

	updateEpParams = params.UpdateGiteaEndpointParams{
		APIBaseURL: &newAPIBaseURL,
	}
	_, err = s.db.UpdateGiteaEndpoint(ctx, testEndpointName, updateEpParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)
	s.Require().EqualError(err, "error updating gitea endpoint: cannot update endpoint URLs with existing credentials")

	updateEpParams = params.UpdateGiteaEndpointParams{
		Description: &newDescription,
	}
	ret, err := s.db.UpdateGiteaEndpoint(ctx, testEndpointName, updateEpParams)
	s.Require().NoError(err)
	s.Require().Equal(newDescription, ret.Description)
}

func (s *GiteaTestSuite) TestListGiteaEndpoints() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	createEpParams := params.CreateGiteaEndpointParams{
		Name:        "deleteme",
		Description: testEndpointDescription,
		APIBaseURL:  testAPIBaseURL,
		BaseURL:     testBaseURL,
	}

	_, err := s.db.CreateGiteaEndpoint(ctx, createEpParams)
	s.Require().NoError(err)

	endpoints, err := s.db.ListGiteaEndpoints(ctx)
	s.Require().NoError(err)
	s.Require().Len(endpoints, 2)
}

func TestGiteaTestSuite(t *testing.T) {
	suite.Run(t, new(GiteaTestSuite))
}
