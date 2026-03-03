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

//go:build testing

package pool

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/locking"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	runnerCommonMocks "github.com/cloudbase/garm/runner/common/mocks"
)

func init() {
	watcher.SetWatcher(&garmTesting.MockWatcher{})
}

type PoolStressTestSuite struct {
	suite.Suite

	store          dbCommon.Store
	adminCtx       context.Context
	entity         params.ForgeEntity
	pool           params.Pool
	mgr            *basePoolManager
	providerMock   *runnerCommonMocks.Provider
	ghcliMock      *runnerCommonMocks.GithubClient
	controllerInfo params.ControllerInfo
}

func (s *PoolStressTestSuite) SetupTest() {
	dbCfg := garmTesting.GetTestSqliteDBConfig(s.T())
	db, err := database.NewDatabase(context.Background(), dbCfg)
	s.Require().NoError(err)

	s.store = db
	s.adminCtx = garmTesting.ImpersonateAdminContext(context.Background(), db, s.T())

	endpoint := garmTesting.CreateDefaultGithubEndpoint(s.adminCtx, db, s.T())
	creds := garmTesting.CreateTestGithubCredentials(s.adminCtx, "stress-creds", db, s.T(), endpoint)

	repo, err := db.CreateRepository(
		s.adminCtx,
		"test-owner",
		"test-repo",
		creds,
		"test-webhook-secret",
		params.PoolBalancerTypeRoundRobin,
		false,
	)
	s.Require().NoError(err)

	entity, err := repo.GetEntity()
	s.Require().NoError(err)
	s.entity = entity

	s.providerMock = runnerCommonMocks.NewProvider(s.T())
	s.ghcliMock = runnerCommonMocks.NewGithubClient(s.T())

	pool, err := db.CreateEntityPool(s.adminCtx, entity, params.CreatePoolParams{
		ProviderName:   "test-provider",
		MaxRunners:     10,
		MinIdleRunners: 0,
		Image:          "test-image",
		Flavor:         "test-flavor",
		OSType:         "linux",
		OSArch:         "amd64",
		Tags:           []string{"self-hosted", "linux", "x64"},
		Enabled:        true,
	})
	s.Require().NoError(err)
	s.pool = pool

	// Populate the entity cache so ensureMinIdleRunners can find pools.
	cache.SetEntity(entity)
	cache.SetEntityPool(entity.ID, pool)

	s.controllerInfo, err = db.InitController()
	s.Require().NoError(err)

	backoff, err := locking.NewInstanceDeleteBackoff(context.Background())
	s.Require().NoError(err)

	s.mgr = &basePoolManager{
		ctx:              s.adminCtx,
		consumerID:       "test-consumer",
		entity:           entity,
		store:            db,
		controllerInfo:   s.controllerInfo,
		providers:        map[string]common.Provider{"test-provider": s.providerMock},
		jobs:             make(map[int64]params.Job),
		checkedJobs:      make(map[int64]time.Time),
		quit:             make(chan struct{}),
		consumer:         &garmTesting.MockConsumer{},
		wg:               &sync.WaitGroup{},
		backoff:          backoff,
		ghcli:            s.ghcliMock,
		managerIsRunning: true,
	}
}

// makeWorkflowJob creates a params.WorkflowJob matching the test entity.
func (s *PoolStressTestSuite) makeWorkflowJob(jobID int64, action, status string, runnerName string, labels []string) params.WorkflowJob {
	wj := params.WorkflowJob{
		Action: action,
	}
	wj.WorkflowJob.ID = jobID
	wj.WorkflowJob.RunID = 1000 + jobID
	wj.WorkflowJob.Name = fmt.Sprintf("test-job-%d", jobID)
	wj.WorkflowJob.Status = status
	wj.WorkflowJob.Labels = labels
	wj.WorkflowJob.RunnerName = runnerName
	wj.Repository.Name = "test-repo"
	wj.Repository.Owner.Login = "test-owner"
	wj.Repository.HTMLURL = "https://github.com/test-owner/test-repo"
	return wj
}

// syncJobsFromDB populates mgr.jobs from the database.
func (s *PoolStressTestSuite) syncJobsFromDB() {
	allJobs, err := s.store.ListAllJobs(s.adminCtx)
	s.Require().NoError(err)

	s.mgr.mux.Lock()
	defer s.mgr.mux.Unlock()
	// Clear stale entries so the map reflects current DB state.
	for k := range s.mgr.jobs {
		delete(s.mgr.jobs, k)
	}
	for _, j := range allJobs {
		s.mgr.jobs[j.WorkflowJobID] = j
	}
}

