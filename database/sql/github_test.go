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
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

const (
	testUploadBaseURL       string = "https://uploads.example.com"
	testBaseURL             string = "https://example.com"
	testAPIBaseURL          string = "https://api.example.com"
	testEndpointName        string = "test-endpoint"
	testEndpointDescription string = "test description"
	testCredsName           string = "test-creds"
	testCredsDescription    string = "test creds"
)

type GithubTestSuite struct {
	suite.Suite

	db common.Store
}

func (s *GithubTestSuite) SetupTest() {
	db, err := NewSQLDatabase(context.Background(), garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
	s.db = db
}

func (s *GithubTestSuite) TestDefaultEndpointGetsCreatedAutomatically() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())
	endpoint, err := s.db.GetGithubEndpoint(ctx, defaultGithubEndpoint)
	s.Require().NoError(err)
	s.Require().NotNil(endpoint)
}

func (s *GithubTestSuite) TestDeletingDefaultEndpointFails() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())
	err := s.db.DeleteGithubEndpoint(ctx, defaultGithubEndpoint)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)
}

func (s *GithubTestSuite) TestCreatingEndpoint() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	createEpParams := params.CreateGithubEndpointParams{
		Name:          testEndpointName,
		Description:   testEndpointDescription,
		APIBaseURL:    testAPIBaseURL,
		UploadBaseURL: testUploadBaseURL,
		BaseURL:       testBaseURL,
	}

	endpoint, err := s.db.CreateGithubEndpoint(ctx, createEpParams)
	s.Require().NoError(err)
	s.Require().NotNil(endpoint)
	s.Require().Equal(testEndpointName, endpoint.Name)
}

func (s *GithubTestSuite) TestCreatingDuplicateEndpointFails() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	createEpParams := params.CreateGithubEndpointParams{
		Name:          testEndpointName,
		Description:   testEndpointDescription,
		APIBaseURL:    testAPIBaseURL,
		UploadBaseURL: testUploadBaseURL,
		BaseURL:       testBaseURL,
	}

	_, err := s.db.CreateGithubEndpoint(ctx, createEpParams)
	s.Require().NoError(err)

	_, err = s.db.CreateGithubEndpoint(ctx, createEpParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrDuplicateEntity)
}

func (s *GithubTestSuite) TestGetEndpoint() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	endpoint, err := s.db.GetGithubEndpoint(ctx, defaultGithubEndpoint)
	s.Require().NoError(err)
	s.Require().NotNil(endpoint)
	s.Require().Equal(defaultGithubEndpoint, endpoint.Name)
}

func (s *GithubTestSuite) TestGetNonExistingEndpointFailsWithNotFoundError() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	_, err := s.db.GetGithubEndpoint(ctx, "non-existing")
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GithubTestSuite) TestDeletingNonExistingEndpointIsANoop() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	err := s.db.DeleteGithubEndpoint(ctx, "non-existing")
	s.Require().NoError(err)
}

