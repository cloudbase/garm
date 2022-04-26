package database

import (
	"context"
	"fmt"
	"runner-manager/config"
	"runner-manager/database/common"
	"runner-manager/database/sql"
)

func NewDatabase(ctx context.Context, cfg config.Database) (common.Store, error) {
	dbBackend := cfg.DbBackend
	switch dbBackend {
	case config.MySQLBackend, config.SQLiteBackend:
		return sql.NewSQLDatabase(ctx, cfg)
	default:
		return nil, fmt.Errorf("no team manager backend available for db backend %s", dbBackend)
	}

}