// setupProviderMocks sets up the mock expectations needed for AddRunner.
func (s *PoolStressTestSuite) setupProviderMocks() {
	s.providerMock.On("DisableJITConfig").Return(true).Maybe()
	s.ghcliMock.On("GetEntityJITConfig",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(map[string]string{}, nil, nil).Maybe()
	s.ghcliMock.On("RemoveEntityRunner",
		mock.Anything, mock.Anything,
	).Return(nil).Maybe()
}

// TestHandleWorkflowJobFullLifecycle tests the complete lifecycle of a single job:
// queued → in_progress → completed.
func (s *PoolStressTestSuite) TestHandleWorkflowJobFullLifecycle() {
	s.setupProviderMocks()

	labels := []string{"self-hosted", "linux", "x64"}

	// Phase 1: queued
	queuedJob := s.makeWorkflowJob(1001, "queued", "queued", "", labels)
	err := s.mgr.HandleWorkflowJob(queuedJob)
	s.Require().NoError(err)

	// Verify job was persisted
	dbJob, err := s.store.GetJobByID(s.adminCtx, 1001)
	s.Require().NoError(err)
	s.Equal("queued", dbJob.Action)
	s.Equal("queued", dbJob.Status)

	// Sync jobs to in-memory map and consume
	s.syncJobsFromDB()
	err = s.mgr.consumeQueuedJobs()
	s.Require().NoError(err)

	// Verify a runner was created
	instances, err := s.store.ListPoolInstances(s.adminCtx, s.pool.ID, false)
	s.Require().NoError(err)
	s.Require().NotEmpty(instances, "expected at least one instance after consuming queued job")
	runnerName := instances[0].Name

	// Phase 2: in_progress
	inProgressJob := s.makeWorkflowJob(1001, "in_progress", "in_progress", runnerName, labels)
	err = s.mgr.HandleWorkflowJob(inProgressJob)
	s.Require().NoError(err)

	// Verify runner is marked active
	inst, err := s.store.GetInstance(s.adminCtx, runnerName)
	s.Require().NoError(err)
	s.Equal(params.RunnerActive, inst.RunnerStatus)

	// Phase 3: completed
	completedJob := s.makeWorkflowJob(1001, "completed", "completed", runnerName, labels)
	err = s.mgr.HandleWorkflowJob(completedJob)
	s.Require().NoError(err)

	// Verify runner is marked for deletion
	inst, err = s.store.GetInstance(s.adminCtx, runnerName)
	s.Require().NoError(err)
	s.Equal(params.RunnerTerminated, inst.RunnerStatus)
	s.Equal(commonParams.InstancePendingDelete, inst.Status)

	// Verify DB: job should be completed.
	completedJobs, err := s.store.ListJobsByStatus(s.adminCtx, params.JobStatusCompleted)
	s.Require().NoError(err)
	s.Len(completedJobs, 1, "exactly 1 completed job should exist in DB")
}

// TestConcurrentQueuedJobs sends N queued webhooks concurrently with different job IDs
// and verifies all are persisted without panics.
func (s *PoolStressTestSuite) TestConcurrentQueuedJobs() {
	s.setupProviderMocks()
	labels := []string{"self-hosted", "linux", "x64"}

	const numJobs = 20
	var wg sync.WaitGroup
	errs := make([]error, numJobs)

	for i := range numJobs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			jobID := int64(2000 + idx)
			job := s.makeWorkflowJob(jobID, "queued", "queued", "", labels)
			errs[idx] = s.mgr.HandleWorkflowJob(job)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		s.Require().NoError(err, "HandleWorkflowJob failed for job %d", 2000+i)
	}

	// Verify all jobs were persisted
	allJobs, err := s.store.ListAllJobs(s.adminCtx)
	s.Require().NoError(err)
	s.GreaterOrEqual(len(allJobs), numJobs, "expected at least %d jobs", numJobs)

	// Sync and consume
	s.syncJobsFromDB()
	err = s.mgr.consumeQueuedJobs()
	s.Require().NoError(err)

	// Verify runners were created (up to MaxRunners)
	instances, err := s.store.ListPoolInstances(s.adminCtx, s.pool.ID, false)
	s.Require().NoError(err)
	s.LessOrEqual(uint(len(instances)), s.pool.MaxRunners)
	s.NotEmpty(instances, "expected at least one instance created")

	// Verify DB: all 20 jobs persisted (none lost or duplicated).
	queuedJobs, err := s.store.ListJobsByStatus(s.adminCtx, params.JobStatusQueued)
	s.Require().NoError(err)
	s.Len(queuedJobs, numJobs, "expected %d queued jobs in DB", numJobs)
}

