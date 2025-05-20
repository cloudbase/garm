//go:build testing
// +build testing

// Copyright 2025 Cloudbase Solutions SRL
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
package watcher

import "github.com/cloudbase/garm/database/common"

// SetWatcher sets the watcher to be used by the database package.
// This function is intended for use in tests only.
func SetWatcher(w common.Watcher) {
	databaseWatcher = w
}

// GetWatcher returns the current watcher.
func GetWatcher() common.Watcher {
	return databaseWatcher
}
