//go:build testing
// +build testing

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

package sql

import (
	"context"
	"regexp"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
)

var fkClauseRe = regexp.MustCompile("CONSTRAINT `(fk_[^`]+)` FOREIGN KEY \\(`([^`]+)`\\) REFERENCES `([^`]+)`\\(`[^`]+`\\)((?: ON (?:DELETE|UPDATE) [A-Z][A-Z ]*[A-Z])*)")

type tableSchema struct {
	Columns     []string
	Indexes     []string
	Constraints []string
}

func dumpSchema(t *testing.T, conn *gorm.DB) map[string]tableSchema {
	t.Helper()
	var tables []string
	require.NoError(t, conn.Raw(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN ('migrations', 'file_object_migrations') ORDER BY name`).Scan(&tables).Error)

	out := map[string]tableSchema{}
	for _, table := range tables {
		var schema tableSchema
		require.NoError(t, conn.Raw(`SELECT name FROM pragma_table_info(?) ORDER BY name`, table).Scan(&schema.Columns).Error)
		require.NoError(t, conn.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND tbl_name = ? AND name NOT LIKE 'sqlite_autoindex%' ORDER BY name`, table).Scan(&schema.Indexes).Error)

		var ddl string
		require.NoError(t, conn.Raw(`SELECT sql FROM sqlite_master WHERE type='table' AND name = ?`, table).Scan(&ddl).Error)
		for _, m := range fkClauseRe.FindAllStringSubmatch(ddl, -1) {
			schema.Constraints = append(schema.Constraints, m[1]+"("+m[2]+" -> "+m[3]+")"+m[4])
		}
		sort.Strings(schema.Constraints)
		out[table] = schema
	}
	return out
}

// TestMigratedSchemaMatchesFresh asserts the core schema invariant: a
// database upgraded through the numbered migrations must end up with the
// same tables, columns, indexes and foreign key constraints as a freshly
// created one. Whenever a model gains a column or a constraint, the
// migration that ships it must bring existing databases along too, or this
// test fails.
func TestMigratedSchemaMatchesFresh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher.InitWatcher(ctx)
	cfg := garmTesting.GetTestSqliteDBConfig(t)
	store, err := NewSQLStore(ctx, cfg)
	require.NoError(t, err)
	fresh := dumpSchema(t, store.(*sqlDatabase).conn)
	freshObjects := dumpSchema(t, store.(*sqlDatabase).objectsConn)
	watcher.CloseWatcher()

	watcher.InitWatcher(ctx)
	defer watcher.CloseWatcher()
	cfg2 := garmTesting.GetTestSqliteDBConfig(t)
	blobCfg, err := cfg2.SQLiteBlobDatabaseConfig()
	require.NoError(t, err)
	execSQLFileOnFile(t, cfg2.SQLite.DBFile, "migrations/testdata/v021_schema.sql")
	seedV021Data(t, cfg2.SQLite.DBFile, cfg2.Passphrase)
	execSQLFileOnFile(t, blobCfg.SQLite.DBFile, "migrations/testdata/v021_blob_schema.sql")
	store2, err := NewSQLStore(ctx, cfg2)
	require.NoError(t, err)
	migrated := dumpSchema(t, store2.(*sqlDatabase).conn)
	migratedObjects := dumpSchema(t, store2.(*sqlDatabase).objectsConn)

	compareSchemas(t, fresh, migrated)
	compareSchemas(t, freshObjects, migratedObjects)
}

func compareSchemas(t *testing.T, fresh, migrated map[string]tableSchema) {
	t.Helper()
	require.Equal(t, tableNamesOf(fresh), tableNamesOf(migrated), "table sets differ")
	for table, freshSchema := range fresh {
		migratedSchema := migrated[table]
		require.Equal(t, freshSchema.Columns, migratedSchema.Columns, "columns differ in %s", table)
		require.Equal(t, freshSchema.Indexes, migratedSchema.Indexes, "indexes differ in %s", table)
		require.Equal(t, freshSchema.Constraints, migratedSchema.Constraints, "constraints differ in %s", table)
	}
}

func tableNamesOf(schemas map[string]tableSchema) []string {
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
