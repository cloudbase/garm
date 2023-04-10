package sql

import (
	"context"
	"encoding/json"

	"github.com/cloudbase/garm/database/common"
	runnerErrors "github.com/cloudbase/garm/errors"
	"github.com/cloudbase/garm/params"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ common.JobsStore = &sqlDatabase{}

func sqlWorkflowJobToParamsJob(job WorkflowJob) (params.Job, error) {
	labels := []string{}
	if job.Labels != nil {
		if err := json.Unmarshal(job.Labels, &labels); err != nil {
			return params.Job{}, errors.Wrap(err, "unmarshaling labels")
		}
	}
	return params.Job{
		ID:              job.ID,
		RunID:           job.RunID,
		Action:          job.Action,
		Status:          job.Status,
		Name:            job.Name,
		Conclusion:      job.Conclusion,
		StartedAt:       job.StartedAt,
		CompletedAt:     job.CompletedAt,
		GithubRunnerID:  job.GithubRunnerID,
		RunnerName:      job.RunnerName,
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
	}, nil
}

func paramsJobToWorkflowJob(job params.Job) (WorkflowJob, error) {
	asJson, err := json.Marshal(job.Labels)
	if err != nil {
		return WorkflowJob{}, errors.Wrap(err, "marshaling labels")
	}
	return WorkflowJob{
		ID:              job.ID,
		RunID:           job.RunID,
		Action:          job.Action,
		Status:          job.Status,
		Name:            job.Name,
		Conclusion:      job.Conclusion,
		StartedAt:       job.StartedAt,
		CompletedAt:     job.CompletedAt,
		GithubRunnerID:  job.GithubRunnerID,
		RunnerName:      job.RunnerName,
		RunnerGroupID:   job.RunnerGroupID,
		RunnerGroupName: job.RunnerGroupName,
		RepositoryName:  job.RepositoryName,
		RepositoryOwner: job.RepositoryOwner,
		RepoID:          job.RepoID,
		OrgID:           job.OrgID,
		EnterpriseID:    job.EnterpriseID,
		Labels:          asJson,
		LockedBy:        job.LockedBy,
	}, nil
}

func (s *sqlDatabase) DeleteJob(ctx context.Context, jobID int64) error {
	q := s.conn.Delete(&WorkflowJob{}, jobID)
	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		return errors.Wrap(q.Error, "deleting job")
	}
	return nil
}

func (s *sqlDatabase) LockJob(ctx context.Context, jobID int64, entityID string) error {
	entityUUID, err := uuid.Parse(entityID)
	if err != nil {
		return errors.Wrap(err, "parsing entity id")
	}
	var workflowJob WorkflowJob
	q := s.conn.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", jobID).First(&workflowJob)

	if q.Error != nil {
		if errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return runnerErrors.ErrNotFound
		}
		return errors.Wrap(q.Error, "fetching job")
	}

	if workflowJob.LockedBy != uuid.Nil {
		return runnerErrors.NewConflictError("job is locked by another entity %s", workflowJob.LockedBy.String())
	}

	workflowJob.LockedBy = entityUUID

	if err := s.conn.Save(&workflowJob).Error; err != nil {
		return errors.Wrap(err, "saving job")
	}

	return nil
}

func (s *sqlDatabase) UnlockJob(ctx context.Context, jobID int64, entityID string) error {
	var workflowJob WorkflowJob
	q := s.conn.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", jobID).First(&workflowJob)

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

	return nil
}

func (s *sqlDatabase) CreateOrUpdateJob(ctx context.Context, job params.Job) (params.Job, error) {
	var workflowJob WorkflowJob
	q := s.conn.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", job.ID).First(&workflowJob)

	if q.Error != nil {
		if !errors.Is(q.Error, gorm.ErrRecordNotFound) {
			return params.Job{}, errors.Wrap(q.Error, "fetching job")
		}
	}

	if workflowJob.ID != 0 {
		// Update workflowJob with values from job.
		workflowJob.Status = job.Status
		workflowJob.Action = job.Action
		workflowJob.Conclusion = job.Conclusion
		workflowJob.StartedAt = job.StartedAt
		workflowJob.CompletedAt = job.CompletedAt
		workflowJob.GithubRunnerID = job.GithubRunnerID
		workflowJob.RunnerGroupID = job.RunnerGroupID
		workflowJob.RunnerGroupName = job.RunnerGroupName

		if job.LockedBy != uuid.Nil {
			workflowJob.LockedBy = job.LockedBy
		}

		if job.RunnerName != "" {
			workflowJob.RunnerName = job.RunnerName
		}

		if job.RepoID != uuid.Nil {
			workflowJob.RepoID = job.RepoID
		}

		if job.OrgID != uuid.Nil {
			workflowJob.OrgID = job.OrgID
		}

		if job.EnterpriseID != uuid.Nil {
			workflowJob.EnterpriseID = job.EnterpriseID
		}
		if err := s.conn.Save(&workflowJob).Error; err != nil {
			return params.Job{}, errors.Wrap(err, "saving job")
		}
	} else {
		workflowJob, err := paramsJobToWorkflowJob(job)
		if err != nil {
			return params.Job{}, errors.Wrap(err, "converting job")
		}
		if err := s.conn.Create(&workflowJob).Error; err != nil {
			return params.Job{}, errors.Wrap(err, "creating job")
		}
	}

	return sqlWorkflowJobToParamsJob(workflowJob)
}

// ListJobsByStatus lists all jobs for a given status.
func (s *sqlDatabase) ListJobsByStatus(ctx context.Context, status params.JobStatus) ([]params.Job, error) {
	var jobs []WorkflowJob
	query := s.conn.Model(&WorkflowJob{}).Where("status = ?", status)

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
func (s *sqlDatabase) ListEntityJobsByStatus(ctx context.Context, entityType params.PoolType, entityID string, status params.JobStatus) ([]params.Job, error) {
	u, err := uuid.Parse(entityID)
	if err != nil {
		return nil, err
	}

	var jobs []WorkflowJob
	query := s.conn.Model(&WorkflowJob{}).Where("status = ?", status)

	switch entityType {
	case params.OrganizationPool:
		query = query.Where("org_id = ?", u)
	case params.RepositoryPool:
		query = query.Where("repo_id = ?", u)
	case params.EnterprisePool:
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

// GetJobByID gets a job by id.
func (s *sqlDatabase) GetJobByID(ctx context.Context, jobID int64) (params.Job, error) {
	var job WorkflowJob
	query := s.conn.Model(&WorkflowJob{}).Where("id = ?", jobID)

	if err := query.First(&job); err.Error != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			return params.Job{}, runnerErrors.ErrNotFound
		}
		return params.Job{}, err.Error
	}

	return sqlWorkflowJobToParamsJob(job)
}

// DeleteCompletedJobs deletes all completed jobs.
func (s *sqlDatabase) DeleteCompletedJobs(ctx context.Context) error {
	query := s.conn.Model(&WorkflowJob{}).Where("status = ?", params.JobStatusCompleted)

	if err := query.Unscoped().Delete(&WorkflowJob{}); err.Error != nil {
		if errors.Is(err.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		return err.Error
	}

	return nil
}
