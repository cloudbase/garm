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
	"time"

	"github.com/cloudbase/garm/params"
)

var entityCache *EntityCache

func init() {
	ghEntityCache := &EntityCache{
		entities:  make(map[string]EntityItem),
		pools:     make(map[string]params.Pool),
		scalesets: make(map[uint]params.ScaleSet),
	}
	entityCache = ghEntityCache
}

type RunnerGroupEntry struct {
	RunnerGroupID int64
	time          time.Time
}

type EntityItem struct {
	Entity       params.ForgeEntity
	Pools        map[string]struct{}
	ScaleSets    map[uint]struct{}
	RunnerGroups map[string]RunnerGroupEntry
}

type EntityCache struct {
	mux sync.Mutex
	// entity IDs are UUID4s. It is highly unlikely they will collide (🤞).
	entities  map[string]EntityItem
	pools     map[string]params.Pool
	scalesets map[uint]params.ScaleSet
}

func (e *EntityCache) GetEntityForScaleSet(scaleSetID uint) (params.ForgeEntity, bool) {
	e.mux.Lock()
	defer e.mux.Unlock()

	scaleSet, ok := e.scalesets[scaleSetID]
	if !ok {
		return params.ForgeEntity{}, false
	}

	entity, err := scaleSet.GetEntity()
	if err != nil {
		return params.ForgeEntity{}, false
	}
	if cacheEntity, ok := e.entities[entity.ID]; ok {
		return cacheEntity.Entity, true
	}
	return params.ForgeEntity{}, false
}

func (e *EntityCache) GetEntityForPool(poolID string) (params.ForgeEntity, bool) {
	e.mux.Lock()
	defer e.mux.Unlock()

	pool, ok := e.pools[poolID]
	if !ok {
		return params.ForgeEntity{}, false
	}

	entity, err := pool.GetEntity()
	if err != nil {
		return params.ForgeEntity{}, false
	}
	if cacheEntity, ok := e.entities[entity.ID]; ok {
		return cacheEntity.Entity, true
	}
	return params.ForgeEntity{}, false
}

func (e *EntityCache) GetPoolByID(poolID string) (params.Pool, bool) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if pool, ok := e.pools[poolID]; ok {
		return pool, ok
	}

	return params.Pool{}, false
}

func (e *EntityCache) GetScaleSetByID(scaleSetID uint) (params.ScaleSet, bool) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if scaleSet, ok := e.scalesets[scaleSetID]; ok {
		return scaleSet, ok
	}

	return params.ScaleSet{}, false
}

func (e *EntityCache) UpdateCredentialsInAffectedEntities(creds params.ForgeCredentials) {
	e.mux.Lock()
	defer e.mux.Unlock()

	for entityID, cache := range e.entities {
		if cache.Entity.Credentials.GetID() == creds.GetID() {
			cache.Entity.Credentials = creds
			e.entities[entityID] = cache
		}
	}
}

func (e *EntityCache) GetEntity(entityID string) (params.ForgeEntity, bool) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if cache, ok := e.entities[entityID]; ok {
		var creds params.ForgeCredentials
		var ok bool
		switch cache.Entity.Credentials.ForgeType {
		case params.GithubEndpointType:
			creds, ok = GetGithubCredentials(cache.Entity.Credentials.ID)
		case params.GiteaEndpointType:
			creds, ok = GetGiteaCredentials(cache.Entity.Credentials.ID)
		}
		if ok {
			cache.Entity.Credentials = creds
		}
		return cache.Entity, true
	}
	return params.ForgeEntity{}, false
}

func (e *EntityCache) SetEntity(entity params.ForgeEntity) {
	e.mux.Lock()
	defer e.mux.Unlock()

	cache, ok := e.entities[entity.ID]
	if !ok {
		e.entities[entity.ID] = EntityItem{
			Entity:       entity,
			Pools:        make(map[string]struct{}),
			ScaleSets:    make(map[uint]struct{}),
			RunnerGroups: make(map[string]RunnerGroupEntry),
		}
		return
	}
	cache.Entity = entity
	e.entities[entity.ID] = cache
}

