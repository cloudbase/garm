package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"golang.org/x/mod/semver"
)

const (
	// GiteaRunnerReleasesURL is the public API URL that returns a json of all Gitea runner releases.
	// By default it returns the last 10 releases, which is enough for our needs.
	GiteaRunnerReleasesURL = "https://gitea.com/api/v1/repos/gitea/act_runner/releases"
	// GiteaRunnerMinimumVersion is the minimum version we need in order to support ephemeral runners.
	GiteaRunnerMinimumVersion = "v0.2.12"
)

var nightlyActRunner = GiteaEntityTool{
	TagName:    "nightly",
	Name:       "nightly",
	TarballURL: "https://gitea.com/gitea/act_runner/archive/main.tar.gz",
	Assets: []GiteaToolsAssets{
		{
			Name:        "act_runner-nightly-linux-amd64.xz",
			DownloadURL: "https://dl.gitea.com/act_runner/nightly/act_runner-nightly-linux-amd64.xz",
		},
		{
			Name:        "act_runner-nightly-linux-arm64.xz",
			DownloadURL: "https://dl.gitea.com/act_runner/nightly/act_runner-nightly-linux-arm64.xz",
		},
		{
			Name:        "act_runner-nightly-windows-amd64.exe.xz",
			DownloadURL: "https://dl.gitea.com/act_runner/nightly/act_runner-nightly-windows-amd64.exe.xz",
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

func (g GiteaToolsAssets) GetOS() *string {
	if g.Name == "" {
		return nil
	}

	parts := strings.SplitN(g.Name, "-", 4)
	if len(parts) != 4 {
		return nil
	}

	os := parts[2]
	return &os
}

func (g GiteaToolsAssets) GetArch() *string {
	if g.Name == "" {
		return nil
	}

	parts := strings.SplitN(g.Name, "-", 4)
	if len(parts) != 4 {
		return nil
	}

	archParts := strings.SplitN(parts[3], ".", 2)
	if len(archParts) == 0 {
		return nil
	}
	arch := archParts[0]
	return &arch
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
		if semver.Compare(tool.TagName, GiteaRunnerMinimumVersion) >= 0 {
			return tool, true
		}
	}
	return GiteaEntityTool{}, false
}

func getTools() ([]commonParams.RunnerApplicationDownload, error) {
	resp, err := http.Get(GiteaRunnerReleasesURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tools GiteaEntityTools
	err = json.Unmarshal(data, &tools)
	if err != nil {
		return nil, err
	}

	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools found")
	}

	latest, ok := tools.MinimumVersion()
	if !ok {
		latest = nightlyActRunner
	}

	ret := []commonParams.RunnerApplicationDownload{}

	for _, asset := range latest.Assets {
		ret = append(ret, commonParams.RunnerApplicationDownload{
			OS:           asset.GetOS(),
			Architecture: asset.GetArch(),
			DownloadURL:  &asset.DownloadURL,
			Filename:     &asset.Name,
		})
	}

	return ret, nil
}
