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
	"log/slog"

	"github.com/google/uuid"
	"github.com/pkg/errors"
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
			return params.Job{}, errors.Wrap(err, "unmarshaling labels")
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
	}

	if job.InstanceID != nil {
		jobParam.RunnerName = job.Instance.Name
	}
	return jobParam, nil
}

func (s *sqlDatabase) paramsJobToWorkflowJob(ctx context.Context, job params.Job) (WorkflowJob, error) {
	asJSON, err := json.Marshal(job.Labels)
	if err != nil {
		return WorkflowJob{}, errors.Wrap(err, "marshaling labels")
	}

	workflofJob := WorkflowJob{
		ScaleSetJobID:   job.ScaleSetJobID,
		WorkflowJobID:   job.ID,
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
		instance, err := s.getInstanceByName(s.ctx, job.RunnerName)
		if err != nil {
			// This usually is very normal as not all jobs run on our runners.
			slog.DebugContext(ctx, "failed to get instance by name", "instance_name", job.RunnerName)
		} else {
			workflofJob.InstanceID = &instance.ID
		}
	}

	return workflofJob, nil
}

func (s *sqlDatabase) DeleteJob(_ context.Context, jobID int64) (err error) {
	var workflowJob WorkflowJob
	q := s.conn.Where("workflow_job_id = ?", jobID).Preload("Instance").First(&workflowJob)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		return errors.Wrap(q.Error, "fetching job")
	}
	removedJob, err := sqlWorkflowJobToParamsJob(workflowJob)
	if err != nil {
		return errors.Wrap(err, "converting job")
	}

	defer func() {
		if err == nil {
			if notifyErr := s.sendNotify(common.JobEntityType, common.DeleteOperation, removedJob); notifyErr != nil {
				slog.With(slog.Any("error", notifyErr)).Error("failed to send notify")
			}
		}
	}()
	q = s.conn.Delete(&workflowJob)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		return errors.Wrap(q.Error, "deleting job")
	}
	return nil
}

func (s *sqlDatabase) LockJob(_ context.Context, jobID int64, entityID string) error {
	entityUUID, err := uuid.Parse(entityID)
	if err != nil {
		return errors.Wrap(err, "parsing entity id")
	}
	var workflowJob WorkflowJob
	q := s.conn.Preload("Instance").Where("id = ?", jobID).First(&workflowJob)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return runnerErrors.ErrNotFound
		}
		return errors.Wrap(q.Error, "fetching job")
	}

	if workflowJob.LockedBy.String() == entityID {
		// Already locked by us.
		return nil
	}

	if workflowJob.LockedBy != uuid.Nil {
		return runnerErrors.NewConflictError("job is locked by another entity %s", workflowJob.LockedBy.String())
	}

	workflowJob.LockedBy = entityUUID

	if err := s.conn.Save(&workflowJob).Error; err != nil {
		return errors.Wrap(err, "saving job")
	}

	asParams, err := sqlWorkflowJobToParamsJob(workflowJob)
	if err != nil {
		return errors.Wrap(err, "converting job")
	}
	s.sendNotify(common.JobEntityType, common.UpdateOperation, asParams)

	return nil
}

func (s *sqlDatabase) BreakLockJobIsQueued(_ context.Context, jobID int64) (err error) {
	var workflowJob WorkflowJob
	q := s.conn.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Instance").Where("workflow_job_id = ? and status = ?", jobID, params.JobStatusQueued).First(&workflowJob)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		return errors.Wrap(q.Error, "fetching job")
	}

	if workflowJob.LockedBy == uuid.Nil {
		// Job is already unlocked.
		return nil
	}

	workflowJob.LockedBy = uuid.Nil
	if err := s.conn.Save(&workflowJob).Error; err != nil {
		return errors.Wrap(err, "saving job")
	}
	asParams, err := sqlWorkflowJobToParamsJob(workflowJob)
	if err != nil {
		return errors.Wrap(err, "converting job")
	}
	s.sendNotify(common.JobEntityType, common.UpdateOperation, asParams)
	return nil
}

