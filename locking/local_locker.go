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

package locking

import (
	"context"
	"sync"

	dbCommon "github.com/cloudbase/garm/database/common"
)

const (
	maxBackoffSeconds float64 = 1200 // 20 minutes
)

func NewLocalLocker(_ context.Context, _ dbCommon.Store) (Locker, error) {
	return &keyMutex{}, nil
}

type keyMutex struct {
	muxes sync.Map
}

type lockWithIdent struct {
	mux   sync.Mutex
	ident string
}

var _ Locker = &keyMutex{}

func (k *keyMutex) TryLock(key, identifier string) bool {
	mux, _ := k.muxes.LoadOrStore(key, &lockWithIdent{
		mux: sync.Mutex{},
	})
	keyMux := mux.(*lockWithIdent)
	locked := keyMux.mux.TryLock()
	if locked {
		keyMux.ident = identifier
	}
	return locked
}

func (k *keyMutex) Lock(key, identifier string) {
	mux, _ := k.muxes.LoadOrStore(key, &lockWithIdent{
		mux: sync.Mutex{},
	})
	keyMux := mux.(*lockWithIdent)
	keyMux.ident = identifier
	keyMux.mux.Lock()
}

func (k *keyMutex) Unlock(key string, remove bool) {
	mux, ok := k.muxes.Load(key)
	if !ok {
		return
	}
	keyMux := mux.(*lockWithIdent)
	if remove {
		k.Delete(key)
	}
	keyMux.ident = ""
	keyMux.mux.Unlock()
}

func (k *keyMutex) Delete(key string) {
	k.muxes.Delete(key)
}

func (k *keyMutex) LockedBy(key string) (string, bool) {
	mux, ok := k.muxes.Load(key)
	if !ok {
		return "", false
	}
	keyMux := mux.(*lockWithIdent)
	if keyMux.ident == "" {
		return "", false
	}

	return keyMux.ident, true
}
