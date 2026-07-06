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

package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// controllerInfo0004 is a minimal stub that only declares the new columns:
// the pinned garm-agent version and the cached release index, which replaces
// the previously cached single release. AutoMigrate will add the columns
// without touching existing ones.

type controllerInfo0004 struct {
	GARMAgentVersion        string
	CachedGARMAgentReleases datatypes.JSON
}

func (controllerInfo0004) TableName() string { return "controller_infos" }

func init() {
	Register(&gormigrate.Migration{
		ID: "0004_garm_agent_version",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&controllerInfo0004{}); err != nil {
				return err
			}
			// The single-release cache is superseded by the release index;
			// the sync worker repopulates the new column on its next run.
			if tx.Migrator().HasColumn(&controllerInfo0004{}, "cached_garm_agent_release") {
				return tx.Migrator().DropColumn(&controllerInfo0004{}, "cached_garm_agent_release")
			}
			return nil
		},
	})
}
