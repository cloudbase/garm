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

package util

import (
	"testing"

	"github.com/cloudbase/garm/params"
)

const (
	testVersion010 = "v0.1.0"
	testVersion020 = "v0.2.0"
)

func TestParseReleaseList(t *testing.T) {
	valid := `[
		{"tag_name": "v0.2.0", "assets": [{"name": "garm-agent-linux-amd64", "size": 1, "browser_download_url": "https://example.com/v0.2.0"}]},
		{"tag_name": "v0.1.0", "prerelease": true, "assets": []}
	]`
	releases, err := ParseReleaseList([]byte(valid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(releases))
	}
	if releases[0].TagName != testVersion020 || releases[1].Prerelease != true {
		t.Fatalf("unexpected parse result: %+v", releases)
	}

	invalid := []struct {
		name string
		data string
	}{
		{"single release object", `{"tag_name": "v0.1.0", "assets": []}`},
		{"empty array", `[]`},
		{"array of unrelated objects", `[{"id": 1, "name": "item1"}]`},
		{"garbage", `{"invalid": json}`},
		{"github error object", `{"message": "Not Found"}`},
	}
	for _, tc := range invalid {
		if _, err := ParseReleaseList([]byte(tc.data)); err == nil {
			t.Errorf("%s: expected an error", tc.name)
		}
	}
}

func TestParseRelease(t *testing.T) {
	release, err := ParseRelease([]byte(`{"tag_name": "v0.1.0", "prerelease": true, "assets": []}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release.TagName != testVersion010 || !release.Prerelease {
		t.Fatalf("unexpected parse result: %+v", release)
	}

	if _, err := ParseRelease([]byte(`{"message": "Not Found"}`)); err == nil {
		t.Error("expected an error for an object without tag_name")
	}
	if _, err := ParseRelease([]byte(`[{"tag_name": "v0.1.0"}]`)); err == nil {
		t.Error("expected an error for an array")
	}
}

func TestParseReleaseIndexRoundTrip(t *testing.T) {
	index := AgentReleaseIndex{
		SourceURL: "https://api.github.com/repos/cloudbase/garm-agent/releases",
		Releases: GitHubReleases{
			{TagName: testVersion020},
			{TagName: testVersion010, Prerelease: true},
		},
	}
	data, err := MarshalReleaseIndex(index)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parsed, err := ParseReleaseIndex(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.SourceURL != index.SourceURL || len(parsed.Releases) != 2 {
		t.Fatalf("round trip mismatch: %+v", parsed)
	}

	if _, err := ParseReleaseIndex([]byte("{bad")); err == nil {
		t.Error("expected an error for invalid data")
	}
}

func TestAgentVersionsMatch(t *testing.T) {
	matching := [][2]string{
		{"v1.2.3", "v1.2.3"},
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "v1.2.3"},
	}
	for _, pair := range matching {
		if !AgentVersionsMatch(pair[0], pair[1]) {
			t.Errorf("expected %q and %q to match", pair[0], pair[1])
		}
	}
	if AgentVersionsMatch("v1.2.3", "v1.2.4") {
		t.Error("expected different versions not to match")
	}
}

func TestFindAgentRelease(t *testing.T) {
	releases := GitHubReleases{
		{TagName: testVersion020},
		{TagName: testVersion010},
	}

	release, found := FindAgentRelease(releases, "0.1.0")
	if !found || release.TagName != testVersion010 {
		t.Errorf("expected to find v0.1.0, got %q (found=%v)", release.TagName, found)
	}

	if _, found := FindAgentRelease(releases, "v9.9.9"); found {
		t.Error("expected v9.9.9 not to be found")
	}
	if _, found := FindAgentRelease(nil, testVersion010); found {
		t.Error("expected no match in an empty release list")
	}
}

func TestResolveAgentRelease(t *testing.T) {
	releases := GitHubReleases{
		{TagName: "v0.3.0-rc1", Prerelease: true},
		{TagName: testVersion020},
		{TagName: testVersion010},
	}

	// latest prefers the newest stable release.
	for _, version := range []string{"", params.GARMAgentLatestVersion} {
		release, found := ResolveAgentRelease(releases, version)
		if !found || release.TagName != testVersion020 {
			t.Errorf("version %q: expected v0.2.0, got %q (found=%v)", version, release.TagName, found)
		}
	}

	// With only pre-releases, latest falls back to the newest one.
	preOnly := GitHubReleases{
		{TagName: "v0.2.0-beta2", Prerelease: true},
		{TagName: "v0.1.0-beta1", Prerelease: true},
	}
	release, found := ResolveAgentRelease(preOnly, "")
	if !found || release.TagName != "v0.2.0-beta2" {
		t.Errorf("expected fallback to newest pre-release, got %q (found=%v)", release.TagName, found)
	}

	// Pinning selects by tag, tolerating a missing/extra "v" prefix.
	release, found = ResolveAgentRelease(releases, "0.1.0")
	if !found || release.TagName != testVersion010 {
		t.Errorf("expected pinned v0.1.0, got %q (found=%v)", release.TagName, found)
	}

	// Pre-releases can be pinned explicitly.
	release, found = ResolveAgentRelease(releases, "v0.3.0-rc1")
	if !found || release.TagName != "v0.3.0-rc1" {
		t.Errorf("expected pinned pre-release, got %q (found=%v)", release.TagName, found)
	}

	// A version that is not in the index reports not found, but still
	// resolves to the latest release so callers always have something to
	// serve.
	release, found = ResolveAgentRelease(releases, "v9.9.9")
	if found {
		t.Error("expected v9.9.9 not to be found")
	}
	if release.TagName != testVersion020 {
		t.Errorf("expected fallback to latest stable, got %q", release.TagName)
	}
	if _, found := ResolveAgentRelease(nil, ""); found {
		t.Error("expected empty index not to resolve")
	}
}

func TestToolsFromRelease(t *testing.T) {
	release := GitHubRelease{
		TagName: testVersion010,
		Assets: []GitHubReleaseAsset{
			{Name: "garm-agent-linux-amd64", Size: 100, DownloadURL: "https://example.com/linux-amd64"},
			{Name: "garm-agent-windows-amd64.exe", Size: 200, DownloadURL: "https://example.com/windows-amd64"},
			{Name: "garm-agent-linux-amd64.sha256", Size: 1, DownloadURL: "https://example.com/checksum"},
			{Name: "unrelated-file.tar.gz", Size: 1, DownloadURL: "https://example.com/unrelated"},
		},
	}

	tools := ToolsFromRelease(release)
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %+v", len(tools), tools)
	}
	linux, ok := tools["linux/amd64"]
	if !ok || linux.Version != testVersion010 || linux.DownloadURL != "https://example.com/linux-amd64" {
		t.Errorf("unexpected linux tool: %+v", linux)
	}
	if _, ok := tools["windows/amd64"]; !ok {
		t.Error("expected a windows/amd64 tool")
	}
}
