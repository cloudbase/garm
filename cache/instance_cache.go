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
	"github.com/cloudbase/garm/params"
)

var instanceCache = newKeyedCache[string, params.Instance](50000)

func SetInstanceCache(instance params.Instance) {
	instanceCache.Set(instance.Name, instance)
}

func GetInstanceCache(name string) (params.Instance, bool) {
	return instanceCache.Get(name)
}

func DeleteInstanceCache(name string) {
	instanceCache.Delete(name)
}

func GetAllInstancesCache() []params.Instance {
	instances := instanceCache.List()
	sortByCreationDate(instances)
	return instances
}

func GetInstancesForPool(poolID string) []params.Instance {
	instances := instanceCache.List(func(instance params.Instance) bool {
		return instance.PoolID == poolID
	})
	sortByCreationDate(instances)
	return instances
}

func GetInstancesForScaleSet(scaleSetID uint) []params.Instance {
	instances := instanceCache.List(func(instance params.Instance) bool {
		return instance.ScaleSetID == scaleSetID
	})
	sortByCreationDate(instances)
	return instances
}

func GetEntityInstances(entityID string) []params.Instance {
	pools := GetEntityPools(entityID)
	poolsAsMap := map[string]bool{}
	for _, pool := range pools {
		poolsAsMap[pool.ID] = true
	}

	ret := []params.Instance{}
	for _, val := range GetAllInstancesCache() {
		if _, ok := poolsAsMap[val.PoolID]; ok {
			ret = append(ret, val)
		}
	}
	return ret
}

func GetEntityForInstance(name string) (params.ForgeEntity, bool) {
	instance, ok := GetInstanceCache(name)
	if !ok {
		return params.ForgeEntity{}, false
	}

	var entityID string
	switch {
	case instance.ScaleSetID > 0:
		if scaleSet, ok := GetScaleSetByID(instance.ScaleSetID); ok {
			scaleSetEntity, err := scaleSet.GetEntity()
			if err != nil {
				return params.ForgeEntity{}, false
			}
			entityID = scaleSetEntity.ID
		}
	case instance.PoolID != "":
		if pool, ok := GetPoolByID(instance.PoolID); ok {
			poolEntity, err := pool.GetEntity()
			if err != nil {
				return params.ForgeEntity{}, false
			}
			entityID = poolEntity.ID
		}
	default:
		return params.ForgeEntity{}, false
	}

	if entityID != "" {
		if entity, ok := GetEntity(entityID); ok {
			return entity, true
		}
	}
	return params.ForgeEntity{}, false
}
