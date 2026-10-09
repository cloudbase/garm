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

package sql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

var _ common.ScaleSetJobsStore = &sqlDatabase{}

func sqlScaleSetJobToParams(job ScaleSetJob) (params.ScaleSetJob, error) {
	labels := []string{}
	if job.RequestLabels != nil {
		if err := json.Unmarshal(job.RequestLabels, &labels); err != nil {
			return params.ScaleSetJob{}, fmt.Errorf("error unmarshaling request labels: %w", err)
		}
	}

	ret := params.ScaleSetJob{
		ID:                 job.ID,
		ScaleSetJobID:      job.ScaleSetJobID,
		WorkflowRunID:      job.WorkflowRunID,
		RunnerRequestID:    job.RunnerRequestID,
		JobWorkflowRef:     job.JobWorkflowRef,
		Name:               job.Name,
		Status:             job.Status,
		Result:             job.Result,
		EventName:          job.EventName,
		RequestLabels:      labels,
		RepositoryName:     job.RepositoryName,
		RepositoryOwner:    job.RepositoryOwner,
		RunnerGroupName:    job.RunnerGroupName,
		RunnerID:           job.RunnerID,
		RunnerName:         job.RunnerName,
		WorkflowRunURL:     job.WorkflowRunURL,
		QueueTime:          job.QueueTime,
		ScaleSetAssignTime: job.ScaleSetAssignTime,
		RunnerAssignTime:   job.RunnerAssignTime,
		FinishTime:         job.FinishTime,
		CreatedAt:          job.CreatedAt,
		UpdatedAt:          job.UpdatedAt,
	}

	if job.ScaleSetFkID != nil {
		ret.ScaleSetID = *job.ScaleSetFkID
	}
	return ret, nil
}

// scaleSetJobStatusRank orders job statuses so a message redelivered after
// a restart cannot move a job backwards.
func scaleSetJobStatusRank(status string) int {
	switch params.JobStatus(status) {
	case params.JobStatusQueued:
		return 1
	case params.JobStatusInProgress:
		return 2
	case params.JobStatusCompleted:
		return 3
	}
	return 0
}

func (s *sqlDatabase) CreateOrUpdateScaleSetJob(_ context.Context, job params.ScaleSetJob) (params.ScaleSetJob, error) {
	if job.ScaleSetJobID == "" {
		return params.ScaleSetJob{}, fmt.Errorf("scale set job has no ID")
	}

	asJSON, err := json.Marshal(job.RequestLabels)
	if err != nil {
		return params.ScaleSetJob{}, fmt.Errorf("error marshaling request labels: %w", err)
	}

	var asParams params.ScaleSetJob
	var operation common.OperationType

	err = s.conn.Transaction(func(tx *gorm.DB) error {
		var dbJob ScaleSetJob
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("scale_set_job_id = ?", job.ScaleSetJobID).First(&dbJob)
		if q.Error != nil {
			if !errors.Is(q.Error, gorm.ErrRecordNotFound) {
				return fmt.Errorf("error fetching scale set job: %w", q.Error)
			}
		}

		if dbJob.ID == 0 {
			operation = common.CreateOperation
			dbJob = ScaleSetJob{
				ScaleSetJobID:      job.ScaleSetJobID,
				WorkflowRunID:      job.WorkflowRunID,
				RunnerRequestID:    job.RunnerRequestID,
				JobWorkflowRef:     job.JobWorkflowRef,
				Name:               job.Name,
				Status:             job.Status,
				Result:             job.Result,
				EventName:          job.EventName,
				RequestLabels:      asJSON,
				RepositoryName:     job.RepositoryName,
				RepositoryOwner:    job.RepositoryOwner,
				RunnerGroupName:    job.RunnerGroupName,
				RunnerID:           job.RunnerID,
				RunnerName:         job.RunnerName,
				WorkflowRunURL:     job.WorkflowRunURL,
				QueueTime:          job.QueueTime,
				ScaleSetAssignTime: job.ScaleSetAssignTime,
				RunnerAssignTime:   job.RunnerAssignTime,
				FinishTime:         job.FinishTime,
			}
			if job.ScaleSetID != 0 {
				scaleSetID := job.ScaleSetID
				dbJob.ScaleSetFkID = &scaleSetID
			}
			if err := tx.Create(&dbJob).Error; err != nil {
				return fmt.Errorf("error creating scale set job: %w", err)
			}
		} else {
			operation = common.UpdateOperation
			if scaleSetJobStatusRank(job.Status) >= scaleSetJobStatusRank(dbJob.Status) {
				dbJob.Status = job.Status
				dbJob.Result = job.Result
			}
			if job.RunnerID != 0 {
				dbJob.RunnerID = job.RunnerID
			}
			if job.RunnerName != "" {
				dbJob.RunnerName = job.RunnerName
			}
			if job.ScaleSetID != 0 {
				scaleSetID := job.ScaleSetID
				dbJob.ScaleSetFkID = &scaleSetID
			}
			if !job.QueueTime.IsZero() {
				dbJob.QueueTime = job.QueueTime
			}
			if !job.ScaleSetAssignTime.IsZero() {
				dbJob.ScaleSetAssignTime = job.ScaleSetAssignTime
			}
			if !job.RunnerAssignTime.IsZero() {
				dbJob.RunnerAssignTime = job.RunnerAssignTime
			}
			if !job.FinishTime.IsZero() {
				dbJob.FinishTime = job.FinishTime
			}
			if err := tx.Save(&dbJob).Error; err != nil {
				return fmt.Errorf("error saving scale set job: %w", err)
			}
		}

		var convErr error
		asParams, convErr = sqlScaleSetJobToParams(dbJob)
		if convErr != nil {
			return fmt.Errorf("error converting scale set job: %w", convErr)
		}
		return nil
	})
	if err != nil {
		return params.ScaleSetJob{}, err
	}

	s.sendNotify(common.ScaleSetJobEntityType, operation, asParams)
	return asParams, nil
}

