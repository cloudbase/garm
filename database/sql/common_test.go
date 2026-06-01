// Copyright 2025 Cloudbase Solutions SRL
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
	dbsql "database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
)

const (
	wrongPassphrase = "wrong-passphrase"
	webhookSecret   = "webhook-secret"
	falseString     = "false"
)

// newTestDB creates a database for a test case and registers t.Cleanup to
// close the underlying connection pool. Without explicit close, each SetupTest
// leaks a pool of open connections that only drain after ConnMaxIdleTimeSecs —
// fast backends (e.g. tmpfs PostgreSQL) exhaust max_connections before that.
func newTestDB(t *testing.T) common.Store {
	t.Helper()
	var (
		db  common.Store
		err error
	)
	if os.Getenv("GARM_TEST_POSTGRES_DSN") != "" {
		sqlDB, cfg := garmTesting.OpenTestPostgresDB(t)
		db, err = newSQLStoreFromSQLDB(context.Background(), sqlDB, cfg)
		if err != nil {
			t.Fatalf("failed to create db connection: %s", err)
		}
		t.Cleanup(func() { db.(*sqlDatabase).sqlDB.Close() })
		return db
	}
	db, err = NewSQLStore(context.Background(), garmTesting.GetTestSqliteDBConfig(t))
	if err != nil {
		t.Fatalf("failed to create db connection: %s", err)
	}
	t.Cleanup(func() {
		sqlDB := db.(*sqlDatabase)
		// SQLite opens a separate objectsSQLDB for blobs; PostgreSQL leaves it nil.
		if sqlDB.objectsSQLDB != nil {
			sqlDB.objectsSQLDB.Close()
		}
		sqlDB.sqlDB.Close()
	})
	return db
}

// newSQLStoreFromSQLDB opens a Store from a pre-opened *sql.DB.
// Used in tests to inject a pgx DB with custom RuntimeParams
// (e.g. search_path for schema isolation) without exposing escape hatches
// in the production config.
func newSQLStoreFromSQLDB(ctx context.Context, sqlDB *dbsql.DB, cfg config.Database) (common.Store, error) {
	gormConfig := &gorm.Config{TranslateError: true}
	if !cfg.Debug {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	}
	conn, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}
	if cfg.Debug {
		conn = conn.Debug()
	}

	producer, err := watcher.RegisterProducer(ctx, "sql")
	if err != nil {
		return nil, fmt.Errorf("error registering producer: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.PostgreSQL.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.PostgreSQL.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.PostgreSQL.ConnMaxLifetimeMins) * time.Minute)
	sqlDB.SetConnMaxIdleTime(time.Duration(cfg.PostgreSQL.ConnMaxIdleTimeSecs) * time.Second)

	db := &sqlDatabase{
		conn:        conn,
		sqlDB:       sqlDB,
		objectsConn: conn,
		ctx:         ctx,
		cfg:         cfg,
		producer:    producer,
	}

	if err := db.migrateDB(); err != nil {
		return nil, fmt.Errorf("error migrating database: %w", err)
	}

	return db, nil
}
