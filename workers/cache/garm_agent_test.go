// Copyright 2025 Cloudbase Solutions SRL
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

package cache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const validReleaseList = `[
	{
		"tag_name": "v0.2.0",
		"assets": [
			{
				"name": "garm-agent-linux-amd64",
				"size": 8000000,
				"browser_download_url": "https://example.com/v0.2.0/garm-agent-linux-amd64"
			}
		]
	},
	{
		"tag_name": "v0.1.0",
		"prerelease": true,
		"assets": [
			{
				"name": "garm-agent-linux-amd64",
				"size": 7000000,
				"browser_download_url": "https://example.com/v0.1.0/garm-agent-linux-amd64"
			}
		]
	}
]`

func newReleasesServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestFetchReleaseIndex(t *testing.T) {
	server := newReleasesServer(t, http.StatusOK, validReleaseList)

	releases, err := fetchReleaseIndex(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(releases))
	}
	if releases[0].TagName != "v0.2.0" || !releases[1].Prerelease {
		t.Fatalf("unexpected releases: %+v", releases)
	}
}

func TestFetchReleaseIndexRejectsNonListResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"single release object", `{"tag_name": "v0.1.0", "assets": []}`},
		{"empty array", `[]`},
		{"github error object", `{"message": "Not Found"}`},
		{"garbage", `{"invalid": json}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newReleasesServer(t, http.StatusOK, tc.body)
			if _, err := fetchReleaseIndex(context.Background(), server.URL); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestFetchReleaseIndexErrorStatus(t *testing.T) {
	server := newReleasesServer(t, http.StatusNotFound, `{"message": "Not Found"}`)
	if _, err := fetchReleaseIndex(context.Background(), server.URL); err == nil {
		t.Error("expected an error for a non-200 status")
	}
}

func TestFetchReleaseIndexNetworkError(t *testing.T) {
	_, err := fetchReleaseIndex(context.Background(), "http://invalid-url-that-does-not-exist-12345.local")
	if err == nil {
		t.Error("expected a network error")
	}
}

func TestFetchReleaseByTag(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		if !strings.HasSuffix(r.URL.Path, "/tags/v0.1.0") {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message": "Not Found"}`))
			return
		}
		w.Write([]byte(`{"tag_name": "v0.1.0", "assets": [{"name": "garm-agent-linux-amd64", "size": 1, "browser_download_url": "https://example.com/v0.1.0"}]}`))
	}))
	t.Cleanup(server.Close)

	release, err := fetchReleaseByTag(context.Background(), server.URL+"/repos/org/repo/releases", "v0.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release.TagName != "v0.1.0" {
		t.Errorf("expected v0.1.0, got %q", release.TagName)
	}
	if requestedPath != "/repos/org/repo/releases/tags/v0.1.0" {
		t.Errorf("unexpected request path: %s", requestedPath)
	}

	// A tag the endpoint does not know about is an error; the caller decides
	// on the fallback policy.
	if _, err := fetchReleaseByTag(context.Background(), server.URL+"/repos/org/repo/releases", "v9.9.9"); err == nil {
		t.Error("expected an error for an unknown tag")
	}
}
