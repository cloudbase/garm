package e2e

import (
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
}
