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

package cache

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
	garmUtil "github.com/cloudbase/garm/util"
)

// canonicalReleaseList mirrors the shape of the release list published by a
// third party that builds patched runners, in this case
// canonical/github-actions-runner.
const canonicalReleaseList = `[
	{
		"tag_name": "v2.338.2",
		"prerelease": false,
		"assets": [
			{
				"name": "actions-runner-linux-arm-2.338.2.tar.gz",
				"browser_download_url": "https://example.com/actions-runner-linux-arm-2.338.2.tar.gz"
			},
			{
				"name": "actions-runner-linux-arm64-2.338.2.tar.gz",
				"browser_download_url": "https://example.com/actions-runner-linux-arm64-2.338.2.tar.gz"
			},
			{
				"name": "actions-runner-linux-ppc64le-2.338.2.tar.gz",
				"browser_download_url": "https://example.com/actions-runner-linux-ppc64le-2.338.2.tar.gz",
				"digest": "sha256:deadbeef"
			},
			{
				"name": "actions-runner-linux-s390x-2.338.2.tar.gz",
				"browser_download_url": "https://example.com/actions-runner-linux-s390x-2.338.2.tar.gz"
			},
			{
				"name": "actions-runner-linux-x64-2.338.2.tar.gz",
				"browser_download_url": "https://example.com/actions-runner-linux-x64-2.338.2.tar.gz"
			}
		]
	},
	{
		"tag_name": "v2.337.0",
		"prerelease": false,
		"assets": [
			{
				"name": "actions-runner-linux-x64-2.337.0.tar.gz",
				"browser_download_url": "https://example.com/actions-runner-linux-x64-2.337.0.tar.gz"
			}
		]
	}
]`

func toolsByKey(t *testing.T, metadataURL string) map[string]string {
	t.Helper()
	tools, err := getRunnerToolsFromMetadataURL(context.Background(), metadataURL, params.ForgeCredentials{})
	if err != nil {
		t.Fatalf("getRunnerToolsFromMetadataURL: %v", err)
	}
	ret := make(map[string]string, len(tools))
	for _, tool := range tools {
		ret[tool.Platform()] = tool.GetDownloadURL()
	}
	return ret
}

func TestGetRunnerToolsFromMetadataURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(canonicalReleaseList)); err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	defer srv.Close()

	byKey := toolsByKey(t, srv.URL)
	if len(byKey) != 5 {
		t.Fatalf("expected 5 tools, got %d: %v", len(byKey), byKey)
	}

	for _, arch := range []string{"x64", "arm64", "arm", "ppc64le", "s390x"} {
		url, ok := byKey["linux/"+arch]
		if !ok {
			t.Errorf("missing tools for linux/%s", arch)
			continue
		}
		// Only the latest release is used.
		if want := "https://example.com/actions-runner-linux-" + arch + "-2.338.2.tar.gz"; url != want {
			t.Errorf("linux/%s: expected %s, got %s", arch, want, url)
		}
	}
}

func TestGetRunnerToolsFromMetadataURLErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		code int
	}{
		{name: "not found", body: `{"message": "Not Found"}`, code: http.StatusNotFound},
		{name: "single release object", body: `{"tag_name": "v2.338.2", "assets": []}`, code: http.StatusOK},
		{name: "empty list", body: `[]`, code: http.StatusOK},
		{name: "no runner assets", body: `[{"tag_name": "v1.0.0", "assets": [{"name": "README.md", "browser_download_url": "https://example.com/README.md"}]}]`, code: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.code)
				if _, err := w.Write([]byte(tc.body)); err != nil {
					t.Errorf("writing response: %v", err)
				}
			}))
			defer srv.Close()

			if _, err := getRunnerToolsFromMetadataURL(context.Background(), srv.URL, params.ForgeCredentials{}); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestRunnerToolsFromReleaseSkipsVariantPackages(t *testing.T) {
	release := garmUtil.GitHubRelease{
		TagName: "v2.338.2",
		Assets: []garmUtil.GitHubReleaseAsset{
			{Name: "actions-runner-linux-x64-2.338.2-noexternals.tar.gz", DownloadURL: "https://example.com/noexternals"},
			{Name: "actions-runner-linux-x64-2.338.2-trimmedpackages.tar.gz", DownloadURL: "https://example.com/trimmed"},
			{Name: "actions-runner-linux-x64-2.338.2.tar.gz.sha256", DownloadURL: "https://example.com/sha256"},
			{Name: "actions-runner-win-x64-2.338.2.zip", DownloadURL: "https://example.com/win"},
			{Name: "actions-runner-linux-x64-2.338.2.tar.gz", DownloadURL: "https://example.com/linux"},
			{Name: "actions-runner-linux-m68k-2.338.2.tar.gz", DownloadURL: "https://example.com/m68k"},
		},
	}

	tools := runnerToolsFromRelease(context.Background(), release)
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	byKey := map[string]string{}
	for _, tool := range tools {
		byKey[tool.Platform()] = tool.GetDownloadURL()
	}
	if got := byKey["linux/x64"]; got != "https://example.com/linux" {
		t.Errorf("linux/x64: got %s", got)
	}
	if got := byKey["win/x64"]; got != "https://example.com/win" {
		t.Errorf("win/x64: got %s", got)
	}
}

