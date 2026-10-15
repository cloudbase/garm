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
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cloudbase/garm/database/sql/migrations"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
)

var fkClauseRe = regexp.MustCompile("CONSTRAINT `(fk_[^`]+)` FOREIGN KEY \\(`([^`]+)`\\) REFERENCES `([^`]+)`\\(`[^`]+`\\)((?: ON (?:DELETE|UPDATE) [A-Z][A-Z ]*[A-Z])*)")

type tableSchema struct {
	Columns     []string
	Indexes     []string
	Constraints []string
}

// dumpSchema describes every table: its columns and their types, its index
// names and its foreign keys. Both arms of a parity test run on the same
// backend, so types are compared as that backend reports them.
func dumpSchema(t *testing.T, conn *gorm.DB) map[string]tableSchema {
	t.Helper()
	if conn.Name() == "postgres" {
		return dumpSchemaPostgres(t, conn)
	}
	return dumpSchemaSQLite(t, conn)
}

// sqliteAffinity reduces a declared type to the affinity SQLite derives from
// it. SQLite stores the declared type verbatim but only acts on the affinity,
// so varchar(64) and varchar(255) are the same column to it.
func sqliteAffinity(declared string) string {
	decl := strings.ToUpper(declared)
	switch {
	case strings.Contains(decl, "INT"):
		return "INTEGER"
	case strings.Contains(decl, "CHAR"), strings.Contains(decl, "CLOB"), strings.Contains(decl, "TEXT"):
		return "TEXT"
	case decl == "", strings.Contains(decl, "BLOB"):
		return "BLOB"
	//nolint:misspell // DOUB is the prefix SQLite's affinity rules match on
	case strings.Contains(decl, "REAL"), strings.Contains(decl, "FLOA"), strings.Contains(decl, "DOUB"):
		return "REAL"
	default:
		return "NUMERIC"
	}
}

func dumpSchemaSQLite(t *testing.T, conn *gorm.DB) map[string]tableSchema {
	t.Helper()
	var tables []string
	require.NoError(t, conn.Raw(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN ('migrations', 'file_object_migrations') ORDER BY name`).Scan(&tables).Error)

	out := map[string]tableSchema{}
	for _, table := range tables {
		var schema tableSchema
		var columns []struct {
			Name string
			Type string
		}
		require.NoError(t, conn.Raw(`SELECT name, type FROM pragma_table_info(?) ORDER BY name`, table).Scan(&columns).Error)
		for _, column := range columns {
			schema.Columns = append(schema.Columns, column.Name+" "+sqliteAffinity(column.Type))
		}
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

func dumpSchemaPostgres(t *testing.T, conn *gorm.DB) map[string]tableSchema {
	t.Helper()
	var tables []string
	require.NoError(t, conn.Raw(`SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() AND table_type = 'BASE TABLE' AND table_name NOT IN ('migrations', 'file_object_migrations') ORDER BY table_name`).Scan(&tables).Error)

	out := map[string]tableSchema{}
	for _, table := range tables {
		var schema tableSchema
		var columns []struct {
			ColumnName string
			DataType   string
			MaxLength  int64
		}
		require.NoError(t, conn.Raw(`SELECT column_name, data_type, coalesce(character_maximum_length, 0) AS max_length FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? ORDER BY column_name`, table).Scan(&columns).Error)
		for _, column := range columns {
			dataType := column.DataType
			if column.MaxLength > 0 {
				dataType = fmt.Sprintf("%s(%d)", dataType, column.MaxLength)
			}
			schema.Columns = append(schema.Columns, column.ColumnName+" "+dataType)
		}
		require.NoError(t, conn.Raw(`SELECT indexname FROM pg_indexes WHERE schemaname = current_schema() AND tablename = ? ORDER BY indexname`, table).Scan(&schema.Indexes).Error)

		var fks []struct {
			ConstraintName string
			ColumnName     string
			ForeignTable   string
			DeleteRule     string
			UpdateRule     string
		}
		require.NoError(t, conn.Raw(`
			SELECT tc.constraint_name, kcu.column_name, ccu.table_name AS foreign_table, rc.delete_rule, rc.update_rule
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu
				ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
			JOIN information_schema.constraint_column_usage ccu
				ON tc.constraint_name = ccu.constraint_name AND tc.table_schema = ccu.table_schema
			JOIN information_schema.referential_constraints rc
				ON tc.constraint_name = rc.constraint_name AND tc.constraint_schema = rc.constraint_schema
			WHERE tc.table_schema = current_schema() AND tc.table_name = ? AND tc.constraint_type = 'FOREIGN KEY'`, table).Scan(&fks).Error)
		for _, fk := range fks {
			entry := fk.ConstraintName + "(" + fk.ColumnName + " -> " + fk.ForeignTable + ")"
			if fk.DeleteRule != "NO ACTION" {
				entry += " ON DELETE " + fk.DeleteRule
			}
			if fk.UpdateRule != "NO ACTION" {
				entry += " ON UPDATE " + fk.UpdateRule
			}
			schema.Constraints = append(schema.Constraints, entry)
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

// requireMigrationsReapplyCleanly re-runs every numbered migration against a
// database that already carries the current schema. Migrations run without a
// transaction, so an interrupted upgrade leaves some of them to run again,
// and a stub that declares a column differently than the full models do
// would rewrite that column. Neither may change the schema.
func requireMigrationsReapplyCleanly(t *testing.T, db *sqlDatabase) {
	t.Helper()
	before := dumpSchema(t, db.conn)

	require.NoError(t, db.conn.Exec("DELETE FROM migrations").Error)
	require.NoError(t, db.migrateDB(), "re-running the migrations failed")
	require.Len(t, migrationIDs(t, db.conn, "migrations"), len(migrations.All()))

	compareSchemas(t, before, dumpSchema(t, db.conn))
}

func TestMigrationsReapplyCleanly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher.InitWatcher(ctx)
	defer watcher.CloseWatcher()

	store, err := NewSQLStore(ctx, garmTesting.GetTestSqliteDBConfig(t))
	require.NoError(t, err)
	requireMigrationsReapplyCleanly(t, store.(*sqlDatabase))
}

// TestMigrationsReapplyCleanlyOnPostgres is the PostgreSQL arm of the parity
// invariant. SQLite only acts on type affinity, so this is the arm that sees
// a stub declaring a column the backend types differently than the models do.
// PostgreSQL support postdates gormigrate, so no PostgreSQL database takes
// the v0.2.1 brownfield path the SQLite arm covers. What those databases do
// run is the migrations that landed after they were created, over a schema
// the full models built.
func TestMigrationsReapplyCleanlyOnPostgres(t *testing.T) {
	if os.Getenv("GARM_TEST_POSTGRES_DSN") == "" {
		t.Skip("GARM_TEST_POSTGRES_DSN not set")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher.InitWatcher(ctx)
	defer watcher.CloseWatcher()

	sqlDB, cfg := garmTesting.OpenTestPostgresDB(t)
	store, err := newSQLStoreFromSQLDB(ctx, sqlDB, cfg)
	require.NoError(t, err)
	db := store.(*sqlDatabase)
	t.Cleanup(func() { db.sqlDB.Close() })

	requireMigrationsReapplyCleanly(t, db)
}
