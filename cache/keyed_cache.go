// Copyright 2026 Cloudbase Solutions SRL
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
)

// keyedCache is the common core of the per-entity caches: a mutex guarded
// map with copy-out reads. Entity specific behavior (sort order, filtered
// lists, cross cache updates) lives in the per-entity wrappers, either as
// predicates passed to List or as compound operations run under the lock
// via Update.
type keyedCache[K comparable, T any] struct {
	mux sync.Mutex

	cache    map[K]T
	sizeHint int
}

func newKeyedCache[K comparable, T any](sizeHint int) *keyedCache[K, T] {
	return &keyedCache[K, T]{
		cache:    make(map[K]T, sizeHint),
		sizeHint: sizeHint,
	}
}

func (c *keyedCache[K, T]) Set(key K, value T) {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.cache[key] = value
}

func (c *keyedCache[K, T]) Get(key K) (T, bool) {
	c.mux.Lock()
	defer c.mux.Unlock()

	if value, ok := c.cache[key]; ok {
		return value, true
	}
	var zero T
	return zero, false
}

func (c *keyedCache[K, T]) Delete(key K) {
	c.mux.Lock()
	defer c.mux.Unlock()

	delete(c.cache, key)
}

// List returns the values that match all supplied filters. With no filters,
// all values are returned. Order is unspecified; callers sort as needed.
func (c *keyedCache[K, T]) List(filters ...func(T) bool) []T {
	c.mux.Lock()
	defer c.mux.Unlock()

	ret := make([]T, 0, len(c.cache))
values:
	for _, value := range c.cache {
		for _, filter := range filters {
			if !filter(value) {
				continue values
			}
		}
		ret = append(ret, value)
	}
	return ret
}

func (c *keyedCache[K, T]) AsMap() map[K]T {
	c.mux.Lock()
	defer c.mux.Unlock()

	ret := make(map[K]T, len(c.cache))
	for key, value := range c.cache {
		ret[key] = value
	}
	return ret
}

// Update runs a compound operation against the raw map while holding the
// cache lock. The callback must not call back into this cache.
func (c *keyedCache[K, T]) Update(fn func(map[K]T)) {
	c.mux.Lock()
	defer c.mux.Unlock()

	fn(c.cache)
}

func (c *keyedCache[K, T]) Clear() {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.cache = make(map[K]T, c.sizeHint)
}