func (s *sqlDatabase) UnlockJob(_ context.Context, jobID int64, entityID string) error {
	var workflowJob WorkflowJob
	q := s.conn.Clauses(clause.Locking{Strength: "UPDATE"}).Where("workflow_job_id = ?", jobID).First(&workflowJob)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return runnerErrors.ErrNotFound
		}
		return errors.Wrap(q.Error, "fetching job")
	}

	if workflowJob.LockedBy == uuid.Nil {
		// Job is already unlocked.
		return nil
	}

	if workflowJob.LockedBy != uuid.Nil && workflowJob.LockedBy.String() != entityID {
		return runnerErrors.NewConflictError("job is locked by another entity %s", workflowJob.LockedBy.String())
	}

	workflowJob.LockedBy = uuid.Nil
	if err := s.conn.Save(&workflowJob).Error; err != nil {
		return errors.Wrap(err, "saving job")
	}

	asParams, err := sqlWorkflowJobToParamsJob(workflowJob)
	if err != nil {
		return errors.Wrap(err, "converting job")
	}
	s.sendNotify(common.JobEntityType, common.UpdateOperation, asParams)
	return nil
}

func (s *sqlDatabase) CreateOrUpdateJob(ctx context.Context, job params.Job) (params.Job, error) {
	var workflowJob WorkflowJob
	var err error

	searchField := "workflow_job_id = ?"
	var searchVal any = job.ID
	if job.ScaleSetJobID != "" {
		searchField = "scale_set_job_id = ?"
		searchVal = job.ScaleSetJobID
	}
	q := s.conn.Preload("Instance").Where(searchField, searchVal).First(&workflowJob)

	if q.Error != nil {
		if !errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return params.Job{}, errors.Wrap(q.Error, "fetching job")
		}
	}
	var operation common.OperationType
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
		if job.RunID != 0 && workflowJob.RunID == 0 {
			workflowJob.RunID = job.RunID
		}

		if job.LockedBy != uuid.Nil {
			workflowJob.LockedBy = job.LockedBy
		}

		if job.RunnerName != "" {
			instance, err := s.getInstanceByName(ctx, job.RunnerName)
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
		if err := s.conn.Save(&workflowJob).Error; err != nil {
			return params.Job{}, errors.Wrap(err, "saving job")
		}
	} else {
		operation = common.CreateOperation

		workflowJob, err = s.paramsJobToWorkflowJob(ctx, job)
		if err != nil {
			return params.Job{}, errors.Wrap(err, "converting job")
		}
		if err := s.conn.Create(&workflowJob).Error; err != nil {
			return params.Job{}, errors.Wrap(err, "creating job")
		}
	}

	asParams, err := sqlWorkflowJobToParamsJob(workflowJob)
	if err != nil {
		return params.Job{}, errors.Wrap(err, "converting job")
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
			return nil, errors.Wrap(err, "converting job")
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
			return nil, errors.Wrap(err, "converting job")
		}
		ret[idx] = jobParam
	}
	return ret, nil
}

func (s *sqlDatabase) ListAllJobs(_ context.Context) ([]params.Job, error) {
	var jobs []WorkflowJob
	query := s.conn.Model(&WorkflowJob{})

	if err := query.Preload("Instance").Find(&jobs); err.Error != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			return []params.Job{}, nil
		}
		return nil, err.Error
	}

	ret := make([]params.Job, len(jobs))
	for idx, job := range jobs {
		jobParam, err := sqlWorkflowJobToParamsJob(job)
		if err != nil {
			return nil, errors.Wrap(err, "converting job")
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

// DeleteCompletedJobs deletes all completed jobs.
func (s *sqlDatabase) DeleteCompletedJobs(_ context.Context) error {
	query := s.conn.Model(&WorkflowJob{}).Where("status = ?", params.JobStatusCompleted)

	if err := query.Unscoped().Delete(&WorkflowJob{}); err.Error != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		return err.Error
	}

	return nil
}