// TestJobStuckInQueuedWithMinIdleEqMaxRunners tests the key scenario where
// min_idle_runners == max_runners and jobs should not get stuck.
func (s *PoolStressTestSuite) TestJobStuckInQueuedWithMinIdleEqMaxRunners() {
	s.setupProviderMocks()

	// Reconfigure pool: min_idle=3, max=3
	maxRunners := uint(3)
	minIdle := uint(3)
	pool, err := s.store.UpdateEntityPool(s.adminCtx, s.entity, s.pool.ID, params.UpdatePoolParams{
		MaxRunners:     &maxRunners,
		MinIdleRunners: &minIdle,
	})
	s.Require().NoError(err)
	s.pool = pool
	cache.SetEntityPool(s.entity.ID, pool)

	// Step 1: Ensure 3 idle runners exist
	err = s.mgr.ensureIdleRunnersForOnePool(pool)
	s.Require().NoError(err)

	instances, err := s.store.ListPoolInstances(s.adminCtx, pool.ID, false)
	s.Require().NoError(err)
	s.Require().Len(instances, 3, "expected 3 idle runners")

	labels := []string{"self-hosted", "linux", "x64"}

	// Step 2: Send 3 queued webhooks
	for i := range 3 {
		jobID := int64(3001 + i)
		job := s.makeWorkflowJob(jobID, "queued", "queued", "", labels)
		err := s.mgr.HandleWorkflowJob(job)
		s.Require().NoError(err)
	}

	// Step 3: Simulate runners picking up jobs (in_progress)
	instances, err = s.store.ListPoolInstances(s.adminCtx, pool.ID, false)
	s.Require().NoError(err)
	for i, inst := range instances {
		jobID := int64(3001 + i)
		job := s.makeWorkflowJob(jobID, "in_progress", "in_progress", inst.Name, labels)
		err := s.mgr.HandleWorkflowJob(job)
		s.Require().NoError(err)
	}

	// Step 4: Verify all runners are now active
	instances, err = s.store.ListPoolInstances(s.adminCtx, pool.ID, false)
	s.Require().NoError(err)
	activeCount := 0
	for _, inst := range instances {
		if inst.RunnerStatus == params.RunnerActive {
			activeCount++
		}
	}
	s.Equal(3, activeCount, "all 3 runners should be active")

	// ensureIdleRunnersForOnePool should not be able to create more (at max)
	err = s.mgr.ensureIdleRunnersForOnePool(pool)
	s.Require().NoError(err)
	instances, err = s.store.ListPoolInstances(s.adminCtx, pool.ID, false)
	s.Require().NoError(err)
	s.Len(instances, 3, "should still have 3 instances (max reached)")

	// Step 5: Complete all jobs
	for i, inst := range instances {
		if inst.RunnerStatus != params.RunnerActive {
			continue
		}
		jobID := int64(3001 + i)
		job := s.makeWorkflowJob(jobID, "completed", "completed", inst.Name, labels)
		err := s.mgr.HandleWorkflowJob(job)
		s.Require().NoError(err)
	}

	// Step 6: Verify runners are marked for deletion
	instances, err = s.store.ListPoolInstances(s.adminCtx, pool.ID, false)
	s.Require().NoError(err)
	pendingDeleteCount := 0
	for _, inst := range instances {
		if inst.Status == commonParams.InstancePendingDelete {
			pendingDeleteCount++
		}
	}
	s.Equal(3, pendingDeleteCount, "all 3 runners should be pending_delete")

	// Verify DB: first 3 jobs should be completed.
	completedJobs, err := s.store.ListJobsByStatus(s.adminCtx, params.JobStatusCompleted)
	s.Require().NoError(err)
	s.Len(completedJobs, 3, "all 3 jobs should be completed in DB")

	// Step 7: Simulate deletion of old runners
	for _, inst := range instances {
		err := s.store.DeleteInstance(s.adminCtx, pool.ID, inst.Name)
		s.Require().NoError(err)
	}

	// Step 8: ensureMinIdleRunners should now create 3 new runners
	err = s.mgr.ensureIdleRunnersForOnePool(pool)
	s.Require().NoError(err)

	newInstances, err := s.store.ListPoolInstances(s.adminCtx, pool.ID, false)
	s.Require().NoError(err)
	s.Len(newInstances, 3, "should have 3 new idle runners")

	// Step 9: Send 3 more queued webhooks
	for i := 0; i < 3; i++ {
		jobID := int64(3101 + i)
		job := s.makeWorkflowJob(jobID, "queued", "queued", "", labels)
		err := s.mgr.HandleWorkflowJob(job)
		s.Require().NoError(err)
	}

	// Step 10: consumeQueuedJobs should try to create runners for these jobs.
	// Since we're at max_runners already (3 idle), it should fail to create new ones
	// but the jobs should not be stuck forever — the idle runners should pick them up.
	s.syncJobsFromDB()
	err = s.mgr.consumeQueuedJobs()
	s.Require().NoError(err)

	// Verify: jobs are in DB and instances exist (idle runners that can pick them up).
	queuedJobs, err := s.store.ListJobsByStatus(s.adminCtx, params.JobStatusQueued)
	s.Require().NoError(err)
	// The key assertion: jobs exist in queued state AND idle runners exist to pick them up.
	s.NotEmpty(queuedJobs, "queued jobs should exist")

	runnerInstances, err := s.store.ListPoolInstances(s.adminCtx, pool.ID, false)
	s.Require().NoError(err)
	idleCount := 0
	for _, inst := range runnerInstances {
		if inst.RunnerStatus != params.RunnerActive && inst.RunnerStatus != params.RunnerTerminated {
			idleCount++
		}
	}
	s.Equal(3, idleCount, "should have 3 idle runners available to pick up queued jobs")
}

