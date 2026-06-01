// Copyright 2022 Cloudbase Solutions SRL
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

// Concurrency invariant: read-modify-write transactions in this package take a
// row lock via SELECT ... FOR UPDATE (clause.Locking{Strength: "UPDATE"}) to
// serialize concurrent updates of the same row and prevent lost updates. On a
// single-writer backend (SQLite) the lock is effectively a no-op, but it is
// load-bearing on any backend that permits concurrent writers — do not remove it.
package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/sql/migrations"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/internal/templates"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/appdefaults"
)

const (
	repositoryFieldName   string = "Repository"
	organizationFieldName string = "Organization"
	enterpriseFieldName   string = "Enterprise"
)

// newDBConn returns a new gorm db connection, given the config
func newDBConn(dbCfg config.Database) (conn *gorm.DB, err error) {
	dbType, connURI, err := dbCfg.GormParams()
	if err != nil {
		return nil, fmt.Errorf("error getting DB URI string: %w", err)
	}

	gormConfig := &gorm.Config{
		TranslateError: true,
	}
	if !dbCfg.Debug {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	}

	switch dbType {
	case config.MySQLBackend:
		conn, err = gorm.Open(mysql.Open(connURI), gormConfig)
	case config.SQLiteBackend:
		conn, err = gorm.Open(sqlite.Open(connURI), gormConfig)
	case config.PostgreSQLBackend:
		conn, err = gorm.Open(postgres.Open(connURI), gormConfig)
	}
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	if dbCfg.Debug {
		conn = conn.Debug()
	}

	return conn, nil
}

func NewSQLStore(ctx context.Context, cfg config.Database) (common.Store, error) {
	conn, err := newDBConn(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating DB connection: %w", err)
	}
	producer, err := watcher.RegisterProducer(ctx, "sql")
	if err != nil {
		return nil, fmt.Errorf("error registering producer: %w", err)
	}

	sqlDB, err := conn.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying database connection: %w", err)
	}

	// SQLite only supports one concurrent writer per database file.
	// Limit the pool to a single connection to prevent deadlocks with _txlock=immediate.
	if cfg.DbBackend == config.SQLiteBackend {
		sqlDB.SetMaxOpenConns(1)
	}

	if cfg.DbBackend == config.PostgreSQLBackend {
		sqlDB.SetMaxOpenConns(cfg.PostgreSQL.MaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.PostgreSQL.MaxIdleConns)
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.PostgreSQL.ConnMaxLifetimeMins) * time.Minute)
		sqlDB.SetConnMaxIdleTime(time.Duration(cfg.PostgreSQL.ConnMaxIdleTimeSecs) * time.Second)
	}

	db := &sqlDatabase{
		conn:     conn,
		sqlDB:    sqlDB,
		ctx:      ctx,
		cfg:      cfg,
		producer: producer,
	}

	switch cfg.DbBackend {
	case config.SQLiteBackend:
		// SQLite uses a separate file for blobs to avoid WAL contention during large writes.
		objectsCfg, err := cfg.SQLiteBlobDatabaseConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get blob DB config: %w", err)
		}

		objectsConn, err := newDBConn(objectsCfg)
		if err != nil {
			return nil, fmt.Errorf("error creating objects DB connection: %w", err)
		}

		objectsSQLDB, err := objectsConn.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to get underlying objects database connection: %w", err)
		}
		objectsSQLDB.SetMaxOpenConns(1)
		db.objectsConn = objectsConn
		db.objectsSQLDB = objectsSQLDB
	case config.PostgreSQLBackend:
		// PostgreSQL reuses the main connection — no separate DB needed for blobs.
		db.objectsConn = conn
	}

	if err := db.migrateDB(); err != nil {
		return nil, fmt.Errorf("error migrating database: %w", err)
	}

	if cfg.DbBackend == config.SQLiteBackend {
		go db.startSQLiteMaintenance()
	}

	return db, nil
}

type sqlDatabase struct {
	conn  *gorm.DB
	sqlDB *sql.DB

	// objectsConn is a separate GORM connection to the objects database
	objectsConn  *gorm.DB
	objectsSQLDB *sql.DB

	ctx      context.Context
	cfg      config.Database
	producer common.Producer
}

