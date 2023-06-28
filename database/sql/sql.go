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

package sql

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/pkg/errors"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/database/common"
)

// newDBConn returns a new gorm db connection, given the config
func newDBConn(dbCfg config.Database) (conn *gorm.DB, err error) {
	dbType, connURI, err := dbCfg.GormParams()
	if err != nil {
		return nil, errors.Wrap(err, "getting DB URI string")
	}

	gormConfig := &gorm.Config{}
	if !dbCfg.Debug {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	}

	switch dbType {
	case config.MySQLBackend:
		conn, err = gorm.Open(mysql.Open(connURI), gormConfig)
	case config.SQLiteBackend:
		conn, err = gorm.Open(sqlite.Open(connURI), gormConfig)
	}
	if err != nil {
		return nil, errors.Wrap(err, "connecting to database")
	}

	if dbCfg.Debug {
		conn = conn.Debug()
	}
	return conn, nil
}

func NewSQLDatabase(ctx context.Context, cfg config.Database) (common.Store, error) {
	conn, err := newDBConn(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "creating DB connection")
	}
	db := &sqlDatabase{
		conn: conn,
		ctx:  ctx,
		cfg:  cfg,
	}

	if err := db.migrateDB(); err != nil {
		return nil, errors.Wrap(err, "migrating database")
	}
	return db, nil
}

type sqlDatabase struct {
	conn *gorm.DB
	ctx  context.Context
	cfg  config.Database
}

var renameTemplate = `
PRAGMA foreign_keys = OFF;
BEGIN TRANSACTION;

ALTER TABLE %s RENAME TO %s_old;
COMMIT;
`

var restoreNameTemplate = `
PRAGMA foreign_keys = OFF;
BEGIN TRANSACTION;
DROP TABLE IF EXISTS %s;
ALTER TABLE %s_old RENAME TO %s;
COMMIT;
`

var copyContentsTemplate = `
PRAGMA foreign_keys = OFF;
BEGIN TRANSACTION;
INSERT INTO %s SELECT * FROM %s_old;
DROP TABLE %s_old;

COMMIT;
`

func (s *sqlDatabase) cascadeMigrationSQLite(model interface{}, name string, justDrop bool) error {
	if !s.conn.Migrator().HasTable(name) {
		return nil
	}
	defer s.conn.Exec("PRAGMA foreign_keys = ON;")

	var data string
	var indexes []string
	if err := s.conn.Raw(fmt.Sprintf("select sql from sqlite_master where tbl_name='%s' and name='%s'", name, name)).Scan(&data).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to get table %s: %w", name, err)
		}
	}

	if err := s.conn.Raw(fmt.Sprintf("SELECT name FROM sqlite_master WHERE type == 'index' AND tbl_name == '%s' and name not like 'sqlite_%%'", name)).Scan(&indexes).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to get table indexes %s: %w", name, err)
		}
	}

	if strings.Contains(data, "ON DELETE CASCADE") {
		return nil
	}

	if justDrop {
		if err := s.conn.Migrator().DropTable(model); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", name, err)
		}
		return nil
	}

	for _, index := range indexes {
		if err := s.conn.Migrator().DropIndex(model, index); err != nil {
			return fmt.Errorf("failed to drop index %s: %w", index, err)
		}
	}

	err := s.conn.Exec(fmt.Sprintf(renameTemplate, name, name)).Error
	if err != nil {
		return fmt.Errorf("failed to rename table %s: %w", name, err)
	}

	if model != nil {
		if err := s.conn.Migrator().AutoMigrate(model); err != nil {
			if err := s.conn.Exec(fmt.Sprintf(restoreNameTemplate, name, name, name)).Error; err != nil {
				log.Printf("failed to restore table %s: %s", name, err)
			}
			return fmt.Errorf("failed to create table %s: %w", name, err)
		}
	}
	err = s.conn.Exec(fmt.Sprintf(copyContentsTemplate, name, name, name)).Error
	if err != nil {
		return fmt.Errorf("failed to copy contents to table %s: %w", name, err)
	}

	return nil
}

func (s *sqlDatabase) cascadeMigration() error {
	switch s.cfg.DbBackend {
	case config.SQLiteBackend:
		if err := s.cascadeMigrationSQLite(&Address{}, "addresses", true); err != nil {
			return fmt.Errorf("failed to drop table addresses: %w", err)
		}

		if err := s.cascadeMigrationSQLite(&InstanceStatusUpdate{}, "instance_status_updates", true); err != nil {
			return fmt.Errorf("failed to drop table instance_status_updates: %w", err)
		}

		if err := s.cascadeMigrationSQLite(&Tag{}, "pool_tags", false); err != nil {
			return fmt.Errorf("failed to migrate addresses: %w", err)
		}
	case config.MySQLBackend:
		return nil
	default:
		return fmt.Errorf("invalid db backend: %s", s.cfg.DbBackend)
	}
	return nil
}

func (s *sqlDatabase) migrateDB() error {
	if s.conn.Migrator().HasIndex(&Organization{}, "idx_organizations_name") {
		if err := s.conn.Migrator().DropIndex(&Organization{}, "idx_organizations_name"); err != nil {
			log.Printf("failed to drop index idx_organizations_name: %s", err)
		}
	}

	if s.conn.Migrator().HasIndex(&Repository{}, "idx_owner") {
		if err := s.conn.Migrator().DropIndex(&Repository{}, "idx_owner"); err != nil {
			log.Printf("failed to drop index idx_owner: %s", err)
		}
	}

	if err := s.cascadeMigration(); err != nil {
		return errors.Wrap(err, "running cascade migration")
	}

	if err := s.conn.AutoMigrate(
		&Tag{},
		&Pool{},
		&Repository{},
		&Organization{},
		&Enterprise{},
		&Address{},
		&InstanceStatusUpdate{},
		&Instance{},
		&ControllerInfo{},
		&User{},
		&WorkflowJob{},
	); err != nil {
		return errors.Wrap(err, "running auto migrate")
	}

	return nil
}
