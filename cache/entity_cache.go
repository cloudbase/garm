package cache

import (
	"log/slog"
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
	Entity    params.GithubEntity
	Pools     map[string]params.Pool
	ScaleSets map[uint]params.ScaleSet
}

type EntityCache struct {
	mux sync.Mutex
	// entity IDs are UUID4s. It is highly unlikely they will collide (🤞).
	entities map[string]EntityItem
}

func (e *EntityCache) GetEntity(entityID string) (params.GithubEntity, bool) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if cache, ok := e.entities[entityID]; ok {
		// Updating specific credential details will not update entity cache which
		// uses those credentials.
		// Entity credentials in the cache are only updated if you swap the creds
		// on the entity. We get the updated credentials from the credentials cache.
		creds, ok := GetGithubCredentials(cache.Entity.Credentials.ID)
		if ok {
			cache.Entity.Credentials = creds
		}
		return cache.Entity, true
	}
	return params.GithubEntity{}, false
}

func (e *EntityCache) SetEntity(entity params.GithubEntity) {
	e.mux.Lock()
	defer e.mux.Unlock()

	_, ok := e.entities[entity.ID]
	if !ok {
		e.entities[entity.ID] = EntityItem{
			Entity:    entity,
			Pools:     make(map[string]params.Pool),
			ScaleSets: make(map[uint]params.ScaleSet),
		}
		return
	}

	e.entities[entity.ID] = EntityItem{
		Entity: entity,
	}
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

func (e *EntityCache) ReplaceEntityScaleSets(entityID string, scaleSets map[uint]params.ScaleSet) {
	e.mux.Lock()
	defer e.mux.Unlock()

	if cache, ok := e.entities[entityID]; ok {
		cache.ScaleSets = scaleSets
		e.entities[entityID] = cache
	}
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
		slog.Debug("Finding pools matching all tags", "entityID", entityID, "tags", tags, "pools", cache.Pools)
		for _, pool := range cache.Pools {
			if pool.HasRequiredLabels(tags) {
				pools = append(pools, pool)
			}
		}
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
		return scaleSets
	}
	return nil
}

func GetEntity(entityID string) (params.GithubEntity, bool) {
	return entityCache.GetEntity(entityID)
}

func SetEntity(entity params.GithubEntity) {
	entityCache.SetEntity(entity)
}

func ReplaceEntityPools(entityID string, pools []params.Pool) {
	entityCache.ReplaceEntityPools(entityID, pools)
}

func ReplaceEntityScaleSets(entityID string, scaleSets map[uint]params.ScaleSet) {
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