func (s *GithubTestSuite) TestDeletingEndpoint() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	createEpParams := params.CreateGithubEndpointParams{
		Name:          testEndpointName,
		Description:   testEndpointDescription,
		APIBaseURL:    testAPIBaseURL,
		UploadBaseURL: testUploadBaseURL,
		BaseURL:       testBaseURL,
	}

	endpoint, err := s.db.CreateGithubEndpoint(ctx, createEpParams)
	s.Require().NoError(err)
	s.Require().NotNil(endpoint)

	err = s.db.DeleteGithubEndpoint(ctx, testEndpointName)
	s.Require().NoError(err)

	_, err = s.db.GetGithubEndpoint(ctx, testEndpointName)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GithubTestSuite) TestUpdateEndpoint() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	createEpParams := params.CreateGithubEndpointParams{
		Name:          testEndpointName,
		Description:   testEndpointDescription,
		APIBaseURL:    testAPIBaseURL,
		UploadBaseURL: testUploadBaseURL,
		BaseURL:       testBaseURL,
	}

	endpoint, err := s.db.CreateGithubEndpoint(ctx, createEpParams)
	s.Require().NoError(err)
	s.Require().NotNil(endpoint)

	newDescription := "new description"
	newAPIBaseURL := "https://new-api.example.com"
	newUploadBaseURL := "https://new-uploads.example.com"
	newBaseURL := "https://new.example.com"
	caCertBundle, err := os.ReadFile("../../testdata/certs/srv-pub.pem")
	s.Require().NoError(err)
	updateEpParams := params.UpdateGithubEndpointParams{
		Description:   &newDescription,
		APIBaseURL:    &newAPIBaseURL,
		UploadBaseURL: &newUploadBaseURL,
		BaseURL:       &newBaseURL,
		CACertBundle:  caCertBundle,
	}

	updatedEndpoint, err := s.db.UpdateGithubEndpoint(ctx, testEndpointName, updateEpParams)
	s.Require().NoError(err)
	s.Require().NotNil(updatedEndpoint)
	s.Require().Equal(newDescription, updatedEndpoint.Description)
	s.Require().Equal(newAPIBaseURL, updatedEndpoint.APIBaseURL)
	s.Require().Equal(newUploadBaseURL, updatedEndpoint.UploadBaseURL)
	s.Require().Equal(newBaseURL, updatedEndpoint.BaseURL)
	s.Require().Equal(caCertBundle, updatedEndpoint.CACertBundle)
}

func (s *GithubTestSuite) TestUpdatingNonExistingEndpointReturnsNotFoundError() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	newDescription := "test"
	updateEpParams := params.UpdateGithubEndpointParams{
		Description: &newDescription,
	}

	_, err := s.db.UpdateGithubEndpoint(ctx, "non-existing", updateEpParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GithubTestSuite) TestListEndpoints() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	createEpParams := params.CreateGithubEndpointParams{
		Name:          testEndpointName,
		Description:   testEndpointDescription,
		APIBaseURL:    testAPIBaseURL,
		UploadBaseURL: testUploadBaseURL,
		BaseURL:       testBaseURL,
	}

	_, err := s.db.CreateGithubEndpoint(ctx, createEpParams)
	s.Require().NoError(err)

	endpoints, err := s.db.ListGithubEndpoints(ctx)
	s.Require().NoError(err)
	s.Require().Len(endpoints, 2)
}

func (s *GithubTestSuite) TestCreateCredentialsFailsWithUnauthorizedForAnonUser() {
	ctx := context.Background()

	_, err := s.db.CreateGithubCredentials(ctx, params.CreateGithubCredentialsParams{})
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrUnauthorized)
}

func (s *GithubTestSuite) TestCreateCredentialsFailsWhenEndpointNameIsEmpty() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	_, err := s.db.CreateGithubCredentials(ctx, params.CreateGithubCredentialsParams{})
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)
	s.Require().Regexp("endpoint name is required", err.Error())
}

func (s *GithubTestSuite) TestCreateCredentialsFailsWhenEndpointDoesNotExist() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	_, err := s.db.CreateGithubCredentials(ctx, params.CreateGithubCredentialsParams{Endpoint: "non-existing"})
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
	s.Require().Regexp("endpoint not found", err.Error())
}

func (s *GithubTestSuite) TestCreateCredentialsFailsWhenAuthTypeIsInvalid() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	_, err := s.db.CreateGithubCredentials(ctx, params.CreateGithubCredentialsParams{Endpoint: defaultGithubEndpoint, AuthType: "invalid"})
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)
	s.Require().Regexp("invalid auth type", err.Error())
}

func (s *GithubTestSuite) TestCreateCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	credParams := params.CreateGithubCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    defaultGithubEndpoint,
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGithubCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)
	s.Require().Equal(credParams.Name, creds.Name)
	s.Require().Equal(credParams.Description, creds.Description)
	s.Require().Equal(credParams.Endpoint, creds.Endpoint.Name)
	s.Require().Equal(credParams.AuthType, creds.AuthType)
}

