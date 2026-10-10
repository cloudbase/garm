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
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Webhook jobs gain a pool_id column. It records which pool's runner picked
// up the job, so per pool stats can be derived from job records. The stubs
// mirror the relation declarations of the real models so GORM derives
// identical constraint names.

type workflowJob0011 struct {
	ID int64 `gorm:"index"`

	// The real model derives the column type from the pools primary key
	// through the relation. The stub has no relation, so spell it out or
	// PostgreSQL gets a text column that cannot reference a uuid.
	PoolID *uuid.UUID `gorm:"type:uuid;index"`
}

func (workflowJob0011) TableName() string { return "workflow_jobs" }

type pool0011 struct {
	ID uuid.UUID `gorm:"type:uuid;primary_key;"`

	Jobs []workflowJob0011 `gorm:"foreignKey:PoolID;constraint:OnDelete:SET NULL"`
}

func (pool0011) TableName() string { return "pools" }

func init() {
	Register(&gormigrate.Migration{
		ID: "0011_job_pool_attribution",
		Migrate: func(tx *gorm.DB) error {
			// The stub has no relation fields, so this only adds the new
			// column and its index.
			if err := tx.AutoMigrate(&workflowJob0011{}); err != nil {
				return fmt.Errorf("adding job pool attribution column: %w", err)
			}

			createConstraint := func() error {
				if tx.Migrator().HasConstraint(&pool0011{}, "fk_pools_jobs") {
					return nil
				}
				if err := tx.Migrator().CreateConstraint(&pool0011{}, "fk_pools_jobs"); err != nil {
					return fmt.Errorf("creating constraint fk_pools_jobs: %w", err)
				}
				return nil
			}

			if tx.Name() != "sqlite" {
				return createConstraint()
			}

			// Adding the constraint rebuilds the table on SQLite. The new
			// column is NULL everywhere, which means the added constraint
			// cannot be violated.
			return sqliteTableRebuild(tx, []string{"workflow_jobs"}, createConstraint)
		},
	})
}
