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
	// Replace the plain (file_object_id, tag) index with a functional
	// (file_object_id, LOWER(tag)) index so that LOWER(tag) = LOWER(?)
	// predicates can use the index on both SQLite and PostgreSQL.
	// The previous index used TEXT COLLATE NOCASE at the column level
	// (SQLite-only), which is not valid on PostgreSQL.
	//
	// Also adds lo_oid to file_blobs for PostgreSQL Large Object storage.
	// A value of 0 means the row uses the SQLite Content column instead.
	RegisterFileObjects(&gormigrate.Migration{
		ID: "0002_fileobject_postgres_compat",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Exec("DROP INDEX IF EXISTS idx_fileobject_tags_tag").Error; err != nil {
				return err
			}
			if err := tx.Exec("CREATE INDEX idx_fileobject_tags_tag ON file_object_tags(file_object_id, LOWER(tag))").Error; err != nil {
				return err
			}
			return tx.Exec("ALTER TABLE file_blobs ADD COLUMN lo_oid bigint NOT NULL DEFAULT 0").Error
		},
	})
}
