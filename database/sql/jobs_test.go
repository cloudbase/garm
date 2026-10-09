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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type JobsTestSuite struct {
	suite.Suite
	Store    dbCommon.Store
	adminCtx context.Context
}

func (s *JobsTestSuite) SetupTest() {
	ctx := context.Background()
	watcher.InitWatcher(ctx)

	// Create testing sqlite database
	db := newTestDB(s.T())
	s.Store = db

	adminCtx := garmTesting.ImpersonateAdminContext(ctx, db, s.T())
	s.adminCtx = adminCtx
}

func (s *JobsTestSuite) TearDownTest() {
	watcher.CloseWatcher()
}

func TestJobsTestSuite(t *testing.T) {
	suite.Run(t, new(JobsTestSuite))
}

// TestDeleteInactionableJobs verifies the deletion logic for jobs
func (s *JobsTestSuite) TestDeleteInactionableJobs() {
	db := s.Store.(*sqlDatabase)

	// Create mix of jobs to test all conditions:
	// 1. Queued jobs (should NOT be deleted)
	queuedJob := params.Job{
		WorkflowJobID:   12345,
		RunID:           67890,
		Action:          "test-action",
		Status:          string(params.JobStatusQueued),
		Name:            "queued-job",
		RepositoryName:  "test-repo",
		RepositoryOwner: "test-owner",
	}
	_, err := s.Store.CreateOrUpdateJob(s.adminCtx, queuedJob)
	s.Require().NoError(err)

	// 2. In-progress job without instance (should be deleted)
	inProgressNoInstance := params.Job{
		WorkflowJobID:   12346,
		RunID:           67890,
		Action:          "test-action",
		Status:          string(params.JobStatusInProgress),
		Name:            "inprogress-no-instance",
		RepositoryName:  "test-repo",
		RepositoryOwner: "test-owner",
	}
	_, err = s.Store.CreateOrUpdateJob(s.adminCtx, inProgressNoInstance)
	s.Require().NoError(err)

	// 3. Completed job without instance (should be deleted)
	completedNoInstance := params.Job{
		WorkflowJobID:   12347,
		RunID:           67890,
		Action:          "test-action",
		Status:          string(params.JobStatusCompleted),
		Conclusion:      "success",
		Name:            "completed-no-instance",
		RepositoryName:  "test-repo",
		RepositoryOwner: "test-owner",
	}
	_, err = s.Store.CreateOrUpdateJob(s.adminCtx, completedNoInstance)
	s.Require().NoError(err)

	// Count total jobs before deletion
	var countBefore int64
	err = db.conn.Model(&WorkflowJob{}).Count(&countBefore).Error
	s.Require().NoError(err)
	s.Require().Equal(int64(3), countBefore, "Should have 3 jobs before deletion")

	// Run deletion
	err = s.Store.DeleteInactionableJobs(s.adminCtx, 0)
	s.Require().NoError(err)

	// Count remaining jobs - should only have the queued job
	var countAfter int64
	err = db.conn.Model(&WorkflowJob{}).Count(&countAfter).Error
	s.Require().NoError(err)
	s.Require().Equal(int64(1), countAfter, "Should have 1 job remaining (queued)")

	// Verify the remaining job is the queued one
	var remaining WorkflowJob
	err = db.conn.Where("workflow_job_id = ?", 12345).First(&remaining).Error
	s.Require().NoError(err)
	s.Require().Equal("queued", remaining.Status)
}

