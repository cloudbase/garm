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
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sync"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	garmCache "github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	garmUtil "github.com/cloudbase/garm/util"
)

// releaseIndexTTL is how long a cached release index is considered fresh.
const releaseIndexTTL = 24 * time.Hour

// fetchURL retrieves the body of a URL, bounded by the request context.
func fetchURL(ctx context.Context, uri string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", uri, err)
	}
	resp, err := http.DefaultClient.Do(req) // #nosec G704 -- the releases URL is an operator-configured controller setting
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", uri, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("URL %s returned status %d", uri, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from URL %s: %w", uri, err)
	}
	return data, nil
}

// fetchReleaseIndex fetches the list of releases from a GitHub API-compatible
// /releases endpoint. The endpoint must return an array of releases; the
// releases URL is validated when it is configured on the controller, so
// anything else is treated as an error here.
func fetchReleaseIndex(ctx context.Context, releasesEndpoint string) (garmUtil.GitHubReleases, error) {
	data, err := fetchURL(ctx, releasesEndpoint)
	if err != nil {
		return nil, err
	}
	releases, err := garmUtil.ParseReleaseList(data)
	if err != nil {
		return nil, fmt.Errorf("invalid release list from URL %s: %w", releasesEndpoint, err)
	}
	return releases, nil
}

