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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// githubDotComCredentials are the credentials of the default github.com
// endpoint, with a usable PAT payload.
func githubDotComCredentials() ForgeCredentials {
	return ForgeCredentials{
		APIBaseURL:         "https://api.github.com/",
		UploadBaseURL:      "https://uploads.github.com/",
		BaseURL:            "https://github.com/",
		CredentialsPayload: []byte(`{"oauth2_token": "super-secret"}`),
	}
}

func TestServesURL(t *testing.T) {
	ghes := ForgeCredentials{
		APIBaseURL: "https://ghes.example.com/api/v3",
		BaseURL:    "https://ghes.example.com",
	}

	for _, tc := range []struct {
		name  string
		creds ForgeCredentials
		url   string
		want  bool
	}{
		{"github api", githubDotComCredentials(), "https://api.github.com/repos/canonical/github-actions-runner/releases", true},
		{"github web", githubDotComCredentials(), "https://github.com/cloudbase/garm/releases", true},
		{"github uploads", githubDotComCredentials(), "https://uploads.github.com/anything", true},
		{"third party host", githubDotComCredentials(), "https://releases.example.com/runner/releases", false},
		{"look alike subdomain", githubDotComCredentials(), "https://api.github.com.evil.example.com/releases", false},
		{"plaintext look alike", githubDotComCredentials(), "http://api.github.com/releases", false},
		{"default port", githubDotComCredentials(), "https://api.github.com:443/releases", true},
		{"ghes api path", ghes, "https://ghes.example.com/api/v3/repos/acme/runner/releases", true},
		{"ghes mixed case host", ghes, "https://GHES.Example.COM/api/v3/repos/acme/runner/releases", true},
		{"ghes other host", ghes, "https://other.example.com/api/v3/repos/acme/runner/releases", false},
		{"empty url", githubDotComCredentials(), "", false},
		{"no scheme", githubDotComCredentials(), "api.github.com/releases", false},
		{"no forge urls", ForgeCredentials{}, "https://api.github.com/releases", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.creds.ServesURL(tc.url); got != tc.want {
				t.Errorf("ServesURL(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

// authHeaderFor reports the Authorization header the client for rawURL sends.
func authHeaderFor(t *testing.T, creds ForgeCredentials, serverURL, rawURL string) string {
	t.Helper()

	cli, err := creds.HTTPClientForURL(context.Background(), rawURL)
	if err != nil {
		t.Fatalf("HTTPClientForURL: %v", err)
	}
	resp, err := cli.Get(serverURL) // #nosec G107 -- the URL is a test server
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	return resp.Header.Get("X-Seen-Authorization")
}

// A metadata URL on the forge itself gets the token, so the fetch counts
// against the quota of the credentials instead of the anonymous rate limit.
// A URL anywhere else must never see it.
func TestHTTPClientForURLAuthentication(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Seen-Authorization", r.Header.Get("Authorization"))
	}))
	defer srv.Close()

	creds := githubDotComCredentials()
	if got := authHeaderFor(t, creds, srv.URL, "https://api.github.com/repos/acme/runner/releases"); got == "" {
		t.Error("expected the forge URL to be fetched with credentials")
	}
	if got := authHeaderFor(t, creds, srv.URL, "https://releases.example.com/runner/releases"); got != "" {
		t.Errorf("expected no credentials sent to a third party URL, got %q", got)
	}
}
