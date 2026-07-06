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

//go:build testing
// +build testing

package cache

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
	garmUtil "github.com/cloudbase/garm/util"
)

// fakeToolsManager stands in for the runner behind params.GARMToolsManager;
// it records what the sync worker would have uploaded.
type fakeToolsManager struct {
	tools   []params.GARMAgentTool
	created []params.CreateGARMToolParams
}

func (f *fakeToolsManager) ListAllGARMTools(_ context.Context) ([]params.GARMAgentTool, error) {
	return f.tools, nil
}

func (f *fakeToolsManager) CreateGARMTool(_ context.Context, param params.CreateGARMToolParams, reader io.Reader) (params.FileObject, error) {
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return params.FileObject{}, err
	}
	f.created = append(f.created, param)
	return params.FileObject{}, nil
}

func (f *fakeToolsManager) DeleteGarmTool(_ context.Context, _, _ string) error {
	return nil
}

func newTestStore(t *testing.T) dbCommon.Store {
	t.Helper()
	watcher.InitWatcher(context.Background())
	t.Cleanup(func() { watcher.CloseWatcher() })
	dbCfg := garmTesting.GetTestSqliteDBConfig(t)
	db, err := database.NewDatabase(context.Background(), dbCfg)
	if err != nil {
		t.Fatalf("failed to create db connection: %v", err)
	}
	if _, err := db.InitController(); err != nil {
		t.Fatalf("failed to init controller: %v", err)
	}
	return db
}

func newTestSync(store dbCommon.Store, toolsManager params.GARMToolsManager) *garmToolsSync {
	return &garmToolsSync{
		ctx:              context.Background(),
		store:            store,
		garmToolsManager: toolsManager,
	}
}

