// Copyright 2026 Cloudbase Solutions SRL
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

package migrations

import (
	"sort"

	"github.com/go-gormigrate/gormigrate/v2"
)

var (
	registry            []*gormigrate.Migration
	fileObjectsRegistry []*gormigrate.Migration
)

// Register adds a migration to the main database registry. Each migration file
// should call this in an init() function.
func Register(m *gormigrate.Migration) {
	registry = append(registry, m)
}

// RegisterFileObjects adds a migration to the file objects database registry.
func RegisterFileObjects(m *gormigrate.Migration) {
	fileObjectsRegistry = append(fileObjectsRegistry, m)
}

func sorted(migrations []*gormigrate.Migration) []*gormigrate.Migration {
	result := make([]*gormigrate.Migration, len(migrations))
	copy(result, migrations)
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// All returns all registered main database migrations sorted alphanumerically by ID.
func All() []*gormigrate.Migration {
	return sorted(registry)
}

// AllFileObjects returns all registered file objects migrations sorted alphanumerically by ID.
func AllFileObjects() []*gormigrate.Migration {
	return sorted(fileObjectsRegistry)
}
