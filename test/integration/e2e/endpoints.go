package e2e

import (
	"os"
	"path/filepath"

	"github.com/cloudbase/garm/params"
)

func MustDefaultGithubEndpoint() {
	ep := GetGithubEndpoint("github.com")
	if ep == nil {
		panic("Default GitHub endpoint not found")
	}

	if ep.Name != "github.com" {
		panic("Default GitHub endpoint name mismatch")
	}
}

func checkEndpointParamsAreEqual(a, b params.GithubEndpoint) {
	if a.Name != b.Name {
		panic("Endpoint name mismatch")
	}

	if a.Description != b.Description {
		panic("Endpoint description mismatch")
	}

	if a.BaseURL != b.BaseURL {
		panic("Endpoint base URL mismatch")
	}

	if a.APIBaseURL != b.APIBaseURL {
		panic("Endpoint API base URL mismatch")
	}

	if a.UploadBaseURL != b.UploadBaseURL {
		panic("Endpoint upload base URL mismatch")
	}

	if string(a.CACertBundle) != string(b.CACertBundle) {
		panic("Endpoint CA cert bundle mismatch")
	}
}

func TestGithubEndpointOperations() {
	baseDir := os.Getenv("GARM_CHECKOUT_DIR")
	if baseDir == "" {
		panic("GARM_CHECKOUT_DIR not set")
	}
	caBundle, err := os.ReadFile(filepath.Join(baseDir, "testdata/certs/srv-pub.pem"))
	if err != nil {
		panic(err)
	}
	endpointParams := params.CreateGithubEndpointParams{
		Name:          "test-endpoint",
		Description:   "Test endpoint",
		BaseURL:       "https://ghes.example.com",
		APIBaseURL:    "https://api.ghes.example.com/",
		UploadBaseURL: "https://uploads.ghes.example.com/",
		CACertBundle:  caBundle,
	}

	endpoint := CreateGithubEndpoint(endpointParams)
	if endpoint.Name != endpointParams.Name {
		panic("Endpoint name mismatch")
	}

	if endpoint.Description != endpointParams.Description {
		panic("Endpoint description mismatch")
	}

	if endpoint.BaseURL != endpointParams.BaseURL {
		panic("Endpoint base URL mismatch")
	}

	if endpoint.APIBaseURL != endpointParams.APIBaseURL {
		panic("Endpoint API base URL mismatch")
	}

	if endpoint.UploadBaseURL != endpointParams.UploadBaseURL {
		panic("Endpoint upload base URL mismatch")
	}

	if string(endpoint.CACertBundle) != string(caBundle) {
		panic("Endpoint CA cert bundle mismatch")
	}

	endpoint2 := GetGithubEndpoint(endpointParams.Name)
	if endpoint == nil || endpoint2 == nil {
		panic("endpoint is nil")
	}
	checkEndpointParamsAreEqual(*endpoint, *endpoint2)

	endpoints := ListGithubEndpoints()
	var found bool
	for _, ep := range endpoints {
		if ep.Name == endpointParams.Name {
			checkEndpointParamsAreEqual(*endpoint, ep)
			found = true
			break
		}
	}
	if !found {
		panic("Endpoint not found in list")
	}

	DeleteGithubEndpoint(endpoint.Name)

	if err := deleteGithubEndpoint(cli, authToken, "github.com"); err == nil {
		panic("expected error when attempting to delete the default github.com endpoint")
	}
}
