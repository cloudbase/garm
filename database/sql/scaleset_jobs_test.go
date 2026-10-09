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
	"time"

	"github.com/stretchr/testify/suite"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type ScaleSetJobsTestSuite struct {
	suite.Suite
	Store    dbCommon.Store
	adminCtx context.Context

	repo       params.Repository
	repoEntity params.ForgeEntity
	scaleSet   params.ScaleSet
}

func (s *ScaleSetJobsTestSuite) SetupTest() {
	ctx := context.Background()
	watcher.InitWatcher(ctx)

	db := newTestDB(s.T())
	s.Store = db
	s.adminCtx = garmTesting.ImpersonateAdminContext(ctx, db, s.T())

	githubEndpoint := garmTesting.CreateDefaultGithubEndpoint(s.adminCtx, db, s.T())
	creds := garmTesting.CreateTestGithubCredentials(s.adminCtx, "new-creds", db, s.T(), githubEndpoint)

	var err error
	s.repo, err = s.Store.CreateRepository(s.adminCtx, "test-org", "test-repo", creds, "test-webhookSecret", params.PoolBalancerTypeRoundRobin, false)
	s.Require().NoError(err)
	s.repoEntity, err = s.repo.GetEntity()
	s.Require().NoError(err)

	s.scaleSet, err = s.Store.CreateEntityScaleSet(s.adminCtx, s.repoEntity, params.CreateScaleSetParams{
		Name:         "test-scaleset",
		ProviderName: "test-provider",
		MaxRunners:   10,
		Image:        "test-image",
		Flavor:       "test-flavor",
		OSType:       commonParams.Linux,
		OSArch:       commonParams.Amd64,
	})
	s.Require().NoError(err)
}

func (s *ScaleSetJobsTestSuite) TearDownTest() {
	watcher.CloseWatcher()
}

func (s *ScaleSetJobsTestSuite) message(msgType string) params.ScaleSetJobMessage {
	return params.ScaleSetJobMessage{
		MessageType:     msgType,
		JobID:           "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		RunnerRequestID: 42,
		RepositoryName:  "test-repo",
		OwnerName:       "test-org",
		JobDisplayName:  "build",
		WorkflowRunID:   1000,
		EventName:       "push",
		RequestLabels:   []string{"test-scaleset"},
		QueueTime:       time.Now().UTC(),
	}
}

func (s *ScaleSetJobsTestSuite) TestJobLifecycle() {
	// JobAssigned records the job as queued.
	assigned := s.message(params.MessageTypeJobAssigned)
	job, err := s.Store.CreateOrUpdateScaleSetJob(s.adminCtx, assigned.ToScaleSetJob(s.scaleSet.ID, "Default"))
	s.Require().NoError(err)
	s.Require().Equal(string(params.JobStatusQueued), job.Status)
	s.Require().Equal(s.scaleSet.ID, job.ScaleSetID)
	s.Require().Equal("Default", job.RunnerGroupName)
	s.Require().Equal([]string{"test-scaleset"}, job.RequestLabels)

	// JobStarted brings the runner.
	started := s.message(params.MessageTypeJobStarted)
	started.RunnerName = "scaleset-runner-0"
	started.RunnerID = 77
	started.RunnerAssignTime = time.Now().UTC()
	job, err = s.Store.CreateOrUpdateScaleSetJob(s.adminCtx, started.ToScaleSetJob(s.scaleSet.ID, "Default"))
	s.Require().NoError(err)
	s.Require().Equal(string(params.JobStatusInProgress), job.Status)
	s.Require().Equal("scaleset-runner-0", job.RunnerName)
	s.Require().EqualValues(77, job.RunnerID)
	s.Require().False(job.RunnerAssignTime.IsZero())

	// JobCompleted finishes it. Only one record exists throughout.
	completed := s.message(params.MessageTypeJobCompleted)
	completed.RunnerName = "scaleset-runner-0"
	completed.Result = "succeeded"
	completed.FinishTime = time.Now().UTC()
	job, err = s.Store.CreateOrUpdateScaleSetJob(s.adminCtx, completed.ToScaleSetJob(s.scaleSet.ID, "Default"))
	s.Require().NoError(err)
	s.Require().Equal(string(params.JobStatusCompleted), job.Status)
	s.Require().Equal("succeeded", job.Result)
	s.Require().False(job.FinishTime.IsZero())

	jobs, err := s.Store.ListScaleSetJobs(s.adminCtx, s.scaleSet.ID)
	s.Require().NoError(err)
	s.Require().Len(jobs, 1)
}

func (s *ScaleSetJobsTestSuite) TestStatusNeverRegresses() {
	completed := s.message(params.MessageTypeJobCompleted)
	completed.Result = "succeeded"
	_, err := s.Store.CreateOrUpdateScaleSetJob(s.adminCtx, completed.ToScaleSetJob(s.scaleSet.ID, ""))
	s.Require().NoError(err)

	// A redelivered started message must not revert completion.
	started := s.message(params.MessageTypeJobStarted)
	job, err := s.Store.CreateOrUpdateScaleSetJob(s.adminCtx, started.ToScaleSetJob(s.scaleSet.ID, ""))
	s.Require().NoError(err)
	s.Require().Equal(string(params.JobStatusCompleted), job.Status)
	s.Require().Equal("succeeded", job.Result)
}