// TestDeleteInactionableJobs_AllScenarios verifies all deletion rules
func (s *JobsTestSuite) TestDeleteInactionableJobs_AllScenarios() {
	db := s.Store.(*sqlDatabase)

	// Rule 1: Queued jobs are NEVER deleted (regardless of instance_id)
	queuedNoInstance := params.Job{
		WorkflowJobID:   20001,
		RunID:           67890,
		Status:          string(params.JobStatusQueued),
		Name:            "queued-no-instance",
		RepositoryName:  "test-repo",
		RepositoryOwner: "test-owner",
	}
	_, err := s.Store.CreateOrUpdateJob(s.adminCtx, queuedNoInstance)
	s.Require().NoError(err)

	// Rule 2: Non-queued jobs WITHOUT instance_id ARE deleted
	inProgressNoInstance := params.Job{
		WorkflowJobID:   20002,
		RunID:           67890,
		Status:          string(params.JobStatusInProgress),
		Name:            "inprogress-no-instance",
		RepositoryName:  "test-repo",
		RepositoryOwner: "test-owner",
	}
	_, err = s.Store.CreateOrUpdateJob(s.adminCtx, inProgressNoInstance)
	s.Require().NoError(err)

	completedNoInstance := params.Job{
		WorkflowJobID:   20003,
		RunID:           67890,
		Status:          string(params.JobStatusCompleted),
		Conclusion:      "success",
		Name:            "completed-no-instance",
		RepositoryName:  "test-repo",
		RepositoryOwner: "test-owner",
	}
	_, err = s.Store.CreateOrUpdateJob(s.adminCtx, completedNoInstance)
	s.Require().NoError(err)

	// Count jobs before deletion
	var countBefore int64
	err = db.conn.Model(&WorkflowJob{}).Count(&countBefore).Error
	s.Require().NoError(err)
	s.Require().Equal(int64(3), countBefore)

	// Run deletion
	err = s.Store.DeleteInactionableJobs(s.adminCtx, 0)
	s.Require().NoError(err)

	// After deletion, only queued job should remain
	var countAfter int64
	err = db.conn.Model(&WorkflowJob{}).Count(&countAfter).Error
	s.Require().NoError(err)
	s.Require().Equal(int64(1), countAfter, "Only queued job should remain")

	// Verify it's the queued job that remains
	var jobs []WorkflowJob
	err = db.conn.Find(&jobs).Error
	s.Require().NoError(err)
	s.Require().Len(jobs, 1)
	s.Require().Equal(string(params.JobStatusQueued), jobs[0].Status)
}

// TestDeleteInactionableJobs_WithDuration verifies the duration-based filtering
func (s *JobsTestSuite) TestDeleteInactionableJobs_WithDuration() {
	db := s.Store.(*sqlDatabase)

	// Create an inactionable job (completed, no instance) with recent created_at
	recentJob := params.Job{
		WorkflowJobID:   30001,
		RunID:           67890,
		Status:          string(params.JobStatusCompleted),
		Name:            "recent-completed",
		RepositoryName:  "test-repo",
		RepositoryOwner: "test-owner",
	}
	_, err := s.Store.CreateOrUpdateJob(s.adminCtx, recentJob)
	s.Require().NoError(err)

	// Create an inactionable job and backdate its created_at to 2 hours ago
	oldJob := params.Job{
		WorkflowJobID:   30002,
		RunID:           67890,
		Status:          string(params.JobStatusCompleted),
		Name:            "old-completed",
		RepositoryName:  "test-repo",
		RepositoryOwner: "test-owner",
	}
	_, err = s.Store.CreateOrUpdateJob(s.adminCtx, oldJob)
	s.Require().NoError(err)

	// Backdate the old job's created_at
	err = db.conn.Model(&WorkflowJob{}).
		Where("workflow_job_id = ?", 30002).
		Update("created_at", time.Now().Add(-2*time.Hour)).Error
	s.Require().NoError(err)

	var countBefore int64
	err = db.conn.Model(&WorkflowJob{}).Count(&countBefore).Error
	s.Require().NoError(err)
	s.Require().Equal(int64(2), countBefore)

	// Delete inactionable jobs older than 1 hour — should only delete the old job
	err = s.Store.DeleteInactionableJobs(s.adminCtx, 1*time.Hour)
	s.Require().NoError(err)

	var countAfter int64
	err = db.conn.Model(&WorkflowJob{}).Count(&countAfter).Error
	s.Require().NoError(err)
	s.Require().Equal(int64(1), countAfter, "Only the recent job should remain")

	// Verify the remaining job is the recent one
	var remaining WorkflowJob
	err = db.conn.Where("workflow_job_id = ?", 30001).First(&remaining).Error
	s.Require().NoError(err)
	s.Require().Equal("recent-completed", remaining.Name)
}

