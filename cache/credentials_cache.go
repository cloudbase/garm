package cache

import (
	"sync"

	"github.com/cloudbase/garm/params"
)

var credentialsCache *GithubCredentials

func init() {
	ghCredentialsCache := &GithubCredentials{
		cache: make(map[uint]params.GithubCredentials),
	}
	credentialsCache = ghCredentialsCache
}

type GithubCredentials struct {
	mux sync.Mutex

	cache map[uint]params.GithubCredentials
}

func (g *GithubCredentials) SetCredentials(credentials params.GithubCredentials) {
	g.mux.Lock()
	defer g.mux.Unlock()

	g.cache[credentials.ID] = credentials
	UpdateCredentialsInAffectedEntities(credentials)
}

func (g *GithubCredentials) GetCredentials(id uint) (params.GithubCredentials, bool) {
	g.mux.Lock()
	defer g.mux.Unlock()

	if creds, ok := g.cache[id]; ok {
		return creds, true
	}
	return params.GithubCredentials{}, false
}

func (g *GithubCredentials) DeleteCredentials(id uint) {
	g.mux.Lock()
	defer g.mux.Unlock()

	delete(g.cache, id)
}

func (g *GithubCredentials) GetAllCredentials() []params.GithubCredentials {
	g.mux.Lock()
	defer g.mux.Unlock()

	creds := make([]params.GithubCredentials, 0, len(g.cache))
	for _, cred := range g.cache {
		creds = append(creds, cred)
	}
	return creds
}

func SetGithubCredentials(credentials params.GithubCredentials) {
	credentialsCache.SetCredentials(credentials)
}

func GetGithubCredentials(id uint) (params.GithubCredentials, bool) {
	return credentialsCache.GetCredentials(id)
}

func DeleteGithubCredentials(id uint) {
	credentialsCache.DeleteCredentials(id)
}

func GetAllGithubCredentials() []params.GithubCredentials {
	return credentialsCache.GetAllCredentials()
}
