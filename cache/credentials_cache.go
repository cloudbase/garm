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
	"sync"

	"github.com/cloudbase/garm/params"
)

var (
	credentialsCache      *CredentialCache
	giteaCredentialsCache *CredentialCache
)

func init() {
	ghCredentialsCache := &CredentialCache{
		cache: make(map[uint]params.ForgeCredentials),
	}
	gtCredentialsCache := &CredentialCache{
		cache: make(map[uint]params.ForgeCredentials),
	}

	credentialsCache = ghCredentialsCache
	giteaCredentialsCache = gtCredentialsCache
}

type CredentialCache struct {
	mux sync.Mutex

	cache map[uint]params.ForgeCredentials
}

func (g *CredentialCache) SetCredentialsRateLimit(credsID uint, rateLimit params.GithubRateLimit) {
	g.mux.Lock()
	defer g.mux.Unlock()

	if creds, ok := g.cache[credsID]; ok {
		creds.RateLimit = &rateLimit
		g.cache[credsID] = creds
	}
}

func (g *CredentialCache) SetCredentials(credentials params.ForgeCredentials) {
	g.mux.Lock()
	defer g.mux.Unlock()

	g.cache[credentials.ID] = credentials
	UpdateCredentialsInAffectedEntities(credentials)
}

func (g *CredentialCache) GetCredentials(id uint) (params.ForgeCredentials, bool) {
	g.mux.Lock()
	defer g.mux.Unlock()

	if creds, ok := g.cache[id]; ok {
		return creds, true
	}
	return params.ForgeCredentials{}, false
}

func (g *CredentialCache) DeleteCredentials(id uint) {
	g.mux.Lock()
	defer g.mux.Unlock()

	delete(g.cache, id)
}

func (g *CredentialCache) GetAllCredentials() []params.ForgeCredentials {
	g.mux.Lock()
	defer g.mux.Unlock()

	creds := make([]params.ForgeCredentials, 0, len(g.cache))
	for _, cred := range g.cache {
		creds = append(creds, cred)
	}

	// Sort the credentials by ID
	sortByID(creds)
	return creds
}

func (g *CredentialCache) GetAllCredentialsAsMap() map[uint]params.ForgeCredentials {
	g.mux.Lock()
	defer g.mux.Unlock()

	creds := make(map[uint]params.ForgeCredentials, len(g.cache))
	for id, cred := range g.cache {
		creds[id] = cred
	}

	return creds
}

func SetGithubCredentials(credentials params.ForgeCredentials) {
	credentialsCache.SetCredentials(credentials)
}

func GetGithubCredentials(id uint) (params.ForgeCredentials, bool) {
	return credentialsCache.GetCredentials(id)
}

func DeleteGithubCredentials(id uint) {
	credentialsCache.DeleteCredentials(id)
}

func GetAllGithubCredentials() []params.ForgeCredentials {
	return credentialsCache.GetAllCredentials()
}

func SetCredentialsRateLimit(credsID uint, rateLimit params.GithubRateLimit) {
	credentialsCache.SetCredentialsRateLimit(credsID, rateLimit)
}

func GetAllGithubCredentialsAsMap() map[uint]params.ForgeCredentials {
	return credentialsCache.GetAllCredentialsAsMap()
}

func SetGiteaCredentials(credentials params.ForgeCredentials) {
	giteaCredentialsCache.SetCredentials(credentials)
}

func GetGiteaCredentials(id uint) (params.ForgeCredentials, bool) {
	return giteaCredentialsCache.GetCredentials(id)
}

func DeleteGiteaCredentials(id uint) {
	giteaCredentialsCache.DeleteCredentials(id)
}

func GetAllGiteaCredentials() []params.ForgeCredentials {
	return giteaCredentialsCache.GetAllCredentials()
}

func GetAllGiteaCredentialsAsMap() map[uint]params.ForgeCredentials {
	return giteaCredentialsCache.GetAllCredentialsAsMap()
}