// newCountingServer serves the given body for every request and counts how
// often it was hit, so tests can assert exactly how much network traffic a
// reconciliation produced.
func newCountingServer(t *testing.T, body string) (*httptest.Server, *int) {
	t.Helper()
	count := new(int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*count++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, count
}

func testAgentReleases(assetBaseURL string) garmUtil.GitHubReleases {
	return garmUtil.GitHubReleases{
		{
			TagName: "v0.2.0",
			Assets: []garmUtil.GitHubReleaseAsset{
				{Name: "garm-agent-linux-amd64", Size: 4, DownloadURL: assetBaseURL + "/v0.2.0/garm-agent-linux-amd64"},
			},
		},
		{
			TagName: "v0.1.0",
			Assets: []garmUtil.GitHubReleaseAsset{
				{Name: "garm-agent-linux-amd64", Size: 4, DownloadURL: assetBaseURL + "/v0.1.0/garm-agent-linux-amd64"},
			},
		},
	}
}

func marshalIndex(t *testing.T, index garmUtil.AgentReleaseIndex) []byte {
	t.Helper()
	data, err := garmUtil.MarshalReleaseIndex(index)
	if err != nil {
		t.Fatalf("failed to marshal index: %v", err)
	}
	return data
}

func storedIndex(t *testing.T, store dbCommon.Store) (garmUtil.AgentReleaseIndex, bool) {
	t.Helper()
	info, err := store.ControllerInfo()
	if err != nil {
		t.Fatalf("failed to get controller info: %v", err)
	}
	if len(info.CachedGARMAgentReleases) == 0 {
		return garmUtil.AgentReleaseIndex{}, false
	}
	index, err := garmUtil.ParseReleaseIndex(info.CachedGARMAgentReleases)
	if err != nil {
		t.Fatalf("failed to parse stored index: %v", err)
	}
	return index, true
}

func syncedTool(version, origin string) params.GARMAgentTool {
	return params.GARMAgentTool{
		Name:    "garm-agent-linux-amd64",
		Version: version,
		OSType:  commonParams.Linux,
		OSArch:  commonParams.Amd64,
		Origin:  origin,
	}
}

// TestReconcilePinnedSteadyStateTouchesNothing pins a version whose release
// is in the cached index and whose tools are already in the object store: a
// reconciliation must not produce a single network request or store write,
// no matter how old the index is.
func TestReconcilePinnedSteadyStateTouchesNothing(t *testing.T) {
	server, count := newCountingServer(t, `[]`)
	store := newTestStore(t)
	// The synced tool version deliberately drops the "v" prefix; matching is
	// prefix tolerant.
	manager := &fakeToolsManager{tools: []params.GARMAgentTool{syncedTool("0.1.0", server.URL)}}

	fetchedAt := time.Now().Add(-3 * releaseIndexTTL)
	ctrlInfo := params.ControllerInfo{
		GARMAgentVersion:     "v0.1.0",
		GARMAgentReleasesURL: server.URL,
		SyncGARMAgentTools:   true,
		CachedGARMAgentReleases: marshalIndex(t, garmUtil.AgentReleaseIndex{
			SourceURL: server.URL,
			Releases:  testAgentReleases(server.URL),
		}),
		CachedGARMAgentReleaseFetchedAt: &fetchedAt,
	}

	g := newTestSync(store, manager)
	if err := g.reconcile(ctrlInfo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *count != 0 {
		t.Errorf("expected no network requests, got %d", *count)
	}
	if len(manager.created) != 0 {
		t.Errorf("expected no tools to be synced, got %d", len(manager.created))
	}
	if _, ok := storedIndex(t, store); ok {
		t.Error("expected the stored index to remain untouched")
	}
}

// TestReconcilePinnedSyncsDivergedTools pins a version the object store does
// not hold yet: only the asset download may hit the network; the release
// index itself must not be refetched.
func TestReconcilePinnedSyncsDivergedTools(t *testing.T) {
	assetServer, assetCount := newCountingServer(t, "binary")
	indexServer, indexCount := newCountingServer(t, `[]`)
	store := newTestStore(t)
	manager := &fakeToolsManager{tools: []params.GARMAgentTool{syncedTool("v0.2.0", indexServer.URL)}}

	fetchedAt := time.Now().Add(-3 * releaseIndexTTL)
	ctrlInfo := params.ControllerInfo{
		GARMAgentVersion:     "v0.1.0",
		GARMAgentReleasesURL: indexServer.URL,
		SyncGARMAgentTools:   true,
		CachedGARMAgentReleases: marshalIndex(t, garmUtil.AgentReleaseIndex{
			SourceURL: indexServer.URL,
			Releases:  testAgentReleases(assetServer.URL),
		}),
		CachedGARMAgentReleaseFetchedAt: &fetchedAt,
	}

	g := newTestSync(store, manager)
	if err := g.reconcile(ctrlInfo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *indexCount != 0 {
		t.Errorf("expected the release index not to be refetched, got %d requests", *indexCount)
	}
	if *assetCount != 1 {
		t.Errorf("expected exactly one asset download, got %d", *assetCount)
	}
	if len(manager.created) != 1 || manager.created[0].Version != "v0.1.0" {
		t.Fatalf("expected one synced tool at v0.1.0, got %+v", manager.created)
	}
}

// TestReconcilePinnedFetchesMissingPinByTag pins a release that is not part
// of the cached index: it is fetched by tag and recorded in the index, so
// subsequent reconciliations resolve it locally.
func TestReconcilePinnedFetchesMissingPinByTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/tags/v0.0.5") {
			t.Errorf("unexpected request path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"tag_name": "v0.0.5", "assets": [{"name": "garm-agent-linux-amd64", "size": 1, "browser_download_url": "https://example.com/v0.0.5/garm-agent-linux-amd64"}]}`))
	}))
	t.Cleanup(server.Close)
	store := newTestStore(t)

	fetchedAt := time.Now()
	ctrlInfo := params.ControllerInfo{
		GARMAgentVersion:     "v0.0.5",
		GARMAgentReleasesURL: server.URL,
		CachedGARMAgentReleases: marshalIndex(t, garmUtil.AgentReleaseIndex{
			SourceURL: server.URL,
			Releases:  testAgentReleases(server.URL),
		}),
		CachedGARMAgentReleaseFetchedAt: &fetchedAt,
	}

	g := newTestSync(store, &fakeToolsManager{})
	if err := g.reconcile(ctrlInfo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	index, ok := storedIndex(t, store)
	if !ok {
		t.Fatal("expected the extended index to be persisted")
	}
	if len(index.Releases) != 3 {
		t.Fatalf("expected 3 releases in the extended index, got %d", len(index.Releases))
	}
	if _, found := garmUtil.FindAgentRelease(index.Releases, "v0.0.5"); !found {
		t.Error("expected the fetched release to be part of the persisted index")
	}
}

// TestReconcilePinnedMissingPinFallsBackToLatest pins a version the releases
// endpoint does not know about at all: reconciliation falls back to the
// latest known release instead of failing.
func TestReconcilePinnedMissingPinFallsBackToLatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`))
	}))
	t.Cleanup(server.Close)
	store := newTestStore(t)
	// The object store already holds the latest release, so the fallback has
	// nothing left to sync.
	manager := &fakeToolsManager{tools: []params.GARMAgentTool{syncedTool("v0.2.0", server.URL)}}

	fetchedAt := time.Now()
	ctrlInfo := params.ControllerInfo{
		GARMAgentVersion:     "v9.9.9",
		GARMAgentReleasesURL: server.URL,
		SyncGARMAgentTools:   true,
		CachedGARMAgentReleases: marshalIndex(t, garmUtil.AgentReleaseIndex{
			SourceURL: server.URL,
			Releases:  testAgentReleases(server.URL),
		}),
		CachedGARMAgentReleaseFetchedAt: &fetchedAt,
	}

	g := newTestSync(store, manager)
	if err := g.reconcile(ctrlInfo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(manager.created) != 0 {
		t.Errorf("expected no tools to be synced, got %+v", manager.created)
	}
	if _, ok := storedIndex(t, store); ok {
		t.Error("expected the failed by-tag fetch not to modify the stored index")
	}
}

// TestReconcilePinnedRefetchesWhenURLChanges points the controller at a new
// releases URL: the cached index no longer matches and must be refetched,
// even while pinned.
func TestReconcilePinnedRefetchesWhenURLChanges(t *testing.T) {
	server, count := newCountingServer(t, validReleaseList)
	store := newTestStore(t)

	fetchedAt := time.Now()
	ctrlInfo := params.ControllerInfo{
		GARMAgentVersion:     "v0.1.0",
		GARMAgentReleasesURL: server.URL,
		CachedGARMAgentReleases: marshalIndex(t, garmUtil.AgentReleaseIndex{
			SourceURL: "http://old.example.com/releases",
			Releases:  testAgentReleases(server.URL),
		}),
		CachedGARMAgentReleaseFetchedAt: &fetchedAt,
	}

	g := newTestSync(store, &fakeToolsManager{})
	if err := g.reconcile(ctrlInfo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *count != 1 {
		t.Errorf("expected exactly one index fetch, got %d", *count)
	}
	index, ok := storedIndex(t, store)
	if !ok {
		t.Fatal("expected the refetched index to be persisted")
	}
	if index.SourceURL != server.URL {
		t.Errorf("expected the index source URL to be %q, got %q", server.URL, index.SourceURL)
	}
}

// TestReconcileLatestRefreshesStaleIndex follows latest with an index past
// its TTL: discovering new releases requires refetching, so the index must
// be refreshed.
func TestReconcileLatestRefreshesStaleIndex(t *testing.T) {
	for _, version := range []string{"", params.GARMAgentLatestVersion} {
		t.Run("version "+version, func(t *testing.T) {
			server, count := newCountingServer(t, validReleaseList)
			store := newTestStore(t)

			fetchedAt := time.Now().Add(-2 * releaseIndexTTL)
			ctrlInfo := params.ControllerInfo{
				GARMAgentVersion:     version,
				GARMAgentReleasesURL: server.URL,
				CachedGARMAgentReleases: marshalIndex(t, garmUtil.AgentReleaseIndex{
					SourceURL: server.URL,
					Releases:  testAgentReleases(server.URL),
				}),
				CachedGARMAgentReleaseFetchedAt: &fetchedAt,
			}

			g := newTestSync(store, &fakeToolsManager{})
			if err := g.reconcile(ctrlInfo); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if *count != 1 {
				t.Errorf("expected exactly one index fetch, got %d", *count)
			}
			if _, ok := storedIndex(t, store); !ok {
				t.Error("expected the refreshed index to be persisted")
			}
		})
	}
}

// TestReconcileLatestFreshIndexNoRefetch follows latest with a fresh index:
// reconciliation converges without network traffic.
func TestReconcileLatestFreshIndexNoRefetch(t *testing.T) {
	server, count := newCountingServer(t, validReleaseList)
	store := newTestStore(t)
	manager := &fakeToolsManager{tools: []params.GARMAgentTool{syncedTool("v0.2.0", server.URL)}}

	fetchedAt := time.Now()
	ctrlInfo := params.ControllerInfo{
		GARMAgentReleasesURL: server.URL,
		SyncGARMAgentTools:   true,
		CachedGARMAgentReleases: marshalIndex(t, garmUtil.AgentReleaseIndex{
			SourceURL: server.URL,
			Releases:  testAgentReleases(server.URL),
		}),
		CachedGARMAgentReleaseFetchedAt: &fetchedAt,
	}

	g := newTestSync(store, manager)
	if err := g.reconcile(ctrlInfo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *count != 0 {
		t.Errorf("expected no network requests, got %d", *count)
	}
	if len(manager.created) != 0 {
		t.Errorf("expected no tools to be synced, got %d", len(manager.created))
	}
}
