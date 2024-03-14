package pool

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
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
	if p.poolCacheType == params.PoolBalancerTypeStack {
		// When we service a list of jobs, we want to try each pool in turn
		// for each job. Pools are sorted by priority so we always start from the
		// highest priority pool and move on to the next if the first one is full.
		poolCache.Reset()
	}
	return poolCache, true
}

func (p *poolsForTags) Add(tags []string, pools []params.Pool) *poolRoundRobin {
	sort.Slice(pools, func(i, j int) bool {
		return pools[i].Priority > pools[j].Priority
	})

	sort.Strings(tags)
	key := strings.Join(tags, "^")

	poolRR := &poolRoundRobin{pools: pools}
	v, _ := p.pools.LoadOrStore(key, poolRR)
	return v.(*poolRoundRobin)
}
