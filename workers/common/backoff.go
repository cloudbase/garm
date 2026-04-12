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

package common

import (
	"crypto/rand"
	"math/big"
	"sync"
	"time"
)

// BackoffConfig configures the exponential backoff parameters.
type BackoffConfig struct {
	// Initial is the initial backoff duration after the first failure.
	Initial time.Duration
	// Multiplier is the geometric progression factor applied on each failure.
	Multiplier float64
	// Max is the maximum backoff duration.
	Max time.Duration
	// JitterFrac adds randomization to prevent thundering herd. A value of 0.2
	// means the actual delay is within ±20% of the computed backoff.
	JitterFrac float64
}

// DefaultBackoffConfig returns a BackoffConfig with sensible defaults:
// 10s initial, 1.5x multiplier, 5m max, 20% jitter.
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		Initial:    10 * time.Second,
		Multiplier: 1.5,
		Max:        5 * time.Minute,
		JitterFrac: 0.2,
	}
}

type backoffEntry struct {
	backoff             time.Duration
	lastRecordedFailure time.Time
	mux                 sync.Mutex
}

// Backoff tracks per-key exponential backoff state. It is safe for concurrent use.
type Backoff struct {
	config BackoffConfig
	// map[string]*backoffEntry
	entries sync.Map
}

// NewBackoff creates a new Backoff tracker with the given configuration.
func NewBackoff(config BackoffConfig) *Backoff {
	return &Backoff{
		config: config,
	}
}

// ShouldRetry returns true if the key has no recorded failure or if the
// backoff deadline has passed.
func (b *Backoff) ShouldRetry(key string) bool {
	val, ok := b.entries.Load(key)
	if !ok {
		return true
	}

	entry := val.(*backoffEntry)
	entry.mux.Lock()
	defer entry.mux.Unlock()

	if entry.lastRecordedFailure.IsZero() || entry.backoff == 0 {
		return true
	}

	deadline := entry.lastRecordedFailure.Add(entry.backoff)
	return time.Now().UTC().After(deadline)
}

// RecordFailure records a failure for the given key, setting or increasing
// the backoff duration with jitter.
func (b *Backoff) RecordFailure(key string) {
	val, _ := b.entries.LoadOrStore(key, &backoffEntry{})
	entry := val.(*backoffEntry)
	entry.mux.Lock()
	defer entry.mux.Unlock()

	entry.lastRecordedFailure = time.Now().UTC()
	if entry.backoff == 0 {
		entry.backoff = b.config.Initial
	} else {
		entry.backoff = time.Duration(float64(entry.backoff) * b.config.Multiplier)
	}
	if entry.backoff > b.config.Max {
		entry.backoff = b.config.Max
	}

	// Apply jitter
	if b.config.JitterFrac > 0 {
		const precision = 1 << 16
		n, err := rand.Int(rand.Reader, big.NewInt(precision))
		if err == nil {
			rnd := float64(n.Int64()) / precision
			jitter := 1.0 + (rnd*2-1)*b.config.JitterFrac
			entry.backoff = min(time.Duration(float64(entry.backoff)*jitter), b.config.Max)
		}
	}
}

// RecordSuccess clears the backoff state for the given key.
func (b *Backoff) RecordSuccess(key string) {
	b.entries.Delete(key)
}