// startSQLiteMaintenance runs periodic WAL checkpoint and VACUUM on both the
// main database and the objects database (if present). This reclaims disk space
// from deleted rows/blobs and keeps the WAL file from growing unbounded.
func (s *sqlDatabase) startSQLiteMaintenance() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.runSQLiteMaintenance(s.conn, "main")
			if s.objectsConn != nil {
				s.runSQLiteMaintenance(s.objectsConn, "objects")
			}
		}
	}
}

func (s *sqlDatabase) runSQLiteMaintenance(conn *gorm.DB, dbName string) {
	if err := conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(s.ctx, "failed to checkpoint WAL", "database", dbName)
	}
	if err := conn.Exec("PRAGMA incremental_vacuum").Error; err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(s.ctx, "failed to incremental vacuum database", "database", dbName)
	}
}

func (s *sqlDatabase) ensureGithubEndpoint() error {
	// Create the default Github endpoint.
	createEndpointParams := params.CreateGithubEndpointParams{
		Name:          "github.com",
		Description:   "The github.com endpoint",
		APIBaseURL:    appdefaults.GithubDefaultBaseURL,
		BaseURL:       appdefaults.DefaultGithubURL,
		UploadBaseURL: appdefaults.GithubDefaultUploadBaseURL,
	}

	var epCount int64
	if err := s.conn.Model(&GithubEndpoint{}).Count(&epCount).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("error counting github endpoints: %w", err)
		}
	}

	if epCount == 0 {
		if _, err := s.CreateGithubEndpoint(context.Background(), createEndpointParams); err != nil {
			if !errors.Is(err, runnerErrors.ErrDuplicateEntity) {
				return fmt.Errorf("error creating default github endpoint: %w", err)
			}
		}
	}

	return nil
}

// fileObjectMigrationOptions tracks file-object migrations in a dedicated
// "file_object_migrations" table so the file-object migration history does
// not collide with the main migration history when both run on the same
// database connection (e.g. PostgreSQL). For the upgrade path from versions
// that used gormigrate's default table name, see migrateLegacyFileObjectTracking.
var fileObjectMigrationOptions = &gormigrate.Options{
	TableName:                 "file_object_migrations",
	IDColumnName:              gormigrate.DefaultOptions.IDColumnName,
	IDColumnSize:              gormigrate.DefaultOptions.IDColumnSize,
	UseTransaction:            gormigrate.DefaultOptions.UseTransaction,
	ValidateUnknownMigrations: gormigrate.DefaultOptions.ValidateUnknownMigrations,
}

func (s *sqlDatabase) migrateFileObjects() error {
	if s.objectsConn == nil {
		return nil
	}
	if err := s.migrateLegacyFileObjectTracking(); err != nil {
		return fmt.Errorf("error migrating legacy file object tracking table: %w", err)
	}
	m := gormigrate.New(s.objectsConn, fileObjectMigrationOptions, migrations.AllFileObjects())
	m.InitSchema(func(tx *gorm.DB) error {
		return tx.AutoMigrate(&FileObject{}, &FileBlob{}, &FileObjectTag{})
	})

	if err := m.Migrate(); err != nil {
		return fmt.Errorf("error running file objects migrations: %w", err)
	}

	return nil
}

// migrateLegacyFileObjectTracking renames the pre-PostgreSQL "migrations"
// tracking table in the objects DB to "file_object_migrations" so file-object
// migration history is preserved across the rename. Only runs when objectsConn
// is a separate DB (SQLite); on PostgreSQL objectsConn == conn and the
// "migrations" table belongs to the main migrator and must not be touched.
//
// Without this shim, an existing SQLite install upgrading from a version that
// predates fileObjectMigrationOptions would find file_object_migrations empty,
// trigger gormigrate's InitSchema, and mark every registered migration as
// already applied without running its SQL. DDL that reshapes existing schema
// (e.g. 0002's swap of idx_fileobject_tags_tag to a functional LOWER(tag)
// index) would be silently skipped, and the risk would compound with every
// new file-object migration added to the branch.
func (s *sqlDatabase) migrateLegacyFileObjectTracking() error {
	if s.objectsConn == s.conn {
		return nil
	}
	migrator := s.objectsConn.Migrator()
	if migrator.HasTable("file_object_migrations") {
		return nil
	}
	if !migrator.HasTable("migrations") {
		return nil
	}
	return s.objectsConn.Exec("ALTER TABLE migrations RENAME TO file_object_migrations").Error
}

