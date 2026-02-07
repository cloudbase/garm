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
	"testing"
)

func TestGetLatestGithubReleaseFromURL(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		wantErr      bool
		errContains  string
		wantTagName  string
		wantAssets   int
	}{
		{
			name: "valid /releases array with single release",
			responseBody: `[
				{
					"tag_name": "v0.1.0-beta1",
					"assets": [
						{
							"name": "garm-agent-linux-amd64-v0.1.0-beta1",
							"size": 7749816,
							"browser_download_url": "https://github.com/cloudbase/garm-agent/releases/download/v0.1.0-beta1/garm-agent-linux-amd64-v0.1.0-beta1"
						},
						{
							"name": "garm-agent-linux-arm64-v0.1.0-beta1",
							"size": 7274680,
							"browser_download_url": "https://github.com/cloudbase/garm-agent/releases/download/v0.1.0-beta1/garm-agent-linux-arm64-v0.1.0-beta1"
						}
					]
				}
			]`,
			wantErr:     false,
			wantTagName: "v0.1.0-beta1",
			wantAssets:  2,
		},
		{
			name: "valid /releases array with multiple releases",
			responseBody: `[
				{
					"tag_name": "v0.2.0",
					"assets": [
						{
							"name": "garm-agent-linux-amd64-v0.2.0",
							"size": 8000000,
							"browser_download_url": "https://example.com/v0.2.0/garm-agent-linux-amd64"
						}
					]
				},
				{
					"tag_name": "v0.1.0",
					"assets": [
						{
							"name": "garm-agent-linux-amd64-v0.1.0",
							"size": 7000000,
							"browser_download_url": "https://example.com/v0.1.0/garm-agent-linux-amd64"
						}
					]
				}
			]`,
			wantErr:     false,
			wantTagName: "v0.2.0", // Should return first (latest) release
			wantAssets:  1,
		},
		{
			name: "valid /releases/latest single object",
			responseBody: `{
				"tag_name": "v0.1.0-beta1",
				"name": "v0.1.0-beta1",
				"draft": false,
				"prerelease": false,
				"assets": [
					{
						"name": "garm-agent-linux-amd64-v0.1.0-beta1",
						"size": 7749816,
						"browser_download_url": "https://github.com/cloudbase/garm-agent/releases/download/v0.1.0-beta1/garm-agent-linux-amd64-v0.1.0-beta1"
					},
					{
						"name": "garm-agent-windows-amd64-v0.1.0-beta1.exe",
						"size": 7843328,
						"browser_download_url": "https://github.com/cloudbase/garm-agent/releases/download/v0.1.0-beta1/garm-agent-windows-amd64-v0.1.0-beta1.exe"
					}
				]
			}`,
			wantErr:     false,
			wantTagName: "v0.1.0-beta1",
			wantAssets:  2,
		},
		{
			name:         "empty array",
			responseBody: `[]`,
			wantErr:      true,
			errContains:  "failed to unmarshal", // Empty array tries to parse as single object and fails
		},
		{
			name:         "empty object",
			responseBody: `{}`,
			wantErr:      true,
			errContains:  "missing tag_name",
		},
		{
			name:         "object without tag_name",
			responseBody: `{"name": "some-release", "draft": false}`,
			wantErr:      true,
			errContains:  "missing tag_name",
		},
		{
			name: "release without assets",
			responseBody: `{
				"tag_name": "v1.0.0",
				"assets": []
			}`,
			wantErr:     true,
			errContains: "no downloadable assets",
		},
		{
			name: "array with release without assets",
			responseBody: `[
				{
					"tag_name": "v1.0.0",
					"assets": []
				}
			]`,
			wantErr:     true,
			errContains: "no downloadable assets",
		},
		{
			name:         "invalid JSON",
			responseBody: `{"invalid": json}`,
			wantErr:      true,
			errContains:  "failed to unmarshal",
		},
		{
			name: "unrelated valid JSON object",
			responseBody: `{
				"message": "Not Found",
				"documentation_url": "https://docs.github.com/rest/releases/releases#get-the-latest-release"
			}`,
			wantErr:     true,
			errContains: "missing tag_name",
		},
		{
			name: "unrelated valid JSON array",
			responseBody: `[
				{"id": 1, "name": "item1"},
				{"id": 2, "name": "item2"}
			]`,
			wantErr:     true,
			errContains: "no downloadable assets", // Array parses successfully but has no assets
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			// Call the function
			release, err := getLatestGithubReleaseFromURL(context.Background(), server.URL)

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			// Check success expectations
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if release.TagName != tt.wantTagName {
				t.Errorf("expected tag_name %q, got %q", tt.wantTagName, release.TagName)
			}

			if len(release.Assets) != tt.wantAssets {
				t.Errorf("expected %d assets, got %d", tt.wantAssets, len(release.Assets))
			}
		})
	}
}

func TestGetLatestGithubReleaseFromURL_NetworkError(t *testing.T) {
	// Test with invalid URL to trigger network error
	_, err := getLatestGithubReleaseFromURL(context.Background(), "http://invalid-url-that-does-not-exist-12345.local")
	if err == nil {
		t.Error("expected network error, got nil")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
