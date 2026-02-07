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
	"os"
	"sync"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	garmCache "github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	garmUtil "github.com/cloudbase/garm/util"
)

// getLatestGithubReleaseFromURL fetches release information from a GitHub API-compatible endpoint.
// This function is flexible and supports:
//   - Array of releases: /repos/{owner}/{repo}/releases (returns first/latest)
//   - Single release object: /repos/{owner}/{repo}/releases/latest
//   - Custom URLs: Users can configure a fork or custom repository URL as long as it
//     follows the GitHub release API format
//
// The response must follow GitHub's release API JSON structure with 'tag_name' and 'assets' fields.
// Arrays are tried first to avoid false positives (empty JSON objects can parse as valid releases).
func getLatestGithubReleaseFromURL(_ context.Context, releasesEndpoint string) (garmUtil.GitHubRelease, error) {
	//nolint:gosec // G107: releasesEndpoint is a user-configured GitHub API URL from controller settings
	resp, err := http.Get(releasesEndpoint)
	if err != nil {
		return garmUtil.GitHubRelease{}, fmt.Errorf("failed to fetch URL %s: %w", releasesEndpoint, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return garmUtil.GitHubRelease{}, fmt.Errorf("failed to read response from URL %s: %w", releasesEndpoint, err)
	}

	// Try to unmarshal as an array first (for /releases endpoint)
	var tools garmUtil.GitHubReleases
	err = json.Unmarshal(data, &tools)
	if err == nil && len(tools) > 0 {
		// Successfully parsed as array with at least one release
		if len(tools[0].Assets) == 0 {
			return garmUtil.GitHubRelease{}, fmt.Errorf("no downloadable assets found from URL %s", releasesEndpoint)
		}
		return tools[0], nil
	}

	// If that fails or array is empty, try as a single release object (for /releases/latest endpoint)
	var release garmUtil.GitHubRelease
	err = json.Unmarshal(data, &release)
	if err != nil {
		return garmUtil.GitHubRelease{}, fmt.Errorf("failed to unmarshal response from URL %s: %w", releasesEndpoint, err)
	}

	// Validate the single release has required fields
	if release.TagName == "" {
		return garmUtil.GitHubRelease{}, fmt.Errorf("invalid release format from URL %s: missing tag_name", releasesEndpoint)
	}

	if len(release.Assets) == 0 {
		return garmUtil.GitHubRelease{}, fmt.Errorf("no downloadable assets found from URL %s", releasesEndpoint)
	}

	return release, nil
}

type garmToolsSync struct {
	ctx              context.Context
	store            common.Store
	garmToolsManager params.GARMToolsManager
	consumerID       string
	consumer         common.Consumer

	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func newGARMToolsSync(ctx context.Context, store common.Store, garmToolsManager params.GARMToolsManager) *garmToolsSync {
	consumerID := "garm-tools-sync"
	ctx = garmUtil.WithSlogContext(
		ctx,
		slog.Any("worker", consumerID))
	return &garmToolsSync{
		ctx:              ctx,
		store:            store,
		consumerID:       consumerID,
		garmToolsManager: garmToolsManager,
		quit:             make(chan struct{}),
	}
}

func (g *garmToolsSync) Start() error {
	g.mux.Lock()
	defer g.mux.Unlock()

	if g.running {
		return nil
	}

	// Register our own consumer to watch for controller info updates
	consumer, err := watcher.RegisterConsumer(
		g.ctx, g.consumerID,
		watcher.WithEntityTypeFilter(common.ControllerEntityType))
	if err != nil {
		return fmt.Errorf("registering consumer for garm tools sync: %w", err)
	}
	g.consumer = consumer

	g.running = true
	g.quit = make(chan struct{})
	go g.loop()
	return nil
}

func (g *garmToolsSync) Stop() error {
	g.mux.Lock()
	defer g.mux.Unlock()

	if !g.running {
		return nil
	}

	g.running = false
	close(g.quit)
	return nil
}

func (g *garmToolsSync) syncToolsFromRelease(release garmUtil.GitHubRelease, originURL string) error {
	// Get all existing tools once before the loop
	allTools, err := g.garmToolsManager.ListAllGARMTools(g.ctx)
	if err != nil {
		slog.WarnContext(g.ctx, "failed to list existing tools", "error", err)
	}

	// Build a map of manually uploaded tools by os/arch for quick lookup
	manualTools := make(map[string]bool)
	for _, tool := range allTools {
		if tool.Origin == "manual" {
			key := string(tool.OSType) + "/" + string(tool.OSArch)
			manualTools[key] = true
		}
	}

	// For each asset in the release, determine OS type and arch, then sync to DB
	for _, asset := range release.Assets {
		// Parse the asset name to determine OS type and arch
		// Expected format: garm-agent-{os}-{arch}[.exe]
		// Examples: garm-agent-linux-amd64, garm-agent-windows-amd64.exe
		osType, osArch, err := garmUtil.ParseGARMAgentAssetName(asset.Name)
		if err != nil {
			slog.WarnContext(g.ctx, "skipping asset with unparseable name",
				"asset_name", asset.Name,
				"error", err)
			continue
		}

		// Check if there's already a manually uploaded tool for this os/arch combination
		toolKey := osType + "/" + osArch
		if manualTools[toolKey] {
			slog.WarnContext(g.ctx, "skipping sync for tool with manually uploaded version",
				"os_type", osType,
				"os_arch", osArch,
				"upstream_version", release.TagName)
			continue
		}

		// Download the asset to a temporary file first to avoid locking the DB during download
		resp, err := http.Get(asset.DownloadURL)
		if err != nil {
			return fmt.Errorf("failed to download asset %s: %w", asset.Name, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to download asset %s: status %d", asset.Name, resp.StatusCode)
		}

		// Create temporary file
		tmpFile, err := os.CreateTemp("", fmt.Sprintf("garm-agent-sync-%s-*", asset.Name))
		if err != nil {
			return fmt.Errorf("failed to create temp file for %s: %w", asset.Name, err)
		}
		tmpPath := tmpFile.Name()
		defer func() {
			tmpFile.Close()
			os.Remove(tmpPath)
		}()

		// Download to temp file
		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			return fmt.Errorf("failed to download asset %s to temp file: %w", asset.Name, err)
		}

		// Seek to beginning for upload
		if _, err := tmpFile.Seek(0, 0); err != nil {
			return fmt.Errorf("failed to seek to beginning of temp file: %w", err)
		}

		// Create GARM tool params
		createParams := params.CreateGARMToolParams{
			Name:        asset.Name,
			Description: fmt.Sprintf("GARM Agent %s for %s/%s", release.TagName, osType, osArch),
			Size:        int64(asset.Size),
			Version:     release.TagName,
			OSType:      commonParams.OSType(osType),
			OSArch:      commonParams.OSArch(osArch),
			Origin:      originURL, // Set origin to the releases URL
		}

		// Upload to GARM tools storage from temp file
		if _, err := g.garmToolsManager.CreateGARMTool(g.ctx, createParams, tmpFile); err != nil {
			return fmt.Errorf("failed to create GARM tool for %s: %w", asset.Name, err)
		}

		slog.InfoContext(g.ctx, "synced GARM agent tool",
			"name", asset.Name,
			"version", release.TagName,
			"os_type", osType,
			"os_arch", osArch,
			"origin", originURL)
	}

	return nil
}

func (g *garmToolsSync) syncIfNeeded() error {
	// Get controller info from cache
	ctrlInfo := garmCache.ControllerInfo()

	// Check cache freshness (at most once per day)
	// We always need the release JSON, even when sync is disabled, because
	// we serve GitHub URLs directly when sync is off
	cachedData := ctrlInfo.CachedGARMAgentRelease
	var fetchedAt time.Time
	if ctrlInfo.CachedGARMAgentReleaseFetchedAt != nil {
		fetchedAt = *ctrlInfo.CachedGARMAgentReleaseFetchedAt
	}

	cacheFresh := len(cachedData) > 0 && time.Since(fetchedAt) < 24*time.Hour

	// If cache is fresh, we can skip fetching
	if cacheFresh {
		slog.DebugContext(g.ctx, "cached GARM agent release is still fresh",
			"fetched_at", fetchedAt,
			"age", time.Since(fetchedAt),
			"sync_enabled", ctrlInfo.SyncGARMAgentTools)
		return nil
	}

	// If sync is disabled and we need to fetch, just fetch and cache (don't sync to object store)
	if !ctrlInfo.SyncGARMAgentTools {
		release, releaseJSON, err := g.fetchRelease(ctrlInfo)
		if err != nil {
			return err
		}
		return g.updateCache(release.TagName, releaseJSON, false)
	}

	// Sync is enabled, proceed with full sync (fetch + sync to object store)
	return g.fetchAndSyncRelease(ctrlInfo)
}

// fetchRelease fetches the latest release from GitHub and marshals it to JSON
// Returns the release struct and JSON bytes for use by callers
func (g *garmToolsSync) fetchRelease(ctrlInfo params.ControllerInfo) (garmUtil.GitHubRelease, []byte, error) {
	slog.InfoContext(g.ctx, "fetching latest GARM agent release",
		"url", ctrlInfo.GARMAgentReleasesURL,
		"sync_enabled", ctrlInfo.SyncGARMAgentTools)

	release, err := getLatestGithubReleaseFromURL(g.ctx, ctrlInfo.GARMAgentReleasesURL)
	if err != nil {
		return garmUtil.GitHubRelease{}, nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	releaseJSON, err := json.Marshal(release)
	if err != nil {
		return garmUtil.GitHubRelease{}, nil, fmt.Errorf("failed to marshal release: %w", err)
	}

	return release, releaseJSON, nil
}

// fetchAndSyncRelease fetches the latest release from GitHub, syncs tools to object store if version changed, and updates the cache
func (g *garmToolsSync) fetchAndSyncRelease(ctrlInfo params.ControllerInfo) error {
	// Fetch fresh release data from GitHub
	release, releaseJSON, err := g.fetchRelease(ctrlInfo)
	if err != nil {
		return err
	}

	// Check if version changed by comparing with cached version
	cachedData := ctrlInfo.CachedGARMAgentRelease
	versionChanged := true // Default to true if no cache exists

	if len(cachedData) > 0 {
		var cachedRelease garmUtil.GitHubRelease
		if err := json.Unmarshal(cachedData, &cachedRelease); err != nil {
			slog.WarnContext(g.ctx, "failed to unmarshal cached release, will re-sync", "error", err)
		} else if cachedRelease.TagName == release.TagName {
			// Version hasn't changed, just update timestamp
			slog.InfoContext(g.ctx, "GARM agent release version unchanged, updating timestamp only",
				"version", release.TagName)
			versionChanged = false
		}
	}

	// Only sync to object store if version actually changed
	if versionChanged {
		slog.InfoContext(g.ctx, "new GARM agent release version detected, syncing to object store",
			"version", release.TagName)

		if err := g.syncToolsFromRelease(release, ctrlInfo.GARMAgentReleasesURL); err != nil {
			return fmt.Errorf("failed to sync tools to object store: %w", err)
		}
	}

	// Update cache with fresh data and timestamp
	return g.updateCache(release.TagName, releaseJSON, versionChanged)
}

// updateCache updates the database with the release data.
// The in-memory cache is automatically updated via the database watcher notification.
func (g *garmToolsSync) updateCache(version string, releaseJSON []byte, synced bool) error {
	now := time.Now()

	// Update database - this triggers a watcher notification that updates the in-memory cache
	if err := g.store.UpdateCachedGARMAgentRelease(releaseJSON, now); err != nil {
		return fmt.Errorf("failed to update cached release: %w", err)
	}

	slog.InfoContext(g.ctx, "successfully updated GARM agent release cache",
		"version", version,
		"synced_to_object_store", synced)

	return nil
}

func (g *garmToolsSync) loop() {
	defer g.Stop()

	// Check every hour (syncIfNeeded will skip if cache is fresh)
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Trigger an immediate check after a short delay to allow GARM to start accepting requests
	initialSync := time.NewTimer(5 * time.Second)
	defer initialSync.Stop()

	for {
		select {
		case <-g.quit:
			return
		case <-g.ctx.Done():
			return
		case <-initialSync.C:
			// Initial sync after startup delay (fires once)
			if err := g.syncIfNeeded(); err != nil {
				slog.ErrorContext(g.ctx, "failed initial sync of GARM agent tools", "error", err)
			}
			// Nil the channel so this case is never selected again
			initialSync = nil
		case <-ticker.C:
			if err := g.syncIfNeeded(); err != nil {
				slog.ErrorContext(g.ctx, "failed to sync GARM agent tools", "error", err)
			}
		case event, ok := <-g.consumer.Watch():
			if !ok {
				slog.InfoContext(g.ctx, "consumer channel closed")
				return
			}
			slog.InfoContext(g.ctx, "got controller update event", "event_type", event.EntityType, "operation", event.Operation)
			// Filter for controller info update events
			if event.EntityType == common.ControllerEntityType && event.Operation == common.UpdateOperation {
				g.handleControllerUpdate(event)
			}
		}
	}
}

func (g *garmToolsSync) handleControllerUpdate(event common.ChangePayload) {
	ctrlInfo, ok := event.Payload.(params.ControllerInfo)
	if !ok {
		slog.WarnContext(g.ctx, "invalid payload type for controller update event")
		return
	}
	// Check if sync is enabled
	if !ctrlInfo.SyncGARMAgentTools {
		slog.WarnContext(g.ctx, "tools sync is disabled, skipping force sync")
		return
	}

	if err := g.syncIfNeeded(); err != nil {
		slog.ErrorContext(g.ctx, "failed to force sync GARM agent tools", "error", err)
	}
}
