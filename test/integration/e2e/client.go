package e2e

import (
	"log/slog"
	"net/url"

	"github.com/go-openapi/runtime"
	openapiRuntimeClient "github.com/go-openapi/runtime/client"

	"github.com/cloudbase/garm/client"
	"github.com/cloudbase/garm/params"
)

var (
	cli       *client.GarmAPI
	authToken runtime.ClientAuthInfoWriter
)

func InitClient(baseURL string) {
	garmURL, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}
	apiPath, err := url.JoinPath(garmURL.Path, client.DefaultBasePath)
	if err != nil {
		panic(err)
	}
	transportCfg := client.DefaultTransportConfig().
		WithHost(garmURL.Host).
		WithBasePath(apiPath).
		WithSchemes([]string{garmURL.Scheme})
	cli = client.NewHTTPClientWithConfig(nil, transportCfg)
}

func FirstRun(adminUsername, adminPassword, adminFullName, adminEmail string) *params.User {
	slog.Info("First run")
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
	slog.Info("Login")
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
