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
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func init() {
	// Allow pool and scale set labels longer than 64 characters.
	Register(&gormigrate.Migration{
		ID: "0009_tag_name_length",
		Migrate: func(tx *gorm.DB) error {
			// SQLite does not enforce varchar lengths.
			if tx.Name() == "sqlite" {
				return nil
			}
			return tx.Exec("ALTER TABLE tags ALTER COLUMN name TYPE varchar(255)").Error
		},
	})
}
