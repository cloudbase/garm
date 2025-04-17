package locking

import (
	"context"
	"sync"
	"time"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/runner/common"
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

var _ Locker = &keyMutex{}

func (k *keyMutex) TryLock(key string) bool {
	mux, _ := k.muxes.LoadOrStore(key, &sync.Mutex{})
	keyMux := mux.(*sync.Mutex)
	return keyMux.TryLock()
}

func (k *keyMutex) Lock(key string) {
	mux, _ := k.muxes.LoadOrStore(key, &sync.Mutex{})
	keyMux := mux.(*sync.Mutex)
	keyMux.Lock()
}

func (k *keyMutex) Unlock(key string, remove bool) {
	mux, ok := k.muxes.Load(key)
	if !ok {
		return
	}
	keyMux := mux.(*sync.Mutex)
	if remove {
		k.Delete(key)
	}
	keyMux.Unlock()
}

func (k *keyMutex) Delete(key string) {
	k.muxes.Delete(key)
}

func NewInstanceDeleteBackoff(_ context.Context) (InstanceDeleteBackoff, error) {
	return &instanceDeleteBackoff{}, nil
}

type instanceBackOff struct {
	backoffSeconds          float64
	lastRecordedFailureTime time.Time
	mux                     sync.Mutex
}

type instanceDeleteBackoff struct {
	muxes sync.Map
}

func (i *instanceDeleteBackoff) ShouldProcess(key string) (bool, time.Time) {
	backoff, loaded := i.muxes.LoadOrStore(key, &instanceBackOff{})
	if !loaded {
		return true, time.Time{}
	}

	ib := backoff.(*instanceBackOff)
	ib.mux.Lock()
	defer ib.mux.Unlock()

	if ib.lastRecordedFailureTime.IsZero() || ib.backoffSeconds == 0 {
		return true, time.Time{}
	}

	now := time.Now().UTC()
	deadline := ib.lastRecordedFailureTime.Add(time.Duration(ib.backoffSeconds) * time.Second)
	return deadline.After(now), deadline
}

func (i *instanceDeleteBackoff) Delete(key string) {
	i.muxes.Delete(key)
}

func (i *instanceDeleteBackoff) RecordFailure(key string) {
	backoff, _ := i.muxes.LoadOrStore(key, &instanceBackOff{})
	ib := backoff.(*instanceBackOff)
	ib.mux.Lock()
	defer ib.mux.Unlock()

	ib.lastRecordedFailureTime = time.Now().UTC()
	if ib.backoffSeconds == 0 {
		ib.backoffSeconds = common.PoolConsilitationInterval.Seconds()
	} else {
		// Geometric progression of 1.5
		newBackoff := ib.backoffSeconds * 1.5
		// Cap the backoff to 20 minutes
		ib.backoffSeconds = min(newBackoff, maxBackoffSeconds)
	}
}