// TestMissedWebhookRecovery simulates a job that was queued while GARM was offline.
func (s *PoolStressTestSuite) TestMissedWebhookRecovery() {
	s.setupProviderMocks()

	labels := []string{"self-hosted", "linux", "x64"}

	// Directly insert a queued job into the DB (simulating a job we received but
	// the runner was never created because GARM was restarted).
	job := params.Job{
		WorkflowJobID:   4001,
		Action:          "queued",
		Status:          "queued",
		Labels:          labels,
		RepositoryName:  "test-repo",
		RepositoryOwner: "test-owner",
		RepoID:          garmTesting.Ptr(mustParseUUID(s.entity.ID)),
	}
	_, err := s.store.CreateOrUpdateJob(s.adminCtx, job)
	s.Require().NoError(err)

	// Populate the in-memory jobs map
	s.syncJobsFromDB()

	// consumeQueuedJobs should create a runner for this orphaned job
	err = s.mgr.consumeQueuedJobs()
	s.Require().NoError(err)

	instances, err := s.store.ListPoolInstances(s.adminCtx, s.pool.ID, false)
	s.Require().NoError(err)
	s.NotEmpty(instances, "expected a runner to be created for the missed job")

	// Verify DB: the orphaned job should still be queued but locked.
	queuedJobs, err := s.store.ListJobsByStatus(s.adminCtx, params.JobStatusQueued)
	s.Require().NoError(err)
	s.Len(queuedJobs, 1, "exactly 1 queued job should exist")
	s.NotEqual(uuid.UUID{}, queuedJobs[0].LockedBy, "job should be locked after consume")
}

// TestConsumeQueuedJobsRespectsBackoff verifies the JobBackoff fix.
func (s *PoolStressTestSuite) TestConsumeQueuedJobsRespectsBackoff() {
	s.setupProviderMocks()

	labels := []string{"self-hosted", "linux", "x64"}

	// Set a 5-second backoff
	s.mgr.controllerInfo.MinimumJobAgeBackoff = 5

	// Insert a queued job that was just created (UpdatedAt = now)
	job := params.Job{
		WorkflowJobID:   5001,
		Action:          "queued",
		Status:          "queued",
		Labels:          labels,
		RepositoryName:  "test-repo",
		RepositoryOwner: "test-owner",
		RepoID:          garmTesting.Ptr(mustParseUUID(s.entity.ID)),
	}
	_, err := s.store.CreateOrUpdateJob(s.adminCtx, job)
	s.Require().NoError(err)
	s.syncJobsFromDB()

	// consumeQueuedJobs should skip this job (backoff not reached)
	err = s.mgr.consumeQueuedJobs()
	s.Require().NoError(err)

	instances, err := s.store.ListPoolInstances(s.adminCtx, s.pool.ID, false)
	s.Require().NoError(err)
	s.Empty(instances, "no runner should be created while backoff is active")

	// Verify DB: job should still be queued and unlocked during backoff.
	queuedJobs, err := s.store.ListJobsByStatus(s.adminCtx, params.JobStatusQueued)
	s.Require().NoError(err)
	s.Len(queuedJobs, 1, "job should remain queued during backoff")
	s.Equal(uuid.UUID{}, queuedJobs[0].LockedBy, "job should not be locked during backoff")

	// Now set backoff to 0 and retry — job should be consumed
	s.mgr.controllerInfo.MinimumJobAgeBackoff = 0
	s.syncJobsFromDB()

	err = s.mgr.consumeQueuedJobs()
	s.Require().NoError(err)

	instances, err = s.store.ListPoolInstances(s.adminCtx, s.pool.ID, false)
	s.Require().NoError(err)
	s.NotEmpty(instances, "runner should be created after backoff expires")

	// Verify DB: job should now be locked.
	queuedJobs, err = s.store.ListJobsByStatus(s.adminCtx, params.JobStatusQueued)
	s.Require().NoError(err)
	s.Len(queuedJobs, 1, "job should still be queued")
	s.NotEqual(uuid.UUID{}, queuedJobs[0].LockedBy, "job should be locked after consume")
}

