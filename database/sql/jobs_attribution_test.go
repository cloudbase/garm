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
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type JobAttributionTestSuite struct {
	suite.Suite
	Store    dbCommon.Store
	adminCtx context.Context

	repoEntity params.ForgeEntity
	repoUUID   uuid.UUID
}

func (s *JobAttributionTestSuite) SetupTest() {
	ctx := context.Background()
	watcher.InitWatcher(ctx)

	db := newTestDB(s.T())
	s.Store = db
	s.adminCtx = garmTesting.ImpersonateAdminContext(ctx, db, s.T())

	githubEndpoint := garmTesting.CreateDefaultGithubEndpoint(s.adminCtx, db, s.T())
	creds := garmTesting.CreateTestGithubCredentials(s.adminCtx, "new-creds", db, s.T(), githubEndpoint)

	repo, err := s.Store.CreateRepository(s.adminCtx, "test-org", "test-repo", creds, "test-webhookSecret", params.PoolBalancerTypeRoundRobin, false)
	s.Require().NoError(err)
	s.repoEntity, err = repo.GetEntity()
	s.Require().NoError(err)
	s.repoUUID, err = uuid.Parse(repo.ID)
	s.Require().NoError(err)
}

func (s *JobAttributionTestSuite) TearDownTest() {
	watcher.CloseWatcher()
}

func (s *JobAttributionTestSuite) webhookJob(workflowJobID, runID int64, name string) params.Job {
	return params.Job{
		WorkflowJobID:   workflowJobID,
		RunID:           runID,
		Action:          string(params.JobStatusQueued),
		Status:          string(params.JobStatusQueued),
		Name:            name,
		RepositoryName:  "test-repo",
		RepositoryOwner: "test-org",
		Labels:          []string{"self-hosted"},
		RepoID:          &s.repoUUID,
	}
}

func (s *JobAttributionTestSuite) createPool() params.Pool {
	pool, err := s.Store.CreateEntityPool(s.adminCtx, s.repoEntity, params.CreatePoolParams{
		ProviderName:   "test-provider",
		MaxRunners:     4,
		MinIdleRunners: 0,
		Image:          "test-image",
		Flavor:         "test-flavor",
		OSType:         commonParams.Linux,
		Tags:           []string{"self-hosted"},
	})
	s.Require().NoError(err)
	return pool
}

// TestPoolAttributionIsSticky stamps the pool on the in_progress update and
// keeps it through later updates that carry no pool.
func (s *JobAttributionTestSuite) TestPoolAttributionIsSticky() {
	pool := s.createPool()
	poolUUID, err := uuid.Parse(pool.ID)
	s.Require().NoError(err)

	// The queued webhook does not know the pool yet.
	created, err := s.Store.CreateOrUpdateJob(s.adminCtx, s.webhookJob(100, 1000, "build"))
	s.Require().NoError(err)
	s.Require().Nil(created.PoolID)

	// A pool runner picked up the job.
	update := s.webhookJob(100, 1000, "build")
	update.Status = string(params.JobStatusInProgress)
	update.PoolID = &poolUUID
	updated, err := s.Store.CreateOrUpdateJob(s.adminCtx, update)
	s.Require().NoError(err)
	s.Require().NotNil(updated.PoolID)
	s.Require().Equal(poolUUID, *updated.PoolID)

	// The completed update carries no pool. Attribution must survive.
	completed := s.webhookJob(100, 1000, "build")
	completed.Status = string(params.JobStatusCompleted)
	final, err := s.Store.CreateOrUpdateJob(s.adminCtx, completed)
	s.Require().NoError(err)
	s.Require().NotNil(final.PoolID)
	s.Require().Equal(poolUUID, *final.PoolID)
}

// TestPoolDeleteKeepsJobs verifies pool deletion clears the attribution
// instead of removing the job record.
func (s *JobAttributionTestSuite) TestPoolDeleteKeepsJobs() {
	pool := s.createPool()
	poolUUID, err := uuid.Parse(pool.ID)
	s.Require().NoError(err)

	job := s.webhookJob(100, 1000, "build")
	job.PoolID = &poolUUID
	_, err = s.Store.CreateOrUpdateJob(s.adminCtx, job)
	s.Require().NoError(err)

	s.Require().NoError(s.Store.DeleteEntityPool(s.adminCtx, s.repoEntity, pool.ID))

	jobs, err := s.Store.ListAllJobs(s.adminCtx)
	s.Require().NoError(err)
	s.Require().Len(jobs, 1)
	s.Require().Nil(jobs[0].PoolID, "pool attribution must be cleared, not the job")
}

func TestJobAttributionTestSuite(t *testing.T) {
	suite.Run(t, new(JobAttributionTestSuite))
}
