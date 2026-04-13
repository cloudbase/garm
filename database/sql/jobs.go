// Copyright 2025 Cloudbase Solutions SRL
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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

var _ common.JobsStore = &sqlDatabase{}

func sqlWorkflowJobToParamsJob(job WorkflowJob) (params.Job, error) {
	labels := []string{}
	if job.Labels != nil {
		if err := json.Unmarshal(job.Labels, &labels); err != nil {
			return params.Job{}, fmt.Errorf("error unmarshaling labels: %w", err)
		}
	}

	jobParam := params.Job{
		ID:              job.ID,
		WorkflowJobID:   job.WorkflowJobID,
		ScaleSetJobID:   job.ScaleSetJobID,
		RunID:           job.RunID,
		Action:          job.Action,
		Status:          job.Status,
		Name:            job.Name,
		Conclusion:      job.Conclusion,
		StartedAt:       job.StartedAt,
		CompletedAt:     job.CompletedAt,
		GithubRunnerID:  job.GithubRunnerID,
		RunnerGroupID:   job.RunnerGroupID,
		RunnerGroupName: job.RunnerGroupName,
		RepositoryName:  job.RepositoryName,
		RepositoryOwner: job.RepositoryOwner,
		RepoID:          job.RepoID,
		OrgID:           job.OrgID,
		EnterpriseID:    job.EnterpriseID,
		Labels:          labels,
		CreatedAt:       job.CreatedAt,
		UpdatedAt:       job.UpdatedAt,
		LockedBy:        job.LockedBy,
		WorkflowRunURL:  job.WorkflowRunURL,
	}

	if job.InstanceID != nil {
		jobParam.RunnerName = job.Instance.Name
	}

	return jobParam, nil
}

func (s *sqlDatabase) paramsJobToWorkflowJob(ctx context.Context, conn *gorm.DB, job params.Job) (WorkflowJob, error) {
	asJSON, err := json.Marshal(job.Labels)
	if err != nil {
		return WorkflowJob{}, fmt.Errorf("error marshaling labels: %w", err)
	}

	workflofJob := WorkflowJob{
		ScaleSetJobID:   job.ScaleSetJobID,
		WorkflowJobID:   job.WorkflowJobID,
		RunID:           job.RunID,
		Action:          job.Action,
		Status:          job.Status,
		Name:            job.Name,
		Conclusion:      job.Conclusion,
		StartedAt:       job.StartedAt,
		CompletedAt:     job.CompletedAt,
		GithubRunnerID:  job.GithubRunnerID,
		RunnerGroupID:   job.RunnerGroupID,
		RunnerGroupName: job.RunnerGroupName,
		RepositoryName:  job.RepositoryName,
		RepositoryOwner: job.RepositoryOwner,
		RepoID:          job.RepoID,
		OrgID:           job.OrgID,
		EnterpriseID:    job.EnterpriseID,
		Labels:          asJSON,
		LockedBy:        job.LockedBy,
	}

	if job.RunnerName != "" {
		instance, err := s.getInstance(s.ctx, conn, job.RunnerName)
		if err != nil {
			// This usually is very normal as not all jobs run on our runners.
			slog.DebugContext(ctx, "failed to get instance by name", "instance_name", job.RunnerName)
		} else {
			workflofJob.InstanceID = &instance.ID
		}
	}

	return workflofJob, nil
}

