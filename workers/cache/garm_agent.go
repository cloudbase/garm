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
	"net/http"
	"sync"
	"time"
)

type GitHubReleaseAsset struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	Size          uint      `json:"size"`
	DownloadCount uint      `json:"download_count"`
	CreatedAt     time.Time `json:"created_at"`
	Digest        string    `json:"digest"`
	DownloadURL   string    `json:"browser_download_url"`
}

type GitHubRelease struct {
	// TagName is the semver version of the release.
	TagName    string               `json:"tag_name"`
	Name       string               `json:"name"`
	TarballURL string               `json:"tarball_url"`
	Assets     []GitHubReleaseAsset `json:"assets"`
}

type GitHubReleases []GitHubRelease

func getLatestGithubReleaseFromURL(_ context.Context, releasesEndpoint string) (GitHubRelease, error) {
	resp, err := http.Get(releasesEndpoint)
	if err != nil {
		return GitHubRelease{}, fmt.Errorf("failed to fetch URL %s: %w", releasesEndpoint, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return GitHubRelease{}, fmt.Errorf("failed to read response from URL %s: %w", releasesEndpoint, err)
	}

	var tools GitHubReleases
	err = json.Unmarshal(data, &tools)
	if err != nil {
		return GitHubRelease{}, fmt.Errorf("failed to unmarshal response from URL %s: %w", releasesEndpoint, err)
	}

	if len(tools) == 0 {
		return GitHubRelease{}, fmt.Errorf("no tools found from URL %s", releasesEndpoint)
	}

	if len(tools[0].Assets) == 0 {
		return GitHubRelease{}, fmt.Errorf("no downloadable assets found from URL %s", releasesEndpoint)
	}

	return tools[0], nil
}

type garmToolsSync struct {
	ctx context.Context

	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func (g *garmToolsSync) loop() {
	
}
