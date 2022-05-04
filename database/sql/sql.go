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
