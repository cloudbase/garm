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
	"errors"
	"fmt"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Scale set jobs move out of workflow_jobs into their own table. They are
// an informational ledger of jobs GitHub routed to a scale set, while
// workflow_jobs holds the jobs pools act on. The stubs mirror the relation
// declarations of the real models so GORM derives identical constraint
// names.

type scaleSet0010 struct {
	ID uint `gorm:"primarykey"`

	Jobs []scaleSetJob0010 `gorm:"foreignKey:ScaleSetFkID;constraint:OnDelete:CASCADE"`
}

func (scaleSet0010) TableName() string { return "scale_sets" }

type scaleSetJob0010 struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time

	ScaleSetJobID string `gorm:"uniqueIndex"`

	ScaleSetFkID *uint        `gorm:"index"`
	ScaleSet     scaleSet0010 `gorm:"foreignKey:ScaleSetFkID"`

	WorkflowRunID   int64 `gorm:"index"`
	RunnerRequestID int64
	JobWorkflowRef  string
	Name            string
	Status          string `gorm:"index"`
	Result          string
	EventName       string
	RequestLabels   datatypes.JSON
	RepositoryName  string
	RepositoryOwner string
	RunnerGroupName string
	RunnerID        int64
	RunnerName      string
	WorkflowRunURL  string

	QueueTime          time.Time
	ScaleSetAssignTime time.Time
	RunnerAssignTime   time.Time
	FinishTime         time.Time
}

func (scaleSetJob0010) TableName() string { return "scale_set_jobs" }

type workflowJob0010 struct {
	ScaleSetJobID string
}

func (workflowJob0010) TableName() string { return "workflow_jobs" }

// moveScaleSetJobs copies the scale set job records out of workflow_jobs
// and removes them from the old table. The owning scale set was never
// recorded, so scale_set_fk_id stays NULL for moved rows. Runner names were
// stored as instance references and the instance may be long gone, so take
// the name when it is still there.
//
// Migrations run without a transaction, so a crash between the copy and
// the delete leaves the rows in both tables. The conflict clause makes the
// rerun skip rows that were already copied instead of tripping over the
// unique scale set job ID. It also drops historical duplicates, which the
// old table never guarded against.
func moveScaleSetJobs(tx *gorm.DB) error {
	moveSQL := `
INSERT INTO scale_set_jobs (
    created_at, updated_at, scale_set_job_id, workflow_run_id, name, status,
    result, event_name, request_labels, repository_name, repository_owner,
    runner_group_name, runner_id, runner_name, workflow_run_url,
    runner_assign_time, finish_time
)
SELECT
    w.created_at, w.updated_at, w.scale_set_job_id, w.run_id, w.name, w.status,
    w.conclusion, w.action, w.labels, w.repository_name, w.repository_owner,
    w.runner_group_name, w.github_runner_id, COALESCE(i.name, ''), w.workflow_run_url,
    w.started_at, w.completed_at
FROM workflow_jobs w
LEFT JOIN instances i ON i.id = w.instance_id
WHERE w.scale_set_job_id IS NOT NULL AND w.scale_set_job_id != '' AND w.deleted_at IS NULL
ON CONFLICT DO NOTHING`
	if err := tx.Exec(moveSQL).Error; err != nil {
		return fmt.Errorf("moving scale set jobs: %w", err)
	}

	if err := tx.Exec("DELETE FROM workflow_jobs WHERE scale_set_job_id IS NOT NULL AND scale_set_job_id != ''").Error; err != nil {
		return fmt.Errorf("removing moved scale set jobs: %w", err)
	}
	return nil
}

func init() {
	Register(&gormigrate.Migration{
		ID: "0010_scaleset_jobs",
		Migrate: func(tx *gorm.DB) error {
			// Creating a new table emits the foreign key inline, so no
			// table rebuild happens on SQLite here.
			if err := tx.AutoMigrate(&scaleSetJob0010{}); err != nil {
				return fmt.Errorf("creating scale_set_jobs: %w", err)
			}

			if err := moveScaleSetJobs(tx); err != nil {
				return err
			}

			// The index goes first so the capture below does not try to
			// restore an index on a column that no longer exists.
			if err := tx.Exec("DROP INDEX IF EXISTS scaleset_job_id_idx").Error; err != nil {
				return fmt.Errorf("dropping scaleset_job_id_idx: %w", err)
			}

			if tx.Name() != "sqlite" {
				if err := tx.Migrator().DropColumn(&workflowJob0010{}, "scale_set_job_id"); err != nil {
					return fmt.Errorf("dropping scale_set_job_id: %w", err)
				}
				return nil
			}

			// SQLite cannot drop a column in place, so GORM rebuilds the
			// table. With foreign keys enforced, dropping a table that has
			// child rows fails, so wrap the rebuild in SQLite's documented
			// ALTER procedure. The rebuild also drops the table's standalone
			// indexes without recreating them, so capture their DDL first
			// and restore any that went missing. See 0008 for details.
			var indexes []struct {
				Name string
				SQL  string
			}
			err := tx.Raw(`SELECT name, sql FROM sqlite_master WHERE type='index' AND sql IS NOT NULL AND tbl_name = 'workflow_jobs'`).Scan(&indexes).Error
			if err != nil {
				return fmt.Errorf("capturing index definitions: %w", err)
			}

			if err := tx.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
				return fmt.Errorf("disabling foreign keys: %w", err)
			}
			migrateErr := tx.Migrator().DropColumn(&workflowJob0010{}, "scale_set_job_id")
			if err := tx.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
				return errors.Join(migrateErr, fmt.Errorf("re-enabling foreign keys: %w", err))
			}
			if migrateErr != nil {
				return fmt.Errorf("dropping scale_set_job_id: %w", migrateErr)
			}

			for _, index := range indexes {
				var count int64
				if err := tx.Raw("SELECT count(*) FROM sqlite_master WHERE type='index' AND name = ?", index.Name).Scan(&count).Error; err != nil {
					return fmt.Errorf("checking index %s: %w", index.Name, err)
				}
				if count == 0 {
					if err := tx.Exec(index.SQL).Error; err != nil {
						return fmt.Errorf("restoring index %s: %w", index.Name, err)
					}
				}
			}
			return nil
		},
	})
}
