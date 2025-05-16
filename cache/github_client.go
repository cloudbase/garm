package cache

import (
	"sync"

	"github.com/cloudbase/garm/runner/common"
)

var ghClientCache *GithubClientCache

type GithubClientCache struct {
	mux sync.Mutex

	cache map[string]common.GithubClient
}

func init() {
	clientCache := &GithubClientCache{
		cache: make(map[string]common.GithubClient),
	}
	ghClientCache = clientCache
}

func (g *GithubClientCache) SetClient(entityID string, client common.GithubClient) {
	g.mux.Lock()
	defer g.mux.Unlock()

	g.cache[entityID] = client
}

func (g *GithubClientCache) GetClient(entityID string) (common.GithubClient, bool) {
	g.mux.Lock()
	defer g.mux.Unlock()

	if client, ok := g.cache[entityID]; ok {
		return client, true
	}
	return nil, false
}

func SetGithubClient(entityID string, client common.GithubClient) {
	ghClientCache.SetClient(entityID, client)
}

func GetGithubClient(entityID string) (common.GithubClient, bool) {
	return ghClientCache.GetClient(entityID)
}
