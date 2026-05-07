// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/util/appdefaults"
)

var githubArchMapping = map[string]string{
	"x86_64":  "x64",
	"amd64":   "x64",
	"armv7l":  "arm",
	"aarch64": "arm64",
	"x64":     "x64",
	"arm":     "arm",
	"arm64":   "arm64",
}

// Known prefixes for Gitea runner asset names. The new name (gitea-runner)
// contains a hyphen, so we can't simply split on "-" to parse the asset name.
// We strip the prefix first, then split the remainder.
var giteaRunnerPrefixes = []string{
	"gitea-runner-",
	"act_runner-",
}

var nightlyGiteaRunner = GiteaEntityTool{
	TagName:    "nightly",
	Name:       "nightly",
	TarballURL: "https://gitea.com/gitea/runner/archive/main.tar.gz",
	Assets: []GiteaToolsAssets{
		{
			Name:        "gitea-runner-nightly-linux-amd64",
			DownloadURL: "https://dl.gitea.com/runner/nightly/gitea-runner-nightly-linux-amd64",
		},
		{
			Name:        "gitea-runner-nightly-linux-arm64",
			DownloadURL: "https://dl.gitea.com/runner/nightly/gitea-runner-nightly-linux-arm64",
		},
		{
			Name:        "gitea-runner-nightly-windows-amd64.exe",
			DownloadURL: "https://dl.gitea.com/runner/nightly/gitea-runner-nightly-windows-amd64.exe",
		},
	},
}

type GiteaToolsAssets struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	Size          uint      `json:"size"`
	DownloadCount uint      `json:"download_count"`
	CreatedAt     time.Time `json:"created_at"`
	UUID          string    `json:"uuid"`
	DownloadURL   string    `json:"browser_download_url"`
}

// stripRunnerPrefix removes the known runner prefix from an asset name and
// returns the remainder (e.g. "nightly-linux-amd64"). It returns an error
// if the name doesn't match any known prefix.
func stripRunnerPrefix(name string) (string, error) {
	for _, prefix := range giteaRunnerPrefixes {
		if remainder, ok := strings.CutPrefix(name, prefix); ok {
			return remainder, nil
		}
	}
	return "", fmt.Errorf("asset name %q does not match any known runner prefix", name)
}

func (g GiteaToolsAssets) GetOS() (*string, error) {
	if g.Name == "" {
		return nil, fmt.Errorf("gitea tools name is empty")
	}

	remainder, err := stripRunnerPrefix(g.Name)
	if err != nil {
		return nil, fmt.Errorf("could not parse asset name: %w", err)
	}

	// remainder is "{version}-{os}-{arch}[.exe]"
	parts := strings.SplitN(remainder, "-", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("could not parse asset name")
	}

	os := parts[1]
	return &os, nil
}

func (g GiteaToolsAssets) GetArch() (*string, error) {
	if g.Name == "" {
		return nil, fmt.Errorf("gitea tools name is empty")
	}

	remainder, err := stripRunnerPrefix(g.Name)
	if err != nil {
		return nil, fmt.Errorf("could not parse asset name: %w", err)
	}

	// remainder is "{version}-{os}-{arch}[.exe]"
	parts := strings.SplitN(remainder, "-", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("could not parse asset name")
	}

	archParts := strings.SplitN(parts[2], ".", 2)
	if len(archParts) == 0 {
		return nil, fmt.Errorf("unexpected asset name format")
	}
	arch := githubArchMapping[archParts[0]]
	if arch == "" {
		return nil, fmt.Errorf("could not find arch for %s", archParts[0])
	}
	return &arch, nil
}

type GiteaEntityTool struct {
	// TagName is the semver version of the release.
	TagName    string             `json:"tag_name"`
	Name       string             `json:"name"`
	TarballURL string             `json:"tarball_url"`
	Assets     []GiteaToolsAssets `json:"assets"`
}

type GiteaEntityTools []GiteaEntityTool

func (g GiteaEntityTools) GetLatestVersion() string {
	if len(g) == 0 {
		return ""
	}
	return g[0].TagName
}

func (g GiteaEntityTools) MinimumVersion() (GiteaEntityTool, bool) {
	if len(g) == 0 {
		return GiteaEntityTool{}, false
	}
	for _, tool := range g {
		if semver.Compare(tool.TagName, appdefaults.GiteaRunnerMinimumVersion) >= 0 {
			return tool, true
		}
	}
	return GiteaEntityTool{}, false
}

func getReleasesFromURL(ctx context.Context, metadataURL string) (GiteaEntityTool, error) {
	if metadataURL == "" {
		metadataURL = appdefaults.GiteaRunnerReleasesURL
	}
	// We don't return the result to the user. We get the data and attempt to unmarshal
	// the result as a specific json. If that fails, we error out. The value is set by the
	// admin/user of GARM after authentication. If they have admin rights to GARM, they most
	// likely have admin rights to the machine running GARM, in which case, they can just
	// GET the metadataURL manually from that server.
	resp, err := http.Get(metadataURL) // nolint
	if err != nil {
		return GiteaEntityTool{}, fmt.Errorf("failed to fetch URL %s: %w", metadataURL, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return GiteaEntityTool{}, fmt.Errorf("failed to read response from URL %s: %w", metadataURL, err)
	}

	var tools GiteaEntityTools
	err = json.Unmarshal(data, &tools)
	if err != nil {
		return GiteaEntityTool{}, fmt.Errorf("failed to unmarshal response from URL %s: %w", metadataURL, err)
	}

	if len(tools) == 0 {
		return GiteaEntityTool{}, fmt.Errorf("no tools found from URL %s", metadataURL)
	}

	latest, ok := tools.MinimumVersion()
	if !ok {
		slog.InfoContext(ctx, "failed to find tools, falling back to nightly")
		latest = nightlyGiteaRunner
	}
	return latest, nil
}

func getTools(ctx context.Context, metadataURL string, useInternal bool) ([]commonParams.RunnerApplicationDownload, error) {
	if metadataURL == "" {
		metadataURL = appdefaults.GiteaRunnerReleasesURL
	}
	var latest GiteaEntityTool
	var err error
	if useInternal {
		latest = nightlyGiteaRunner
	} else {
		latest, err = getReleasesFromURL(ctx, metadataURL)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get tools from metadata URL", "error", err)
			return nil, fmt.Errorf("failed to get tools: %w", err)
		}
	}

	ret := []commonParams.RunnerApplicationDownload{}

	for _, asset := range latest.Assets {
		arch, err := asset.GetArch()
		if err != nil {
			slog.InfoContext(ctx, "ignoring unrecognized tools arch", "tool", asset.Name)
			continue
		}
		os, err := asset.GetOS()
		if err != nil {
			slog.InfoContext(ctx, "ignoring unrecognized tools os", "tool", asset.Name)
			continue
		}
		if strings.HasSuffix(asset.DownloadURL, ".xz") || strings.HasSuffix(asset.DownloadURL, ".sha256") {
			// filter out compressed files and sha256 sums. Windows does not have any way to uncompress
			// .xz by default.
			continue
		}
		slog.DebugContext(ctx, "found valid tools", "download_url", asset.DownloadURL, "os", os, "arch", arch, "file_name", asset.Name)
		ret = append(ret, commonParams.RunnerApplicationDownload{
			OS:           os,
			Architecture: arch,
			DownloadURL:  &asset.DownloadURL,
			Filename:     &asset.Name,
		})
	}

	return ret, nil
}
