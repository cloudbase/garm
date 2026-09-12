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
	"time"

	"github.com/cloudbase/garm/params"
)

var (
	credentialsCache      = &credentialCache{newKeyedCache[uint, params.ForgeCredentials](0)}
	giteaCredentialsCache = &credentialCache{newKeyedCache[uint, params.ForgeCredentials](0)}
)

// credentialCache adds the credentials specific compound operations on top
// of the generic cache. Setting credentials also refreshes the credentials
// embedded in cached entities.
type credentialCache struct {
	*keyedCache[uint, params.ForgeCredentials]
}

func (g *credentialCache) SetCredentialsRateLimit(credsID uint, rateLimit params.GithubRateLimit) {
	g.Update(func(cache map[uint]params.ForgeCredentials) {
		if creds, ok := cache[credsID]; ok {
			creds.RateLimit = &rateLimit
			cache[credsID] = creds
		}
	})
}

func (g *credentialCache) UpdateCredentialsUsingEndpoint(ep params.ForgeEndpoint) {
	g.Update(func(cache map[uint]params.ForgeCredentials) {
		for _, creds := range cache {
			if creds.Endpoint.Name == ep.Name {
				creds.Endpoint = ep
				cache[creds.ID] = creds
				UpdateCredentialsInAffectedEntities(creds)
			}
		}
	})
}

func (g *credentialCache) SetCredentials(credentials params.ForgeCredentials) {
	g.Update(func(cache map[uint]params.ForgeCredentials) {
		// Credentials sourced from the database carry no rate limit info, so
		// an update clears the recorded values. That is fine as an update may
		// be a token swap, and rate limits are per token. The clients record
		// fresh values on every forge response and the cache worker's rate
		// limit loop repolls within 30 seconds either way.
		cache[credentials.ID] = credentials
		UpdateCredentialsInAffectedEntities(credentials)
	})
}

func (g *credentialCache) GetAllCredentials() []params.ForgeCredentials {
	creds := g.List()
	sortByID(creds)
	return creds
}

func SetGithubCredentials(credentials params.ForgeCredentials) {
	credentialsCache.SetCredentials(credentials)
}

func GetGithubCredentials(id uint) (params.ForgeCredentials, bool) {
	return credentialsCache.Get(id)
}

func DeleteGithubCredentials(id uint) {
	credentialsCache.Delete(id)
}

func GetAllGithubCredentials() []params.ForgeCredentials {
	return credentialsCache.GetAllCredentials()
}

func SetCredentialsRateLimit(credsID uint, rateLimit params.GithubRateLimit) {
	credentialsCache.SetCredentialsRateLimit(credsID, rateLimit)
}

func GetAllGithubCredentialsAsMap() map[uint]params.ForgeCredentials {
	return credentialsCache.AsMap()
}

func SetGiteaCredentials(credentials params.ForgeCredentials) {
	giteaCredentialsCache.SetCredentials(credentials)
}

func GetGiteaCredentials(id uint) (params.ForgeCredentials, bool) {
	return giteaCredentialsCache.Get(id)
}

func DeleteGiteaCredentials(id uint) {
	giteaCredentialsCache.Delete(id)
}

func GetAllGiteaCredentials() []params.ForgeCredentials {
	return giteaCredentialsCache.GetAllCredentials()
}

func GetAllGiteaCredentialsAsMap() map[uint]params.ForgeCredentials {
	return giteaCredentialsCache.AsMap()
}

func UpdateCredentialsUsingEndpoint(ep params.ForgeEndpoint) {
	giteaCredentialsCache.UpdateCredentialsUsingEndpoint(ep)
}

// GetForgeCredentials returns the cached credentials with the given ID for
// the specified forge type. Unlike the copies workers hold on their entities,
// the cached credentials carry the most recently observed rate limit values.
func GetForgeCredentials(forgeType params.EndpointType, id uint) (params.ForgeCredentials, bool) {
	switch forgeType {
	case params.GithubEndpointType:
		return GetGithubCredentials(id)
	case params.GiteaEndpointType:
		return GetGiteaCredentials(id)
	}
	return params.ForgeCredentials{}, false
}

// entityCredentials resolves the freshest credentials for an entity: the
// entity cache tracks credential swaps, and the credentials cache carries
// the most recently observed rate limit values. Workers hold set-once
// copies of both, so rate limit checks must go through here.
func entityCredentials(entityID string) (params.ForgeCredentials, bool) {
	entity, ok := GetEntity(entityID)
	if !ok {
		return params.ForgeCredentials{}, false
	}
	return GetForgeCredentials(entity.Credentials.ForgeType, entity.Credentials.ID)
}

// EntityRateLimitReached reports whether the credentials currently assigned
// to the given entity should be considered rate limited for normal
// (non-critical) operations, and when the quota resets. Entities or
// credentials missing from the cache are never limited.
func EntityRateLimitReached(entityID string) (bool, time.Time) {
	creds, ok := entityCredentials(entityID)
	if !ok {
		return false, time.Time{}
	}
	return creds.RateLimitReached()
}

// EntityRateLimitExhausted reports whether the quota of the credentials
// currently assigned to the given entity is fully spent, meaning even
// critical operations cannot succeed, and when it resets. Entities or
// credentials missing from the cache are never limited.
func EntityRateLimitExhausted(entityID string) (bool, time.Time) {
	creds, ok := entityCredentials(entityID)
	if !ok {
		return false, time.Time{}
	}
	return creds.CriticalRateLimitReached()
}
