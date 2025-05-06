//go:build integration
// +build integration

package integration

import (
	"github.com/cloudbase/garm/params"
)

const (
	defaultEndpointName  string = "github.com"
	dummyCredentialsName string = "dummy"
)

func (suite *GarmSuite) TestGithubCredentialsErrorOnDuplicateCredentialsName() {
	t := suite.T()
	t.Log("Testing error on duplicate credentials name")
	creds, err := suite.createDummyCredentials(dummyCredentialsName, defaultEndpointName)
	suite.NoError(err)
	t.Cleanup(func() {
		suite.DeleteGithubCredential(int64(creds.ID)) //nolint:gosec
	})

	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        dummyCredentialsName,
		Endpoint:    defaultEndpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "dummy",
		},
	}
	_, err = createGithubCredentials(suite.cli, suite.authToken, createCredsParams)
	suite.Error(err, "expected error when creating credentials with duplicate name")
}

func (suite *GarmSuite) TestGithubCredentialsFailsToDeleteWhenInUse() {
	t := suite.T()
	t.Log("Testing error when deleting credentials in use")
	creds, err := suite.createDummyCredentials(dummyCredentialsName, defaultEndpointName)
	suite.NoError(err)

	orgName := "dummy-owner"
	repoName := "dummy-repo"
	createParams := params.CreateRepoParams{
		Owner:           orgName,
		Name:            repoName,
		CredentialsName: creds.Name,
		WebhookSecret:   "superSecret@123BlaBla",
	}

	t.Logf("Create repository with owner_name: %s, repo_name: %s", orgName, repoName)
	repo, err := createRepo(suite.cli, suite.authToken, createParams)
	suite.NoError(err)
	t.Cleanup(func() {
		deleteRepo(suite.cli, suite.authToken, repo.ID)
		deleteGithubCredentials(suite.cli, suite.authToken, int64(creds.ID)) //nolint:gosec
	})

	err = deleteGithubCredentials(suite.cli, suite.authToken, int64(creds.ID)) //nolint:gosec
	suite.Error(err, "expected error when deleting credentials in use")
}

func (suite *GarmSuite) TestGithubCredentialsFailsOnInvalidAuthType() {
	t := suite.T()
	t.Log("Testing error on invalid auth type")
	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        dummyCredentialsName,
		Endpoint:    defaultEndpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthType("invalid"),
		PAT: params.GithubPAT{
			OAuth2Token: "dummy",
		},
	}
	_, err := createGithubCredentials(suite.cli, suite.authToken, createCredsParams)
	suite.Error(err, "expected error when creating credentials with invalid auth type")
	expectAPIStatusCode(err, 400)
}

func (suite *GarmSuite) TestGithubCredentialsFailsWhenAuthTypeParamsAreIncorrect() {
	t := suite.T()
	t.Log("Testing error when auth type params are incorrect")
	privateKeyBytes, err := getTestFileContents("certs/srv-key.pem")
	suite.NoError(err)
	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        dummyCredentialsName,
		Endpoint:    defaultEndpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypePAT,
		App: params.GithubApp{
			AppID:           123,
			InstallationID:  456,
			PrivateKeyBytes: privateKeyBytes,
		},
	}
	_, err = createGithubCredentials(suite.cli, suite.authToken, createCredsParams)
	suite.Error(err, "expected error when creating credentials with invalid auth type params")

	expectAPIStatusCode(err, 400)
}

func (suite *GarmSuite) TestGithubCredentialsFailsWhenAuthTypeParamsAreMissing() {
	t := suite.T()
	t.Log("Testing error when auth type params are missing")
	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        dummyCredentialsName,
		Endpoint:    defaultEndpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypeApp,
	}
	_, err := createGithubCredentials(suite.cli, suite.authToken, createCredsParams)
	suite.Error(err, "expected error when creating credentials with missing auth type params")
	expectAPIStatusCode(err, 400)
}

