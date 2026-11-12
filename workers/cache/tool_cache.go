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
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	garmUtil "github.com/cloudbase/garm/util"
	"github.com/cloudbase/garm/util/appdefaults"
	"github.com/cloudbase/garm/util/github"
)

var (
	// githubToolsUpdateDeadline in minutes
	githubToolsUpdateDeadline = 40
	// githubToolsMetadataUpdateDeadline in minutes. The download URLs in a
	// release do not expire, so this can be a lot more relaxed than the
	// deadline for the tools the GitHub API hands out.
	githubToolsMetadataUpdateDeadline = 180
	// giteaToolsUpdateDeadline in minutes
	giteaToolsUpdateDeadline = 180
)

func newToolsUpdater(ctx context.Context, entity params.ForgeEntity, store common.Store) *toolsUpdater {
	workerID := fmt.Sprintf("tools-updater-%s-%s", entity, entity.Credentials.Endpoint.Name)
	ctx = garmUtil.WithSlogContext(
		ctx,
		slog.Any("worker", workerID))

	return &toolsUpdater{
		ctx:    ctx,
		entity: entity,
		quit:   make(chan struct{}),
		store:  store,
	}
}

type toolsUpdater struct {
	ctx context.Context

	entity     params.ForgeEntity
	lastUpdate time.Time
	store      common.Store

	// forgeTools are the tools the forge API reports. On GitHub, fetching
	// them doubles as the canary for the entity credentials, so the call is
	// made on every refresh even when a tools metadata URL is configured.
	forgeTools []commonParams.RunnerApplicationDownload

	// metadataTools are the tools published at the tools metadata URL
	// configured on the entity endpoint. They take precedence over
	// forgeTools for the OS/arch combos they cover. metadataURL is the URL
	// they were fetched from, so that changing it on the endpoint discards
	// them.
	metadataTools      []commonParams.RunnerApplicationDownload
	metadataURL        string
	lastMetadataUpdate time.Time

	// lastErrorEvent is the last failure reported to the entity event log.
	// A refresh that keeps failing is retried on every tick, and the event
	// log holds a bounded number of events per entity, so an identical
	// failure is only reported once. A success clears it, so a failure that
	// comes back is reported again.
	lastErrorEvent string
	// metadataPending records that metadataTools have moved on since the
	// last publish, so the next successful forge fetch publishes them even
	// if the forge tools were not due for a refresh.
	metadataPending bool
	// useInternalTools is the internal tools metadata setting the Gitea
	// tools were last published with. Together with metadataURL it decides
	// whether the source changed and a refetch is due.
	useInternalTools bool

	mux     sync.Mutex
	running bool
	quit    chan struct{}

	reset chan struct{}
}

func (t *toolsUpdater) Start() error {
	t.mux.Lock()
	defer t.mux.Unlock()

	if t.running {
		return nil
	}

	t.running = true
	t.quit = make(chan struct{})

	slog.DebugContext(t.ctx, "starting tools updater", "entity", t.entity.String(), "forge_type", t.entity.Credentials.ForgeType)

	switch t.entity.Credentials.ForgeType {
	case params.GithubEndpointType:
		go t.loop()
	case params.GiteaEndpointType:
		go t.giteaUpdateLoop()
	}
	return nil
}

func (t *toolsUpdater) Stop() error {
	t.mux.Lock()
	defer t.mux.Unlock()

	if !t.running {
		return nil
	}

	t.running = false
	close(t.quit)

	return nil
}

// updateForgeTools fetches the tools the forge API reports. The call is also
// GARM's canary for the entity credentials, so it is made even when the
// endpoint has a tools metadata URL configured.
func (t *toolsUpdater) updateForgeTools(entity params.ForgeEntity) error {
	slog.DebugContext(t.ctx, "updating tools", "last_update", t.lastUpdate, "entity", t.entity.String(), "forge_type", t.entity.Credentials.ForgeType)
	ghCli, err := github.Client(t.ctx, entity)
	if err != nil {
		return fmt.Errorf("getting github client: %w", err)
	}

	tools, err := garmUtil.FetchTools(t.ctx, ghCli)
	if err != nil {
		return fmt.Errorf("error fetching tools: %w", err)
	}

	if len(tools) == 0 {
		return fmt.Errorf("FetchTools returned no tools")
	}

	t.forgeTools = tools
	return nil
}

