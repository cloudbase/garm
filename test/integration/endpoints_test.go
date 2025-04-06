//go:build integration
// +build integration

package integration

import (
	"github.com/cloudbase/garm/params"
)

func (suite *GarmSuite) TestGithubEndpointOperations() {
	t := suite.T()
	t.Log("Testing endpoint operations")
	suite.MustDefaultGithubEndpoint()

	caBundle, err := getTestFileContents("certs/srv-pub.pem")
	suite.NoError(err)

	endpointParams := params.CreateGithubEndpointParams{
		Name:          "test-endpoint",
		Description:   "Test endpoint",
		BaseURL:       "https://ghes.example.com",
		APIBaseURL:    "https://api.ghes.example.com/",
		UploadBaseURL: "https://uploads.ghes.example.com/",
		CACertBundle:  caBundle,
	}

	endpoint, err := suite.CreateGithubEndpoint(endpointParams)
	suite.NoError(err)
	suite.Equal(endpoint.Name, endpointParams.Name, "Endpoint name mismatch")
	suite.Equal(endpoint.Description, endpointParams.Description, "Endpoint description mismatch")
	suite.Equal(endpoint.BaseURL, endpointParams.BaseURL, "Endpoint base URL mismatch")
	suite.Equal(endpoint.APIBaseURL, endpointParams.APIBaseURL, "Endpoint API base URL mismatch")
	suite.Equal(endpoint.UploadBaseURL, endpointParams.UploadBaseURL, "Endpoint upload base URL mismatch")
	suite.Equal(string(endpoint.CACertBundle), string(caBundle), "Endpoint CA cert bundle mismatch")

	endpoint2 := suite.GetGithubEndpoint(endpointParams.Name)
	suite.NotNil(endpoint, "endpoint is nil")
	suite.NotNil(endpoint2, "endpoint2 is nil")

	err = checkEndpointParamsAreEqual(*endpoint, *endpoint2)
	suite.NoError(err, "endpoint params are not equal")
	endpoints := suite.ListGithubEndpoints()
	suite.NoError(err, "error listing github endpoints")
	var found bool
	for _, ep := range endpoints {
		if ep.Name == endpointParams.Name {
			checkEndpointParamsAreEqual(*endpoint, ep)
			found = true
			break
		}
	}
	suite.Equal(found, true, "endpoint not found in list")

	err = suite.DeleteGithubEndpoint(endpoint.Name)
	suite.NoError(err, "error deleting github endpoint")
}

func (suite *GarmSuite) TestGithubEndpointMustFailToDeleteDefaultGithubEndpoint() {
	t := suite.T()
	t.Log("Testing error when deleting default github.com endpoint")
	err := deleteGithubEndpoint(suite.cli, suite.authToken, "github.com")
	suite.Error(err, "expected error when attempting to delete the default github.com endpoint")
}

func (suite *GarmSuite) TestGithubEndpointFailsOnInvalidCABundle() {
	t := suite.T()
	t.Log("Testing endpoint creation with invalid CA cert bundle")
	badCABundle, err := getTestFileContents("certs/srv-key.pem")
	suite.NoError(err, "error reading CA cert bundle")

	endpointParams := params.CreateGithubEndpointParams{
		Name:          "dummy",
		Description:   "Dummy endpoint",
		BaseURL:       "https://ghes.example.com",
		APIBaseURL:    "https://api.ghes.example.com/",
		UploadBaseURL: "https://uploads.ghes.example.com/",
		CACertBundle:  badCABundle,
	}

	_, err = createGithubEndpoint(suite.cli, suite.authToken, endpointParams)
	suite.Error(err, "expected error when creating endpoint with invalid CA cert bundle")
}

func (suite *GarmSuite) TestGithubEndpointDeletionFailsWhenCredentialsExist() {
	t := suite.T()
	t.Log("Testing endpoint deletion when credentials exist")
	endpointParams := params.CreateGithubEndpointParams{
		Name:          "dummy",
		Description:   "Dummy endpoint",
		BaseURL:       "https://ghes.example.com",
		APIBaseURL:    "https://api.ghes.example.com/",
		UploadBaseURL: "https://uploads.ghes.example.com/",
	}

	endpoint, err := suite.CreateGithubEndpoint(endpointParams)
	suite.NoError(err, "error creating github endpoint")
	creds, err := suite.createDummyCredentials("test-creds", endpoint.Name)
	suite.NoError(err, "error creating dummy credentials")

	err = deleteGithubEndpoint(suite.cli, suite.authToken, endpoint.Name)
	suite.Error(err, "expected error when deleting endpoint with credentials")

	err = suite.DeleteGithubCredential(int64(creds.ID)) //nolint:gosec
	suite.NoError(err, "error deleting credentials")
	err = suite.DeleteGithubEndpoint(endpoint.Name)
	suite.NoError(err, "error deleting endpoint")
}