// TestConcurrentWebhooksForSameJob sends the same job ID as queued from multiple
// goroutines simultaneously and verifies no duplicate runners are created.
func (s *PoolStressTestSuite) TestConcurrentWebhooksForSameJob() {
	s.setupProviderMocks()

	labels := []string{"self-hosted", "linux", "x64"}
	const numGoroutines = 10
	var wg sync.WaitGroup
	errs := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			job := s.makeWorkflowJob(6001, "queued", "queued", "", labels)
			errs[idx] = s.mgr.HandleWorkflowJob(job)
		}(i)
	}
	wg.Wait()

	// All calls should succeed (no panics)
	for i, err := range errs {
		s.Require().NoError(err, "goroutine %d failed", i)
	}

	// CreateOrUpdateJob is wrapped in a transaction, so concurrent goroutines
	// should not create duplicate rows. Exactly 1 job should exist.
	allJobs, err := s.store.ListAllJobs(s.adminCtx)
	s.Require().NoError(err)
	s.Len(allJobs, 1, "transaction should prevent duplicate job rows")

	// Exactly 1 runner should be created.
	s.syncJobsFromDB()
	err = s.mgr.consumeQueuedJobs()
	s.Require().NoError(err)

	instances, err := s.store.ListPoolInstances(s.adminCtx, s.pool.ID, false)
	s.Require().NoError(err)
	s.Len(instances, 1, "exactly 1 runner should be created for 1 job")
}