func (s *GithubTestSuite) TestCreateCredentialsFailsOnDuplicateCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())
	testUser := garmTesting.CreateGARMTestUser(ctx, "testuser", s.db, s.T())
	testUserCtx := auth.PopulateContext(context.Background(), testUser)

	credParams := params.CreateGithubCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    defaultGithubEndpoint,
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	_, err := s.db.CreateGithubCredentials(ctx, credParams)
	s.Require().NoError(err)

	// Creating creds with the same parameters should fail for the same user.
	_, err = s.db.CreateGithubCredentials(ctx, credParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrDuplicateEntity)

	// Creating creds with the same parameters should work for different users.
	_, err = s.db.CreateGithubCredentials(testUserCtx, credParams)
	s.Require().NoError(err)
}

func (s *GithubTestSuite) TestNormalUsersCanOnlySeeTheirOwnCredentialsAdminCanSeeAll() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())
	testUser := garmTesting.CreateGARMTestUser(ctx, "testuser1", s.db, s.T())
	testUser2 := garmTesting.CreateGARMTestUser(ctx, "testuser2", s.db, s.T())
	testUserCtx := auth.PopulateContext(context.Background(), testUser)
	testUser2Ctx := auth.PopulateContext(context.Background(), testUser2)

	credParams := params.CreateGithubCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    defaultGithubEndpoint,
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGithubCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	credParams.Name = "test-creds2"
	creds2, err := s.db.CreateGithubCredentials(testUserCtx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds2)

	credParams.Name = "test-creds3"
	creds3, err := s.db.CreateGithubCredentials(testUser2Ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds3)

	credsList, err := s.db.ListGithubCredentials(ctx)
	s.Require().NoError(err)
	s.Require().Len(credsList, 3)

	credsList, err = s.db.ListGithubCredentials(testUserCtx)
	s.Require().NoError(err)
	s.Require().Len(credsList, 1)
	s.Require().Equal("test-creds2", credsList[0].Name)

	credsList, err = s.db.ListGithubCredentials(testUser2Ctx)
	s.Require().NoError(err)
	s.Require().Len(credsList, 1)
	s.Require().Equal("test-creds3", credsList[0].Name)
}

func (s *GithubTestSuite) TestGetGithubCredentialsFailsWhenCredentialsDontExist() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	_, err := s.db.GetGithubCredentials(ctx, 1, true)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)

	_, err = s.db.GetGithubCredentialsByName(ctx, "non-existing", true)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GithubTestSuite) TestGetGithubCredentialsByNameReturnsOnlyCurrentUserCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())
	testUser := garmTesting.CreateGARMTestUser(ctx, "test-user1", s.db, s.T())
	testUserCtx := auth.PopulateContext(context.Background(), testUser)

	credParams := params.CreateGithubCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    defaultGithubEndpoint,
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGithubCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	creds2, err := s.db.CreateGithubCredentials(testUserCtx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds2)

	creds2Get, err := s.db.GetGithubCredentialsByName(testUserCtx, testCredsName, true)
	s.Require().NoError(err)
	s.Require().NotNil(creds2)
	s.Require().Equal(testCredsName, creds2Get.Name)
	s.Require().Equal(creds2.ID, creds2Get.ID)

	credsGet, err := s.db.GetGithubCredentialsByName(ctx, testCredsName, true)
	s.Require().NoError(err)
	s.Require().NotNil(creds)
	s.Require().Equal(testCredsName, credsGet.Name)
	s.Require().Equal(creds.ID, credsGet.ID)

	// Admin can get any creds by ID
	credsGet, err = s.db.GetGithubCredentials(ctx, creds2.ID, true)
	s.Require().NoError(err)
	s.Require().NotNil(creds2)
	s.Require().Equal(creds2.ID, credsGet.ID)

	// Normal user cannot get other user creds by ID
	_, err = s.db.GetGithubCredentials(testUserCtx, creds.ID, true)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GithubTestSuite) TestGetGithubCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	credParams := params.CreateGithubCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    defaultGithubEndpoint,
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGithubCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	creds2, err := s.db.GetGithubCredentialsByName(ctx, testCredsName, true)
	s.Require().NoError(err)
	s.Require().NotNil(creds2)
	s.Require().Equal(creds.Name, creds2.Name)
	s.Require().Equal(creds.ID, creds2.ID)

	creds2, err = s.db.GetGithubCredentials(ctx, creds.ID, true)
	s.Require().NoError(err)
	s.Require().NotNil(creds2)
	s.Require().Equal(creds.Name, creds2.Name)
	s.Require().Equal(creds.ID, creds2.ID)
}