func (s *sqlDatabase) DeleteJob(_ context.Context, jobID int64) error {
	var removedJob params.Job

	err := s.conn.Transaction(func(tx *gorm.DB) error {
		var workflowJob WorkflowJob
		q := tx.Where("workflow_job_id = ?", jobID).Preload("Instance").First(&workflowJob)
		if q.Error != nil {
			if errors.Is(q.Error, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("error fetching job: %w", q.Error)
		}

		var err error
		removedJob, err = sqlWorkflowJobToParamsJob(workflowJob)
		if err != nil {
			return fmt.Errorf("error converting job: %w", err)
		}

		q = tx.Delete(&workflowJob)
		if q.Error != nil {
			if errors.Is(q.Error, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("error deleting job: %w", q.Error)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if removedJob.ID != 0 {
		if notifyErr := s.sendNotify(common.JobEntityType, common.DeleteOperation, removedJob); notifyErr != nil {
			slog.With(slog.Any("error", notifyErr)).Error("failed to send notify")
		}
	}
	return nil
}

func (s *sqlDatabase) LockJob(_ context.Context, jobID int64, entityID string) error {
	entityUUID, err := uuid.Parse(entityID)
	if err != nil {
		return fmt.Errorf("error parsing entity id: %w", err)
	}

	var asParams params.Job

	err = s.conn.Transaction(func(tx *gorm.DB) error {
		var workflowJob WorkflowJob
		q := tx.Preload("Instance").Where("workflow_job_id = ?", jobID).First(&workflowJob)

		if q.Error != nil {
			if errors.Is(q.Error, gorm.ErrRecordNotFound) {
				return runnerErrors.ErrNotFound
			}
			return fmt.Errorf("error fetching job: %w", q.Error)
		}

		if workflowJob.LockedBy.String() == entityID {
			// Already locked by us.
			return nil
		}

		if workflowJob.LockedBy != uuid.Nil {
			return runnerErrors.NewConflictError("job is locked by another entity %s", workflowJob.LockedBy.String())
		}

		workflowJob.LockedBy = entityUUID

		if err := tx.Save(&workflowJob).Error; err != nil {
			return fmt.Errorf("error saving job: %w", err)
		}

		var err error
		asParams, err = sqlWorkflowJobToParamsJob(workflowJob)
		if err != nil {
			return fmt.Errorf("error converting job: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if asParams.ID != 0 {
		s.sendNotify(common.JobEntityType, common.UpdateOperation, asParams)
	}
	return nil
}

func (s *sqlDatabase) BreakLockJobIsQueued(_ context.Context, jobID int64) error {
	var asParams params.Job

	err := s.conn.Transaction(func(tx *gorm.DB) error {
		var workflowJob WorkflowJob
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Instance").Where("workflow_job_id = ? and status = ?", jobID, params.JobStatusQueued).First(&workflowJob)

		if q.Error != nil {
			if errors.Is(q.Error, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("error fetching job: %w", q.Error)
		}

		if workflowJob.LockedBy == uuid.Nil {
			// Job is already unlocked.
			return nil
		}

		workflowJob.LockedBy = uuid.Nil
		if err := tx.Save(&workflowJob).Error; err != nil {
			return fmt.Errorf("error saving job: %w", err)
		}

		var err error
		asParams, err = sqlWorkflowJobToParamsJob(workflowJob)
		if err != nil {
			return fmt.Errorf("error converting job: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if asParams.ID != 0 {
		s.sendNotify(common.JobEntityType, common.UpdateOperation, asParams)
	}
	return nil
}

func (s *sqlDatabase) UnlockJob(_ context.Context, jobID int64, entityID string) error {
	var asParams params.Job

	err := s.conn.Transaction(func(tx *gorm.DB) error {
		var workflowJob WorkflowJob
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workflow_job_id = ?", jobID).First(&workflowJob)

		if q.Error != nil {
			if errors.Is(q.Error, gorm.ErrRecordNotFound) {
				return runnerErrors.ErrNotFound
			}
			return fmt.Errorf("error fetching job: %w", q.Error)
		}

		if workflowJob.LockedBy == uuid.Nil {
			// Job is already unlocked.
			return nil
		}

		if workflowJob.LockedBy != uuid.Nil && workflowJob.LockedBy.String() != entityID {
			return runnerErrors.NewConflictError("job is locked by another entity %s", workflowJob.LockedBy.String())
		}

		workflowJob.LockedBy = uuid.Nil
		if err := tx.Save(&workflowJob).Error; err != nil {
			return fmt.Errorf("error saving job: %w", err)
		}

		var err error
		asParams, err = sqlWorkflowJobToParamsJob(workflowJob)
		if err != nil {
			return fmt.Errorf("error converting job: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if asParams.ID != 0 {
		s.sendNotify(common.JobEntityType, common.UpdateOperation, asParams)
	}
	return nil
}

func (s *sqlDatabase) CreateOrUpdateJob(ctx context.Context, job params.Job) (params.Job, error) {
	var asParams params.Job
	var operation common.OperationType

	err := s.conn.Transaction(func(tx *gorm.DB) error {
		var workflowJob WorkflowJob

		searchField := "workflow_job_id = ?"
		var searchVal any = job.WorkflowJobID
		if job.ScaleSetJobID != "" {
			searchField = "scale_set_job_id = ?"
			searchVal = job.ScaleSetJobID
		}
		q := tx.Preload("Instance").Where(searchField, searchVal).First(&workflowJob)

		if q.Error != nil {
			if !errors.Is(q.Error, gorm.ErrRecordNotFound) {
				return fmt.Errorf("error fetching job: %w", q.Error)
			}
		}

		if workflowJob.ID != 0 {
			// Update workflowJob with values from job.
			operation = common.UpdateOperation

			workflowJob.Status = job.Status
			workflowJob.Action = job.Action
			workflowJob.Conclusion = job.Conclusion
			workflowJob.StartedAt = job.StartedAt
			workflowJob.CompletedAt = job.CompletedAt
			workflowJob.GithubRunnerID = job.GithubRunnerID
			workflowJob.RunnerGroupID = job.RunnerGroupID
			workflowJob.RunnerGroupName = job.RunnerGroupName
			workflowJob.WorkflowRunURL = job.WorkflowRunURL
			if job.RunID != 0 && workflowJob.RunID == 0 {
				workflowJob.RunID = job.RunID
			}

			if job.LockedBy != uuid.Nil {
				workflowJob.LockedBy = job.LockedBy
			}

			if job.RunnerName != "" {
				instance, err := s.getInstance(ctx, tx, job.RunnerName)
				if err == nil {
					workflowJob.InstanceID = &instance.ID
				} else {
					// This usually is very normal as not all jobs run on our runners.
					slog.DebugContext(ctx, "failed to get instance by name", "instance_name", job.RunnerName)
				}
			}

			if job.RepoID != nil {
				workflowJob.RepoID = job.RepoID
			}

			if job.OrgID != nil {
				workflowJob.OrgID = job.OrgID
			}

			if job.EnterpriseID != nil {
				workflowJob.EnterpriseID = job.EnterpriseID
			}
			if err := tx.Save(&workflowJob).Error; err != nil {
				return fmt.Errorf("error saving job: %w", err)
			}
		} else {
			operation = common.CreateOperation

			var err error
			workflowJob, err = s.paramsJobToWorkflowJob(ctx, tx, job)
			if err != nil {
				return fmt.Errorf("error converting job: %w", err)
			}
			workflowJob.WorkflowRunURL = job.WorkflowRunURL
			if err := tx.Create(&workflowJob).Error; err != nil {
				return fmt.Errorf("error creating job: %w", err)
			}
		}

		var err error
		asParams, err = sqlWorkflowJobToParamsJob(workflowJob)
		if err != nil {
			return fmt.Errorf("error converting job: %w", err)
		}
		return nil
	})
	if err != nil {
		return params.Job{}, err
	}

	s.sendNotify(common.JobEntityType, operation, asParams)

	return asParams, nil
}

// ListJobsByStatus lists all jobs for a given status.
func (s *sqlDatabase) ListJobsByStatus(_ context.Context, status params.JobStatus) ([]params.Job, error) {
	var jobs []WorkflowJob
	query := s.conn.Model(&WorkflowJob{}).Preload("Instance").Where("status = ?", status)

	if err := query.Find(&jobs); err.Error != nil {
		return nil, err.Error
	}

	ret := make([]params.Job, len(jobs))
	for idx, job := range jobs {
		jobParam, err := sqlWorkflowJobToParamsJob(job)
		if err != nil {
			return nil, fmt.Errorf("error converting job: %w", err)
		}
		ret[idx] = jobParam
	}
	return ret, nil
}

// ListEntityJobsByStatus lists all jobs for a given entity type and id.
func (s *sqlDatabase) ListEntityJobsByStatus(_ context.Context, entityType params.ForgeEntityType, entityID string, status params.JobStatus) ([]params.Job, error) {
	u, err := uuid.Parse(entityID)
	if err != nil {
		return nil, err
	}

	var jobs []WorkflowJob
	query := s.conn.
		Model(&WorkflowJob{}).
		Preload("Instance").
		Where("status = ?", status).
		Where("workflow_job_id > 0")

	switch entityType {
	case params.ForgeEntityTypeOrganization:
		query = query.Where("org_id = ?", u)
	case params.ForgeEntityTypeRepository:
		query = query.Where("repo_id = ?", u)
	case params.ForgeEntityTypeEnterprise:
		query = query.Where("enterprise_id = ?", u)
	}

	if err := query.Find(&jobs); err.Error != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			return []params.Job{}, nil
		}
		return nil, err.Error
	}

	ret := make([]params.Job, len(jobs))
	for idx, job := range jobs {
		jobParam, err := sqlWorkflowJobToParamsJob(job)
		if err != nil {
			return nil, fmt.Errorf("error converting job: %w", err)
		}
		ret[idx] = jobParam
	}
	return ret, nil
}

func (s *sqlDatabase) ListAllJobs(_ context.Context) ([]params.Job, error) {
	var jobs []WorkflowJob
	query := s.conn.Model(&WorkflowJob{})

	if err := query.
		Preload("Instance").
		Find(&jobs); err.Error != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			return []params.Job{}, nil
		}
		return nil, err.Error
	}

	ret := make([]params.Job, len(jobs))
	for idx, job := range jobs {
		jobParam, err := sqlWorkflowJobToParamsJob(job)
		if err != nil {
			return nil, fmt.Errorf("error converting job: %w", err)
		}
		ret[idx] = jobParam
	}
	return ret, nil
}

// GetJobByID gets a job by id.
func (s *sqlDatabase) GetJobByID(_ context.Context, jobID int64) (params.Job, error) {
	var job WorkflowJob
	query := s.conn.Model(&WorkflowJob{}).Preload("Instance").Where("workflow_job_id = ?", jobID)

	if err := query.First(&job); err.Error != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			return params.Job{}, runnerErrors.ErrNotFound
		}
		return params.Job{}, err.Error
	}

	return sqlWorkflowJobToParamsJob(job)
}

// DeleteInactionableJobs will delete jobs that are not in queued state and have no
// runner associated with them. This can happen if we have a pool that matches labels
// defined on a job, but the job itself was picked up by a runner we don't manage.
// When a job transitions from queued to anything else, GARM only uses them for informational
// purposes. So they are safe to delete.
// Also deletes completed jobs with GARM runners attached as they are no longer needed.
func (s *sqlDatabase) DeleteInactionableJobs(_ context.Context, olderThan time.Duration) error {
	// Fetch and delete within a transaction to avoid races.
	var jobs []WorkflowJob

	err := s.conn.Transaction(func(tx *gorm.DB) error {
		q := tx.
			Model(&WorkflowJob{}).
			Preload("Instance").
			Where("(status != ? AND instance_id IS NULL) OR (status = ? AND instance_id IS NOT NULL)", params.JobStatusQueued, params.JobStatusCompleted)
		if olderThan > 0 {
			q = q.Where("created_at < ?", time.Now().Add(-olderThan))
		}
		if err := q.Find(&jobs).Error; err != nil {
			return fmt.Errorf("fetching inactionable jobs: %w", err)
		}

		if len(jobs) == 0 {
			return nil
		}

		ids := make([]int64, len(jobs))
		for i, j := range jobs {
			ids[i] = j.ID
		}

		if err := tx.Unscoped().Where("id IN ?", ids).Delete(&WorkflowJob{}).Error; err != nil {
			return fmt.Errorf("deleting inactionable jobs: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	for _, j := range jobs {
		asParams, err := sqlWorkflowJobToParamsJob(j)
		if err != nil {
			slog.With(slog.Any("error", err)).Error("failed to convert job for notify")
			continue
		}
		if notifyErr := s.sendNotify(common.JobEntityType, common.DeleteOperation, asParams); notifyErr != nil {
			slog.With(slog.Any("error", notifyErr)).Error("failed to send delete notify for job")
		}
	}

	return nil
}
