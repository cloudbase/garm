package pool

import (
	"sync"
	"testing"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
)

func TestPoolRoundRobinRollsOver(t *testing.T) {
	p := &poolRoundRobin{
		pools: []params.Pool{
			{
				ID: "1",
			},
			{
				ID: "2",
			},
		},
	}

	pool, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if pool.ID != "1" {
		t.Fatalf("expected pool 1, got %s", pool.ID)
	}

	pool, err = p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if pool.ID != "2" {
		t.Fatalf("expected pool 2, got %s", pool.ID)
	}

	pool, err = p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if pool.ID != "1" {
		t.Fatalf("expected pool 1, got %s", pool.ID)
	}
}

func TestPoolRoundRobinEmptyPoolErrorsOut(t *testing.T) {
	p := &poolRoundRobin{}

	_, err := p.Next()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err != runnerErrors.ErrNoPoolsAvailable {
		t.Fatalf("expected ErrNoPoolsAvailable, got %s", err)
	}
}

func TestPoolRoundRobinLen(t *testing.T) {
	p := &poolRoundRobin{
		pools: []params.Pool{
			{
				ID: "1",
			},
			{
				ID: "2",
			},
		},
	}

	if p.Len() != 2 {
		t.Fatalf("expected 2, got %d", p.Len())
	}
}

func TestPoolRoundRobinReset(t *testing.T) {
	p := &poolRoundRobin{
		pools: []params.Pool{
			{
				ID: "1",
			},
			{
				ID: "2",
			},
		},
	}

	p.Next()
	p.Reset()
	if p.next != 0 {
		t.Fatalf("expected 0, got %d", p.next)
	}
}

func TestPoolsForTagsPackGet(t *testing.T) {
	p := &poolsForTags{
		poolCacheType: params.PoolBalancerTypePack,
	}

	pools := []params.Pool{
		{
			ID:       "1",
			Priority: 0,
		},
		{
			ID:       "2",
			Priority: 100,
		},
	}
	_ = p.Add([]string{"key"}, pools)
	cache, ok := p.Get([]string{"key"})
	if !ok {
		t.Fatalf("expected true, got false")
	}
	if cache.Len() != 2 {
		t.Fatalf("expected 2, got %d", cache.Len())
	}

	poolRR, ok := cache.(*poolRoundRobin)
	if !ok {
		t.Fatalf("expected poolRoundRobin, got %v", cache)
	}
	if poolRR.next != 0 {
		t.Fatalf("expected 0, got %d", poolRR.next)
	}
	pool, err := poolRR.Next()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if pool.ID != "2" {
		t.Fatalf("expected pool 2, got %s", pool.ID)
	}

	if poolRR.next != 1 {
		t.Fatalf("expected 1, got %d", poolRR.next)
	}
	// Getting the pool cache again should reset next
	cache, ok = p.Get([]string{"key"})
	if !ok {
		t.Fatalf("expected true, got false")
	}
	poolRR, ok = cache.(*poolRoundRobin)
	if !ok {
		t.Fatalf("expected poolRoundRobin, got %v", cache)
	}
	if poolRR.next != 0 {
		t.Fatalf("expected 0, got %d", poolRR.next)
	}
}

func TestPoolsForTagsRoundRobinGet(t *testing.T) {
	p := &poolsForTags{
		poolCacheType: params.PoolBalancerTypeRoundRobin,
	}

	pools := []params.Pool{
		{
			ID:       "1",
			Priority: 0,
		},
		{
			ID:       "2",
			Priority: 100,
		},
	}
	_ = p.Add([]string{"key"}, pools)
	cache, ok := p.Get([]string{"key"})
	if !ok {
		t.Fatalf("expected true, got false")
	}
	if cache.Len() != 2 {
		t.Fatalf("expected 2, got %d", cache.Len())
	}

	pool, err := cache.Next()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if pool.ID != "2" {
		t.Fatalf("expected pool 2, got %s", pool.ID)
	}
	// Getting the pool cache again should not reset next, and
	// should return the next pool.
	cache, ok = p.Get([]string{"key"})
	if !ok {
		t.Fatalf("expected true, got false")
	}
	pool, err = cache.Next()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if pool.ID != "1" {
		t.Fatalf("expected pool 1, got %s", pool.ID)
	}
}

func TestPoolsForTagsNoPoolsForTag(t *testing.T) {
	p := &poolsForTags{
		pools: sync.Map{},
	}

	_, ok := p.Get([]string{"key"})
	if ok {
		t.Fatalf("expected false, got true")
	}
}

func TestPoolsForTagsBalancerTypePack(t *testing.T) {
	p := &poolsForTags{
		pools:         sync.Map{},
		poolCacheType: params.PoolBalancerTypePack,
	}

	poolCache := &poolRoundRobin{}
	p.pools.Store("key", poolCache)

	cache, ok := p.Get([]string{"key"})
	if !ok {
		t.Fatalf("expected true, got false")
	}
	if cache != poolCache {
		t.Fatalf("expected poolCache, got %v", cache)
	}
	if poolCache.next != 0 {
		t.Fatalf("expected 0, got %d", poolCache.next)
	}
}