func (s *JobsTestSuite) TestListJobsFilterAndPagination() {
	db := s.Store.(*sqlDatabase)

	// Five jobs, two of them completed, recorded one hour apart.
	for i := int64(1); i <= 5; i++ {
		status := params.JobStatusQueued
		if i >= 4 {
			status = params.JobStatusCompleted
		}
		_, err := s.Store.CreateOrUpdateJob(s.adminCtx, params.Job{
			WorkflowJobID:   100 + i,
			RunID:           1000,
			Status:          string(status),
			Name:            fmt.Sprintf("job-%d", i),
			RepositoryName:  "test-repo",
			RepositoryOwner: "test-owner",
		})
		s.Require().NoError(err)
		createdAt := time.Now().UTC().Add(-time.Duration(6-i) * time.Hour)
		s.Require().NoError(db.conn.Exec(
			"UPDATE workflow_jobs SET created_at = ? WHERE workflow_job_id = ?", createdAt, 100+i).Error)
	}

	// The default listing leaves out completed jobs.
	resp, err := s.Store.ListJobs(s.adminCtx, params.ListJobsFilter{})
	s.Require().NoError(err)
	s.Require().EqualValues(3, resp.TotalCount)
	s.Require().Len(resp.Results, 3)

	// IncludeCompleted lists everything, newest first.
	resp, err = s.Store.ListJobs(s.adminCtx, params.ListJobsFilter{IncludeCompleted: true})
	s.Require().NoError(err)
	s.Require().EqualValues(5, resp.TotalCount)
	s.Require().EqualValues(105, resp.Results[0].WorkflowJobID)

	// Pagination bookkeeping.
	resp, err = s.Store.ListJobs(s.adminCtx, params.ListJobsFilter{IncludeCompleted: true, PageSize: 2})
	s.Require().NoError(err)
	s.Require().Len(resp.Results, 2)
	s.Require().EqualValues(3, resp.Pages)
	s.Require().EqualValues(1, resp.CurrentPage)
	s.Require().Nil(resp.PreviousPage)
	s.Require().NotNil(resp.NextPage)
	s.Require().EqualValues(2, *resp.NextPage)

	resp, err = s.Store.ListJobs(s.adminCtx, params.ListJobsFilter{IncludeCompleted: true, PageSize: 2, Page: 3})
	s.Require().NoError(err)
	s.Require().Len(resp.Results, 1)
	s.Require().Nil(resp.NextPage)
	s.Require().NotNil(resp.PreviousPage)

	// Time window filtering. Jobs sit at T-5h through T-1h.
	since := time.Now().UTC().Add(-3*time.Hour - 30*time.Minute)
	resp, err = s.Store.ListJobs(s.adminCtx, params.ListJobsFilter{IncludeCompleted: true, Since: since})
	s.Require().NoError(err)
	s.Require().EqualValues(3, resp.TotalCount)

	until := time.Now().UTC().Add(-3*time.Hour - 30*time.Minute)
	resp, err = s.Store.ListJobs(s.adminCtx, params.ListJobsFilter{IncludeCompleted: true, Until: until})
	s.Require().NoError(err)
	s.Require().EqualValues(2, resp.TotalCount)

	resp, err = s.Store.ListJobs(s.adminCtx, params.ListJobsFilter{
		IncludeCompleted: true,
		Since:            time.Now().UTC().Add(-4*time.Hour - 30*time.Minute),
		Until:            time.Now().UTC().Add(-2*time.Hour - 30*time.Minute),
	})
	s.Require().NoError(err)
	s.Require().EqualValues(2, resp.TotalCount)
}