func (e *EntityCache) ReplaceEntityPools(entityID string, pools []params.Pool) {
	e.mux.Lock()
	defer e.mux.Unlock()

	cache, ok := e.entities[entityID]
	if !ok {
		return
	}

	poolsByID := map[string]struct{}{}
	for _, pool := range pools {
		poolEntity, err := pool.GetEntity()
		if err != nil || poolEntity.ID != entityID {
			continue
		}
		e.pools[pool.ID] = pool
		// map the pool ID to the entity. We have to do an extra lookup
		// in the pools map, but it makes it easier to lookup just pools later
		// when we want to find the pool for the instance.
		poolsByID[pool.ID] = struct{}{}
	}
	cache.Pools = poolsByID
	e.entities[entityID] = cache
}

func (e *EntityCache) ReplaceEntityScaleSets(entityID string, scaleSets []params.ScaleSet) {
	e.mux.Lock()
	defer e.mux.Unlock()

	cache, ok := e.entities[entityID]
	if !ok {
		return
	}

	scaleSetsByID := map[uint]struct{}{}
	for _, scaleSet := range scaleSets {
		scaleSetEntity, err := scaleSet.GetEntity()
		if err != nil || scaleSetEntity.ID != entityID {
			continue
		}
		e.scalesets[scaleSet.ID] = scaleSet
		scaleSetsByID[scaleSet.ID] = struct{}{}
	}
	cache.ScaleSets = scaleSetsByID
	e.entities[entityID] = cache
}

func (e *EntityCache) DeleteEntity(entityID string) {
	e.mux.Lock()
	defer e.mux.Unlock()
	delete(e.entities, entityID)
}

func (e *EntityCache) SetEntityPool(entityID string, pool params.Pool) {
	e.mux.Lock()
	defer e.mux.Unlock()

	poolEntity, err := pool.GetEntity()
	if err != nil || poolEntity.ID != entityID {
		return
	}

	if cache, ok := e.entities[entityID]; ok {
		e.pools[pool.ID] = pool
		cache.Pools[pool.ID] = struct{}{}
		e.entities[entityID] = cache
	}
}

func (e *EntityCache) SetEntityScaleSet(entityID string, scaleSet params.ScaleSet) {
	e.mux.Lock()
	defer e.mux.Unlock()

	scaleSetEntity, err := scaleSet.GetEntity()
	if err != nil || scaleSetEntity.ID != entityID {
		return
	}

	if cache, ok := e.entities[entityID]; ok {
		e.scalesets[scaleSet.ID] = scaleSet
		cache.ScaleSets[scaleSet.ID] = struct{}{}
		e.entities[entityID] = cache
	}
}

func (e *EntityCache) DeleteEntityPool(entityID string, poolID string) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if cache, ok := e.entities[entityID]; ok {
		delete(cache.Pools, poolID)
		e.entities[entityID] = cache
	}
}

func (e *EntityCache) DeleteEntityScaleSet(entityID string, scaleSetID uint) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if cache, ok := e.entities[entityID]; ok {
		delete(cache.ScaleSets, scaleSetID)
		e.entities[entityID] = cache
	}
}

func (e *EntityCache) GetEntityPool(entityID string, poolID string) (params.Pool, bool) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if cache, ok := e.entities[entityID]; ok {
		if _, ok := cache.Pools[poolID]; ok {
			if cachePool, ok := e.pools[poolID]; ok {
				return cachePool, true
			}
		}
	}
	return params.Pool{}, false
}

func (e *EntityCache) GetEntityScaleSet(entityID string, scaleSetID uint) (params.ScaleSet, bool) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if cache, ok := e.entities[entityID]; ok {
		if _, ok := cache.ScaleSets[scaleSetID]; ok {
			if scaleSet, ok := e.scalesets[scaleSetID]; ok {
				return scaleSet, true
			}
		}
	}
	return params.ScaleSet{}, false
}