func (s *sqlDatabase) ListScaleSetJobs(_ context.Context, scaleSetID uint) ([]params.ScaleSetJob, error) {
	var jobs []ScaleSetJob
	q := s.conn.Model(&ScaleSetJob{}).Where("scale_set_fk_id = ?", scaleSetID).Order("id desc").Find(&jobs)
	if q.Error != nil {
		return nil, fmt.Errorf("error fetching scale set jobs: %w", q.Error)
	}

	ret := make([]params.ScaleSetJob, len(jobs))
	for idx, job := range jobs {
		converted, err := sqlScaleSetJobToParams(job)
		if err != nil {
			return nil, fmt.Errorf("error converting scale set job: %w", err)
		}
		ret[idx] = converted
	}
	return ret, nil
}

func (s *sqlDatabase) ListAllScaleSetJobs(_ context.Context) ([]params.ScaleSetJob, error) {
	var jobs []ScaleSetJob
	q := s.conn.Model(&ScaleSetJob{}).Order("id desc").Find(&jobs)
	if q.Error != nil {
		return nil, fmt.Errorf("error fetching scale set jobs: %w", q.Error)
	}

	ret := make([]params.ScaleSetJob, len(jobs))
	for idx, job := range jobs {
		converted, err := sqlScaleSetJobToParams(job)
		if err != nil {
			return nil, fmt.Errorf("error converting scale set job: %w", err)
		}
		ret[idx] = converted
	}
	return ret, nil
}

// DeleteOldScaleSetJobs prunes job records that saw no update for the given
// duration, regardless of status. The records are informational and the
// scale set listener refreshes any job that is still live, so a record this
// stale belongs to a job we will never hear about again.
func (s *sqlDatabase) DeleteOldScaleSetJobs(_ context.Context, olderThan time.Duration) error {
	var jobs []ScaleSetJob

	err := s.conn.Transaction(func(tx *gorm.DB) error {
		q := tx.
			Model(&ScaleSetJob{}).
			Where("updated_at < ?", time.Now().Add(-olderThan))
		if err := q.Find(&jobs).Error; err != nil {
			return fmt.Errorf("fetching completed scale set jobs: %w", err)
		}

		if len(jobs) == 0 {
			return nil
		}

		ids := make([]uint, len(jobs))
		for i, j := range jobs {
			ids[i] = j.ID
		}

		if err := tx.Where("id IN ?", ids).Delete(&ScaleSetJob{}).Error; err != nil {
			return fmt.Errorf("deleting completed scale set jobs: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, j := range jobs {
		asParams, err := sqlScaleSetJobToParams(j)
		if err != nil {
			slog.With(slog.Any("error", err)).Error("failed to convert scale set job for notify")
			continue
		}
		if notifyErr := s.sendNotify(common.ScaleSetJobEntityType, common.DeleteOperation, asParams); notifyErr != nil {
			slog.With(slog.Any("error", notifyErr)).Error("failed to send delete notify for scale set job")
		}
	}
	return nil
}