// publishTools merges the two tools sources and stores the result in the
// tools cache.
func (t *toolsUpdater) publishTools(entity params.ForgeEntity) error {
	tools := garmUtil.MergeTools(t.forgeTools, t.metadataTools)
	if len(tools) == 0 {
		return fmt.Errorf("no tools to publish")
	}

	slog.DebugContext(t.ctx, "updating tools cache", "entity", t.entity.String(), "tool_count", len(tools))
	cache.SetGithubToolsCache(entity, tools)
	return nil
}

// refreshToolsMetadata refetches the tools published at the endpoint tools
// metadata URL when the URL changed or the copy we hold is past its deadline.
// It reports whether the metadata tools changed.
func (t *toolsUpdater) refreshToolsMetadata(entity params.ForgeEntity) (changed bool) {
	// Read the endpoint from its own cache rather than the copy embedded in
	// the credentials. SetEndpoint always updates the endpoint cache, while
	// the embedded copy is only refreshed for credentials that are
	// themselves cached, so it can lag.
	ep, ok := cache.GetEndpoint(entity.Credentials.Endpoint.Name)
	if !ok {
		// Endpoints are cached before entities, and one only disappears when
		// it is deleted, which stops this worker. Treat a miss as "nothing
		// changed" rather than as a cleared URL, so it cannot silently drop
		// the metadata tools.
		slog.DebugContext(t.ctx, "endpoint not in cache; leaving tools metadata alone", "endpoint", entity.Credentials.Endpoint.Name)
		return false
	}

	metadataURL := ep.ToolsMetadataURL
	if metadataURL != t.metadataURL {
		// The operator changed the source of truth. Discard what the old URL
		// gave us before going to the new one.
		slog.DebugContext(t.ctx, "tools metadata URL changed", "old_url", t.metadataURL, "new_url", metadataURL)
		t.metadataURL = metadataURL
		t.metadataTools = nil
		t.lastMetadataUpdate = time.Time{}
		changed = true
	}

	if t.metadataURL == "" {
		return changed
	}

	if !time.Now().UTC().After(t.lastMetadataUpdate.Add(time.Duration(githubToolsMetadataUpdateDeadline) * time.Minute)) {
		return changed
	}

	tools, err := getRunnerToolsFromMetadataURL(t.ctx, t.metadataURL, entity.Credentials)
	if err != nil {
		slog.ErrorContext(t.ctx, "updating tools from metadata URL", "error", err, "metadata_url", t.metadataURL)
		t.addFailureEvent(fmt.Sprintf("failed to update tools using metadata URL %s: %q", t.metadataURL, err))
		return changed
	}

	t.metadataTools = tools
	t.lastMetadataUpdate = time.Now().UTC()
	t.addSuccessEvent(fmt.Sprintf("successfully updated tools using metadata URL %s", t.metadataURL))
	return true
}

// refreshForgeTools fetches the tools the forge API reports and reports
// whether it succeeded.
func (t *toolsUpdater) refreshForgeTools(entity params.ForgeEntity) bool {
	// The tools updater is a critical consumer: pool managers are disabled
	// when the tools cache expires (1 hour validity), so it keeps refreshing
	// even when the remaining quota has dipped into the configured reserve,
	// and only skips calls that are guaranteed to fail. Skip silently: the
	// call cannot succeed and a status event per tick would just spam the
	// entity event log. With the 1 minute tick, a refresh happens at most a
	// minute after the quota resets.
	if limited, resetAt := cache.EntityRateLimitExhausted(t.entity.ID); limited {
		slog.DebugContext(t.ctx, "rate limit exhausted; deferring tools update", "reset_at", resetAt)
		return false
	}

	if err := t.updateForgeTools(entity); err != nil {
		slog.ErrorContext(t.ctx, "updating tools", "error", err)
		t.addFailureEvent(fmt.Sprintf("failed to update tools: %q", err))
		return false
	}

	// Tools are usually valid for 1 hour.
	t.lastUpdate = time.Now().UTC()
	t.addSuccessEvent("successfully updated tools")
	return true
}

