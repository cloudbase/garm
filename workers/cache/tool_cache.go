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
	tools      []commonParams.RunnerApplicationDownload
	lastUpdate time.Time
	store      common.Store

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

func (t *toolsUpdater) updateTools() error {
	slog.DebugContext(t.ctx, "updating tools", "last_update", t.lastUpdate, "entity", t.entity.String(), "forge_type", t.entity.Credentials.ForgeType)
	entity, ok := cache.GetEntity(t.entity.ID)
	if !ok {
		return fmt.Errorf("getting entity from cache: %s", t.entity.ID)
	}
	ghCli, err := github.Client(t.ctx, entity)
	if err != nil {
		return fmt.Errorf("getting github client: %w", err)
	}

	tools, err := garmUtil.FetchTools(t.ctx, ghCli)
	if err != nil {
		return fmt.Errorf("fetching tools: %w", err)
	}
	t.lastUpdate = time.Now().UTC()
	t.tools = tools

	slog.DebugContext(t.ctx, "updating tools cache", "entity", t.entity.String())
	cache.SetGithubToolsCache(entity, tools)
	return nil
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

	oldMetadataURL := ""
	oldUseInternal := false
reset:
	metadataURL := appdefaults.GiteaRunnerReleasesURL
	var useInternal bool
	ep, ok := cache.GetEndpoint(t.entity.Credentials.Endpoint.Name)
	if ok {
		if ep.ToolsMetadataURL != "" {
			metadataURL = ep.ToolsMetadataURL
		}
		if ep.UseInternalToolsMetadata != nil {
			useInternal = *ep.UseInternalToolsMetadata
		}
	}

	now := time.Now().UTC()
	if now.After(t.lastUpdate.Add(time.Duration(giteaToolsUpdateDeadline)*time.Minute)) || oldMetadataURL != metadataURL || oldUseInternal != useInternal {
		tools, err := getTools(t.ctx, metadataURL, useInternal)
		if err != nil {
			t.addStatusEvent(fmt.Sprintf("failed to update gitea tools: %q", err), params.EventError)
		} else {
			if useInternal {
				t.addStatusEvent("using internal tools metadata", params.EventInfo)
			} else {
				t.addStatusEvent(fmt.Sprintf("successfully updated tools using metadata URL %s", metadataURL), params.EventInfo)
			}
			t.lastUpdate = now
			oldMetadataURL = metadataURL
			oldUseInternal = useInternal
			cache.SetGithubToolsCache(t.entity, tools)
		}
	}

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
			now := time.Now().UTC()
			if !now.After(t.lastUpdate.Add(time.Duration(giteaToolsUpdateDeadline)*time.Minute)) || oldMetadataURL != metadataURL || oldUseInternal != useInternal {
				continue
			}
			ep, ok := cache.GetEndpoint(t.entity.Credentials.Endpoint.Name)
			if ok {
				if ep.ToolsMetadataURL != "" {
					metadataURL = ep.ToolsMetadataURL
				}
				if ep.UseInternalToolsMetadata != nil {
					useInternal = *ep.UseInternalToolsMetadata
				}
			}
			tools, err := getTools(t.ctx, metadataURL, useInternal)
			if err != nil {
				t.addStatusEvent(fmt.Sprintf("failed to update gitea tools: %q", err), params.EventError)
				slog.DebugContext(t.ctx, "failed to update gitea tools", "error", err)
				continue
			}
			if useInternal {
				t.addStatusEvent("using internal tools metadata", params.EventInfo)
			} else {
				t.addStatusEvent(fmt.Sprintf("successfully updated tools using metadata URL %s", metadataURL), params.EventInfo)
			}
			t.lastUpdate = now
			oldMetadataURL = metadataURL
			oldUseInternal = useInternal
			cache.SetGithubToolsCache(t.entity, tools)
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
	now := time.Now().UTC()
	if now.After(t.lastUpdate.Add(time.Duration(githubToolsUpdateDeadline) * time.Minute)) {
		slog.DebugContext(t.ctx, "last update after deadline", "last_update", t.lastUpdate, "deadline", t.lastUpdate.Add(time.Duration(githubToolsUpdateDeadline)*time.Minute))
		if err := t.updateTools(); err != nil {
			slog.ErrorContext(t.ctx, "updating tools", "error", err)
			t.addStatusEvent(fmt.Sprintf("failed to update tools: %q", err), params.EventError)
		} else {
			// Tools are usually valid for 1 hour.
			t.lastUpdate = now
			t.addStatusEvent("successfully updated tools", params.EventInfo)
		}
	}

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
		case <-timer.C:
			now := time.Now().UTC()
			if !now.After(t.lastUpdate.Add(time.Duration(githubToolsUpdateDeadline) * time.Minute)) {
				continue
			}
			slog.DebugContext(t.ctx, "updating tools")
			if err := t.updateTools(); err != nil {
				slog.ErrorContext(t.ctx, "updating tools", "error", err)
				t.addStatusEvent(fmt.Sprintf("failed to update tools: %q", err), params.EventError)
			} else {
				// Tools are usually valid for 1 hour.
				t.addStatusEvent("successfully updated tools", params.EventInfo)
			}
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
