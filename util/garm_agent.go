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

package util

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

// GitHubReleaseAsset represents an asset from a GitHub release
type GitHubReleaseAsset struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	Size          uint      `json:"size"`
	DownloadCount uint      `json:"download_count"`
	CreatedAt     time.Time `json:"created_at"`
	Digest        string    `json:"digest"`
	DownloadURL   string    `json:"browser_download_url"`
}

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName    string               `json:"tag_name"`
	Name       string               `json:"name"`
	TarballURL string               `json:"tarball_url"`
	Assets     []GitHubReleaseAsset `json:"assets"`
}

// GitHubReleases represents an array of GitHub releases
type GitHubReleases []GitHubRelease

// ParseGARMAgentAssetName parses a garm-agent asset name to extract OS type and architecture
func ParseGARMAgentAssetName(name string) (osType, osArch string, err error) {
	// Skip checksum files
	if strings.HasSuffix(name, ".sha256") || strings.HasSuffix(name, ".md5") {
		return "", "", fmt.Errorf("checksum file, skipping")
	}

	// Remove .exe extension if present
	name = strings.TrimSuffix(name, ".exe")

	// Expected format: garm-agent-{os}-{arch}[-{version}]
	const prefix = "garm-agent-"
	if len(name) < len(prefix) || !strings.HasPrefix(name, prefix) {
		return "", "", fmt.Errorf("invalid asset name format: %s (expected to start with %s)", name, prefix)
	}

	// Split the remainder after "garm-agent-"
	remainder := name[len(prefix):]
	parts := strings.Split(remainder, "-")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid asset name format: %s (expected {os}-{arch})", name)
	}

	osType = parts[0]
	osArch = parts[1]

	return osType, osArch, nil
}

// ParseToolsFromRelease parses cached release data and extracts GARM agent tool information
func ParseToolsFromRelease(releaseData []byte) (map[string]params.GARMAgentTool, error) {
	// Try to unmarshal as an array first
	var releases GitHubReleases
	var release GitHubRelease

	err := json.Unmarshal(releaseData, &releases)
	if err == nil && len(releases) > 0 {
		// Successfully parsed as array with at least one release
		release = releases[0]
	} else {
		// Try as a single release object
		if err := json.Unmarshal(releaseData, &release); err != nil {
			return nil, fmt.Errorf("failed to unmarshal release data: %w", err)
		}
		// Validate it has required fields
		if release.TagName == "" {
			return nil, fmt.Errorf("invalid release format: missing tag_name")
		}
	}

	tools := make(map[string]params.GARMAgentTool)
	for _, asset := range release.Assets {
		// Skip checksum files
		if strings.HasSuffix(asset.Name, ".sha256") || strings.HasSuffix(asset.Name, ".md5") {
			continue
		}

		// Parse asset name
		osType, osArch, err := ParseGARMAgentAssetName(asset.Name)
		if err != nil {
			continue
		}

		// Create key
		key := osType + "/" + osArch

		// Create tool
		tools[key] = params.GARMAgentTool{
			Name:        asset.Name,
			Description: fmt.Sprintf("GARM Agent %s for %s/%s", release.TagName, osType, osArch),
			Size:        int64(asset.Size),
			Version:     release.TagName,
			OSType:      commonParams.OSType(osType),
			OSArch:      commonParams.OSArch(osArch),
			DownloadURL: asset.DownloadURL,
		}
	}

	return tools, nil
}