// fetchReleaseByTag fetches one specific release from a GitHub API-compatible
// releases endpoint, using the /releases/tags/{tag} convention. It is used
// when a pinned version is not part of the cached release index (for example
// a version older than the releases the index page returned).
func fetchReleaseByTag(ctx context.Context, releasesEndpoint, tag string) (garmUtil.GitHubRelease, error) {
	tagURL, err := url.JoinPath(releasesEndpoint, "tags", tag)
	if err != nil {
		return garmUtil.GitHubRelease{}, fmt.Errorf("failed to build tag URL for %s: %w", tag, err)
	}
	data, err := fetchURL(ctx, tagURL)
	if err != nil {
		return garmUtil.GitHubRelease{}, err
	}
	release, err := garmUtil.ParseRelease(data)
	if err != nil {
		return garmUtil.GitHubRelease{}, fmt.Errorf("invalid release from URL %s: %w", tagURL, err)
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

// reconcile brings the release cache and the object store in line with the
// desired state. What the desired release is — and what it costs to find
// out — depends on the mode, so each mode has its own path: following latest
// means periodically refreshing the index, since a new release can only be
// discovered upstream, while a pinned version is fully known up front and
// only repairs actual divergence, never touching the network when the cache
// and the object store already match it. Every step is idempotent, so being
// triggered by our own cache update notifications simply converges to a
// no-op.
func (g *garmToolsSync) reconcile(ctrlInfo params.ControllerInfo) error {
	version := ctrlInfo.GARMAgentVersion
	if version == "" || version == params.GARMAgentLatestVersion {
		return g.reconcileLatest(ctrlInfo)
	}
	return g.reconcilePinned(ctrlInfo, version)
}

// reconcileLatest follows the newest release published at the releases URL.
// The index is refreshed once it goes stale — that is how new releases are
// discovered — and, when tools sync is enabled, the object store is brought
// up to date with whatever "latest" resolves to.
func (g *garmToolsSync) reconcileLatest(ctrlInfo params.ControllerInfo) error {
	index, err := g.refreshIndexIfStale(ctrlInfo)
	if err != nil {
		return err
	}

	latest, ok := garmUtil.LatestAgentRelease(index.Releases)
	if !ok {
		return fmt.Errorf("no usable garm-agent release found at %s", ctrlInfo.GARMAgentReleasesURL)
	}

	if !ctrlInfo.SyncGARMAgentTools {
		return nil
	}
	return g.ensureToolsSynced(latest, ctrlInfo.GARMAgentReleasesURL)
}

// reconcilePinned converges on an explicitly pinned release. The pin alone
// determines the desired version, so there is nothing to discover upstream
// and the index is never refreshed on age: the network is only involved when
// something actually diverges — the cached index is unusable, the pinned
// release is not part of it, or the object store does not hold the pinned
// version yet. In steady state this touches nothing but the local database.
func (g *garmToolsSync) reconcilePinned(ctrlInfo params.ControllerInfo, pin string) error {
	index, err := g.usableIndex(ctrlInfo)
	if err != nil {
		return err
	}

	desired, err := g.resolvePinned(index, pin)
	if err != nil {
		return err
	}

	if !ctrlInfo.SyncGARMAgentTools {
		return nil
	}
	return g.ensureToolsSynced(desired, ctrlInfo.GARMAgentReleasesURL)
}

// currentIndex returns the cached release index, if a valid one exists.
func (g *garmToolsSync) currentIndex(ctrlInfo params.ControllerInfo) (garmUtil.AgentReleaseIndex, bool) {
	if len(ctrlInfo.CachedGARMAgentReleases) == 0 {
		return garmUtil.AgentReleaseIndex{}, false
	}
	index, err := garmUtil.ParseReleaseIndex(ctrlInfo.CachedGARMAgentReleases)
	if err != nil {
		slog.WarnContext(g.ctx, "discarding unparsable cached release index", "error", err)
		return garmUtil.AgentReleaseIndex{}, false
	}
	return index, true
}

// refreshIndexIfStale returns the cached release index, refreshing it first
// when it is missing, older than releaseIndexTTL or fetched from a different
// URL than the one currently configured. This is the latest-following
// freshness policy; pinned reconciliation uses usableIndex, which does not
// care about age.
func (g *garmToolsSync) refreshIndexIfStale(ctrlInfo params.ControllerInfo) (garmUtil.AgentReleaseIndex, error) {
	index, ok := g.currentIndex(ctrlInfo)

	var fetchedAt time.Time
	if ctrlInfo.CachedGARMAgentReleaseFetchedAt != nil {
		fetchedAt = *ctrlInfo.CachedGARMAgentReleaseFetchedAt
	}

	fresh := ok &&
		index.SourceURL == ctrlInfo.GARMAgentReleasesURL &&
		time.Since(fetchedAt) < releaseIndexTTL
	if fresh {
		return index, nil
	}

	slog.InfoContext(g.ctx, "refreshing GARM agent release index",
		"url", ctrlInfo.GARMAgentReleasesURL,
		"had_index", ok,
		"fetched_at", fetchedAt)
	return g.fetchAndPersistIndex(ctrlInfo.GARMAgentReleasesURL)
}

// usableIndex returns the cached release index as long as it can serve
// pinned release lookups: it parses and was fetched from the releases URL
// currently configured on the controller. Unlike refreshIndexIfStale, age
// alone never triggers a refetch — an old index is a perfectly good source
// for looking up a fixed version.
func (g *garmToolsSync) usableIndex(ctrlInfo params.ControllerInfo) (garmUtil.AgentReleaseIndex, error) {
	index, ok := g.currentIndex(ctrlInfo)
	if ok && index.SourceURL == ctrlInfo.GARMAgentReleasesURL {
		return index, nil
	}

	slog.InfoContext(g.ctx, "fetching GARM agent release index",
		"url", ctrlInfo.GARMAgentReleasesURL,
		"had_index", ok)
	return g.fetchAndPersistIndex(ctrlInfo.GARMAgentReleasesURL)
}

// fetchAndPersistIndex fetches the release list from the given URL and
// persists it as the new cached index.
func (g *garmToolsSync) fetchAndPersistIndex(releasesURL string) (garmUtil.AgentReleaseIndex, error) {
	releases, err := fetchReleaseIndex(g.ctx, releasesURL)
	if err != nil {
		return garmUtil.AgentReleaseIndex{}, fmt.Errorf("failed to fetch release index: %w", err)
	}

	index := garmUtil.AgentReleaseIndex{
		SourceURL: releasesURL,
		Releases:  releases,
	}
	if err := g.persistIndex(index); err != nil {
		return garmUtil.AgentReleaseIndex{}, err
	}
	return index, nil
}

// persistIndex stores the release index on the controller. The in-memory
// cache is updated via the resulting database watcher notification.
func (g *garmToolsSync) persistIndex(index garmUtil.AgentReleaseIndex) error {
	data, err := garmUtil.MarshalReleaseIndex(index)
	if err != nil {
		return err
	}
	if err := g.store.UpdateCachedGARMAgentReleases(data, time.Now()); err != nil {
		return fmt.Errorf("failed to update cached release index: %w", err)
	}
	slog.InfoContext(g.ctx, "updated GARM agent release index",
		"url", index.SourceURL,
		"release_count", len(index.Releases))
	return nil
}

// resolvePinned returns the release matching the pinned version. A pin that
// is already part of the index resolves without touching the network; one
// that is not (for example a release older than the page the index holds) is
// fetched by tag and recorded in the index, so every later reconciliation
// finds it locally. When the pin cannot be found at all, the latest known
// release is returned as a fallback and a warning is logged.
func (g *garmToolsSync) resolvePinned(index garmUtil.AgentReleaseIndex, pin string) (garmUtil.GitHubRelease, error) {
	if release, found := garmUtil.FindAgentRelease(index.Releases, pin); found {
		return release, nil
	}

	byTag, err := fetchReleaseByTag(g.ctx, index.SourceURL, pin)
	if err == nil {
		slog.InfoContext(g.ctx, "pinned GARM agent version fetched by tag",
			"pinned_version", pin,
			"resolved_tag", byTag.TagName)
		extended := index
		extended.Releases = append(slices.Clone(index.Releases), byTag)
		if err := g.persistIndex(extended); err != nil {
			return garmUtil.GitHubRelease{}, err
		}
		return byTag, nil
	}

	latest, ok := garmUtil.LatestAgentRelease(index.Releases)
	if !ok {
		return garmUtil.GitHubRelease{}, fmt.Errorf("pinned garm-agent version %q not found and no releases exist at %s", pin, index.SourceURL)
	}
	slog.WarnContext(g.ctx, "pinned GARM agent version not found at the releases URL; defaulting to the latest release",
		"pinned_version", pin,
		"latest_version", latest.TagName,
		"url", index.SourceURL,
		"error", err)
	return latest, nil
}

// ensureToolsSynced brings the object store in line with the given release:
// every os/arch binary the release ships is downloaded, unless a manually
// uploaded tool covers that platform or the stored tool already has the
// desired version. The check is against the object store itself, which makes
// this reconciliation idempotent. originURL is recorded on the stored tools
// as the place they were synced from.
func (g *garmToolsSync) ensureToolsSynced(release garmUtil.GitHubRelease, originURL string) error {
	existing, err := g.garmToolsManager.ListAllGARMTools(g.ctx)
	if err != nil {
		return fmt.Errorf("failed to list existing tools: %w", err)
	}

	manual := make(map[string]bool)
	syncedVersion := make(map[string]string)
	for _, tool := range existing {
		key := string(tool.OSType) + "/" + string(tool.OSArch)
		if tool.Origin == "manual" {
			manual[key] = true
			continue
		}
		syncedVersion[key] = tool.Version
	}

	for _, asset := range release.Assets {
		osType, osArch, err := garmUtil.ParseGARMAgentAssetName(asset.Name)
		if err != nil {
			slog.DebugContext(g.ctx, "skipping asset with unparseable name",
				"asset_name", asset.Name,
				"error", err)
			continue
		}
		key := osType + "/" + osArch

		if manual[key] {
			slog.WarnContext(g.ctx, "skipping sync for tool with manually uploaded version",
				"os_type", osType,
				"os_arch", osArch,
				"upstream_version", release.TagName)
			continue
		}
		if garmUtil.AgentVersionsMatch(syncedVersion[key], release.TagName) {
			slog.DebugContext(g.ctx, "tool already at the desired version",
				"os_type", osType,
				"os_arch", osArch,
				"version", release.TagName)
			continue
		}

		if err := g.syncAsset(release, asset, osType, osArch, originURL); err != nil {
			return err
		}
	}

	return nil
}

// syncAsset downloads one release asset and stores it as a GARM agent tool.
// Uploading a tool replaces any previous version for the same os/arch.
func (g *garmToolsSync) syncAsset(release garmUtil.GitHubRelease, asset garmUtil.GitHubReleaseAsset, osType, osArch, originURL string) error {
	tmpFile, err := g.downloadAssetToTempFile(asset)
	if err != nil {
		return err
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name()) // #nosec G703 -- path comes from os.CreateTemp
	}()

	createParams := params.CreateGARMToolParams{
		Name:        asset.Name,
		Description: fmt.Sprintf("GARM Agent %s for %s/%s", release.TagName, osType, osArch),
		Size:        int64(asset.Size),
		Version:     release.TagName,
		OSType:      commonParams.OSType(osType),
		OSArch:      commonParams.OSArch(osArch),
		Origin:      originURL,
	}

	if _, err := g.garmToolsManager.CreateGARMTool(g.ctx, createParams, tmpFile); err != nil {
		return fmt.Errorf("failed to create GARM tool for %s: %w", asset.Name, err)
	}

	slog.InfoContext(g.ctx, "synced GARM agent tool",
		"name", asset.Name,
		"version", release.TagName,
		"os_type", osType,
		"os_arch", osArch)
	return nil
}

// downloadAssetToTempFile downloads an asset to a temporary file, so the
// database is not locked for the duration of the download. The caller owns
// closing and removing the file.
func (g *garmToolsSync) downloadAssetToTempFile(asset garmUtil.GitHubReleaseAsset) (*os.File, error) {
	req, err := http.NewRequestWithContext(g.ctx, http.MethodGet, asset.DownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for asset %s: %w", asset.Name, err)
	}
	resp, err := http.DefaultClient.Do(req) // #nosec G704 -- asset URLs come from the operator-configured releases endpoint
	if err != nil {
		return nil, fmt.Errorf("failed to download asset %s: %w", asset.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download asset %s: status %d", asset.Name, resp.StatusCode)
	}

	// os.CreateTemp only uses the asset name as a name pattern and confines
	// the file to the temp dir.
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("garm-agent-sync-%s-*", asset.Name)) // #nosec G703
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for %s: %w", asset.Name, err)
	}
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name()) // #nosec G703 -- path comes from os.CreateTemp
		return nil, fmt.Errorf("failed to download asset %s to temp file: %w", asset.Name, err)
	}
	if _, err := tmpFile.Seek(0, 0); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name()) // #nosec G703 -- path comes from os.CreateTemp
		return nil, fmt.Errorf("failed to seek to beginning of temp file: %w", err)
	}
	return tmpFile, nil
}