func TestRunnerToolsFromReleaseSetsChecksum(t *testing.T) {
	release := garmUtil.GitHubRelease{
		TagName: "v2.338.2",
		Assets: []garmUtil.GitHubReleaseAsset{
			{Name: "actions-runner-linux-ppc64le-2.338.2.tar.gz", DownloadURL: "https://example.com/ppc64le", Digest: "sha256:deadbeef"},
			{Name: "actions-runner-linux-s390x-2.338.2.tar.gz", DownloadURL: "https://example.com/s390x", Digest: "md5:cafe"},
		},
	}

	tools := runnerToolsFromRelease(context.Background(), release)
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	for _, tool := range tools {
		switch tool.GetArchitecture() {
		case "ppc64le":
			if got := tool.GetSHA256Checksum(); got != "deadbeef" {
				t.Errorf("ppc64le checksum: got %q", got)
			}
		case "s390x":
			// Only sha256 digests are usable as a sha256 checksum.
			if got := tool.GetSHA256Checksum(); got != "" {
				t.Errorf("s390x checksum: expected empty, got %q", got)
			}
		}
	}
}

// Providers report architectures under several names. The aliases need to
// normalize to the same key the forge API uses, or the merge would add a
// duplicate entry instead of overriding.
func TestRunnerToolsFromReleaseNormalizesArchAliases(t *testing.T) {
	release := garmUtil.GitHubRelease{
		TagName: "v2.338.2",
		Assets: []garmUtil.GitHubReleaseAsset{
			{Name: "actions-runner-linux-amd64-2.338.2.tar.gz", DownloadURL: "https://example.com/amd64"},
			{Name: "actions-runner-linux-ppc64el-2.338.2.tar.gz", DownloadURL: "https://example.com/ppc64el"},
		},
	}

	tools := runnerToolsFromRelease(context.Background(), release)
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if got := tools[0].GetArchitecture(); got != "x64" {
		t.Errorf("amd64 should normalize to x64, got %s", got)
	}
	if got := tools[1].GetArchitecture(); got != "ppc64le" {
		t.Errorf("ppc64el should normalize to ppc64le, got %s", got)
	}
}

// A tools metadata URL may sit behind the private CA configured on the
// endpoint, which the system trust store knows nothing about.
func TestGetRunnerToolsFromMetadataURLPrivateCA(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(canonicalReleaseList)); err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	defer srv.Close()

	// Without the CA the handshake fails against the system trust store.
	if _, err := getRunnerToolsFromMetadataURL(context.Background(), srv.URL, params.ForgeCredentials{}); err == nil {
		t.Fatal("expected an untrusted certificate to fail")
	}

	caBundle := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: srv.Certificate().Raw,
	})

	tools, err := getRunnerToolsFromMetadataURL(context.Background(), srv.URL, params.ForgeCredentials{CABundle: caBundle})
	if err != nil {
		t.Fatalf("expected the endpoint CA to be trusted: %v", err)
	}
	if len(tools) != 5 {
		t.Errorf("expected 5 tools, got %d", len(tools))
	}
}

