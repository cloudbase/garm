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
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	commonUtil "github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

const (
	seedUserID         = "11111111-1111-1111-1111-111111111111"
	seedRepoID         = "22222222-2222-2222-2222-222222222222"
	seedOrgID          = "23232323-2323-2323-2323-232323232323"
	seedEnterpriseID   = "24242424-2424-2424-2424-242424242424"
	seedRepoPoolID     = "33333333-3333-3333-3333-333333333333"
	seedOrgPoolID      = "34343434-3434-3434-3434-343434343434"
	seedEntPoolID      = "35353535-3535-3535-3535-353535353535"
	seedTagID          = "36363636-3636-3636-3636-363636363636"
	seedPoolRunnerID   = "44444444-4444-4444-4444-444444444444"
	seedOrgRunnerID    = "45454545-4545-4545-4545-454545454545"
	seedSSRunnerID     = "55555555-5555-5555-5555-555555555555"
	seedSS2RunnerID    = "56565656-5656-5656-5656-565656565656"
	seedAddressID      = "57575757-5757-5757-5757-575757575757"
	seedStatusUpdateID = "58585858-5858-5858-5858-585858585858"
	seedControllerID   = "59595959-5959-5959-5959-595959595959"
)

// execSQLOnFile executes the given SQL statements against a SQLite database
// file, creating it if needed.
func execSQLOnFile(t *testing.T, dbFile, statements string, args ...interface{}) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "opening %s", dbFile)
	require.NoError(t, db.Exec(statements, args...).Error, "executing statements on %s", dbFile)
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

func sealed(t *testing.T, passphrase, data string) []byte {
	t.Helper()
	out, err := commonUtil.Seal([]byte(data), []byte(passphrase))
	require.NoError(t, err)
	return out
}