func (suite *GarmSuite) TestGithubCredentialsUpdateFailsWhenBothPATAndAppAreSupplied() {
	t := suite.T()
	t.Log("Testing error when both PAT and App are supplied")
	creds, err := suite.createDummyCredentials(dummyCredentialsName, defaultEndpointName)
	suite.NoError(err)
	t.Cleanup(func() {
		suite.DeleteGithubCredential(int64(creds.ID)) //nolint:gosec
	})

	privateKeyBytes, err := getTestFileContents("certs/srv-key.pem")
	suite.NoError(err)
	updateCredsParams := params.UpdateGithubCredentialsParams{
		PAT: &params.GithubPAT{
			OAuth2Token: "dummy",
		},
		App: &params.GithubApp{
			AppID:           123,
			InstallationID:  456,
			PrivateKeyBytes: privateKeyBytes,
		},
	}
	_, err = updateGithubCredentials(suite.cli, suite.authToken, int64(creds.ID), updateCredsParams) //nolint:gosec
	suite.Error(err, "expected error when updating credentials with both PAT and App")
	expectAPIStatusCode(err, 400)
}

func (suite *GarmSuite) TestGithubCredentialsFailWhenAppKeyIsInvalid() {
	t := suite.T()
	t.Log("Testing error when app key is invalid")
	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        dummyCredentialsName,
		Endpoint:    defaultEndpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypeApp,
		App: params.GithubApp{
			AppID:           123,
			InstallationID:  456,
			PrivateKeyBytes: []byte("invalid"),
		},
	}
	_, err := createGithubCredentials(suite.cli, suite.authToken, createCredsParams)
	suite.Error(err, "expected error when creating credentials with invalid app key")
	expectAPIStatusCode(err, 400)
}

func (suite *GarmSuite) TestGithubCredentialsFailWhenEndpointDoesntExist() {
	t := suite.T()
	t.Log("Testing error when endpoint doesn't exist")
	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        dummyCredentialsName,
		Endpoint:    "iDontExist.example.com",
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "dummy",
		},
	}
	_, err := createGithubCredentials(suite.cli, suite.authToken, createCredsParams)
	suite.Error(err, "expected error when creating credentials with invalid endpoint")
	expectAPIStatusCode(err, 404)
}

func (suite *GarmSuite) TestGithubCredentialsFailsOnDuplicateName() {
	t := suite.T()
	t.Log("Testing error on duplicate credentials name")
	creds, err := suite.createDummyCredentials(dummyCredentialsName, defaultEndpointName)
	suite.NoError(err)
	t.Cleanup(func() {
		suite.DeleteGithubCredential(int64(creds.ID)) //nolint:gosec
	})

	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        dummyCredentialsName,
		Endpoint:    defaultEndpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "dummy",
		},
	}
	_, err = createGithubCredentials(suite.cli, suite.authToken, createCredsParams)
	suite.Error(err, "expected error when creating credentials with duplicate name")
	expectAPIStatusCode(err, 409)
}

func (suite *GarmSuite) createDummyCredentials(name, endpointName string) (*params.GithubCredentials, error) {
	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        name,
		Endpoint:    endpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "dummy",
		},
	}
	return suite.CreateGithubCredentials(createCredsParams)
}

func (suite *GarmSuite) CreateGithubCredentials(credentialsParams params.CreateGithubCredentialsParams) (*params.GithubCredentials, error) {
	t := suite.T()
	t.Log("Create GitHub credentials")
	credentials, err := createGithubCredentials(suite.cli, suite.authToken, credentialsParams)
	if err != nil {
		return nil, err
	}

	return credentials, nil
}

func (suite *GarmSuite) DeleteGithubCredential(id int64) error {
	t := suite.T()
	t.Log("Delete GitHub credential")
	if err := deleteGithubCredentials(suite.cli, suite.authToken, id); err != nil {
		return err
	}
	return nil
}
