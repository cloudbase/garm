// Copyright 2025 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package pool

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/go-github/v71/github"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/cache"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
)

type poolCacheStore interface {
	Next() (params.Pool, error)
	Reset()
	Len() int
}

type poolRoundRobin struct {
	pools []params.Pool
	next  uint32
}

func (p *poolRoundRobin) Next() (params.Pool, error) {
	if len(p.pools) == 0 {
		return params.Pool{}, runnerErrors.ErrNoPoolsAvailable
	}

	n := atomic.AddUint32(&p.next, 1)
	return p.pools[(int(n)-1)%len(p.pools)], nil
}

func (p *poolRoundRobin) Len() int {
	return len(p.pools)
}

func (p *poolRoundRobin) Reset() {
	atomic.StoreUint32(&p.next, 0)
}

type poolsForTags struct {
	pools         sync.Map
	poolCacheType params.PoolBalancerType
}

func (p *poolsForTags) Get(tags []string) (poolCacheStore, bool) {
	sort.Strings(tags)
	key := strings.Join(tags, "^")

	v, ok := p.pools.Load(key)
	if !ok {
		return nil, false
	}
	poolCache := v.(*poolRoundRobin)
	if p.poolCacheType == params.PoolBalancerTypePack {
		// When we service a list of jobs, we want to try each pool in turn
		// for each job. Pools are sorted by priority so we always start from the
		// highest priority pool and move on to the next if the first one is full.
		poolCache.Reset()
	}
	return poolCache, true
}

func (p *poolsForTags) Add(tags []string, pools []params.Pool) poolCacheStore {
	sort.Slice(pools, func(i, j int) bool {
		return pools[i].Priority > pools[j].Priority
	})

	sort.Strings(tags)
	key := strings.Join(tags, "^")

	poolRR := &poolRoundRobin{pools: pools}
	v, _ := p.pools.LoadOrStore(key, poolRR)
	return v.(*poolRoundRobin)
}

func instanceInList(instanceName string, instances []commonParams.ProviderInstance) (commonParams.ProviderInstance, bool) {
	for _, val := range instances {
		if val.Name == instanceName {
			return val, true
		}
	}
	return commonParams.ProviderInstance{}, false
}

func controllerIDFromLabels(labels []string) string {
	for _, lbl := range labels {
		if strings.HasPrefix(lbl, controllerLabelPrefix) {
			trimLength := min(len(controllerLabelPrefix)+1, len(lbl))
			return lbl[trimLength:]
		}
	}
	return ""
}

func labelsFromRunner(runner *github.Runner) []string {
	if runner == nil || runner.Labels == nil {
		return []string{}
	}

	var labels []string
	for _, val := range runner.Labels {
		if val == nil {
			continue
		}
		labels = append(labels, val.GetName())
	}
	return labels
}

// isManagedRunner returns true if labels indicate the runner belongs to a pool
// this manager is responsible for.
func isManagedRunner(labels []string, controllerID string) bool {
	runnerControllerID := controllerIDFromLabels(labels)
	return runnerControllerID == controllerID
}

func composeWatcherFilters(entity params.ForgeEntity) dbCommon.PayloadFilterFunc {
	// We want to watch for changes in either the controller or the
	// entity itself.
	return watcher.WithAny(
		watcher.WithAll(
			// Updates to the controller
			watcher.WithEntityTypeFilter(dbCommon.ControllerEntityType),
			watcher.WithOperationTypeFilter(dbCommon.UpdateOperation),
		),
		// Any operation on the entity we're managing the pool for.
		watcher.WithEntityFilter(entity),
		// Watch for changes to the github credentials
		watcher.WithForgeCredentialsFilter(entity.Credentials),
	)
}

func (r *basePoolManager) waitForToolsOrCancel() (hasTools, stopped bool) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	select {
	case <-ticker.C:
		if _, err := cache.GetGithubToolsCache(r.entity.ID); err != nil {
			return false, false
		}
		return true, false
	case <-r.quit:
		return false, true
	case <-r.ctx.Done():
		return false, true
	}
}