func (e *EntityCache) FindPoolsMatchingAllTags(entityID string, tags []string) []params.Pool {
	e.mux.Lock()
	defer e.mux.Unlock()

	if cache, ok := e.entities[entityID]; ok {
		var pools []params.Pool
		for poolID := range cache.Pools {
			if pool, ok := e.pools[poolID]; ok {
				if pool.HasRequiredLabels(tags) {
					pools = append(pools, pool)
				}
			}
		}
		// Sort the pools by creation date.
		sortByCreationDate(pools)
		return pools
	}
	return nil
}

func (e *EntityCache) GetEntityPools(entityID string) []params.Pool {
	e.mux.Lock()
	defer e.mux.Unlock()

	if cache, ok := e.entities[entityID]; ok {
		var pools []params.Pool
		for poolID := range cache.Pools {
			if pool, ok := e.pools[poolID]; ok {
				pools = append(pools, pool)
			}
		}
		// Sort the pools by creation date.
		sortByCreationDate(pools)
		return pools
	}
	return nil
}

func (e *EntityCache) GetEntityScaleSets(entityID string) []params.ScaleSet {
	e.mux.Lock()
	defer e.mux.Unlock()

	if cache, ok := e.entities[entityID]; ok {
		var scaleSets []params.ScaleSet
		for scaleSetID := range cache.ScaleSets {
			if scaleSet, ok := e.scalesets[scaleSetID]; ok {
				scaleSets = append(scaleSets, scaleSet)
			}
		}
		// Sort the scale sets by creation date.
		sortByID(scaleSets)
		return scaleSets
	}
	return nil
}

func (e *EntityCache) GetEntitiesUsingCredentials(creds params.ForgeCredentials) []params.ForgeEntity {
	e.mux.Lock()
	defer e.mux.Unlock()

	var entities []params.ForgeEntity
	for _, cache := range e.entities {
		if cache.Entity.Credentials.ForgeType != creds.ForgeType {
			continue
		}

		if cache.Entity.Credentials.GetID() == creds.GetID() {
			entities = append(entities, cache.Entity)
		}
	}
	sortByCreationDate(entities)
	return entities
}

func (e *EntityCache) GetAllEntities() []params.ForgeEntity {
	e.mux.Lock()
	defer e.mux.Unlock()

	var entities []params.ForgeEntity
	for _, cache := range e.entities {
		// Get the credentials from the credentials cache.
		var creds params.ForgeCredentials
		var ok bool
		switch cache.Entity.Credentials.ForgeType {
		case params.GithubEndpointType:
			creds, ok = GetGithubCredentials(cache.Entity.Credentials.ID)
		case params.GiteaEndpointType:
			creds, ok = GetGiteaCredentials(cache.Entity.Credentials.ID)
		}
		if ok {
			cache.Entity.Credentials = creds
		}
		entities = append(entities, cache.Entity)
	}
	sortByCreationDate(entities)
	return entities
}

func (e *EntityCache) GetAllPools() []params.Pool {
	e.mux.Lock()
	defer e.mux.Unlock()

	var pools []params.Pool
	for _, pool := range e.pools {
		pools = append(pools, pool)
	}
	sortByCreationDate(pools)
	return pools
}

func (e *EntityCache) GetAllScaleSets() []params.ScaleSet {
	e.mux.Lock()
	defer e.mux.Unlock()

	var scaleSets []params.ScaleSet
	for _, scaleSet := range e.scalesets {
		scaleSets = append(scaleSets, scaleSet)
	}
	sortByID(scaleSets)
	return scaleSets
}

func (e *EntityCache) SetEntityRunnerGroup(entityID, runnerGroupName string, runnerGroupID int64) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if _, ok := e.entities[entityID]; ok {
		e.entities[entityID].RunnerGroups[runnerGroupName] = RunnerGroupEntry{
			RunnerGroupID: runnerGroupID,
			time:          time.Now().UTC(),
		}
	}
}