func (s *ScaleSetJobsTestSuite) TestScaleSetDeleteCascadesJobs() {
	assigned := s.message(params.MessageTypeJobAssigned)
	_, err := s.Store.CreateOrUpdateScaleSetJob(s.adminCtx, assigned.ToScaleSetJob(s.scaleSet.ID, ""))
	s.Require().NoError(err)

	s.Require().NoError(s.Store.DeleteScaleSetByID(s.adminCtx, s.scaleSet.ID))

	jobs, err := s.Store.ListAllScaleSetJobs(s.adminCtx)
	s.Require().NoError(err)
	s.Require().Empty(jobs, "scale set jobs must be removed with the scale set")
}

func (s *ScaleSetJobsTestSuite) TestPruneOldJobs() {
	completed := s.message(params.MessageTypeJobCompleted)
	_, err := s.Store.CreateOrUpdateScaleSetJob(s.adminCtx, completed.ToScaleSetJob(s.scaleSet.ID, ""))
	s.Require().NoError(err)

	queued := s.message(params.MessageTypeJobAssigned)
	queued.JobID = "bbbbbbbb-cccc-dddd-eeee-ffffffffffff"
	_, err = s.Store.CreateOrUpdateScaleSetJob(s.adminCtx, queued.ToScaleSetJob(s.scaleSet.ID, ""))
	s.Require().NoError(err)

	// Nothing is old enough yet. Live jobs keep getting message updates,
	// which refresh updated_at and keep them out of the prune.
	s.Require().NoError(s.Store.DeleteOldScaleSetJobs(s.adminCtx, time.Hour))
	jobs, err := s.Store.ListScaleSetJobs(s.adminCtx, s.scaleSet.ID)
	s.Require().NoError(err)
	s.Require().Len(jobs, 2)

	// With no grace, everything goes regardless of status. A record that
	// saw no update for the retention age is a job we will never hear
	// about again.
	s.Require().NoError(s.Store.DeleteOldScaleSetJobs(s.adminCtx, -time.Minute))
	jobs, err = s.Store.ListScaleSetJobs(s.adminCtx, s.scaleSet.ID)
	s.Require().NoError(err)
	s.Require().Empty(jobs)
}

func TestScaleSetJobsTestSuite(t *testing.T) {
	suite.Run(t, new(ScaleSetJobsTestSuite))
}

func (s *ScaleSetJobsTestSuite) TestListScaleSetJobsPaginated() {
	// One completed and one queued job on the suite's scale set.
	completed := s.message(params.MessageTypeJobCompleted)
	_, err := s.Store.CreateOrUpdateScaleSetJob(s.adminCtx, completed.ToScaleSetJob(s.scaleSet.ID, ""))
	s.Require().NoError(err)
	queued := s.message(params.MessageTypeJobAssigned)
	queued.JobID = "bbbbbbbb-cccc-dddd-eeee-ffffffffffff"
	_, err = s.Store.CreateOrUpdateScaleSetJob(s.adminCtx, queued.ToScaleSetJob(s.scaleSet.ID, ""))
	s.Require().NoError(err)

	// A queued job on a second scale set.
	otherScaleSet, err := s.Store.CreateEntityScaleSet(s.adminCtx, s.repoEntity, params.CreateScaleSetParams{
		Name:         "other-scaleset",
		ProviderName: "test-provider",
		MaxRunners:   10,
		Image:        "test-image",
		Flavor:       "test-flavor",
		OSType:       commonParams.Linux,
		OSArch:       commonParams.Amd64,
	})
	s.Require().NoError(err)
	otherJob := s.message(params.MessageTypeJobAssigned)
	otherJob.JobID = "cccccccc-dddd-eeee-ffff-000000000000"
	_, err = s.Store.CreateOrUpdateScaleSetJob(s.adminCtx, otherJob.ToScaleSetJob(otherScaleSet.ID, ""))
	s.Require().NoError(err)

	// The default listing leaves out completed jobs, across all scale sets.
	resp, err := s.Store.ListScaleSetJobsPaginated(s.adminCtx, 0, params.ListJobsFilter{})
	s.Require().NoError(err)
	s.Require().EqualValues(2, resp.TotalCount)

	// IncludeCompleted lists everything.
	resp, err = s.Store.ListScaleSetJobsPaginated(s.adminCtx, 0, params.ListJobsFilter{IncludeCompleted: true})
	s.Require().NoError(err)
	s.Require().EqualValues(3, resp.TotalCount)

	// A scale set ID narrows the listing to that scale set.
	resp, err = s.Store.ListScaleSetJobsPaginated(s.adminCtx, otherScaleSet.ID, params.ListJobsFilter{IncludeCompleted: true})
	s.Require().NoError(err)
	s.Require().EqualValues(1, resp.TotalCount)
	s.Require().Equal(otherScaleSet.ID, resp.Results[0].ScaleSetID)

	// Pagination bookkeeping.
	resp, err = s.Store.ListScaleSetJobsPaginated(s.adminCtx, 0, params.ListJobsFilter{IncludeCompleted: true, PageSize: 2})
	s.Require().NoError(err)
	s.Require().Len(resp.Results, 2)
	s.Require().EqualValues(2, resp.Pages)
	s.Require().NotNil(resp.NextPage)
}
