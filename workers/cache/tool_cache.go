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
	"github.com/cloudbase/garm/util/github"
)

func newToolsUpdater(ctx context.Context, entity params.ForgeEntity, store common.Store) *toolsUpdater {
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
	slog.DebugContext(t.ctx, "updating tools", "entity", t.entity.String(), "forge_type", t.entity.Credentials.ForgeType)
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

	if t.entity.Credentials.ForgeType == params.GiteaEndpointType {
		// no need to reset the gitea tools updater when credentials
		// are updated.
		return
	}

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

	// add some jitter. When spinning up multiple entities, we add
	// jitter to prevent stampeeding herd.
	randInt, err := rand.Int(rand.Reader, big.NewInt(3000))
	if err != nil {
		randInt = big.NewInt(0)
	}
	t.sleepWithCancel(time.Duration(randInt.Int64()) * time.Millisecond)
	tools, err := getTools(t.ctx)
	if err != nil {
		t.addStatusEvent(fmt.Sprintf("failed to update gitea tools: %q", err), params.EventError)
	} else {
		t.addStatusEvent("successfully updated tools", params.EventInfo)
		cache.SetGithubToolsCache(t.entity, tools)
	}

	// Once every 3 hours should be enough. Tools don't expire.
	ticker := time.NewTicker(3 * time.Hour)

	for {
		select {
		case <-t.quit:
			slog.DebugContext(t.ctx, "stopping tools updater")
			return
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			tools, err := getTools(t.ctx)
			if err != nil {
				t.addStatusEvent(fmt.Sprintf("failed to update gitea tools: %q", err), params.EventError)
				slog.DebugContext(t.ctx, "failed to update gitea tools", "error", err)
				continue
			}
			t.addStatusEvent("successfully updated tools", params.EventInfo)
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

	var resetTime time.Time
	now := time.Now().UTC()
	if now.After(t.lastUpdate.Add(40 * time.Minute)) {
		if err := t.updateTools(); err != nil {
			slog.ErrorContext(t.ctx, "updating tools", "error", err)
			t.addStatusEvent(fmt.Sprintf("failed to update tools: %q", err), params.EventError)
			resetTime = now.Add(5 * time.Minute)
		} else {
			// Tools are usually valid for 1 hour.
			resetTime = t.lastUpdate.Add(40 * time.Minute)
			t.addStatusEvent("successfully updated tools", params.EventInfo)
		}
	}

	for {
		if t.reset == nil {
			t.reset = make(chan struct{})
		}
		// add some jitter
		randInt, err := rand.Int(rand.Reader, big.NewInt(300))
		if err != nil {
			randInt = big.NewInt(0)
		}
		timer := time.NewTimer(resetTime.Sub(now) + time.Duration(randInt.Int64())*time.Second)
		select {
		case <-t.quit:
			slog.DebugContext(t.ctx, "stopping tools updater")
			timer.Stop()
			return
		case <-timer.C:
			slog.DebugContext(t.ctx, "updating tools")
			now = time.Now().UTC()
			if err := t.updateTools(); err != nil {
				slog.ErrorContext(t.ctx, "updating tools", "error", err)
				t.addStatusEvent(fmt.Sprintf("failed to update tools: %q", err), params.EventError)
				resetTime = now.Add(5 * time.Minute)
			} else {
				// Tools are usually valid for 1 hour.
				resetTime = t.lastUpdate.Add(40 * time.Minute)
				t.addStatusEvent("successfully updated tools", params.EventInfo)
			}
		case <-t.reset:
			slog.DebugContext(t.ctx, "resetting tools updater")
			timer.Stop()
			now = time.Now().UTC()
			if err := t.updateTools(); err != nil {
				slog.ErrorContext(t.ctx, "updating tools", "error", err)
				t.addStatusEvent(fmt.Sprintf("failed to update tools: %q", err), params.EventError)
				resetTime = now.Add(5 * time.Minute)
			} else {
				// Tools are usually valid for 1 hour.
				resetTime = t.lastUpdate.Add(40 * time.Minute)
				t.addStatusEvent("successfully updated tools", params.EventInfo)
			}
		}
		timer.Stop()
	}
}

func (t *toolsUpdater) addStatusEvent(msg string, level params.EventLevel) {
	if err := t.store.AddEntityEvent(t.ctx, t.entity, params.StatusEvent, level, msg, 30); err != nil {
		slog.With(slog.Any("error", err)).Error("failed to add entity event")
	}
}