// refreshGithubTools refreshes the two tools sources and republishes the
// merged result. The sources have their own deadlines: the tools the GitHub
// API hands out carry short lived download tokens, while the assets of a
// release do not expire.
func (t *toolsUpdater) refreshGithubTools() {
	// One snapshot of the entity for the whole pass, so the credentials the
	// two fetches use and the entity the result is published against cannot
	// disagree with each other.
	entity, ok := cache.GetEntity(t.entity.ID)
	if !ok {
		slog.DebugContext(t.ctx, "entity not in cache; skipping tools refresh", "entity_id", t.entity.ID)
		return
	}

	if t.refreshToolsMetadata(entity) {
		// Remember that the published set is behind the metadata source, so
		// a forge fetch that fails now still leads to a publish once it
		// succeeds.
		t.metadataPending = true
	}

	forgeDue := time.Now().UTC().After(t.lastUpdate.Add(time.Duration(githubToolsUpdateDeadline) * time.Minute))
	if !forgeDue && !t.metadataPending {
		return
	}

	// Publishing resets the one hour validity of the tools cache, and the
	// download URLs the forge API hands out expire on roughly the same
	// clock. A publish therefore always needs a fresh forge fetch behind it,
	// rather than extending the validity of tools we already hold.
	if !t.refreshForgeTools(entity) {
		return
	}

	if err := t.publishTools(entity); err != nil {
		slog.ErrorContext(t.ctx, "publishing tools", "error", err)
		t.addFailureEvent(fmt.Sprintf("failed to publish tools: %q", err))
		return
	}
	t.metadataPending = false
}

func (t *toolsUpdater) Reset() {
	t.mux.Lock()
	defer t.mux.Unlock()

	if !t.running {
		return
	}
	slog.DebugContext(t.ctx, "resetting tools worker", "reset", fmt.Sprintf("%v", t.reset))

	if t.reset != nil {
		close(t.reset)
		t.reset = nil
	}
}

func (t *toolsUpdater) sleepWithCancel(sleepTime time.Duration) (canceled bool) {
	if sleepTime == 0 {
		return false
	}
	ticker := time.NewTicker(sleepTime)
	defer ticker.Stop()

	select {
	case <-ticker.C:
		return false
	case <-t.quit:
	case <-t.ctx.Done():
	}
	return true
}

// refreshGiteaTools refetches the Gitea runner tools when the configured
// source changed or the copy we published is past its deadline. Unlike
// GitHub, the act runner can be downloaded without a token, so there is no
// forge API call to pair this with.
func (t *toolsUpdater) refreshGiteaTools() {
	entity, ok := cache.GetEntity(t.entity.ID)
	if !ok {
		slog.DebugContext(t.ctx, "entity not in cache; skipping tools refresh", "entity_id", t.entity.ID)
		return
	}

	// See refreshToolsMetadata on why this reads the endpoint cache rather
	// than the copy embedded in the credentials.
	metadataURL := appdefaults.GiteaRunnerReleasesURL
	var useInternal bool
	if ep, ok := cache.GetEndpoint(entity.Credentials.Endpoint.Name); ok {
		if ep.ToolsMetadataURL != "" {
			metadataURL = ep.ToolsMetadataURL
		}
		if ep.UseInternalToolsMetadata != nil {
			useInternal = *ep.UseInternalToolsMetadata
		}
	}

	sourceChanged := metadataURL != t.metadataURL || useInternal != t.useInternalTools
	now := time.Now().UTC()
	if !sourceChanged && !now.After(t.lastUpdate.Add(time.Duration(giteaToolsUpdateDeadline)*time.Minute)) {
		return
	}

	tools, err := getTools(t.ctx, metadataURL, useInternal, entity.Credentials)
	if err != nil {
		slog.DebugContext(t.ctx, "failed to update gitea tools", "error", err)
		t.addFailureEvent(fmt.Sprintf("failed to update gitea tools: %q", err))
		return
	}

	if useInternal {
		t.addSuccessEvent("using internal tools metadata")
	} else {
		t.addSuccessEvent(fmt.Sprintf("successfully updated tools using metadata URL %s", metadataURL))
	}
	t.lastUpdate = now
	t.metadataURL = metadataURL
	t.useInternalTools = useInternal
	cache.SetGithubToolsCache(entity, tools)
}