// The override is per OS and architecture, not only for the architectures
// GitHub is missing. This pins the worked example in the docs: merging the
// Canonical release over what the GitHub API reports leaves only the
// combinations Canonical does not build coming from GitHub.
func TestMergeOverridesEveryCombinationItPublishes(t *testing.T) {
	// What ListEntityRunnerApplicationDownloads reports for github.com,
	// using the asset set of actions/runner v2.338.2.
	forge := []commonParams.RunnerApplicationDownload{}
	for _, combo := range [][2]string{
		{"linux", "x64"},
		{"linux", "arm"},
		{"linux", "arm64"},
		{"osx", "x64"},
		{"osx", "arm64"},
		{"win", "x64"},
		{"win", "arm64"},
	} {
		osType, arch := combo[0], combo[1]
		downloadURL := "https://github.com/actions/runner/" + osType + "-" + arch
		forge = append(forge, commonParams.RunnerApplicationDownload{
			OS: &osType, Architecture: &arch, DownloadURL: &downloadURL,
		})
	}

	// canonical/github-actions-runner v2.338.2 publishes linux only.
	release := garmUtil.GitHubRelease{TagName: "v2.338.2"}
	for _, arch := range []string{"arm", "arm64", "ppc64le", "s390x", "x64"} {
		release.Assets = append(release.Assets, garmUtil.GitHubReleaseAsset{
			Name:        "actions-runner-linux-" + arch + "-2.338.2.tar.gz",
			DownloadURL: "https://github.com/canonical/github-actions-runner/linux-" + arch,
		})
	}

	merged := garmUtil.MergeTools(forge, runnerToolsFromRelease(context.Background(), release))

	byPlatform := map[string]string{}
	for _, tool := range merged {
		byPlatform[tool.Platform()] = tool.GetDownloadURL()
	}
	if len(byPlatform) != 9 {
		t.Fatalf("expected 9 platforms, got %d: %v", len(byPlatform), byPlatform)
	}

	// Everything Canonical builds comes from Canonical, including the three
	// architectures GitHub also publishes.
	for _, arch := range []string{"x64", "arm", "arm64", "ppc64le", "s390x"} {
		want := "https://github.com/canonical/github-actions-runner/linux-" + arch
		if got := byPlatform["linux/"+arch]; got != want {
			t.Errorf("linux/%s: expected %s, got %s", arch, want, got)
		}
	}

	// Only what Canonical does not build still comes from GitHub.
	for _, platform := range []string{"osx/x64", "osx/arm64", "win/x64", "win/arm64"} {
		if got := byPlatform[platform]; !strings.HasPrefix(got, "https://github.com/actions/runner/") {
			t.Errorf("%s: expected the GitHub binary, got %s", platform, got)
		}
	}
}

// githubRunnerDownloads is a real response from GitHub's
// /actions/runners/downloads endpoint, taken from go-github's own test
// fixture. go-github does no parsing of its own: the endpoint returns os and
// architecture as structured fields and the struct is a plain unmarshal
// target.
const githubRunnerDownloads = `[
	{"os":"osx","architecture":"x64","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-osx-x64-2.164.0.tar.gz","filename":"actions-runner-osx-x64-2.164.0.tar.gz"},
	{"os":"linux","architecture":"x64","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-x64-2.164.0.tar.gz","filename":"actions-runner-linux-x64-2.164.0.tar.gz"},
	{"os":"linux","architecture":"arm","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-arm-2.164.0.tar.gz","filename":"actions-runner-linux-arm-2.164.0.tar.gz"},
	{"os":"win","architecture":"x64","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-win-x64-2.164.0.zip","filename":"actions-runner-win-x64-2.164.0.zip"},
	{"os":"linux","architecture":"arm64","download_url":"https://github.com/actions/runner/releases/download/v2.164.0/actions-runner-linux-arm64-2.164.0.tar.gz","filename":"actions-runner-linux-arm64-2.164.0.tar.gz"}
]`

// The whole override rests on one invariant: the os and architecture parsed
// out of an asset name must be spelled the way the forge API spells them in
// its own os and architecture fields. If they ever diverge, a metadata URL
// would add a second entry for a platform instead of replacing the one the
// API reported.
//
// GitHub states both in the same payload, so this checks one against the
// other: every download it reports carries a filename in exactly the form a
// release asset uses, so parsing that filename has to reproduce the os and
// architecture it reported alongside it.
func TestAssetNameParsingMatchesForgeAPIVocabulary(t *testing.T) {
	var downloads []commonParams.RunnerApplicationDownload
	if err := json.Unmarshal([]byte(githubRunnerDownloads), &downloads); err != nil {
		t.Fatalf("unmarshaling downloads: %v", err)
	}
	if len(downloads) != 5 {
		t.Fatalf("expected 5 downloads, got %d", len(downloads))
	}

	release := garmUtil.GitHubRelease{TagName: "v2.164.0"}
	for _, dl := range downloads {
		release.Assets = append(release.Assets, garmUtil.GitHubReleaseAsset{
			Name:        dl.GetFilename(),
			DownloadURL: dl.GetDownloadURL(),
		})
	}

	parsed := runnerToolsFromRelease(context.Background(), release)
	if len(parsed) != len(downloads) {
		t.Fatalf("expected every download filename to parse, got %d of %d", len(parsed), len(downloads))
	}

	for i, dl := range downloads {
		if got, want := parsed[i].Platform(), dl.Platform(); got != want {
			t.Errorf("%s: parsed platform %q, API reported %q", dl.GetFilename(), got, want)
		}
	}
}
