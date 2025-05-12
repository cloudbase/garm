package cache

import (
	"sync"

	"github.com/cloudbase/garm/params"
)

var entityCache *EntityCache

func init() {
	ghEntityCache := &EntityCache{
		entities: make(map[string]EntityItem),
	}
	entityCache = ghEntityCache
}

type EntityItem struct {
	Entity    params.ForgeEntity
	Pools     map[string]params.Pool
	ScaleSets map[uint]params.ScaleSet
}

type EntityCache struct {
	mux sync.Mutex
	// entity IDs are UUID4s. It is highly unlikely they will collide (🤞).
	entities map[string]EntityItem
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
		// Get the credentials from the credentials cache.
		var forgeCredsGetter params.ForgeCredentialsGetter
		var credsOk bool
		switch cache.Entity.Credentials.ForgeType {
		case params.GithubEndpointType:
			forgeCredsGetter, credsOk = GetGithubCredentials(cache.Entity.Credentials.GetID())
		case params.GiteaEndpointType:
			// add gitea credentials getter
			return cache.Entity, false
		}
		if credsOk {
			cache.Entity.Credentials = forgeCredsGetter.GetForgeCredentials()
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
			Entity:    entity,
			Pools:     make(map[string]params.Pool),
			ScaleSets: make(map[uint]params.ScaleSet),
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

	poolsByID := map[string]params.Pool{}
	for _, pool := range pools {
		poolsByID[pool.ID] = pool
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

	scaleSetsByID := map[uint]params.ScaleSet{}
	for _, scaleSet := range scaleSets {
		scaleSetsByID[scaleSet.ID] = scaleSet
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

	if cache, ok := e.entities[entityID]; ok {
		cache.Pools[pool.ID] = pool
		e.entities[entityID] = cache
	}
}

func (e *EntityCache) SetEntityScaleSet(entityID string, scaleSet params.ScaleSet) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if cache, ok := e.entities[entityID]; ok {
		cache.ScaleSets[scaleSet.ID] = scaleSet
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
		if pool, ok := cache.Pools[poolID]; ok {
			return pool, true
		}
	}
	return params.Pool{}, false
}

func (e *EntityCache) GetEntityScaleSet(entityID string, scaleSetID uint) (params.ScaleSet, bool) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if cache, ok := e.entities[entityID]; ok {
		if scaleSet, ok := cache.ScaleSets[scaleSetID]; ok {
			return scaleSet, true
		}
	}
	return params.ScaleSet{}, false
}

func (e *EntityCache) FindPoolsMatchingAllTags(entityID string, tags []string) []params.Pool {
	e.mux.Lock()
	defer e.mux.Unlock()

	if cache, ok := e.entities[entityID]; ok {
		var pools []params.Pool
		for _, pool := range cache.Pools {
			if pool.HasRequiredLabels(tags) {
				pools = append(pools, pool)
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
		for _, pool := range cache.Pools {
			pools = append(pools, pool)
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
		for _, scaleSet := range cache.ScaleSets {
			scaleSets = append(scaleSets, scaleSet)
		}
		// Sort the scale sets by creation date.
		sortByID(scaleSets)
		return scaleSets
	}
	return nil
}

func (e *EntityCache) GetEntitiesUsingGredentials(credsID uint) []params.ForgeEntity {
	e.mux.Lock()
	defer e.mux.Unlock()

	var entities []params.ForgeEntity
	for _, cache := range e.entities {
		if cache.Entity.Credentials.GetID() == credsID {
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
		var forgeCredsGetter params.ForgeCredentialsGetter
		var credsOk bool
		switch cache.Entity.Credentials.ForgeType {
		case params.GithubEndpointType:
			forgeCredsGetter, credsOk = GetGithubCredentials(cache.Entity.Credentials.GetID())
		case params.GiteaEndpointType:
			// add gitea credentials getter
			return nil
		}
		if credsOk {
			cache.Entity.Credentials = forgeCredsGetter.GetForgeCredentials()
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
	for _, cache := range e.entities {
		for _, pool := range cache.Pools {
			pools = append(pools, pool)
		}
	}
	sortByCreationDate(pools)
	return pools
}

func (e *EntityCache) GetAllScaleSets() []params.ScaleSet {
	e.mux.Lock()
	defer e.mux.Unlock()

	var scaleSets []params.ScaleSet
	for _, cache := range e.entities {
		for _, scaleSet := range cache.ScaleSets {
			scaleSets = append(scaleSets, scaleSet)
		}
	}
	sortByID(scaleSets)
	return scaleSets
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

func GetEntitiesUsingGredentials(credsID uint) []params.ForgeEntity {
	return entityCache.GetEntitiesUsingGredentials(credsID)
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
