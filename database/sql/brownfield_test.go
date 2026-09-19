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
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
)

// execSQLOnFile executes the given SQL statements against a SQLite database
// file, creating it if needed.
func execSQLOnFile(t *testing.T, dbFile, statements string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "opening %s", dbFile)
	require.NoError(t, db.Exec(statements).Error, "executing statements on %s", dbFile)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

func execSQLFileOnFile(t *testing.T, dbFile, sqlPath string) {
	t.Helper()
	contents, err := os.ReadFile(sqlPath)
	require.NoError(t, err, "reading %s", sqlPath)
	execSQLOnFile(t, dbFile, string(contents))
}

// seedV021Data populates a v0.2.1-schema database with the shape that made
// issue #890 blow up: live instances (and a job) referencing a scale set.
func seedV021Data(t *testing.T, dbFile string) {
	t.Helper()
	execSQLOnFile(t, dbFile, `
INSERT INTO github_endpoints (name, created_at, updated_at, endpoint_type, description, api_base_url, upload_base_url, base_url, tools_metadata_url, use_internal_tools_metadata)
	VALUES ('github.com', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'github', 'GitHub.com', 'https://api.github.com', 'https://uploads.github.com', 'https://github.com', '', 1);
INSERT INTO users (id, created_at, updated_at, username, full_name, email, password, generation, is_admin, enabled)
	VALUES ('11111111-1111-1111-1111-111111111111', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'admin', 'Admin', 'admin@example.com', 'not-a-real-hash', 1, 1, 1);
INSERT INTO repositories (id, created_at, updated_at, owner, name, webhook_secret, pool_balancer_type, agent_mode, endpoint_name, pool_manager_running, pool_manager_failure_reason)
	VALUES ('22222222-2222-2222-2222-222222222222', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'gsamfira', 'garm-testing', X'00', 'roundrobin', 0, 'github.com', 0, '');
INSERT INTO pools (id, created_at, updated_at, provider_name, runner_prefix, max_runners, min_idle_runners, runner_bootstrap_timeout, image, flavor, os_type, os_arch, enabled, git_hub_runner_group, enable_shell, generation, repo_id, priority)
	VALUES ('33333333-3333-3333-3333-333333333333', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'test-provider', 'garm', 4, 1, 20, 'ubuntu:24.04', 'default', 'linux', 'amd64', 1, '', 0, 1, '22222222-2222-2222-2222-222222222222', 0);
INSERT INTO scale_sets (id, created_at, updated_at, scale_set_id, name, git_hub_runner_group, disable_update, state, extended_state, provider_name, runner_prefix, max_runners, min_idle_runners, runner_bootstrap_timeout, image, flavor, os_type, os_arch, enabled, last_message_id, desired_runner_count, enable_shell, generation, repo_id)
	VALUES (1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 10, 'ubuntu-noble', 'Default', 0, 'active', '', 'test-provider', 'garm', 4, 0, 20, 'ubuntu:24.04', 'default', 'linux', 'amd64', 1, 0, 1, 0, 1, '22222222-2222-2222-2222-222222222222');
INSERT INTO instances (id, created_at, updated_at, provider_id, name, agent_id, os_type, os_arch, status, runner_status, create_attempt, token_fetched, generation, pool_id)
	VALUES ('44444444-4444-4444-4444-444444444444', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'prov-1', 'garm-pool-runner', 1, 'linux', 'amd64', 'running', 'idle', 1, 1, 1, '33333333-3333-3333-3333-333333333333');
INSERT INTO instances (id, created_at, updated_at, provider_id, name, agent_id, os_type, os_arch, status, runner_status, create_attempt, token_fetched, generation, scale_set_fk_id)
	VALUES ('55555555-5555-5555-5555-555555555555', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'prov-2', 'garm-scaleset-runner', 2, 'linux', 'amd64', 'running', 'active', 1, 1, 1, 1);
INSERT INTO workflow_jobs (id, workflow_job_id, scale_set_job_id, run_id, action, conclusion, status, name, github_runner_id, instance_id, runner_group_id, runner_group_name, repository_name, repository_owner, labels, workflow_run_url, repo_id, locked_by, created_at, updated_at)
	VALUES (1, 0, 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee', 100, 'in_progress', '', 'in_progress', 'test job', 2, '55555555-5555-5555-5555-555555555555', 1, 'Default', 'garm-testing', 'gsamfira', '["ubuntu-noble"]', '', '22222222-2222-2222-2222-222222222222', '00000000-0000-0000-0000-000000000000', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
`)
}

func migrationIDs(t *testing.T, conn *gorm.DB, table string) []string {
	t.Helper()
	var ids []string
	require.NoError(t, conn.Raw("SELECT id FROM "+table).Scan(&ids).Error)
	return ids
}

func hasColumn(t *testing.T, conn *gorm.DB, table, column string) bool {
	t.Helper()
	var count int64
	require.NoError(t, conn.Raw("SELECT count(*) FROM pragma_table_info(?) WHERE name = ?", table, column).Scan(&count).Error)
	return count == 1
}

func countRows(t *testing.T, conn *gorm.DB, table string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, conn.Raw("SELECT count(*) FROM "+table).Scan(&count).Error)
	return count
}

