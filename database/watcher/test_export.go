//go:build testing
// +build testing

package watcher

import "github.com/cloudbase/garm/database/common"

// SetWatcher sets the watcher to be used by the database package.
// This function is intended for use in tests only.
func SetWatcher(w common.Watcher) {
	databaseWatcher = w
}
