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
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Scale sets gain a runner_statistics column. It caches the last
// RunnerScaleSetStatistic GitHub sent on the message session, so the API can
// report GitHub's own view of the scale set (jobs waiting, busy and idle
// runners). The stub only declares the new column; AutoMigrate ignores the
// rest of the table.

type scaleSet0012 struct {
	ID               uint `gorm:"primarykey"`
	RunnerStatistics datatypes.JSON
}

func (scaleSet0012) TableName() string { return "scale_sets" }

func init() {
	Register(&gormigrate.Migration{
		ID: "0012_scaleset_runner_statistics",
		Migrate: func(tx *gorm.DB) error {
			if tx.Migrator().HasColumn(&scaleSet0012{}, "RunnerStatistics") {
				return nil
			}
			if err := tx.Migrator().AddColumn(&scaleSet0012{}, "RunnerStatistics"); err != nil {
				return fmt.Errorf("adding scale_sets.runner_statistics: %w", err)
			}
			return nil
		},
	})
}