func (suite *GarmSuite) TestGithubEndpointFailsOnDuplicateName() {
	t := suite.T()
	t.Log("Testing endpoint creation with duplicate name")
	endpointParams := params.CreateGithubEndpointParams{
		Name:          "github.com",
		Description:   "Dummy endpoint",
		BaseURL:       "https://ghes.example.com",
		APIBaseURL:    "https://api.ghes.example.com/",
		UploadBaseURL: "https://uploads.ghes.example.com/",
	}

	_, err := createGithubEndpoint(suite.cli, suite.authToken, endpointParams)
	suite.Error(err, "expected error when creating endpoint with duplicate name")
}

func (suite *GarmSuite) TestGithubEndpointUpdateEndpoint() {
	t := suite.T()
	t.Log("Testing endpoint update")
	endpoint, err := suite.createDummyEndpoint("dummy")
	suite.NoError(err, "error creating dummy endpoint")
	t.Cleanup(func() {
		suite.DeleteGithubEndpoint(endpoint.Name)
	})

	newDescription := "Updated description"
	newBaseURL := "https://ghes2.example.com"
	newAPIBaseURL := "https://api.ghes2.example.com/"
	newUploadBaseURL := "https://uploads.ghes2.example.com/"
	newCABundle, err := getTestFileContents("certs/srv-pub.pem")
	suite.NoError(err, "error reading CA cert bundle")

	updateParams := params.UpdateGithubEndpointParams{
		Description:   &newDescription,
		BaseURL:       &newBaseURL,
		APIBaseURL:    &newAPIBaseURL,
		UploadBaseURL: &newUploadBaseURL,
		CACertBundle:  newCABundle,
	}

	updated, err := updateGithubEndpoint(suite.cli, suite.authToken, endpoint.Name, updateParams)
	suite.NoError(err, "error updating github endpoint")

	suite.Equal(updated.Name, endpoint.Name, "Endpoint name mismatch")
	suite.Equal(updated.Description, newDescription, "Endpoint description mismatch")
	suite.Equal(updated.BaseURL, newBaseURL, "Endpoint base URL mismatch")
	suite.Equal(updated.APIBaseURL, newAPIBaseURL, "Endpoint API base URL mismatch")
	suite.Equal(updated.UploadBaseURL, newUploadBaseURL, "Endpoint upload base URL mismatch")
	suite.Equal(string(updated.CACertBundle), string(newCABundle), "Endpoint CA cert bundle mismatch")
}

func (suite *GarmSuite) MustDefaultGithubEndpoint() {
	ep := suite.GetGithubEndpoint("github.com")

	suite.NotNil(ep, "default GitHub endpoint not found")
	suite.Equal(ep.Name, "github.com", "default GitHub endpoint name mismatch")
}

func (suite *GarmSuite) GetGithubEndpoint(name string) *params.GithubEndpoint {
	t := suite.T()
	t.Log("Get GitHub endpoint")
	endpoint, err := getGithubEndpoint(suite.cli, suite.authToken, name)
	suite.NoError(err, "error getting GitHub endpoint")

	return endpoint
}

func (suite *GarmSuite) CreateGithubEndpoint(params params.CreateGithubEndpointParams) (*params.GithubEndpoint, error) {
	t := suite.T()
	t.Log("Create GitHub endpoint")
	endpoint, err := createGithubEndpoint(suite.cli, suite.authToken, params)
	suite.NoError(err, "error creating GitHub endpoint")

	return endpoint, nil
}

func (suite *GarmSuite) DeleteGithubEndpoint(name string) error {
	t := suite.T()
	t.Log("Delete GitHub endpoint")
	err := deleteGithubEndpoint(suite.cli, suite.authToken, name)
	suite.NoError(err, "error deleting GitHub endpoint")

	return nil
}

func (suite *GarmSuite) ListGithubEndpoints() params.GithubEndpoints {
	t := suite.T()
	t.Log("List GitHub endpoints")
	endpoints, err := listGithubEndpoints(suite.cli, suite.authToken)
	suite.NoError(err, "error listing GitHub endpoints")

	return endpoints
}

func (suite *GarmSuite) createDummyEndpoint(name string) (*params.GithubEndpoint, error) {
	endpointParams := params.CreateGithubEndpointParams{
		Name:          name,
		Description:   "Dummy endpoint",
		BaseURL:       "https://ghes.example.com",
		APIBaseURL:    "https://api.ghes.example.com/",
		UploadBaseURL: "https://uploads.ghes.example.com/",
	}

	return suite.CreateGithubEndpoint(endpointParams)
}
