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
	err       error
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
		if cache.err != nil {
			return nil, cache.err
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
		err:       nil,
	}

	if entity.Credentials.ForgeType == params.GithubEndpointType {
		forgeTools.expiresAt = time.Now().Add(1 * time.Hour)
	}

	g.entities[entity.ID] = forgeTools
}

func (g *GithubToolsCache) SetToolsError(entity params.ForgeEntity, err error) {
	g.mux.Lock()
	defer g.mux.Unlock()

	// If the entity is not in the cache, add it with the error.
	cache, ok := g.entities[entity.ID]
	if !ok {
		g.entities[entity.ID] = GithubEntityTools{
			updatedAt: time.Now(),
			entity:    entity,
			err:       err,
		}
		return
	}

	// Update the error for the existing entity.
	cache.err = err
	g.entities[entity.ID] = cache
}

func SetGithubToolsCache(entity params.ForgeEntity, tools []commonParams.RunnerApplicationDownload) {
	githubToolsCache.Set(entity, tools)
}

func GetGithubToolsCache(entityID string) ([]commonParams.RunnerApplicationDownload, error) {
	return githubToolsCache.Get(entityID)
}

func SetGithubToolsCacheError(entity params.ForgeEntity, err error) {
	githubToolsCache.SetToolsError(entity, err)
}