func (s *sqlDatabase) ensureTemplates(migrateTemplates bool) error {
	if !migrateTemplates {
		return nil
	}
	// make sure we have a default forge/OSType template.
	githubWindowsData, err := templates.GetTemplateContent(commonParams.Windows, params.GithubEndpointType)
	if err != nil {
		return fmt.Errorf("failed to get windows template for github: %w", err)
	}

	githubLinuxData, err := templates.GetTemplateContent(commonParams.Linux, params.GithubEndpointType)
	if err != nil {
		return fmt.Errorf("failed to get linux template for github: %w", err)
	}

	giteaLinuxData, err := templates.GetTemplateContent(commonParams.Linux, params.GiteaEndpointType)
	if err != nil {
		return fmt.Errorf("failed to get linux template for gitea: %w", err)
	}

	giteaWindowsData, err := templates.GetTemplateContent(commonParams.Windows, params.GiteaEndpointType)
	if err != nil {
		return fmt.Errorf("failed to get windows template for gitea: %w", err)
	}

	adminCtx := auth.GetAdminContext(s.ctx)

	githubWindowsParams := params.CreateTemplateParams{
		Name:        "github_windows",
		Description: "Default Windows runner install template for GitHub",
		OSType:      commonParams.Windows,
		ForgeType:   params.GithubEndpointType,
		Data:        githubWindowsData,
		IsSystem:    true,
	}
	if _, err := s.CreateTemplate(adminCtx, githubWindowsParams); err != nil {
		return fmt.Errorf("failed to create github windows template: %w", err)
	}

	githubLinuxParams := params.CreateTemplateParams{
		Name:        "github_linux",
		Description: "Default Linux runner install template for GitHub",
		OSType:      commonParams.Linux,
		ForgeType:   params.GithubEndpointType,
		Data:        githubLinuxData,
		IsSystem:    true,
	}
	if _, err := s.CreateTemplate(adminCtx, githubLinuxParams); err != nil {
		return fmt.Errorf("failed to create github linux template: %w", err)
	}

	giteaLinuxParams := params.CreateTemplateParams{
		Name:        "gitea_linux",
		Description: "Default Linux runner install template for Gitea",
		OSType:      commonParams.Linux,
		ForgeType:   params.GiteaEndpointType,
		Data:        giteaLinuxData,
		IsSystem:    true,
	}
	if _, err := s.CreateTemplate(adminCtx, giteaLinuxParams); err != nil {
		return fmt.Errorf("failed to create gitea linux template: %w", err)
	}

	giteaWindowsParams := params.CreateTemplateParams{
		Name:        "gitea_windows",
		Description: "Default Windows runner install template for Gitea",
		OSType:      commonParams.Windows,
		ForgeType:   params.GiteaEndpointType,
		Data:        giteaWindowsData,
		IsSystem:    true,
	}
	if _, err := s.CreateTemplate(adminCtx, giteaWindowsParams); err != nil {
		return fmt.Errorf("failed to create gitea windows template: %w", err)
	}

	return nil
}

func (s *sqlDatabase) initSchema(tx *gorm.DB) error {
	if err := tx.AutoMigrate(
		&User{},
		&GithubEndpoint{},
		&GithubCredentials{},
		&GiteaCredentials{},
		&Tag{},
		&Template{},
		&Pool{},
		&Repository{},
		&Organization{},
		&Enterprise{},
		&EnterpriseEvent{},
		&OrganizationEvent{},
		&RepositoryEvent{},
		&Address{},
		&InstanceStatusUpdate{},
		&Instance{},
		&ControllerInfo{},
		&WorkflowJob{},
		&ScaleSet{},
	); err != nil {
		return fmt.Errorf("error running auto migrate: %w", err)
	}
	return nil
}

func (s *sqlDatabase) migrateDB() error {
	m := gormigrate.New(s.conn, gormigrate.DefaultOptions, migrations.All())
	m.InitSchema(s.initSchema)

	if err := m.Migrate(); err != nil {
		return fmt.Errorf("error running migrations: %w", err)
	}

	// Migrate file object tables in the separate objects database
	if err := s.migrateFileObjects(); err != nil {
		return fmt.Errorf("error migrating file objects: %w", err)
	}

	// Seed default data
	if err := s.ensureGithubEndpoint(); err != nil {
		return fmt.Errorf("error ensuring github endpoint: %w", err)
	}

	var tplCount int64
	s.conn.Model(&Template{}).Count(&tplCount)
	if err := s.ensureTemplates(tplCount == 0); err != nil {
		return fmt.Errorf("failed to create default templates: %w", err)
	}

	return nil
}
