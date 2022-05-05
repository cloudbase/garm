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

package database

import (
	"context"
	"fmt"
	"garm/config"
	"garm/database/common"
	"garm/database/sql"
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
