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
