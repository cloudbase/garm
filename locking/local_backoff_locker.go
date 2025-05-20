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
	"time"

	"github.com/cloudbase/garm/runner/common"
)

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
	return now.After(deadline), deadline
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