func (g *garmToolsSync) loop() {
	defer g.Stop()

	// Reconcile every hour. Following latest, this refreshes the release
	// index once it goes stale; pinned to a version, reconciliation only
	// repairs actual divergence, so ticks are typically no-ops.
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
			if err := g.reconcile(garmCache.ControllerInfo()); err != nil {
				slog.ErrorContext(g.ctx, "failed initial sync of GARM agent tools", "error", err)
			}
			initialSync.Stop()
		case <-ticker.C:
			if err := g.reconcile(garmCache.ControllerInfo()); err != nil {
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

// handleControllerUpdate reconciles after controller info changes. It runs
// even when tools sync is disabled: the cached release index feeds the
// metadata service (which serves upstream URLs when sync is off), so changes
// to the releases URL or the pinned version must be reflected in it.
// Reconciliation converges to a no-op when everything already matches.
//
// The event payload is used directly rather than the in-memory cache: the
// cache is refreshed by its own watcher consumer, which races with this one,
// so the cache may still hold the pre-update controller info at this point.
func (g *garmToolsSync) handleControllerUpdate(event common.ChangePayload) {
	ctrlInfo, ok := event.Payload.(params.ControllerInfo)
	if !ok {
		slog.WarnContext(g.ctx, "invalid payload type for controller update event")
		return
	}
	if err := g.reconcile(ctrlInfo); err != nil {
		slog.ErrorContext(g.ctx, "failed to sync GARM agent tools after controller update", "error", err)
	}
}
