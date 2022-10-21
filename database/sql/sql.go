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
	"log"

	"github.com/pkg/errors"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"garm/config"
	"garm/database/common"
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
	); err != nil {
		return errors.Wrap(err, "running auto migrate")
	}

	return nil
}