// giteaUpdateLoop updates tools for gitea. The act runner can be downloaded
// without a token, unlike the github tools, which for GHES require a token.
func (t *toolsUpdater) giteaUpdateLoop() {
	defer t.Stop()

	// add some jitter
	timerJitter, err := rand.Int(rand.Reader, big.NewInt(120))
	if err != nil {
		timerJitter = big.NewInt(0)
	}
	ticker := time.NewTicker(1*time.Minute + time.Duration(timerJitter.Int64())*time.Second)
	defer ticker.Stop()

reset:
	t.refreshGiteaTools()

	for {
		t.mux.Lock()
		if t.reset == nil {
			t.reset = make(chan struct{})
		}
		t.mux.Unlock()
		select {
		case <-t.quit:
			slog.DebugContext(t.ctx, "stopping tools updater")
			return
		case <-t.reset:
			goto reset
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.refreshGiteaTools()
		}
	}
}

func (t *toolsUpdater) loop() {
	defer t.Stop()

	// add some jitter. When spinning up multiple entities, we add
	// jitter to prevent stampeeding herd.
	randInt, err := rand.Int(rand.Reader, big.NewInt(3000))
	if err != nil {
		randInt = big.NewInt(0)
	}
	t.sleepWithCancel(time.Duration(randInt.Int64()) * time.Millisecond)

	// add some jitter
	timerJitter, err := rand.Int(rand.Reader, big.NewInt(120))
	if err != nil {
		timerJitter = big.NewInt(0)
	}
	timer := time.NewTicker(1*time.Minute + time.Duration(timerJitter.Int64())*time.Second)
	defer timer.Stop()

reset:
	t.refreshGithubTools()

	for {
		t.mux.Lock()
		if t.reset == nil {
			t.reset = make(chan struct{})
		}
		t.mux.Unlock()

		select {
		case <-t.quit:
			slog.DebugContext(t.ctx, "stopping tools updater")
			return
		case <-t.ctx.Done():
			return
		case <-timer.C:
			t.refreshGithubTools()
		case <-t.reset:
			slog.DebugContext(t.ctx, "resetting tools updater")
			goto reset
		}
	}
}

func (t *toolsUpdater) addStatusEvent(msg string, level params.EventLevel) {
	if err := t.store.AddEntityEvent(t.ctx, t.entity, params.StatusEvent, level, msg, 30); err != nil {
		slog.With(slog.Any("error", err)).Error("failed to add entity event")
	}
}

// addFailureEvent reports a failure to the entity event log, skipping the
// identical failure a retry on the next tick would otherwise add again.
func (t *toolsUpdater) addFailureEvent(msg string) {
	if msg == t.lastErrorEvent {
		return
	}
	t.lastErrorEvent = msg
	t.addStatusEvent(msg, params.EventError)
}

// addSuccessEvent reports a successful refresh and clears the suppressed
// failure, so a failure that returns is reported again.
func (t *toolsUpdater) addSuccessEvent(msg string) {
	t.lastErrorEvent = ""
	t.addStatusEvent(msg, params.EventInfo)
}
