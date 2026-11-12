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
	"testing"
	"time"

	garmCache "github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/params"
)

// newMetadataTestUpdater returns a tools updater for an entity whose endpoint
// serves the given release list at a tools metadata URL, along with a counter
// of how often that URL was hit.
func newMetadataTestUpdater(t *testing.T, releaseList string) (*toolsUpdater, *int) {
	t.Helper()
	server, hits := newCountingServer(t, releaseList)

	endpointName := "endpoint-" + t.Name()
	endpoint := params.ForgeEndpoint{
		Name:             endpointName,
		EndpointType:     params.GithubEndpointType,
		ToolsMetadataURL: server.URL,
	}
	garmCache.SetEndpoint(endpoint)
	t.Cleanup(func() { garmCache.RemoveEndpoint(endpointName) })

	entity := params.ForgeEntity{
		ID: "entity-" + t.Name(),
		Credentials: params.ForgeCredentials{
			ForgeType: params.GithubEndpointType,
			Endpoint:  endpoint,
		},
	}
	garmCache.SetEntity(entity)
	t.Cleanup(func() { garmCache.DeleteEntity(entity.ID) })

	return &toolsUpdater{
		ctx:    context.Background(),
		store:  newTestStore(t),
		entity: entity,
	}, hits
}

// entity returns the snapshot refreshGithubTools would take.
func (t *toolsUpdater) testEntity() params.ForgeEntity {
	entity, _ := garmCache.GetEntity(t.entity.ID)
	return entity
}

func metadataArchs(updater *toolsUpdater) map[string]struct{} {
	ret := map[string]struct{}{}
	for _, tool := range updater.metadataTools {
		ret[tool.GetArchitecture()] = struct{}{}
	}
	return ret
}

func TestRefreshToolsMetadata(t *testing.T) {
	updater, hits := newMetadataTestUpdater(t, canonicalReleaseList)

	if !updater.refreshToolsMetadata(updater.testEntity()) {
		t.Fatal("expected the first refresh to report a change")
	}
	if *hits != 1 {
		t.Errorf("expected 1 fetch, got %d", *hits)
	}

	archs := metadataArchs(updater)
	for _, arch := range []string{"x64", "arm64", "arm", "ppc64le", "s390x"} {
		if _, ok := archs[arch]; !ok {
			t.Errorf("missing %s in metadata tools", arch)
		}
	}

	// Within the deadline nothing is refetched and nothing changed.
	if updater.refreshToolsMetadata(updater.testEntity()) {
		t.Error("expected no change within the refresh deadline")
	}
	if *hits != 1 {
		t.Errorf("expected no extra fetch, got %d", *hits)
	}

	// Past the deadline the URL is hit again.
	updater.lastMetadataUpdate = time.Now().UTC().Add(-time.Duration(githubToolsMetadataUpdateDeadline+1) * time.Minute)
	if !updater.refreshToolsMetadata(updater.testEntity()) {
		t.Error("expected a change past the refresh deadline")
	}
	if *hits != 2 {
		t.Errorf("expected 2 fetches, got %d", *hits)
	}
}

func TestRefreshToolsMetadataURLRemoved(t *testing.T) {
	updater, _ := newMetadataTestUpdater(t, canonicalReleaseList)

	if !updater.refreshToolsMetadata(updater.testEntity()) {
		t.Fatal("expected the first refresh to report a change")
	}
	if len(updater.metadataTools) == 0 {
		t.Fatal("expected metadata tools to be populated")
	}

	// Clearing the URL on the endpoint drops the tools it provided, so the
	// entity falls back to only what the forge API reports.
	endpoint := updater.entity.Credentials.Endpoint
	endpoint.ToolsMetadataURL = ""
	garmCache.SetEndpoint(endpoint)

	if !updater.refreshToolsMetadata(updater.testEntity()) {
		t.Error("expected clearing the URL to report a change")
	}
	if len(updater.metadataTools) != 0 {
		t.Errorf("expected metadata tools to be discarded, got %d", len(updater.metadataTools))
	}
}

func TestRefreshToolsMetadataFetchFailure(t *testing.T) {
	// A release list GARM cannot find any runner tools in.
	updater, hits := newMetadataTestUpdater(t, `[{"tag_name": "v1.0.0", "assets": []}]`)

	// Picking up the URL from the endpoint is itself a change, so the merged
	// set is republished with the forge tools alone while the URL is broken.
	if !updater.refreshToolsMetadata(updater.testEntity()) {
		t.Error("expected picking up the new URL to report a change")
	}
	if len(updater.metadataTools) != 0 {
		t.Errorf("expected no metadata tools, got %d", len(updater.metadataTools))
	}

	// A failure does not advance the deadline, so the next tick retries, and
	// a second failure changes nothing.
	if updater.refreshToolsMetadata(updater.testEntity()) {
		t.Error("expected no change on retry")
	}
	if *hits != 2 {
		t.Errorf("expected 2 fetches, got %d", *hits)
	}
}

// A metadata refresh must not publish on its own: publishing resets the one
// hour validity of the tools cache, and the forge download URLs expire on
// roughly the same clock.
func TestRefreshGithubToolsRequiresForgeFetch(t *testing.T) {
	updater, _ := newMetadataTestUpdater(t, canonicalReleaseList)

	// No forge client is reachable here, so the forge fetch fails and
	// nothing is published.
	updater.refreshGithubTools()

	if !updater.metadataPending {
		t.Error("expected the metadata tools to stay pending publication")
	}
	if _, err := garmCache.GetGithubToolsCache(updater.entity.ID); err == nil {
		t.Error("expected nothing to be published without a fresh forge fetch")
	}
}

// A refresh that keeps failing is retried on every tick. The entity event log
// holds a bounded number of events, so an identical failure must be reported
// once, not once per tick.
func TestRefreshToolsMetadataCollapsesRepeatedFailures(t *testing.T) {
	updater, _ := newMetadataTestUpdater(t, `[{"tag_name": "v1.0.0", "assets": []}]`)

	events := 0
	for range 5 {
		before := updater.lastErrorEvent
		updater.refreshToolsMetadata(updater.testEntity())
		if updater.lastErrorEvent != before {
			events++
		}
	}
	if events != 1 {
		t.Errorf("expected 1 reported failure across 5 ticks, got %d", events)
	}
	if updater.lastErrorEvent == "" {
		t.Error("expected the failure to be recorded")
	}

	// A success clears it, so a failure that comes back is reported again.
	updater.addSuccessEvent("recovered")
	if updater.lastErrorEvent != "" {
		t.Error("expected a success to clear the suppressed failure")
	}
	updater.refreshToolsMetadata(updater.testEntity())
	if updater.lastErrorEvent == "" {
		t.Error("expected the returning failure to be reported again")
	}
}
