package cache

import (
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

func (g *GithubToolsCache) Get(entityID string) ([]commonParams.RunnerApplicationDownload, bool) {
	g.mux.Lock()
	defer g.mux.Unlock()

	if cache, ok := g.entities[entityID]; ok {
		if cache.entity.Credentials.ForgeType == params.GithubEndpointType {
			if time.Now().UTC().After(cache.expiresAt.Add(-5 * time.Minute)) {
				// Stale cache, remove it.
				delete(g.entities, entityID)
				return nil, false
			}
		}
		return cache.tools, true
	}
	return nil, false
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
		forgeTools.expiresAt = time.Now().Add(24 * time.Hour)
	}

	g.entities[entity.ID] = forgeTools
}

func SetGithubToolsCache(entity params.ForgeEntity, tools []commonParams.RunnerApplicationDownload) {
	githubToolsCache.Set(entity, tools)
}

func GetGithubToolsCache(entityID string) ([]commonParams.RunnerApplicationDownload, bool) {
	return githubToolsCache.Get(entityID)
}
