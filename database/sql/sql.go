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
	"garm/config"
	"garm/database/common"
	"garm/util"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

func NewSQLDatabase(ctx context.Context, cfg config.Database) (common.Store, error) {
	conn, err := util.NewDBConn(cfg)
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
	if err := s.conn.AutoMigrate(
		&Tag{},
		&Pool{},
		&Repository{},
		&Organization{},
		&Address{},
		&InstanceStatusUpdate{},
		&Instance{},
		&ControllerInfo{},
		&User{},
	); err != nil {
		return err
	}

	return nil
}
