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
	"github.com/cloudbase/garm/database/common"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

const (
	defaultGithubEndpoint   string = "github.com"
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

	_, err = s.db.CreateGithubCredentials(ctx, credParams)
	s.Require().Error(err)
	s.Require().ErrorIs(err, runnerErrors.ErrDuplicateEntity)
}

func TestGithubTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GithubTestSuite))
}
