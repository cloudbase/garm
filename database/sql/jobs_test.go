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
	db, err := NewSQLDatabase(ctx, garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}
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
	err = s.Store.DeleteInactionableJobs(s.adminCtx)
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
	err = s.Store.DeleteInactionableJobs(s.adminCtx)
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
