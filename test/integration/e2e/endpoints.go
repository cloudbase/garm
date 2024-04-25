package e2e

import (
	"log/slog"
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

func getTestFileContents(relPath string) []byte {
	baseDir := os.Getenv("GARM_CHECKOUT_DIR")
	if baseDir == "" {
		panic("GARM_CHECKOUT_DIR not set")
	}
	contents, err := os.ReadFile(filepath.Join(baseDir, "testdata", relPath))
	if err != nil {
		panic(err)
	}
	return contents
}

func TestGithubEndpointOperations() {
	MustDefaultGithubEndpoint()

	caBundle := getTestFileContents("certs/srv-pub.pem")

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
}

func TestGithubEndpointMustFailToDeleteDefaultGithubEndpoint() {
	if err := deleteGithubEndpoint(cli, authToken, "github.com"); err == nil {
		panic("expected error when attempting to delete the default github.com endpoint")
	}
}

func TestGithubEndpointFailsOnInvalidCABundle() {
	slog.Info("Testing endpoint creation with invalid CA cert bundle")
	badCABundle := getTestFileContents("certs/srv-key.pem")

	endpointParams := params.CreateGithubEndpointParams{
		Name:          "dummy",
		Description:   "Dummy endpoint",
		BaseURL:       "https://ghes.example.com",
		APIBaseURL:    "https://api.ghes.example.com/",
		UploadBaseURL: "https://uploads.ghes.example.com/",
		CACertBundle:  badCABundle,
	}

	if _, err := createGithubEndpoint(cli, authToken, endpointParams); err == nil {
		panic("expected error when creating endpoint with invalid CA cert bundle")
	}
}

func TestGithubEndpointDeletionFailsWhenCredentialsExist() {
	slog.Info("Testing endpoint deletion when credentials exist")
	endpointParams := params.CreateGithubEndpointParams{
		Name:          "dummy",
		Description:   "Dummy endpoint",
		BaseURL:       "https://ghes.example.com",
		APIBaseURL:    "https://api.ghes.example.com/",
		UploadBaseURL: "https://uploads.ghes.example.com/",
	}

	endpoint := CreateGithubEndpoint(endpointParams)
	creds := createDummyCredentials("test-creds", endpoint.Name)

	if err := deleteGithubEndpoint(cli, authToken, endpoint.Name); err == nil {
		panic("expected error when deleting endpoint with credentials")
	}

	DeleteGithubCredential(int64(creds.ID))
	DeleteGithubEndpoint(endpoint.Name)
}

func TestGithubEndpointFailsOnDuplicateName() {
	slog.Info("Testing endpoint creation with duplicate name")
	endpointParams := params.CreateGithubEndpointParams{
		Name:          "github.com",
		Description:   "Dummy endpoint",
		BaseURL:       "https://ghes.example.com",
		APIBaseURL:    "https://api.ghes.example.com/",
		UploadBaseURL: "https://uploads.ghes.example.com/",
	}

	if _, err := createGithubEndpoint(cli, authToken, endpointParams); err == nil {
		panic("expected error when creating endpoint with duplicate name")
	}
}

func TestGithubEndpointUpdateEndpoint() {
	slog.Info("Testing endpoint update")
	endpoint := createDummyEndpoint("dummy")
	defer DeleteGithubEndpoint(endpoint.Name)

	newDescription := "Updated description"
	newBaseURL := "https://ghes2.example.com"
	newAPIBaseURL := "https://api.ghes2.example.com/"
	newUploadBaseURL := "https://uploads.ghes2.example.com/"
	newCABundle := getTestFileContents("certs/srv-pub.pem")

	updateParams := params.UpdateGithubEndpointParams{
		Description:   &newDescription,
		BaseURL:       &newBaseURL,
		APIBaseURL:    &newAPIBaseURL,
		UploadBaseURL: &newUploadBaseURL,
		CACertBundle:  newCABundle,
	}

	updated, err := updateGithubEndpoint(cli, authToken, endpoint.Name, updateParams)
	if err != nil {
		panic(err)
	}

	if updated.Name != endpoint.Name {
		panic("Endpoint name mismatch")
	}

	if updated.Description != newDescription {
		panic("Endpoint description mismatch")
	}

	if updated.BaseURL != newBaseURL {
		panic("Endpoint base URL mismatch")
	}

	if updated.APIBaseURL != newAPIBaseURL {
		panic("Endpoint API base URL mismatch")
	}

	if updated.UploadBaseURL != newUploadBaseURL {
		panic("Endpoint upload base URL mismatch")
	}

	if string(updated.CACertBundle) != string(newCABundle) {
		panic("Endpoint CA cert bundle mismatch")
	}
}

func createDummyEndpoint(name string) *params.GithubEndpoint {
	endpointParams := params.CreateGithubEndpointParams{
		Name:          name,
		Description:   "Dummy endpoint",
		BaseURL:       "https://ghes.example.com",
		APIBaseURL:    "https://api.ghes.example.com/",
		UploadBaseURL: "https://uploads.ghes.example.com/",
	}

	return CreateGithubEndpoint(endpointParams)
}