// seedV021Data populates a v0.2.1-schema database with entries in every
// entity table a real deployment would have: endpoints, credentials for
// both forges, all three entity types, pools and scale sets with runners,
// tags, jobs, controller info. Secrets are sealed with the passphrase so
// the store can decode them after the upgrade. Includes the shape that
// made issue #890 blow up: live instances referencing a scale set.
func seedV021Data(t *testing.T, dbFile, passphrase string) {
	t.Helper()
	webhookSecret := sealed(t, passphrase, "webhook-secret")
	ghPayload := sealed(t, passphrase, `{"oauth2_token":"gh-test-token"}`)
	giteaPayload := sealed(t, passphrase, `{"oauth2_token":"gitea-test-token"}`)

	execSQLOnFile(t, dbFile, `
INSERT INTO github_endpoints (name, created_at, updated_at, endpoint_type, description, api_base_url, upload_base_url, base_url, tools_metadata_url, use_internal_tools_metadata)
	VALUES ('github.com', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'github', 'GitHub.com', 'https://api.github.com', 'https://uploads.github.com', 'https://github.com', '', 1);
INSERT INTO github_endpoints (name, created_at, updated_at, endpoint_type, description, api_base_url, upload_base_url, base_url, tools_metadata_url, use_internal_tools_metadata)
	VALUES ('local-gitea', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'gitea', 'Local gitea', 'https://gitea.example.com/api/v1', 'https://gitea.example.com', 'https://gitea.example.com', '', 1);
INSERT INTO users (id, created_at, updated_at, username, full_name, email, password, generation, is_admin, enabled)
	VALUES ('`+seedUserID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'admin', 'Admin', 'admin@example.com', 'not-a-real-hash', 1, 1, 1);
INSERT INTO github_credentials (id, created_at, updated_at, name, user_id, description, auth_type, payload, endpoint_name)
	VALUES (1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'gh-creds', '`+seedUserID+`', 'test creds', 'pat', ?, 'github.com');
INSERT INTO gitea_credentials (id, created_at, updated_at, name, user_id, description, auth_type, payload, endpoint_name)
	VALUES (1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'gitea-creds', '`+seedUserID+`', 'test creds', 'pat', ?, 'local-gitea');
INSERT INTO repositories (id, created_at, updated_at, credentials_id, owner, name, webhook_secret, pool_balancer_type, agent_mode, endpoint_name, pool_manager_running, pool_manager_failure_reason)
	VALUES ('`+seedRepoID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1, 'gsamfira', 'garm-testing', ?, 'roundrobin', 0, 'github.com', 0, '');
INSERT INTO organizations (id, created_at, updated_at, credentials_id, name, webhook_secret, pool_balancer_type, agent_mode, endpoint_name, pool_manager_running, pool_manager_failure_reason)
	VALUES ('`+seedOrgID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1, 'gsamfira-org', ?, 'roundrobin', 0, 'github.com', 0, '');
INSERT INTO enterprises (id, created_at, updated_at, credentials_id, name, webhook_secret, pool_balancer_type, agent_mode, endpoint_name, pool_manager_running, pool_manager_failure_reason)
	VALUES ('`+seedEnterpriseID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1, 'samfira-ent', ?, 'roundrobin', 0, 'github.com', 0, '');
INSERT INTO tags (id, created_at, updated_at, name) VALUES ('`+seedTagID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'self-hosted');
INSERT INTO pools (id, created_at, updated_at, provider_name, runner_prefix, max_runners, min_idle_runners, runner_bootstrap_timeout, image, flavor, os_type, os_arch, enabled, git_hub_runner_group, enable_shell, generation, repo_id, priority)
	VALUES ('`+seedRepoPoolID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'test-provider', 'garm', 4, 1, 20, 'ubuntu:24.04', 'default', 'linux', 'amd64', 1, '', 0, 1, '`+seedRepoID+`', 0);
INSERT INTO pools (id, created_at, updated_at, provider_name, runner_prefix, max_runners, min_idle_runners, runner_bootstrap_timeout, image, flavor, os_type, os_arch, enabled, git_hub_runner_group, enable_shell, generation, org_id, priority)
	VALUES ('`+seedOrgPoolID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'test-provider', 'garm', 4, 0, 20, 'ubuntu:24.04', 'default', 'linux', 'amd64', 1, '', 0, 1, '`+seedOrgID+`', 0);
INSERT INTO pools (id, created_at, updated_at, provider_name, runner_prefix, max_runners, min_idle_runners, runner_bootstrap_timeout, image, flavor, os_type, os_arch, enabled, git_hub_runner_group, enable_shell, generation, enterprise_id, priority)
	VALUES ('`+seedEntPoolID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'test-provider', 'garm', 4, 0, 20, 'ubuntu:24.04', 'default', 'linux', 'amd64', 1, '', 0, 1, '`+seedEnterpriseID+`', 0);
INSERT INTO pool_tags (pool_id, tag_id) VALUES ('`+seedRepoPoolID+`', '`+seedTagID+`');
INSERT INTO scale_sets (id, created_at, updated_at, scale_set_id, name, git_hub_runner_group, disable_update, state, extended_state, provider_name, runner_prefix, max_runners, min_idle_runners, runner_bootstrap_timeout, image, flavor, os_type, os_arch, enabled, last_message_id, desired_runner_count, enable_shell, generation, repo_id)
	VALUES (1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 10, 'ubuntu-noble', 'Default', 0, 'active', '', 'test-provider', 'garm', 4, 0, 20, 'ubuntu:24.04', 'default', 'linux', 'amd64', 1, 0, 1, 0, 1, '`+seedRepoID+`');
INSERT INTO scale_sets (id, created_at, updated_at, scale_set_id, name, git_hub_runner_group, disable_update, state, extended_state, provider_name, runner_prefix, max_runners, min_idle_runners, runner_bootstrap_timeout, image, flavor, os_type, os_arch, enabled, last_message_id, desired_runner_count, enable_shell, generation, org_id)
	VALUES (2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 11, 'ubuntu-noble', 'restricted', 0, 'active', '', 'test-provider', 'garm', 4, 0, 20, 'ubuntu:24.04', 'default', 'linux', 'amd64', 1, 0, 0, 0, 1, '`+seedOrgID+`');
INSERT INTO scaleset_tags (scale_set_id, tag_id) VALUES (1, '`+seedTagID+`');
INSERT INTO instances (id, created_at, updated_at, provider_id, name, agent_id, os_type, os_arch, status, runner_status, create_attempt, token_fetched, generation, pool_id)
	VALUES ('`+seedPoolRunnerID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'prov-1', 'garm-pool-runner', 1, 'linux', 'amd64', 'running', 'idle', 1, 1, 1, '`+seedRepoPoolID+`');
INSERT INTO instances (id, created_at, updated_at, provider_id, name, agent_id, os_type, os_arch, status, runner_status, create_attempt, token_fetched, generation, pool_id)
	VALUES ('`+seedOrgRunnerID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'prov-2', 'garm-org-runner', 2, 'linux', 'amd64', 'running', 'active', 1, 1, 1, '`+seedOrgPoolID+`');
INSERT INTO instances (id, created_at, updated_at, provider_id, name, agent_id, os_type, os_arch, status, runner_status, create_attempt, token_fetched, generation, scale_set_fk_id)
	VALUES ('`+seedSSRunnerID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'prov-3', 'garm-scaleset-runner', 3, 'linux', 'amd64', 'running', 'active', 1, 1, 1, 1);
INSERT INTO instances (id, created_at, updated_at, provider_id, name, agent_id, os_type, os_arch, status, runner_status, create_attempt, token_fetched, generation, scale_set_fk_id)
	VALUES ('`+seedSS2RunnerID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'prov-4', 'garm-scaleset2-runner', 4, 'linux', 'amd64', 'running', 'idle', 1, 1, 1, 2);
INSERT INTO addresses (id, created_at, updated_at, address, type, instance_id)
	VALUES ('`+seedAddressID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '10.0.0.5', 'private', '`+seedPoolRunnerID+`');
INSERT INTO instance_status_updates (id, created_at, updated_at, event_type, event_level, message, instance_id)
	VALUES ('`+seedStatusUpdateID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'provisioning', 'info', 'runner installed', '`+seedPoolRunnerID+`');
INSERT INTO workflow_jobs (id, workflow_job_id, scale_set_job_id, run_id, action, conclusion, status, name, github_runner_id, instance_id, runner_group_id, runner_group_name, repository_name, repository_owner, labels, workflow_run_url, repo_id, locked_by, created_at, updated_at)
	VALUES (1, 0, 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee', 100, 'in_progress', '', 'in_progress', 'scaleset job', 3, '`+seedSSRunnerID+`', 1, 'Default', 'garm-testing', 'gsamfira', '["ubuntu-noble"]', '', '`+seedRepoID+`', '00000000-0000-0000-0000-000000000000', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO workflow_jobs (id, workflow_job_id, scale_set_job_id, run_id, action, conclusion, status, name, github_runner_id, runner_group_id, runner_group_name, repository_name, repository_owner, labels, workflow_run_url, repo_id, locked_by, created_at, updated_at)
	VALUES (2, 201, '', 101, 'queued', '', 'queued', 'pool job', 0, 0, '', 'garm-testing', 'gsamfira', '["self-hosted"]', '', '`+seedRepoID+`', '00000000-0000-0000-0000-000000000000', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO controller_infos (id, created_at, updated_at, controller_id, callback_url, metadata_url, webhook_base_url, agent_url, garm_agent_releases_url, sync_garm_agent_tools, minimum_job_age_backoff)
	VALUES ('`+seedControllerID+`', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '`+seedControllerID+`', 'https://garm.example.com/api/v1/callbacks', 'https://garm.example.com/api/v1/metadata', 'https://garm.example.com/webhooks', 'wss://garm.example.com/api/v1/ws', '', 0, 30);
`, ghPayload, giteaPayload, webhookSecret, webhookSecret, webhookSecret)
}

// seedV021BlobData populates a v0.2.1 objects database with a stored file.
func seedV021BlobData(t *testing.T, dbFile string) {
	t.Helper()
	execSQLOnFile(t, dbFile, `
INSERT INTO file_objects (id, created_at, updated_at, name, description, file_type, size, sha256)
	VALUES (1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'garm-agent-linux-amd64', 'test agent binary', 'application/octet-stream', 4, 'de7d1b721a1e0632b7cf04edf5032c8ecffa9f9a08492152b926f1a5a7e765d7');
INSERT INTO file_blobs (id, created_at, updated_at, file_object_id, content)
	VALUES (1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1, X'74657374');
INSERT INTO file_object_tags (id, file_object_id, tag) VALUES (1, 1, 'linux');
INSERT INTO file_object_tags (id, file_object_id, tag) VALUES (2, 1, 'amd64');
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

var (
	fixtureIndexRe      = regexp.MustCompile("CREATE (?:UNIQUE )?INDEX `([a-zA-Z0-9_]+)`")
	fixtureConstraintRe = regexp.MustCompile("CONSTRAINT `(fk_[a-z0-9_]+)`")
)

// requireFixtureSchemaPreserved asserts that every index and foreign key
// constraint present in the v0.2.1 fixture still exists after the upgrade.
func requireFixtureSchemaPreserved(t *testing.T, conn *gorm.DB, fixturePath string) {
	t.Helper()
	contents, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	var allDDL []string
	require.NoError(t, conn.Raw("SELECT sql FROM sqlite_master WHERE sql IS NOT NULL").Scan(&allDDL).Error)
	joined := strings.Join(allDDL, "\n")

	for _, m := range fixtureIndexRe.FindAllStringSubmatch(string(contents), -1) {
		var count int64
		require.NoError(t, conn.Raw("SELECT count(*) FROM sqlite_master WHERE type='index' AND name = ?", m[1]).Scan(&count).Error)
		require.EqualValues(t, 1, count, "index %s from the v0.2.1 schema is gone after upgrade", m[1])
	}
	for _, m := range fixtureConstraintRe.FindAllStringSubmatch(string(contents), -1) {
		require.Contains(t, joined, "`"+m[1]+"`", "constraint %s from the v0.2.1 schema is gone after upgrade", m[1])
	}
}

// TestBrownfieldMigration upgrades a fully populated v0.2.1 database
// (created by a release that predates gormigrate) to the current schema and
// verifies the result end to end: migrations recorded, v0.2.1 indexes and
// constraints preserved, new schema present, data intact and readable
// through the store, and foreign keys enforced. This covers the exact
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
	seedV021Data(t, cfg.SQLite.DBFile, cfg.Passphrase)
	execSQLFileOnFile(t, blobCfg.SQLite.DBFile, "migrations/testdata/v021_blob_schema.sql")
	seedV021BlobData(t, blobCfg.SQLite.DBFile)

	store, err := NewSQLStore(ctx, cfg)
	require.NoError(t, err, "brownfield migration failed")
	db := store.(*sqlDatabase)

	// The numbered migrations ran; InitSchema did not.
	ids := migrationIDs(t, db.conn, "migrations")
	require.Contains(t, ids, "0001_baseline")
	require.Contains(t, ids, "0006_proxies")
	require.Contains(t, ids, "0008_constraint_parity")
	require.NotContains(t, ids, "SCHEMA_INIT")

	objectIDs := migrationIDs(t, db.objectsConn, "file_object_migrations")
	require.Contains(t, objectIDs, "0001_baseline")
	require.Contains(t, objectIDs, "0002_fileobject_postgres_compat")
	require.NotContains(t, objectIDs, "SCHEMA_INIT")

	// Everything the v0.2.1 schema had is still there.
	requireFixtureSchemaPreserved(t, db.conn, "migrations/testdata/v021_schema.sql")
	requireFixtureSchemaPreserved(t, db.objectsConn, "migrations/testdata/v021_blob_schema.sql")

	// Post-baseline schema is present.
	require.True(t, db.conn.Migrator().HasTable("proxies"))
	require.True(t, db.conn.Migrator().HasTable("forge_instances"))
	require.True(t, hasColumn(t, db.conn, "scale_sets", "proxy_id"))
	require.True(t, hasColumn(t, db.conn, "github_credentials", "reserve_usage_enabled"))
	require.True(t, hasColumn(t, db.objectsConn, "file_blobs", "lo_oid"))

	// Seeded rows survived.
	for table, expected := range map[string]int64{
		"github_endpoints":        2,
		"users":                   1,
		"github_credentials":      1,
		"gitea_credentials":       1,
		"repositories":            1,
		"organizations":           1,
		"enterprises":             1,
		"tags":                    1,
		"pools":                   3,
		"pool_tags":               1,
		"scale_sets":              2,
		"scaleset_tags":           1,
		"instances":               4,
		"addresses":               1,
		"instance_status_updates": 1,
		"workflow_jobs":           2,
		"controller_infos":        1,
	} {
		require.EqualValues(t, expected, countRows(t, db.conn, table), "row count mismatch in %s", table)
	}
	for table, expected := range map[string]int64{
		"file_objects":     1,
		"file_blobs":       1,
		"file_object_tags": 2,
	} {
		require.EqualValues(t, expected, countRows(t, db.objectsConn, table), "row count mismatch in %s", table)
	}

	// The store can read everything back, sealed fields included.
	adminCtx := garmTesting.ImpersonateAdminContext(ctx, store, t)

	repos, err := store.ListRepositories(adminCtx, params.RepositoryFilter{})
	require.NoError(t, err)
	require.Len(t, repos, 1)
	require.Equal(t, "gsamfira", repos[0].Owner)

	orgs, err := store.ListOrganizations(adminCtx, params.OrganizationFilter{})
	require.NoError(t, err)
	require.Len(t, orgs, 1)

	enterprises, err := store.ListEnterprises(adminCtx, params.EnterpriseFilter{})
	require.NoError(t, err)
	require.Len(t, enterprises, 1)

	ghCreds, err := store.ListGithubCredentials(adminCtx)
	require.NoError(t, err)
	require.Len(t, ghCreds, 1)
	require.Equal(t, "github.com", ghCreds[0].Endpoint.Name)

	giteaCreds, err := store.ListGiteaCredentials(adminCtx)
	require.NoError(t, err)
	require.Len(t, giteaCreds, 1)

	pools, err := store.ListAllPools(adminCtx)
	require.NoError(t, err)
	require.Len(t, pools, 3)

	scaleSets, err := store.ListAllScaleSets(adminCtx)
	require.NoError(t, err)
	require.Len(t, scaleSets, 2)

	instances, err := store.ListAllInstances(adminCtx)
	require.NoError(t, err)
	require.Len(t, instances, 4)

	jobs, err := store.ListAllJobs(adminCtx)
	require.NoError(t, err)
	require.Len(t, jobs, 2)

	controllerInfo, err := store.ControllerInfo()
	require.NoError(t, err)
	require.Equal(t, seedControllerID, controllerInfo.ControllerID.String())

	files, err := store.ListFileObjects(adminCtx, 1, 10)
	require.NoError(t, err)
	require.Len(t, files.Results, 1)
	require.Equal(t, "garm-agent-linux-amd64", files.Results[0].Name)

	// Foreign keys are enforced again after the migration.
	err = db.conn.Exec("INSERT INTO instances (id, created_at, updated_at, name, os_type, os_arch, status, runner_status, create_attempt, token_fetched, generation, pool_id) VALUES ('99999999-9999-9999-9999-999999999999', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'bogus', 'linux', 'amd64', 'running', 'idle', 1, 0, 1, '88888888-8888-8888-8888-888888888888')").Error
	require.Error(t, err, "insert referencing a nonexistent pool should be rejected")
	require.Contains(t, strings.ToLower(err.Error()), "foreign key")

	requireNoFKViolations(t, db.conn)
	requireNoFKViolations(t, db.objectsConn)
}

var parityConstraints = map[string]string{
	"fk_forge_instances_events":            "forge_instance_events",
	"fk_forge_instances_endpoint":          "forge_instances",
	"fk_gitea_credentials_forge_instances": "forge_instances",
	"fk_forge_instances_pools":             "pools",
	"fk_proxies_pools":                     "pools",
	"fk_proxies_scale_sets":                "scale_sets",
	"fk_forge_instances_jobs":              "workflow_jobs",
}

func requireConstraintCountPG(t *testing.T, conn *gorm.DB, constraint string, expected int64) {
	t.Helper()
	var count int64
	require.NoError(t, conn.Raw(
		"SELECT count(*) FROM information_schema.table_constraints WHERE constraint_name = ? AND constraint_schema = current_schema()",
		constraint).Scan(&count).Error)
	require.EqualValues(t, expected, count, "constraint %s count mismatch", constraint)
}

// TestConstraintParityOnPostgres exercises 0008 on PostgreSQL for both real
// cohorts. PostgreSQL support landed before the proxies feature, so pg
// databases born in that window have no fk_proxies_* constraints (the 0006
// stub only adds columns): for them 0008 must clean up orphans and create
// the constraints via ALTER TABLE. Databases born later have everything
// already: for them 0008 must be a clean no-op. Constraints are dropped
// manually to reconstruct the first cohort, since 0008 introduces no model
// changes of its own.
func TestConstraintParityOnPostgres(t *testing.T) {
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

	// Phase 1: a database from before the constraints existed, with an
	// orphaned reference the cleanup must resolve.
	for constraint, table := range parityConstraints {
		require.NoError(t, db.conn.Exec("ALTER TABLE "+table+" DROP CONSTRAINT "+constraint).Error)
	}
	require.NoError(t, db.conn.Exec(
		"INSERT INTO pools (id, created_at, updated_at, provider_name, runner_prefix, max_runners, min_idle_runners, runner_bootstrap_timeout, image, flavor, os_type, os_arch, enabled, generation, priority, proxy_id) VALUES ('99999999-9999-9999-9999-999999999999', now(), now(), 'test-provider', 'garm', 4, 0, 20, 'ubuntu:24.04', 'default', 'linux', 'amd64', true, 1, 0, 12345)").Error)
	require.NoError(t, db.conn.Exec("DELETE FROM migrations WHERE id = '0008_constraint_parity'").Error)

	require.NoError(t, db.migrateDB(), "0008 failed on a PostgreSQL database missing the constraints")

	require.Contains(t, migrationIDs(t, db.conn, "migrations"), "0008_constraint_parity")
	for constraint := range parityConstraints {
		requireConstraintCountPG(t, db.conn, constraint, 1)
	}
	var orphanedProxyRefs int64
	require.NoError(t, db.conn.Raw("SELECT count(*) FROM pools WHERE proxy_id IS NOT NULL").Scan(&orphanedProxyRefs).Error)
	require.Zero(t, orphanedProxyRefs, "orphaned proxy reference should have been cleaned up")

	// Phase 2: a database that already has everything; 0008 must no-op.
	require.NoError(t, db.conn.Exec("DELETE FROM migrations WHERE id = '0008_constraint_parity'").Error)
	require.NoError(t, db.migrateDB(), "0008 must be a no-op on a constrained PostgreSQL database")
	for constraint := range parityConstraints {
		requireConstraintCountPG(t, db.conn, constraint, 1)
	}
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
	seedV021Data(t, cfg.SQLite.DBFile, cfg.Passphrase)
	execSQLOnFile(t, cfg.SQLite.DBFile, `
CREATE TABLE migrations (id VARCHAR(255) PRIMARY KEY);
INSERT INTO migrations (id) VALUES ('0001_baseline');
`)
	execSQLFileOnFile(t, blobCfg.SQLite.DBFile, "migrations/testdata/v021_blob_schema.sql")
	seedV021BlobData(t, blobCfg.SQLite.DBFile)
	execSQLOnFile(t, blobCfg.SQLite.DBFile, `
CREATE TABLE file_object_migrations (id VARCHAR(255) PRIMARY KEY);
INSERT INTO file_object_migrations (id) VALUES ('0001_baseline');
`)

	store, err := NewSQLStore(ctx, cfg)
	require.NoError(t, err)
	db := store.(*sqlDatabase)

	ids := migrationIDs(t, db.conn, "migrations")
	require.Contains(t, ids, "0002_lower_indexes")
	require.Contains(t, ids, "0008_constraint_parity")
	require.NotContains(t, ids, "SCHEMA_INIT")

	require.True(t, db.conn.Migrator().HasTable("forge_instances"))
	require.EqualValues(t, 4, countRows(t, db.conn, "instances"))
	requireNoFKViolations(t, db.conn)
}