func requireNoFKViolations(t *testing.T, conn *gorm.DB) {
	t.Helper()
	var violations []map[string]interface{}
	require.NoError(t, conn.Raw("PRAGMA foreign_key_check").Scan(&violations).Error)
	require.Empty(t, violations, "foreign key violations after migration")
}

// TestBrownfieldMigration upgrades a populated v0.2.1 database (created by a
// release that predates gormigrate) to the current schema. This is the exact
// scenario from issue #890.
func TestBrownfieldMigration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher.InitWatcher(ctx)
	defer watcher.CloseWatcher()

	cfg := garmTesting.GetTestSqliteDBConfig(t)
	blobCfg, err := cfg.SQLiteBlobDatabaseConfig()
	require.NoError(t, err)

	execSQLFileOnFile(t, cfg.SQLite.DBFile, "migrations/testdata/v021_schema.sql")
	seedV021Data(t, cfg.SQLite.DBFile)
	execSQLFileOnFile(t, blobCfg.SQLite.DBFile, "migrations/testdata/v021_blob_schema.sql")

	store, err := NewSQLStore(ctx, cfg)
	require.NoError(t, err, "brownfield migration failed")
	db := store.(*sqlDatabase)

	// The numbered migrations ran; InitSchema did not.
	ids := migrationIDs(t, db.conn, "migrations")
	require.Contains(t, ids, "0001_baseline")
	require.Contains(t, ids, "0006_proxies")
	require.Contains(t, ids, "0007_credentials_reserve_usage")
	require.NotContains(t, ids, "SCHEMA_INIT")

	// Post-baseline schema is present.
	require.True(t, db.conn.Migrator().HasTable("proxies"))
	require.True(t, db.conn.Migrator().HasTable("forge_instances"))
	require.True(t, hasColumn(t, db.conn, "scale_sets", "proxy_id"))
	require.True(t, hasColumn(t, db.conn, "github_credentials", "reserve_usage_enabled"))

	// Seeded data survived.
	require.EqualValues(t, 1, countRows(t, db.conn, "users"))
	require.EqualValues(t, 1, countRows(t, db.conn, "repositories"))
	require.EqualValues(t, 1, countRows(t, db.conn, "pools"))
	require.EqualValues(t, 1, countRows(t, db.conn, "scale_sets"))
	require.EqualValues(t, 2, countRows(t, db.conn, "instances"))
	require.EqualValues(t, 1, countRows(t, db.conn, "workflow_jobs"))

	requireNoFKViolations(t, db.conn)
	requireNoFKViolations(t, db.objectsConn)

	objectIDs := migrationIDs(t, db.objectsConn, "file_object_migrations")
	require.Contains(t, objectIDs, "0001_baseline")
	require.NotContains(t, objectIDs, "SCHEMA_INIT")
}

// TestFreshDatabaseUsesInitSchema ensures new installs keep taking the
// single-step InitSchema path.
func TestFreshDatabaseUsesInitSchema(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher.InitWatcher(ctx)
	defer watcher.CloseWatcher()

	cfg := garmTesting.GetTestSqliteDBConfig(t)
	store, err := NewSQLStore(ctx, cfg)
	require.NoError(t, err)
	db := store.(*sqlDatabase)

	ids := migrationIDs(t, db.conn, "migrations")
	require.Contains(t, ids, "SCHEMA_INIT")
	require.True(t, db.conn.Migrator().HasTable("proxies"))
	require.True(t, hasColumn(t, db.conn, "github_credentials", "reserve_usage_enabled"))
}

// TestGormigrateEraMigration ensures databases that already track migrations
// only run the pending ones.
func TestGormigrateEraMigration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher.InitWatcher(ctx)
	defer watcher.CloseWatcher()

	cfg := garmTesting.GetTestSqliteDBConfig(t)
	blobCfg, err := cfg.SQLiteBlobDatabaseConfig()
	require.NoError(t, err)

	execSQLFileOnFile(t, cfg.SQLite.DBFile, "migrations/testdata/v021_schema.sql")
	seedV021Data(t, cfg.SQLite.DBFile)
	execSQLOnFile(t, cfg.SQLite.DBFile, `
CREATE TABLE migrations (id VARCHAR(255) PRIMARY KEY);
INSERT INTO migrations (id) VALUES ('0001_baseline');
`)
	execSQLFileOnFile(t, blobCfg.SQLite.DBFile, "migrations/testdata/v021_blob_schema.sql")
	execSQLOnFile(t, blobCfg.SQLite.DBFile, `
CREATE TABLE file_object_migrations (id VARCHAR(255) PRIMARY KEY);
INSERT INTO file_object_migrations (id) VALUES ('0001_baseline');
`)

	store, err := NewSQLStore(ctx, cfg)
	require.NoError(t, err)
	db := store.(*sqlDatabase)

	ids := migrationIDs(t, db.conn, "migrations")
	require.Contains(t, ids, "0002_lower_indexes")
	require.Contains(t, ids, "0007_credentials_reserve_usage")
	require.NotContains(t, ids, "SCHEMA_INIT")

	require.True(t, db.conn.Migrator().HasTable("forge_instances"))
	require.EqualValues(t, 2, countRows(t, db.conn, "instances"))
	requireNoFKViolations(t, db.conn)
}
