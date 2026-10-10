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
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// sqliteTableRebuild wraps a schema change that makes GORM rebuild one of
// the given tables. SQLite cannot alter constraints or drop columns in
// place, so GORM creates a copy of the table, moves the data over, drops
// the old table and renames the copy. With foreign keys enforced, dropping
// a table that has child rows fails, so the change runs inside SQLite's
// documented ALTER procedure. The rebuild also drops the table's standalone
// indexes without recreating them, so their DDL is captured first and any
// that went missing are restored afterwards. Callers removing a column must
// drop its indexes before calling this, or the restore recreates an index
// on a column that no longer exists. Safe outside a transaction with the
// single pooled connection.
func sqliteTableRebuild(tx *gorm.DB, tables []string, change func() error) error {
	var indexes []struct {
		Name string
		SQL  string
	}
	err := tx.Raw(`SELECT name, sql FROM sqlite_master WHERE type='index' AND sql IS NOT NULL AND tbl_name IN ?`, tables).Scan(&indexes).Error
	if err != nil {
		return fmt.Errorf("capturing index definitions: %w", err)
	}

	if err := tx.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
		return fmt.Errorf("disabling foreign keys: %w", err)
	}
	changeErr := change()
	if err := tx.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return errors.Join(changeErr, fmt.Errorf("re-enabling foreign keys: %w", err))
	}
	if changeErr != nil {
		return changeErr
	}

	for _, index := range indexes {
		var count int64
		if err := tx.Raw("SELECT count(*) FROM sqlite_master WHERE type='index' AND name = ?", index.Name).Scan(&count).Error; err != nil {
			return fmt.Errorf("checking index %s: %w", index.Name, err)
		}
		if count == 0 {
			if err := tx.Exec(index.SQL).Error; err != nil {
				return fmt.Errorf("restoring index %s: %w", index.Name, err)
			}
		}
	}
	return nil
}