func (e *EntityCache) GetEntityRunnerGroup(entityID, runnerGroupName string) (int64, bool) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if _, ok := e.entities[entityID]; ok {
		if runnerGroup, ok := e.entities[entityID].RunnerGroups[runnerGroupName]; ok {
			if time.Now().UTC().After(runnerGroup.time.Add(1 * time.Hour)) {
				delete(e.entities[entityID].RunnerGroups, runnerGroupName)
				return 0, false
			}
			return runnerGroup.RunnerGroupID, true
		}
	}
	return 0, false
}

func SetEntityRunnerGroup(entityID, runnerGroupName string, runnerGroupID int64) {
	entityCache.SetEntityRunnerGroup(entityID, runnerGroupName, runnerGroupID)
}

func GetEntityRunnerGroup(entityID, runnerGroupName string) (int64, bool) {
	return entityCache.GetEntityRunnerGroup(entityID, runnerGroupName)
}

func GetEntity(entityID string) (params.ForgeEntity, bool) {
	return entityCache.GetEntity(entityID)
}

func SetEntity(entity params.ForgeEntity) {
	entityCache.SetEntity(entity)
}

func ReplaceEntityPools(entityID string, pools []params.Pool) {
	entityCache.ReplaceEntityPools(entityID, pools)
}

func ReplaceEntityScaleSets(entityID string, scaleSets []params.ScaleSet) {
	entityCache.ReplaceEntityScaleSets(entityID, scaleSets)
}

func DeleteEntity(entityID string) {
	entityCache.DeleteEntity(entityID)
}

func SetEntityPool(entityID string, pool params.Pool) {
	entityCache.SetEntityPool(entityID, pool)
}

func SetEntityScaleSet(entityID string, scaleSet params.ScaleSet) {
	entityCache.SetEntityScaleSet(entityID, scaleSet)
}

func DeleteEntityPool(entityID string, poolID string) {
	entityCache.DeleteEntityPool(entityID, poolID)
}

func DeleteEntityScaleSet(entityID string, scaleSetID uint) {
	entityCache.DeleteEntityScaleSet(entityID, scaleSetID)
}

func GetEntityPool(entityID string, poolID string) (params.Pool, bool) {
	return entityCache.GetEntityPool(entityID, poolID)
}

func GetEntityScaleSet(entityID string, scaleSetID uint) (params.ScaleSet, bool) {
	return entityCache.GetEntityScaleSet(entityID, scaleSetID)
}

func FindPoolsMatchingAllTags(entityID string, tags []string) []params.Pool {
	return entityCache.FindPoolsMatchingAllTags(entityID, tags)
}

func GetEntityPools(entityID string) []params.Pool {
	return entityCache.GetEntityPools(entityID)
}

func GetEntityScaleSets(entityID string) []params.ScaleSet {
	return entityCache.GetEntityScaleSets(entityID)
}

func UpdateCredentialsInAffectedEntities(creds params.ForgeCredentials) {
	entityCache.UpdateCredentialsInAffectedEntities(creds)
}

func GetEntitiesUsingCredentials(creds params.ForgeCredentials) []params.ForgeEntity {
	return entityCache.GetEntitiesUsingCredentials(creds)
}

func GetAllEntities() []params.ForgeEntity {
	return entityCache.GetAllEntities()
}

func GetAllPools() []params.Pool {
	return entityCache.GetAllPools()
}

func GetAllScaleSets() []params.ScaleSet {
	return entityCache.GetAllScaleSets()
}

func GetEntityForScaleSet(scaleSetID uint) (params.ForgeEntity, bool) {
	return entityCache.GetEntityForScaleSet(scaleSetID)
}

func GetEntityForPool(poolID string) (params.ForgeEntity, bool) {
	return entityCache.GetEntityForPool(poolID)
}

func GetPoolByID(poolID string) (params.Pool, bool) {
	return entityCache.GetPoolByID(poolID)
}

func GetScaleSetByID(scaleSetID uint) (params.ScaleSet, bool) {
	return entityCache.GetScaleSetByID(scaleSetID)
}
