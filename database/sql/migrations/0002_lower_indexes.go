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
	// Convert the case-insensitive indexes from column-level COLLATE NOCASE to
	// functional LOWER(col) indexes, matching what the models now declare. Fresh
	// installs build these via InitSchema/AutoMigrate, so this migration only runs
	// on existing SQLite databases.
	//
	// The queries now issue LOWER(col) = LOWER(?), which the old NOCASE/bare-column
	// indexes can't serve, so lookups would fall back to full scans. Recreating the
	// unique indexes is safe on existing data: NOCASE and LOWER() both fold ASCII,
	// so rows unique under the old index stay unique under the new one.
	//
	// github_endpoints is the exception: its case-insensitivity lives on the
	// PRIMARY KEY's NOCASE collation, which SQLite can't ALTER without a table
	// rebuild, so we leave the PK and only add the functional unique index.
	Register(&gormigrate.Migration{
		ID: "0002_lower_indexes",
		Migrate: func(tx *gorm.DB) error {
			stmts := []string{
				"DROP INDEX IF EXISTS idx_owner_nocase",
				"CREATE UNIQUE INDEX idx_owner_nocase ON repositories(LOWER(owner),LOWER(name),LOWER(endpoint_name))",

				"DROP INDEX IF EXISTS idx_org_name_nocase",
				"CREATE INDEX idx_org_name_nocase ON organizations(LOWER(name),LOWER(endpoint_name))",

				"DROP INDEX IF EXISTS idx_ent_name_nocase",
				"CREATE INDEX idx_ent_name_nocase ON enterprises(LOWER(name),LOWER(endpoint_name))",

				"DROP INDEX IF EXISTS idx_github_credentials",
				"CREATE UNIQUE INDEX idx_github_credentials ON github_credentials(LOWER(name),user_id)",

				"DROP INDEX IF EXISTS idx_gitea_credentials",
				"CREATE UNIQUE INDEX idx_gitea_credentials ON gitea_credentials(LOWER(name),user_id)",

				// PK stays NOCASE; just add the functional unique index.
				"DROP INDEX IF EXISTS idx_endpoint_name_nocase",
				"CREATE UNIQUE INDEX idx_endpoint_name_nocase ON github_endpoints(LOWER(name))",
			}
			for _, q := range stmts {
				if err := tx.Exec(q).Error; err != nil {
					return err
				}
			}
			return nil
		},
	})
}