func (s *GithubTestSuite) TestDeleteGithubCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	credParams := params.CreateGithubCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    defaultGithubEndpoint,
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGithubCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	err = s.db.DeleteGithubCredentials(ctx, creds.ID)
	s.Require().NoError(err)

	_, err = s.db.GetGithubCredentials(ctx, creds.ID, true)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GithubTestSuite) TestDeleteGithubCredentialsByNonAdminUser() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())
	testUser := garmTesting.CreateGARMTestUser(ctx, "test-user4", s.db, s.T())
	testUserCtx := auth.PopulateContext(context.Background(), testUser)

	credParams := params.CreateGithubCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    defaultGithubEndpoint,
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test-creds4",
		},
	}

	// Create creds as admin
	creds, err := s.db.CreateGithubCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	// Deleting non existent creds will return a nil error. For the test user
	// the creds created by the admin should not be visible, which leads to not found
	// which in turn returns no error.
	err = s.db.DeleteGithubCredentials(testUserCtx, creds.ID)
	s.Require().NoError(err)

	// Check that the creds created by the admin are still there.
	credsGet, err := s.db.GetGithubCredentials(ctx, creds.ID, true)
	s.Require().NoError(err)
	s.Require().NotNil(credsGet)
	s.Require().Equal(creds.ID, credsGet.ID)

	// Create the same creds with the test user.
	creds2, err := s.db.CreateGithubCredentials(testUserCtx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds2)

	// Remove creds created by test user.
	err = s.db.DeleteGithubCredentials(testUserCtx, creds2.ID)
	s.Require().NoError(err)

	// The creds created by the test user should be gone.
	_, err = s.db.GetGithubCredentials(testUserCtx, creds2.ID, true)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GithubTestSuite) TestDeleteCredentialsFailsIfReposOrgsOrEntitiesUseIt() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	credParams := params.CreateGithubCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    defaultGithubEndpoint,
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGithubCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	repo, err := s.db.CreateRepository(ctx, "test-owner", "test-repo", creds.Name, "superSecret@123BlaBla", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotNil(repo)

	err = s.db.DeleteGithubCredentials(ctx, creds.ID)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)

	err = s.db.DeleteRepository(ctx, repo.ID)
	s.Require().NoError(err)

	org, err := s.db.CreateOrganization(ctx, "test-org", creds.Name, "superSecret@123BlaBla", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotNil(org)

	err = s.db.DeleteGithubCredentials(ctx, creds.ID)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)

	err = s.db.DeleteOrganization(ctx, org.ID)
	s.Require().NoError(err)

	enterprise, err := s.db.CreateEnterprise(ctx, "test-enterprise", creds.Name, "superSecret@123BlaBla", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotNil(enterprise)

	err = s.db.DeleteGithubCredentials(ctx, creds.ID)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)

	err = s.db.DeleteEnterprise(ctx, enterprise.ID)
	s.Require().NoError(err)

	err = s.db.DeleteGithubCredentials(ctx, creds.ID)
	s.Require().NoError(err)

	_, err = s.db.GetGithubCredentials(ctx, creds.ID, true)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GithubTestSuite) TestUpdateCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	credParams := params.CreateGithubCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    defaultGithubEndpoint,
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGithubCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	newDescription := "new description"
	newName := "new-name"
	newToken := "new-token"
	updateCredParams := params.UpdateGithubCredentialsParams{
		Description: &newDescription,
		Name:        &newName,
		PAT: &params.GithubPAT{
			OAuth2Token: newToken,
		},
	}

	updatedCreds, err := s.db.UpdateGithubCredentials(ctx, creds.ID, updateCredParams)
	s.Require().NoError(err)
	s.Require().NotNil(updatedCreds)
	s.Require().Equal(newDescription, updatedCreds.Description)
	s.Require().Equal(newName, updatedCreds.Name)
}

