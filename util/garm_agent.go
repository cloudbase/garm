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
	Prerelease bool                 `json:"prerelease"`
	// Body is the description of the release.
	Body string `json:"body"`
}

// GitHubReleases represents an array of GitHub releases
type GitHubReleases []GitHubRelease

// AgentReleaseIndex is the cached list of garm-agent releases fetched from
// the controller's GARMAgentReleasesURL. The source URL is stored alongside
// the releases so that changing the URL on the controller invalidates the
// cached index.
type AgentReleaseIndex struct {
	SourceURL string         `json:"source_url"`
	Releases  GitHubReleases `json:"releases"`
}

// IsChecksumAsset returns true for release assets that only carry a checksum
// of another asset (e.g. foo.sha256). These are not agent binaries and are
// redundant in asset listings, which surface the digests directly.
func IsChecksumAsset(name string) bool {
	return strings.HasSuffix(name, ".sha256") || strings.HasSuffix(name, ".md5")
}

// ParseGARMAgentAssetName parses a garm-agent asset name to extract OS type and architecture
func ParseGARMAgentAssetName(name string) (osType, osArch string, err error) {
	// Skip checksum files
	if IsChecksumAsset(name) {
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

// ParseReleaseList parses the response of a GitHub-compatible /releases
// endpoint. The data must be a JSON array of releases, each carrying a
// tag_name; anything else is an error. Single release objects (e.g. from a
// /releases/latest endpoint) are intentionally not accepted: the releases
// URL is validated when it is configured on the controller.
func ParseReleaseList(data []byte) (GitHubReleases, error) {
	var releases GitHubReleases
	if err := json.Unmarshal(data, &releases); err != nil {
		return nil, fmt.Errorf("failed to unmarshal release list: %w", err)
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}
	for i, release := range releases {
		if release.TagName == "" {
			return nil, fmt.Errorf("invalid release list: entry %d is missing tag_name", i)
		}
	}
	return releases, nil
}

// ParseRelease parses a single GitHub release object.
func ParseRelease(data []byte) (GitHubRelease, error) {
	var release GitHubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return GitHubRelease{}, fmt.Errorf("failed to unmarshal release: %w", err)
	}
	if release.TagName == "" {
		return GitHubRelease{}, fmt.Errorf("invalid release format: missing tag_name")
	}
	return release, nil
}

// ParseReleaseIndex parses a cached AgentReleaseIndex blob as stored on the
// controller.
func ParseReleaseIndex(data []byte) (AgentReleaseIndex, error) {
	var index AgentReleaseIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return AgentReleaseIndex{}, fmt.Errorf("failed to unmarshal release index: %w", err)
	}
	return index, nil
}

// MarshalReleaseIndex serializes an AgentReleaseIndex for storage on the
// controller.
func MarshalReleaseIndex(index AgentReleaseIndex) ([]byte, error) {
	data, err := json.Marshal(index)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal release index: %w", err)
	}
	return data, nil
}

// AgentVersionsMatch compares two agent versions, tolerating a leading "v"
// on either side ("v1.2.3" refers to the release tagged "1.2.3" and vice
// versa).
//
// This is deliberately tag identity, not semver precedence: at least one
// side is always a raw tag name from the configured releases endpoint,
// which is not guaranteed to be valid semver. golang.org/x/mod/semver's
// Compare treats all invalid version strings as equal to one another and
// ignores build metadata, either of which would produce false matches when
// deciding whether a stored tool already has the desired version.
func AgentVersionsMatch(a, b string) bool {
	return strings.TrimPrefix(a, "v") == strings.TrimPrefix(b, "v")
}

// LatestAgentRelease returns the newest stable release from a release index,
// falling back to the newest release when only pre-releases exist. Releases
// are expected in the order the API returns them: newest first.
func LatestAgentRelease(releases GitHubReleases) (GitHubRelease, bool) {
	if len(releases) == 0 {
		return GitHubRelease{}, false
	}
	for _, release := range releases {
		if !release.Prerelease {
			return release, true
		}
	}
	return releases[0], true
}

// FindAgentRelease returns the release whose tag matches the given version
// (see AgentVersionsMatch).
func FindAgentRelease(releases GitHubReleases, version string) (GitHubRelease, bool) {
	for _, release := range releases {
		if AgentVersionsMatch(release.TagName, version) {
			return release, true
		}
	}
	return GitHubRelease{}, false
}

// ResolveAgentRelease picks the release the controller should use from a
// release index. An empty version or "latest" resolves to the latest release
// (see LatestAgentRelease); any other version selects the release with the
// matching tag. When the requested version is not present in the index, the
// latest release is returned and found is false — callers decide whether to
// warn or attempt a targeted fetch, but there is always a usable release as
// long as the index is not empty.
func ResolveAgentRelease(releases GitHubReleases, version string) (release GitHubRelease, found bool) {
	if version == "" || version == params.GARMAgentLatestVersion {
		return LatestAgentRelease(releases)
	}
	if release, found := FindAgentRelease(releases, version); found {
		return release, true
	}
	release, _ = LatestAgentRelease(releases)
	return release, false
}

// ToolsFromRelease extracts the GARM agent tools of a single release,
// indexed by "os_type/os_arch". Assets that do not look like agent binaries
// (checksum files, unrelated files) are skipped.
func ToolsFromRelease(release GitHubRelease) map[string]params.GARMAgentTool {
	tools := make(map[string]params.GARMAgentTool)
	for _, asset := range release.Assets {
		osType, osArch, err := ParseGARMAgentAssetName(asset.Name)
		if err != nil {
			continue
		}

		key := osType + "/" + osArch
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
	return tools
}
