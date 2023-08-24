package e2e

import (
	"log"
	"net/url"

	"github.com/cloudbase/garm/client"
	"github.com/cloudbase/garm/params"
	"github.com/go-openapi/runtime"
	openapiRuntimeClient "github.com/go-openapi/runtime/client"
)

var (
	cli       *client.GarmAPI
	authToken runtime.ClientAuthInfoWriter
)

func InitClient(baseURL string) {
	garmUrl, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}
	apiPath, err := url.JoinPath(garmUrl.Path, client.DefaultBasePath)
	if err != nil {
		panic(err)
	}
	transportCfg := client.DefaultTransportConfig().
		WithHost(garmUrl.Host).
		WithBasePath(apiPath).
		WithSchemes([]string{garmUrl.Scheme})
	cli = client.NewHTTPClientWithConfig(nil, transportCfg)
}

func FirstRun(adminUsername, adminPassword, adminFullName, adminEmail string) *params.User {
	log.Println("First run")
	newUser := params.NewUserParams{
		Username: adminUsername,
		Password: adminPassword,
		FullName: adminFullName,
		Email:    adminEmail,
	}
	user, err := firstRun(cli, newUser)
	if err != nil {
		panic(err)
	}
	return &user
}

func Login(username, password string) {
	log.Println("Login")
	loginParams := params.PasswordLoginParams{
		Username: username,
		Password: password,
	}
	token, err := login(cli, loginParams)
	if err != nil {
		panic(err)
	}
	authToken = openapiRuntimeClient.BearerToken(token)
}