func (s *GithubTestSuite) TestUpdateGithubCredentialsFailIfWrongCredentialTypeIsPassed() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	credParams := params.CreateGithubCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    defaultGithubEndpoint,
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	creds, err := s.db.CreateGithubCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	updateCredParams := params.UpdateGithubCredentialsParams{
		App: &params.GithubApp{
			AppID:           1,
			InstallationID:  2,
			PrivateKeyBytes: []byte("test"),
		},
	}

	_, err = s.db.UpdateGithubCredentials(ctx, creds.ID, updateCredParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)
	s.Require().EqualError(err, "updating github credentials: cannot update app credentials for PAT: invalid request")

	credParamsWithApp := params.CreateGithubCredentialsParams{
		Name:        "test-credsApp",
		Description: "test credsApp",
		Endpoint:    defaultGithubEndpoint,
		AuthType:    params.GithubAuthTypeApp,
		App: params.GithubApp{
			AppID:           1,
			InstallationID:  2,
			PrivateKeyBytes: []byte("test"),
		},
	}

	credsApp, err := s.db.CreateGithubCredentials(ctx, credParamsWithApp)
	s.Require().NoError(err)
	s.Require().NotNil(credsApp)

	updateCredParams = params.UpdateGithubCredentialsParams{
		PAT: &params.GithubPAT{
			OAuth2Token: "test",
		},
	}

	_, err = s.db.UpdateGithubCredentials(ctx, credsApp.ID, updateCredParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrBadRequest)
	s.Require().EqualError(err, "updating github credentials: cannot update PAT credentials for app: invalid request")
}

func (s *GithubTestSuite) TestUpdateCredentialsFailsForNonExistingCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())

	updateCredParams := params.UpdateGithubCredentialsParams{
		Description: nil,
	}

	_, err := s.db.UpdateGithubCredentials(ctx, 1, updateCredParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GithubTestSuite) TestUpdateCredentialsFailsIfCredentialsAreOwnedByNonAdminUser() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())
	testUser := garmTesting.CreateGARMTestUser(ctx, "test-user5", s.db, s.T())
	testUserCtx := auth.PopulateContext(context.Background(), testUser)

	credParams := params.CreateGithubCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    defaultGithubEndpoint,
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test-creds5",
		},
	}

	creds, err := s.db.CreateGithubCredentials(ctx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	newDescription := "new description2"
	updateCredParams := params.UpdateGithubCredentialsParams{
		Description: &newDescription,
	}

	_, err = s.db.UpdateGithubCredentials(testUserCtx, creds.ID, updateCredParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrNotFound)
}

func (s *GithubTestSuite) TestAdminUserCanUpdateAnyGithubCredentials() {
	ctx := garmTesting.ImpersonateAdminContext(context.Background(), s.db, s.T())
	testUser := garmTesting.CreateGARMTestUser(ctx, "test-user5", s.db, s.T())
	testUserCtx := auth.PopulateContext(context.Background(), testUser)

	credParams := params.CreateGithubCredentialsParams{
		Name:        testCredsName,
		Description: testCredsDescription,
		Endpoint:    defaultGithubEndpoint,
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "test-creds5",
		},
	}

	creds, err := s.db.CreateGithubCredentials(testUserCtx, credParams)
	s.Require().NoError(err)
	s.Require().NotNil(creds)

	newDescription := "new description2"
	updateCredParams := params.UpdateGithubCredentialsParams{
		Description: &newDescription,
	}

	newCreds, err := s.db.UpdateGithubCredentials(ctx, creds.ID, updateCredParams)
	s.Require().NoError(err)
	s.Require().Equal(newDescription, newCreds.Description)
}

func TestGithubTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GithubTestSuite))
}
