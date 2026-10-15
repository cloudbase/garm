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
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// The 0003 stubs declared the forge instance foreign keys without a column
// type. The real models derive it from the key they reference, but a stub
// has no relation to derive from, so PostgreSQL fell back to text. Those
// columns cannot be compared to or constrained against the uuid and
// varchar(64) keys they point at.
//
// The ID sorts between 0007 and 0008. 0008 compares these columns to the
// keys they reference, which errors out on a text column, so the conversion
// has to happen first.

var forgeInstanceColumns = []struct {
	table  string
	column string
	// reportedAs is the type as information_schema describes it, alterTo
	// the same type as an ALTER statement spells it.
	reportedAs string
	alterTo    string
}{
	{"forge_instance_events", "forge_instance_id", "uuid", "uuid"},
	{"pools", "forge_instance_id", "uuid", "uuid"},
	{"workflow_jobs", "forge_instance_id", "uuid", "uuid"},
	{"forge_instances", "endpoint_name", "character varying(64)", "varchar(64)"},
}

func init() {
	Register(&gormigrate.Migration{
		ID: "0007a_forge_instance_column_types",
		Migrate: func(tx *gorm.DB) error {
			// SQLite only acts on type affinity and converting a column
			// means rebuilding the table for no gain.
			if tx.Name() != "postgres" {
				return nil
			}

			for _, col := range forgeInstanceColumns {
				var current struct {
					DataType  string
					MaxLength int64
				}
				err := tx.Raw(
					"SELECT data_type, coalesce(character_maximum_length, 0) AS max_length FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?",
					col.table, col.column).Scan(&current).Error
				if err != nil {
					return fmt.Errorf("checking %s.%s: %w", col.table, col.column, err)
				}

				reported := current.DataType
				if current.MaxLength > 0 {
					reported = fmt.Sprintf("%s(%d)", reported, current.MaxLength)
				}
				// No such column on databases created from the full models
				// before 0003, and nothing to do if the type already matches.
				if reported == "" || reported == col.reportedAs {
					continue
				}

				// NULLIF covers rows where a zero value was stored as an
				// empty string, which does not cast to a uuid and does not
				// reference an endpoint either.
				stmt := fmt.Sprintf("ALTER TABLE %q ALTER COLUMN %q TYPE %s USING NULLIF(%q, '')::%s",
					col.table, col.column, col.alterTo, col.column, col.alterTo)
				if err := tx.Exec(stmt).Error; err != nil {
					return fmt.Errorf("converting %s.%s to %s: %w", col.table, col.column, col.alterTo, err)
				}
			}
			return nil
		},
	})
}
