package e2e

import (
	"fmt"

	"github.com/cloudbase/garm/params"
)

func EnsureTestCredentials(name string, oauthToken string, endpointName string) {
	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        name,
		Endpoint:    endpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: oauthToken,
		},
	}
	CreateGithubCredentials(createCredsParams)

	createCredsParams.Name = fmt.Sprintf("%s-clone", name)
	CreateGithubCredentials(createCredsParams)
}

func createDummyCredentials(name, endpointName string) *params.GithubCredentials {
	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        name,
		Endpoint:    endpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "dummy",
		},
	}
	return CreateGithubCredentials(createCredsParams)
}

func TestGithubCredentialsErrorOnDuplicateCredentialsName() {
	name := "dummy"
	endpointName := "github.com"
	creds := createDummyCredentials(name, endpointName)
	defer DeleteGithubCredential(int64(creds.ID))

	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        name,
		Endpoint:    endpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "dummy",
		},
	}
	if _, err := createGithubCredentials(cli, authToken, createCredsParams); err == nil {
		panic("expected error when creating credentials with duplicate name")
	}
}

func TestGithubCredentialsFailsToDeleteWhenInUse() {
	name := "dummy"
	endpointName := "github.com"
	creds := createDummyCredentials(name, endpointName)

	repo := CreateRepo("dummy-owner", "dummy-repo", creds.Name, "superSecret@123BlaBla")
	defer func() {
		deleteRepo(cli, authToken, repo.ID)
		deleteGithubCredentials(cli, authToken, int64(creds.ID))
	}()

	if err := deleteGithubCredentials(cli, authToken, int64(creds.ID)); err == nil {
		panic("expected error when deleting credentials in use")
	}
}

func TestGithubCredentialsFailsOnInvalidAuthType() {
	name := "dummy"
	endpointName := "github.com"

	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        name,
		Endpoint:    endpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthType("invalid"),
		PAT: params.GithubPAT{
			OAuth2Token: "dummy",
		},
	}
	_, err := createGithubCredentials(cli, authToken, createCredsParams)
	if err == nil {
		panic("expected error when creating credentials with invalid auth type")
	}
	expectAPIStatusCode(err, 400)
}

func TestGithubCredentialsFailsWhenAuthTypeParamsAreIncorrect() {
	name := "dummy"
	endpointName := "github.com"

	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        name,
		Endpoint:    endpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypePAT,
		App: params.GithubApp{
			AppID:           123,
			InstallationID:  456,
			PrivateKeyBytes: getTestFileContents("certs/srv-key.pem"),
		},
	}
	_, err := createGithubCredentials(cli, authToken, createCredsParams)
	if err == nil {
		panic("expected error when creating credentials with invalid auth type params")
	}
	expectAPIStatusCode(err, 400)
}

func TestGithubCredentialsFailsWhenAuthTypeParamsAreMissing() {
	name := "dummy"
	endpointName := "github.com"

	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        name,
		Endpoint:    endpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypeApp,
	}
	_, err := createGithubCredentials(cli, authToken, createCredsParams)
	if err == nil {
		panic("expected error when creating credentials with missing auth type params")
	}
	expectAPIStatusCode(err, 400)
}

func TestGithubCredentialsUpdateFailsWhenBothPATAndAppAreSupplied() {
	name := "dummy"
	endpointName := "github.com"
	creds := createDummyCredentials(name, endpointName)
	defer DeleteGithubCredential(int64(creds.ID))

	updateCredsParams := params.UpdateGithubCredentialsParams{
		PAT: &params.GithubPAT{
			OAuth2Token: "dummy",
		},
		App: &params.GithubApp{
			AppID:           123,
			InstallationID:  456,
			PrivateKeyBytes: getTestFileContents("certs/srv-key.pem"),
		},
	}
	_, err := updateGithubCredentials(cli, authToken, int64(creds.ID), updateCredsParams)
	if err == nil {
		panic("expected error when updating credentials with both PAT and App")
	}
	expectAPIStatusCode(err, 400)
}

func TestGithubCredentialsFailWhenAppKeyIsInvalid() {
	name := "dummy"
	endpointName := "github.com"

	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        name,
		Endpoint:    endpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypeApp,
		App: params.GithubApp{
			AppID:           123,
			InstallationID:  456,
			PrivateKeyBytes: []byte("invalid"),
		},
	}
	_, err := createGithubCredentials(cli, authToken, createCredsParams)
	if err == nil {
		panic("expected error when creating credentials with invalid app key")
	}
	expectAPIStatusCode(err, 400)
}

func TestGithubCredentialsFailWhenEndpointDoesntExist() {
	name := "dummy"
	endpointName := "nonexistent"

	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        name,
		Endpoint:    endpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "dummy",
		},
	}
	_, err := createGithubCredentials(cli, authToken, createCredsParams)
	if err == nil {
		panic("expected error when creating credentials with invalid endpoint")
	}
	expectAPIStatusCode(err, 404)
}

func TestGithubCredentialsFailsOnDuplicateName() {
	name := "dummy"
	endpointName := "github.com"
	creds := createDummyCredentials(name, endpointName)
	defer DeleteGithubCredential(int64(creds.ID))

	createCredsParams := params.CreateGithubCredentialsParams{
		Name:        name,
		Endpoint:    endpointName,
		Description: "GARM test credentials",
		AuthType:    params.GithubAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "dummy",
		},
	}
	_, err := createGithubCredentials(cli, authToken, createCredsParams)
	if err == nil {
		panic("expected error when creating credentials with duplicate name")
	}
	expectAPIStatusCode(err, 409)
}
