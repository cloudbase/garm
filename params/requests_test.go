// Copyright 2026 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package params

import (
	"testing"
)

func validCreateGithubEndpointParams() CreateGithubEndpointParams {
	return CreateGithubEndpointParams{
		Name:          "github.com",
		BaseURL:       "https://github.com",
		APIBaseURL:    "https://api.github.com",
		UploadBaseURL: "https://uploads.github.com",
	}
}

func TestCreateGithubEndpointParamsToolsMetadataURL(t *testing.T) {
	cases := []struct {
		name        string
		metadataURL string
		wantErr     bool
	}{
		{"unset", "", false},
		{"https releases URL", "https://api.github.com/repos/canonical/github-actions-runner/releases", false},
		{"plain http", "http://internal.example.com/releases", false},
		{"no scheme", "api.github.com/repos/canonical/github-actions-runner/releases", true},
		{"no host", "https://", true},
		{"unsupported scheme", "ftp://example.com/releases", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			param := validCreateGithubEndpointParams()
			param.ToolsMetadataURL = tc.metadataURL

			err := param.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("expected an error for %q", tc.metadataURL)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.metadataURL, err)
			}
		})
	}
}

func TestUpdateGithubEndpointParamsToolsMetadataURL(t *testing.T) {
	// A nil pointer leaves the URL alone, an empty string clears it.
	for _, metadataURL := range []*string{nil, ptrTo(""), ptrTo("https://example.com/releases")} {
		param := UpdateGithubEndpointParams{ToolsMetadataURL: metadataURL}
		if err := param.Validate(); err != nil {
			t.Errorf("unexpected error for %v: %v", metadataURL, err)
		}
	}

	param := UpdateGithubEndpointParams{ToolsMetadataURL: ptrTo("not-a-url")}
	if err := param.Validate(); err == nil {
		t.Error("expected an error for an invalid tools_metadata_url")
	}
}

func ptrTo[T any](val T) *T {
	return &val
}

// A relative URL must be rejected. These validators parse several URLs in
// sequence, and a shadowed package name once turned the later url.Parse calls
// into (*url.URL).Parse, which resolves a relative reference against the
// previously parsed URL instead of failing.
func TestCreateEndpointParamsRejectRelativeURLs(t *testing.T) {
	t.Run("github upload_base_url", func(t *testing.T) {
		param := validCreateGithubEndpointParams()
		param.UploadBaseURL = "relative-not-a-url"
		if err := param.Validate(); err == nil {
			t.Fatal("expected a relative upload_base_url to be rejected")
		}
	})

	t.Run("github base_url", func(t *testing.T) {
		param := validCreateGithubEndpointParams()
		param.BaseURL = "relative-not-a-url"
		if err := param.Validate(); err == nil {
			t.Fatal("expected a relative base_url to be rejected")
		}
	})

	t.Run("gitea base_url", func(t *testing.T) {
		param := CreateGiteaEndpointParams{
			Name:       "gitea",
			APIBaseURL: "https://gitea.example.com/api/v1",
			BaseURL:    "relative-not-a-url",
		}
		if err := param.Validate(); err == nil {
			t.Fatal("expected a relative base_url to be rejected")
		}
	})
}

// The valid forms must keep passing.
func TestCreateEndpointParamsAcceptAbsoluteURLs(t *testing.T) {
	if err := validCreateGithubEndpointParams().Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	gitea := CreateGiteaEndpointParams{
		Name:       "gitea",
		APIBaseURL: "https://gitea.example.com/api/v1",
		BaseURL:    "https://gitea.example.com",
	}
	if err := gitea.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
