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
	"fmt"
	"sync"
	"time"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

var githubToolsCache *GithubToolsCache

func init() {
	ghToolsCache := &GithubToolsCache{
		entities: make(map[string]GithubEntityTools),
	}
	githubToolsCache = ghToolsCache
}

type GithubEntityTools struct {
	updatedAt time.Time
	expiresAt time.Time
	entity    params.ForgeEntity
	tools     []commonParams.RunnerApplicationDownload
}

type GithubToolsCache struct {
	mux sync.Mutex
	// entity IDs are UUID4s. It is highly unlikely they will collide (🤞).
	entities map[string]GithubEntityTools
}

func (g *GithubToolsCache) Get(entityID string) ([]commonParams.RunnerApplicationDownload, error) {
	g.mux.Lock()
	defer g.mux.Unlock()

	if cache, ok := g.entities[entityID]; ok {
		if cache.entity.Credentials.ForgeType == params.GithubEndpointType {
			if time.Now().UTC().After(cache.expiresAt.Add(-5 * time.Minute)) {
				// Stale cache, remove it.
				delete(g.entities, entityID)
				return nil, fmt.Errorf("cache expired for entity %s", entityID)
			}
		}
		return cache.tools, nil
	}
	return nil, fmt.Errorf("no cache found for entity %s", entityID)
}

func (g *GithubToolsCache) Set(entity params.ForgeEntity, tools []commonParams.RunnerApplicationDownload) {
	g.mux.Lock()
	defer g.mux.Unlock()

	forgeTools := GithubEntityTools{
		updatedAt: time.Now(),
		entity:    entity,
		tools:     tools,
	}

	if entity.Credentials.ForgeType == params.GithubEndpointType {
		forgeTools.expiresAt = time.Now().Add(1 * time.Hour)
	}

	g.entities[entity.ID] = forgeTools
}

func (g *GithubToolsCache) Delete(entityID string) {
	g.mux.Lock()
	defer g.mux.Unlock()

	delete(g.entities, entityID)
}

func SetGithubToolsCache(entity params.ForgeEntity, tools []commonParams.RunnerApplicationDownload) {
	githubToolsCache.Set(entity, tools)
}

func GetGithubToolsCache(entityID string) ([]commonParams.RunnerApplicationDownload, error) {
	return githubToolsCache.Get(entityID)
}

func DeleteGithubToolsCache(entityID string) {
	githubToolsCache.Delete(entityID)
}
