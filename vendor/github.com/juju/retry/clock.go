// Copyright 2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package retry

import "time"

// Clock provides an interface for dealing with clocks.
type Clock interface {
	// Now returns the current clock time.
	Now() time.Time

	// After waits for the duration to elapse and then sends the
	// current time on the returned channel.
	After(time.Duration) <-chan time.Time
}
