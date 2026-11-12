// Copyright 2026 Cloudbase Solutions SRL
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
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	commonUtil "github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/params"
	garmUtil "github.com/cloudbase/garm/util"
)

// actionsRunnerAsset matches the runner archives published in an actions
// runner release, for example "actions-runner-linux-ppc64le-2.338.2.tar.gz".
// Capture groups are the OS, the architecture and the version.
//
// The version and the archive extension must make up the entire tail of the
// name. Without that, variant packages such as
// "actions-runner-linux-x64-2.338.2-noexternals.tar.gz" would parse as the
// same OS/arch combo and shadow the full runner archive.
var actionsRunnerAsset = regexp.MustCompile(`^actions-runner-([a-z0-9]+)-([a-z0-9_]+)-v?(\d+(?:\.\d+)*)\.(?:tar\.gz|zip)$`)

// runnerToolsFromRelease converts the assets of an actions runner release
// into runner tools. Assets that are not runner archives, or that are built
// for an architecture GARM does not know how to resolve, are skipped.
func runnerToolsFromRelease(ctx context.Context, release garmUtil.GitHubRelease) []commonParams.RunnerApplicationDownload {
	ret := []commonParams.RunnerApplicationDownload{}
	for _, asset := range release.Assets {
		match := actionsRunnerAsset.FindStringSubmatch(asset.Name)
		if match == nil {
			slog.DebugContext(ctx, "ignoring asset that is not a runner archive", "asset", asset.Name)
			continue
		}

		osType, rawArch := match[1], match[2]
		arch, err := commonUtil.ResolveToGithubArch(rawArch)
		if err != nil {
			slog.InfoContext(ctx, "ignoring unrecognized tools arch", "asset", asset.Name, "arch", rawArch)
			continue
		}

		tool := commonParams.RunnerApplicationDownload{
			OS:           &osType,
			Architecture: &arch,
			DownloadURL:  &asset.DownloadURL,
			Filename:     &asset.Name,
		}
		// A release digest is prefixed with its algorithm, and only a sha256
		// one can serve as the sha256 checksum.
		if checksum, ok := strings.CutPrefix(asset.Digest, "sha256:"); ok {
			tool.SHA256Checksum = &checksum
		}

		slog.DebugContext(ctx, "found valid runner tools", "download_url", asset.DownloadURL, "os", osType, "arch", arch, "file_name", asset.Name)
		ret = append(ret, tool)
	}
	return ret
}

// getRunnerToolsFromMetadataURL fetches a GitHub API compatible release list
// and returns the runner tools of its newest release, preferring a stable one
// and falling back to a pre-release when the list holds nothing else.
//
// This is how GARM learns about runners for architectures upstream GitHub
// does not publish. Third parties such as Canonical build patched runners
// and publish them as releases of their own fork.
func getRunnerToolsFromMetadataURL(ctx context.Context, metadataURL string, creds params.ForgeCredentials) ([]commonParams.RunnerApplicationDownload, error) {
	// The forge transport trusts the endpoint CA bundle on top of the system
	// trust store, and carries no authentication. Both matter here: the URL
	// is operator supplied and need not belong to the forge, so the forge
	// token must not be sent to it.
	transport, err := creds.GetBaseTransport()
	if err != nil {
		return nil, fmt.Errorf("failed to get base transport: %w", err)
	}

	data, err := fetchURL(ctx, &http.Client{Transport: transport}, metadataURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release list: %w", err)
	}

	releases, err := garmUtil.ParseReleaseList(data)
	if err != nil {
		return nil, fmt.Errorf("invalid release list from URL %s: %w", metadataURL, err)
	}

	release, ok := garmUtil.LatestRelease(releases)
	if !ok {
		return nil, fmt.Errorf("no releases found at URL %s", metadataURL)
	}

	tools := runnerToolsFromRelease(ctx, release)
	if len(tools) == 0 {
		return nil, fmt.Errorf("no runner tools found in release %s from URL %s", release.TagName, metadataURL)
	}
	return tools, nil
}
