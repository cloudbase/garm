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
	entity    params.GithubEntity
	tools     []commonParams.RunnerApplicationDownload
}

type GithubToolsCache struct {
	mux sync.Mutex
	// entity IDs are UUID4s. It is highly unlikely they will collide (🤞).
	entities map[string]GithubEntityTools
}

func (g *GithubToolsCache) Get(entity params.GithubEntity) ([]commonParams.RunnerApplicationDownload, bool) {
	g.mux.Lock()
	defer g.mux.Unlock()

	if cache, ok := g.entities[entity.String()]; ok {
		if time.Since(cache.updatedAt) > 1*time.Hour {
			// Stale cache, remove it.
			delete(g.entities, entity.String())
			return nil, false
		}
		return cache.tools, true
	}
	return nil, false
}

func (g *GithubToolsCache) Set(entity params.GithubEntity, tools []commonParams.RunnerApplicationDownload) {
	g.mux.Lock()
	defer g.mux.Unlock()

	g.entities[entity.String()] = GithubEntityTools{
		updatedAt: time.Now(),
		entity:    entity,
		tools:     tools,
	}
}

func SetGithubToolsCache(entity params.GithubEntity, tools []commonParams.RunnerApplicationDownload) {
	githubToolsCache.Set(entity, tools)
}

func GetGithubToolsCache(entity params.GithubEntity) ([]commonParams.RunnerApplicationDownload, bool) {
	return githubToolsCache.Get(entity)
}
