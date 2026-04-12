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
	"context"
	"time"
)

// RetryLoop runs fn immediately and then on a fixed ticker until ctx is
// cancelled or quit is closed. It is intended to be run as a goroutine.
func RetryLoop(ctx context.Context, quit <-chan struct{}, interval time.Duration, fn func()) {
	fn()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fn()
		case <-ctx.Done():
			return
		case <-quit:
			return
		}
	}
}
