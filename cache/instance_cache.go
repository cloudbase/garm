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

var instanceCache *InstanceCache

func init() {
	cache := &InstanceCache{
		cache: make(map[string]params.Instance),
	}
	instanceCache = cache
}

type InstanceCache struct {
	mux sync.Mutex

	cache map[string]params.Instance
}

func (i *InstanceCache) SetInstance(instance params.Instance) {
	i.mux.Lock()
	defer i.mux.Unlock()

	i.cache[instance.Name] = instance
}

func (i *InstanceCache) GetInstance(name string) (params.Instance, bool) {
	i.mux.Lock()
	defer i.mux.Unlock()

	if instance, ok := i.cache[name]; ok {
		return instance, true
	}
	return params.Instance{}, false
}

func (i *InstanceCache) DeleteInstance(name string) {
	i.mux.Lock()
	defer i.mux.Unlock()

	delete(i.cache, name)
}

func (i *InstanceCache) GetAllInstances() []params.Instance {
	i.mux.Lock()
	defer i.mux.Unlock()

	instances := make([]params.Instance, 0, len(i.cache))
	for _, instance := range i.cache {
		instances = append(instances, instance)
	}
	sortByCreationDate(instances)
	return instances
}

func (i *InstanceCache) GetInstancesForPool(poolID string) []params.Instance {
	i.mux.Lock()
	defer i.mux.Unlock()

	var filteredInstances []params.Instance
	for _, instance := range i.cache {
		if instance.PoolID == poolID {
			filteredInstances = append(filteredInstances, instance)
		}
	}
	sortByCreationDate(filteredInstances)
	return filteredInstances
}

func (i *InstanceCache) GetInstancesForScaleSet(scaleSetID uint) []params.Instance {
	i.mux.Lock()
	defer i.mux.Unlock()

	var filteredInstances []params.Instance
	for _, instance := range i.cache {
		if instance.ScaleSetID == scaleSetID {
			filteredInstances = append(filteredInstances, instance)
		}
	}
	sortByCreationDate(filteredInstances)
	return filteredInstances
}

func (i *InstanceCache) GetEntityInstances(entityID string) []params.Instance {
	pools := GetEntityPools(entityID)
	poolsAsMap := map[string]bool{}
	for _, pool := range pools {
		poolsAsMap[pool.ID] = true
	}

	ret := []params.Instance{}
	for _, val := range i.GetAllInstances() {
		if _, ok := poolsAsMap[val.PoolID]; ok {
			ret = append(ret, val)
		}
	}
	return ret
}

func SetInstanceCache(instance params.Instance) {
	instanceCache.SetInstance(instance)
}

func GetInstanceCache(name string) (params.Instance, bool) {
	return instanceCache.GetInstance(name)
}

func DeleteInstanceCache(name string) {
	instanceCache.DeleteInstance(name)
}

func GetAllInstancesCache() []params.Instance {
	return instanceCache.GetAllInstances()
}

func GetInstancesForPool(poolID string) []params.Instance {
	return instanceCache.GetInstancesForPool(poolID)
}

func GetInstancesForScaleSet(scaleSetID uint) []params.Instance {
	return instanceCache.GetInstancesForScaleSet(scaleSetID)
}

func GetEntityInstances(entityID string) []params.Instance {
	return instanceCache.GetEntityInstances(entityID)
}