// TestRapidFireWebhookLifecycle fires 1000 jobs through the full
// queued→in_progress→completed webhook lifecycle in rapid succession.
//
// We process in batches of 200: queue a batch, consume them to spawn runners,
// then drive each runner through in_progress→completed, reap, repeat.
// This simulates a sustained burst of GitHub webhooks hitting GARM and
// verifies every job is correctly processed end-to-end without getting stuck.
func (s *PoolStressTestSuite) TestRapidFireWebhookLifecycle() {
	s.setupProviderMocks()

	// Set MaxRunners high enough to handle a full batch at once.
	maxRunners := uint(200)
	minIdle := uint(0)
	pool, err := s.store.UpdateEntityPool(s.adminCtx, s.entity, s.pool.ID, params.UpdatePoolParams{
		MaxRunners:     &maxRunners,
		MinIdleRunners: &minIdle,
	})
	s.Require().NoError(err)
	s.pool = pool
	cache.SetEntityPool(s.entity.ID, pool)

	labels := []string{"self-hosted", "linux", "x64"}
	const totalJobs = 1000
	const batchSize = 200
	baseJobID := int64(10000)
	totalCompleted := 0

	for batchStart := 0; batchStart < totalJobs; batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > totalJobs {
			batchEnd = totalJobs
		}
		batchCount := batchEnd - batchStart

		// 1) Fire all "queued" webhooks for this batch.
		for i := batchStart; i < batchEnd; i++ {
			jobID := baseJobID + int64(i)
			wh := s.makeWorkflowJob(jobID, "queued", "queued", "", labels)
			err := s.mgr.HandleWorkflowJob(wh)
			s.Require().NoError(err, "queued webhook failed for job %d", jobID)
		}

		// 2) Sync the in-memory job map and let consumeQueuedJobs spawn runners.
		s.syncJobsFromDB()
		err := s.mgr.consumeQueuedJobs()
		s.Require().NoError(err)

		// 3) Get all newly-created runners (pending + pending_create).
		instances, err := s.store.ListPoolInstances(s.adminCtx, s.pool.ID, false)
		s.Require().NoError(err)

		var newRunners []params.Instance
		for _, inst := range instances {
			if inst.RunnerStatus == params.RunnerPending &&
				inst.Status == commonParams.InstancePendingCreate {
				newRunners = append(newRunners, inst)
			}
		}
		s.Require().Len(newRunners, batchCount,
			"batch [%d..%d): expected %d runners spawned, got %d",
			batchStart, batchEnd, batchCount, len(newRunners))

		// 4) Drive each runner through in_progress → completed.
		for _, runner := range newRunners {
			triggeredJobID := jobIDFromLabels(runner.AditionalLabels)
			s.Require().NotZero(triggeredJobID,
				"runner %s should have a job label", runner.Name)

			// in_progress webhook
			ipWH := s.makeWorkflowJob(triggeredJobID, "in_progress", "in_progress", runner.Name, labels)
			err := s.mgr.HandleWorkflowJob(ipWH)
			s.Require().NoError(err,
				"in_progress failed for job %d runner %s", triggeredJobID, runner.Name)

			inst, err := s.store.GetInstance(s.adminCtx, runner.Name)
			s.Require().NoError(err)
			s.Equal(params.RunnerActive, inst.RunnerStatus,
				"runner %s should be active", runner.Name)

			// completed webhook
			cWH := s.makeWorkflowJob(triggeredJobID, "completed", "completed", runner.Name, labels)
			err = s.mgr.HandleWorkflowJob(cWH)
			s.Require().NoError(err,
				"completed failed for job %d runner %s", triggeredJobID, runner.Name)

			inst, err = s.store.GetInstance(s.adminCtx, runner.Name)
			s.Require().NoError(err)
			s.Equal(params.RunnerTerminated, inst.RunnerStatus,
				"runner %s should be terminated", runner.Name)
			s.Equal(commonParams.InstancePendingDelete, inst.Status,
				"runner %s should be pending_delete", runner.Name)

			totalCompleted++
		}

		// 5) Reap all pending_delete instances to free pool capacity for next batch.
		instances, err = s.store.ListPoolInstances(s.adminCtx, s.pool.ID, false)
		s.Require().NoError(err)
		for _, inst := range instances {
			if inst.Status == commonParams.InstancePendingDelete {
				err := s.store.DeleteInstance(s.adminCtx, s.pool.ID, inst.Name)
				s.Require().NoError(err)
			}
		}

		// Pool should be empty, ready for next batch.
		instances, err = s.store.ListPoolInstances(s.adminCtx, s.pool.ID, false)
		s.Require().NoError(err)
		s.Empty(instances, "pool should be empty after reaping batch [%d..%d)",
			batchStart, batchEnd)
	}

	s.Equal(totalJobs, totalCompleted,
		"all %d jobs should have completed the full lifecycle", totalJobs)

	// Verify DB state reflects reality.
	// No instances should remain (all reaped).
	remaining, err := s.store.ListPoolInstances(s.adminCtx, s.pool.ID, false)
	s.Require().NoError(err)
	s.Empty(remaining, "no instances should remain after full lifecycle")

	// No queued jobs should remain.
	queuedJobs, err := s.store.ListJobsByStatus(s.adminCtx, params.JobStatusQueued)
	s.Require().NoError(err)
	s.Empty(queuedJobs, "no jobs should be stuck in queued state")

	// No in_progress jobs should remain.
	ipJobs, err := s.store.ListJobsByStatus(s.adminCtx, params.JobStatusInProgress)
	s.Require().NoError(err)
	s.Empty(ipJobs, "no jobs should be stuck in in_progress state")

	// Whatever jobs survived DeleteInactionableJobs must all be completed.
	allJobs, err := s.store.ListAllJobs(s.adminCtx)
	s.Require().NoError(err)
	completedJobs, err := s.store.ListJobsByStatus(s.adminCtx, params.JobStatusCompleted)
	s.Require().NoError(err)
	s.Len(completedJobs, len(allJobs), "all surviving jobs should be completed")
}

func mustParseUUID(s string) uuid.UUID {
	u, err := uuid.Parse(s)
	if err != nil {
		panic(fmt.Sprintf("failed to parse UUID %q: %v", s, err))
	}
	return u
}

func TestPoolStressTestSuite(t *testing.T) {
	suite.Run(t, new(PoolStressTestSuite))
}
