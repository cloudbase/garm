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

package pool

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

var (
	jobEventBaseTime = time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	jobEventNewer    = jobEventBaseTime.Add(time.Minute)
	jobEventOlder    = jobEventBaseTime.Add(-time.Minute)
)

const (
	burstJobCount      = 200
	burstUpdatesPerJob = 6
)

// Arbitrary shuffle seeds. Borrowed from the PRNG literature for their well spread bits.
const (
	shuffleSeed1 uint64 = 0x2545f4914f6cdd1d // xorshift64* multiplier
	shuffleSeed2 uint64 = 0x9e3779b97f4a7c15 // 2^64/phi, SplitMix64's increment
	shuffleSeed3 uint64 = 0xbf58476d1ce4e5b9 // SplitMix64 mixing multiplier
)

func makeWatcherJob(id int64, status params.JobStatus, updatedAt time.Time, repoID *uuid.UUID) params.Job {
	return params.Job{
		ID:            id,
		WorkflowJobID: 1000 + id,
		RunID:         2000 + id,
		Action:        string(status),
		Status:        string(status),
		RepoID:        repoID,
		UpdatedAt:     updatedAt,
	}
}

func jobEvent(op dbCommon.OperationType, job params.Job) dbCommon.ChangePayload {
	return dbCommon.ChangePayload{
		EntityType: dbCommon.JobEntityType,
		Operation:  op,
		Payload:    job,
	}
}

// deterministicShuffle reorders events with a fixed-seed xorshift64 (Marsaglia),
// so a failing burst always replays identically. Gosec flags math/rand as a weak
// RNG, even in tests...so we need this.
func deterministicShuffle(events []dbCommon.ChangePayload, seed uint64) {
	state := seed
	next := func() uint64 {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		return state
	}
	for i := len(events) - 1; i > 0; i-- {
		// The modulo bounds j by i, which is already a valid index.
		j := int(next() % uint64(i+1))
		events[i], events[j] = events[j], events[i]
	}
}

// jobBurstShape describes how a job in the burst ends up.
type jobBurstShape int

const (
	// burstShapeQueued churns while staying queued: GARM re-notifies a job when
	// it is locked and unlocked.
	burstShapeQueued jobBurstShape = iota
	// burstShapeRunning is picked up by a runner half way through.
	burstShapeRunning
	// burstShapeCompleted runs and then finishes, retiring the job.
	burstShapeCompleted
)

func tombstonedJobIDs(mgr *basePoolManager) []int64 {
	ids := make([]int64, 0, len(mgr.jobTombstones))
	for id := range mgr.jobTombstones {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func queuedJobIDs(mgr *basePoolManager) []int64 {
	queued := mgr.getQueuedJobs()
	ids := make([]int64, 0, len(queued))
	for _, job := range queued {
		ids = append(ids, job.ID)
	}
	slices.Sort(ids)
	return ids
}

type WatcherJobCacheTestSuite struct {
	suite.Suite

	repoID uuid.UUID
	entity params.ForgeEntity
}

func (s *WatcherJobCacheTestSuite) SetupTest() {
	s.repoID = uuid.New()
	s.entity = params.ForgeEntity{
		ID:         s.repoID.String(),
		EntityType: params.ForgeEntityTypeRepository,
	}
}

// newManager returns a manager with just enough state to exercise the job branch
// of handleWatcherEvent, seeded with the given jobs.
func (s *WatcherJobCacheTestSuite) newManager(seed ...params.Job) *basePoolManager {
	jobs := make(map[int64]params.Job, len(seed))
	for _, job := range seed {
		jobs[job.ID] = job
	}
	return &basePoolManager{
		ctx:           context.Background(),
		entity:        s.entity,
		jobs:          jobs,
		jobTombstones: make(map[int64]time.Time),
	}
}

// job builds a job belonging to the suite entity.
func (s *WatcherJobCacheTestSuite) job(id int64, status params.JobStatus, updatedAt time.Time) params.Job {
	return makeWatcherJob(id, status, updatedAt, &s.repoID)
}

// buildJobBurst returns the events for a burst of concurrent jobs, in causal
// order. Every job gets burstUpdatesPerJob events with increasing UpdatedAt, and
// the three shapes are spread evenly across the burst. It also returns what the
// cache should look like once the burst settles: the jobs still there, the ones
// still queued, and the ones that finished and should be tombstoned.
func (s *WatcherJobCacheTestSuite) buildJobBurst() (events []dbCommon.ChangePayload, want map[int64]params.Job, queuedIDs, retiredIDs []int64) {
	want = make(map[int64]params.Job, burstJobCount)
	for i := range burstJobCount {
		id := int64(i + 1)
		shape := jobBurstShape(i % 3)

		for u := range burstUpdatesPerJob {
			status := params.JobStatusQueued
			switch {
			case shape == burstShapeCompleted && u == burstUpdatesPerJob-1:
				// A job's completion is always its newest event: the store
				// rejects any transition back out of completed.
				status = params.JobStatusCompleted
			case shape != burstShapeQueued && u >= burstUpdatesPerJob/2:
				status = params.JobStatusInProgress
			}

			job := s.job(id, status, jobEventBaseTime.Add(time.Duration(u)*time.Second))
			op := dbCommon.UpdateOperation
			if u == 0 {
				op = dbCommon.CreateOperation
			}
			events = append(events, jobEvent(op, job))
			// The last event built for a job is its newest, so this settles on
			// the state the cache is expected to hold.
			want[id] = job
		}

		switch shape {
		case burstShapeQueued:
			queuedIDs = append(queuedIDs, id)
		case burstShapeCompleted:
			// A finished job leaves the cache entirely.
			delete(want, id)
			retiredIDs = append(retiredIDs, id)
		case burstShapeRunning:
		}
	}
	return events, want, queuedIDs, retiredIDs
}

// TestJobOrdering covers the job cache, including the guard that drops updates
// older than what we already recorded.
func (s *WatcherJobCacheTestSuite) TestJobOrdering() {
	otherRepoID := uuid.New()
	recorded := s.job(1, params.JobStatusInProgress, jobEventBaseTime)

	tests := []struct {
		name  string
		seed  []params.Job
		event dbCommon.ChangePayload
		want  map[int64]params.Job
	}{
		{
			name:  "create records the job",
			event: jobEvent(dbCommon.CreateOperation, recorded),
			want:  map[int64]params.Job{1: recorded},
		},
		{
			name:  "newer update replaces the recorded job",
			seed:  []params.Job{recorded},
			event: jobEvent(dbCommon.UpdateOperation, s.job(1, params.JobStatusInProgress, jobEventNewer)),
			want: map[int64]params.Job{
				1: s.job(1, params.JobStatusInProgress, jobEventNewer),
			},
		},
		{
			name:  "stale update is skipped",
			seed:  []params.Job{recorded},
			event: jobEvent(dbCommon.UpdateOperation, s.job(1, params.JobStatusQueued, jobEventOlder)),
			want:  map[int64]params.Job{1: recorded},
		},
		{
			name:  "stale create is skipped",
			seed:  []params.Job{recorded},
			event: jobEvent(dbCommon.CreateOperation, s.job(1, params.JobStatusQueued, jobEventOlder)),
			want:  map[int64]params.Job{1: recorded},
		},
		{
			name:  "update with equal timestamp is recorded",
			seed:  []params.Job{recorded},
			event: jobEvent(dbCommon.UpdateOperation, s.job(1, params.JobStatusQueued, jobEventBaseTime)),
			want: map[int64]params.Job{
				1: s.job(1, params.JobStatusQueued, jobEventBaseTime),
			},
		},
		{
			name:  "stale update for an unrecorded job is recorded",
			seed:  []params.Job{recorded},
			event: jobEvent(dbCommon.UpdateOperation, s.job(2, params.JobStatusQueued, jobEventOlder)),
			want: map[int64]params.Job{
				1: recorded,
				2: s.job(2, params.JobStatusQueued, jobEventOlder),
			},
		},
		{
			name:  "stale update does not evict other jobs",
			seed:  []params.Job{recorded, s.job(2, params.JobStatusQueued, jobEventBaseTime)},
			event: jobEvent(dbCommon.UpdateOperation, s.job(1, params.JobStatusQueued, jobEventOlder)),
			want: map[int64]params.Job{
				1: recorded,
				2: s.job(2, params.JobStatusQueued, jobEventBaseTime),
			},
		},
		{
			name:  "completed job is removed",
			seed:  []params.Job{recorded},
			event: jobEvent(dbCommon.UpdateOperation, s.job(1, params.JobStatusCompleted, jobEventNewer)),
			want:  map[int64]params.Job{},
		},
		{
			// The staleness guard only protects live jobs; a completed job is
			// evicted regardless of how its timestamp compares.
			name:  "stale completed job is still removed",
			seed:  []params.Job{recorded},
			event: jobEvent(dbCommon.UpdateOperation, s.job(1, params.JobStatusCompleted, jobEventOlder)),
			want:  map[int64]params.Job{},
		},
		{
			name:  "delete removes the job",
			seed:  []params.Job{recorded},
			event: jobEvent(dbCommon.DeleteOperation, s.job(1, params.JobStatusInProgress, jobEventNewer)),
			want:  map[int64]params.Job{},
		},
		{
			name:  "job belonging to another entity is ignored",
			seed:  []params.Job{recorded},
			event: jobEvent(dbCommon.DeleteOperation, makeWatcherJob(1, params.JobStatusInProgress, jobEventNewer, &otherRepoID)),
			want:  map[int64]params.Job{1: recorded},
		},
		{
			name: "payload that is not a job is ignored",
			seed: []params.Job{recorded},
			event: dbCommon.ChangePayload{
				EntityType: dbCommon.JobEntityType,
				Operation:  dbCommon.UpdateOperation,
				Payload:    params.Instance{Name: "not-a-job"},
			},
			want: map[int64]params.Job{1: recorded},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			mgr := s.newManager(tt.seed...)
			mgr.handleWatcherEvent(tt.event)
			s.Require().Equal(tt.want, mgr.jobs)
		})
	}
}

// TestJobOutOfOrderSequence walks one job through an out of order burst and
// checks we end up on the newest update, not the last delivered.
func (s *WatcherJobCacheTestSuite) TestJobOutOfOrderSequence() {
	mgr := s.newManager()

	queued := s.job(1, params.JobStatusQueued, jobEventBaseTime)
	inProgress := s.job(1, params.JobStatusInProgress, jobEventNewer)

	mgr.handleWatcherEvent(jobEvent(dbCommon.CreateOperation, queued))
	s.Require().Equal(queued, mgr.jobs[1])

	mgr.handleWatcherEvent(jobEvent(dbCommon.UpdateOperation, inProgress))
	s.Require().Equal(inProgress, mgr.jobs[1])

	// A duplicate of the original queued event arrives late. Without the
	// staleness guard this would roll the cached job back to queued.
	mgr.handleWatcherEvent(jobEvent(dbCommon.UpdateOperation, queued))
	s.Require().Equal(inProgress, mgr.jobs[1], "late queued event must not overwrite the newer in_progress state")

	// The job finishes; the cache entry goes away.
	mgr.handleWatcherEvent(jobEvent(dbCommon.UpdateOperation, s.job(1, params.JobStatusCompleted, jobEventNewer.Add(time.Minute))))
	s.Require().Empty(mgr.jobs)
}

// TestJobBurstOutOfOrder pushes 1200 job events through in several hostile
// orders. However they arrive, we must end up on the newest update for every
// job: a stale queued update that lands on top of a newer in_progress one puts a
// running job back on the queue and buys it a runner it does not need.
func (s *WatcherJobCacheTestSuite) TestJobBurstOutOfOrder() {
	orders := []struct {
		name    string
		reorder func([]dbCommon.ChangePayload)
	}{
		{
			// Worst case: every job's events arrive newest first.
			name:    "reversed",
			reorder: slices.Reverse[[]dbCommon.ChangePayload],
		},
		{
			name:    "shuffled",
			reorder: func(events []dbCommon.ChangePayload) { deterministicShuffle(events, shuffleSeed1) },
		},
		{
			name:    "shuffled with a different seed",
			reorder: func(events []dbCommon.ChangePayload) { deterministicShuffle(events, shuffleSeed2) },
		},
	}

	for _, tt := range orders {
		s.Run(tt.name, func() {
			events, want, queuedIDs, retiredIDs := s.buildJobBurst()
			tt.reorder(events)

			mgr := s.newManager()
			for _, event := range events {
				mgr.handleWatcherEvent(event)
			}

			s.Require().Equal(want, mgr.jobs)
			s.Require().Equal(queuedIDs, queuedJobIDs(mgr),
				"only jobs that are genuinely still queued may be eligible for a runner")
			s.Require().Equal(retiredIDs, tombstonedJobIDs(mgr),
				"every finished job must be retired behind a tombstone, whenever its completion was delivered")
		})
	}
}

// TestJobBurstConcurrent delivers the same burst from several goroutines at
// once. We compare timestamps under r.mux, so the result cannot depend on how
// the deliveries interleave.
func (s *WatcherJobCacheTestSuite) TestJobBurstConcurrent() {
	events, want, queuedIDs, retiredIDs := s.buildJobBurst()
	deterministicShuffle(events, shuffleSeed3)

	mgr := s.newManager()

	const workers = 8
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			for i := start; i < len(events); i += workers {
				mgr.handleWatcherEvent(events[i])
			}
		}(w)
	}
	wg.Wait()

	s.Require().Equal(want, mgr.jobs)
	s.Require().Equal(queuedIDs, queuedJobIDs(mgr))
	s.Require().Equal(retiredIDs, tombstonedJobIDs(mgr))
}

// TestJobCompletionOutOfOrder covers the reorder the cache cannot settle on its
// own: a completion removes the entry we would have compared against, so without
// a tombstone an update arriving behind it puts the job back.
func (s *WatcherJobCacheTestSuite) TestJobCompletionOutOfOrder() {
	queued := s.job(1, params.JobStatusQueued, jobEventBaseTime)
	inProgress := s.job(1, params.JobStatusInProgress, jobEventNewer)
	completed := s.job(1, params.JobStatusCompleted, jobEventNewer.Add(time.Minute))

	// Each case is the same job delivered in a different order. However the
	// events are reordered, the job must not be left in the cache.
	tests := []struct {
		name   string
		events []dbCommon.ChangePayload
	}{
		{
			name: "in order",
			events: []dbCommon.ChangePayload{
				jobEvent(dbCommon.CreateOperation, queued),
				jobEvent(dbCommon.UpdateOperation, inProgress),
				jobEvent(dbCommon.UpdateOperation, completed),
			},
		},
		{
			name: "queued, completion, then the reordered in_progress",
			events: []dbCommon.ChangePayload{
				jobEvent(dbCommon.CreateOperation, queued),
				jobEvent(dbCommon.UpdateOperation, completed),
				jobEvent(dbCommon.UpdateOperation, inProgress),
			},
		},
		{
			name: "completion first, then both updates",
			events: []dbCommon.ChangePayload{
				jobEvent(dbCommon.UpdateOperation, completed),
				jobEvent(dbCommon.CreateOperation, queued),
				jobEvent(dbCommon.UpdateOperation, inProgress),
			},
		},
		{
			name: "completion first, then the queued update",
			events: []dbCommon.ChangePayload{
				jobEvent(dbCommon.UpdateOperation, completed),
				jobEvent(dbCommon.UpdateOperation, queued),
			},
		},
		{
			name: "removal from the store, then a reordered update",
			events: []dbCommon.ChangePayload{
				jobEvent(dbCommon.CreateOperation, queued),
				jobEvent(dbCommon.DeleteOperation, completed),
				jobEvent(dbCommon.UpdateOperation, inProgress),
			},
		},
		{
			name: "a second retirement, itself reordered, keeps the job down",
			events: []dbCommon.ChangePayload{
				jobEvent(dbCommon.UpdateOperation, completed),
				jobEvent(dbCommon.DeleteOperation, inProgress),
				jobEvent(dbCommon.UpdateOperation, queued),
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			mgr := s.newManager()
			for _, event := range tt.events {
				mgr.handleWatcherEvent(event)
			}

			s.Require().Empty(mgr.jobs, "a finished job must not be left in the cache")
			s.Require().Empty(mgr.getQueuedJobs(), "a finished job must not be eligible for a runner")
			s.Require().Contains(mgr.jobTombstones, int64(1), "the retired job must leave a tombstone behind")
		})
	}
}

// TestTombstoneHoldsTheJobDown checks that a tombstoned job stays down whatever
// arrives for it. Dropping newer updates too is safe, as the store never reuses
// a job ID and a completed job never transitions back out.
func (s *WatcherJobCacheTestSuite) TestTombstoneHoldsTheJobDown() {
	mgr := s.newManager()

	mgr.handleWatcherEvent(jobEvent(dbCommon.UpdateOperation, s.job(1, params.JobStatusCompleted, jobEventBaseTime)))
	s.Require().Empty(mgr.jobs)
	s.Require().True(mgr.jobIsRetired(1))

	// Older and newer updates alike are held off while the tombstone stands.
	for _, updatedAt := range []time.Time{jobEventOlder, jobEventNewer} {
		mgr.handleWatcherEvent(jobEvent(dbCommon.UpdateOperation, s.job(1, params.JobStatusQueued, updatedAt)))
		s.Require().Empty(mgr.jobs)
		s.Require().Empty(mgr.getQueuedJobs())
	}

	// Only reaping releases the ID. By then nothing can still be in flight for
	// it, so this is memory reclamation rather than part of the semantics.
	mgr.jobTombstones[1] = time.Now().Add(-jobTombstoneTTL - time.Minute)
	s.Require().True(mgr.jobIsRetired(1), "an old tombstone still holds the job down until it is reaped")

	s.Require().NoError(mgr.reapJobTombstones())
	s.Require().False(mgr.jobIsRetired(1))

	revived := s.job(1, params.JobStatusQueued, jobEventNewer)
	mgr.handleWatcherEvent(jobEvent(dbCommon.UpdateOperation, revived))
	s.Require().Equal(map[int64]params.Job{1: revived}, mgr.jobs)
}

// TestReapJobTombstones checks that we drop tombstones older than
// jobTombstoneTTL and keep the fresh ones.
func (s *WatcherJobCacheTestSuite) TestReapJobTombstones() {
	mgr := s.newManager()

	// Retire three jobs, then age two of the tombstones past the TTL.
	for id := int64(1); id <= 3; id++ {
		mgr.handleWatcherEvent(jobEvent(dbCommon.UpdateOperation, s.job(id, params.JobStatusCompleted, jobEventBaseTime)))
	}
	s.Require().Len(mgr.jobTombstones, 3)

	expired := time.Now().Add(-jobTombstoneTTL - time.Minute)
	for _, id := range []int64{1, 2} {
		mgr.jobTombstones[id] = expired
	}

	s.Require().NoError(mgr.reapJobTombstones())
	s.Require().Len(mgr.jobTombstones, 1)
	s.Require().Contains(mgr.jobTombstones, int64(3), "a fresh tombstone must survive the reaper")

	// Once the tombstone is gone the guard has nothing left to compare against,
	// which is the point of the TTL: by then no update can still be in flight.
	stale := s.job(1, params.JobStatusQueued, jobEventOlder)
	mgr.handleWatcherEvent(jobEvent(dbCommon.UpdateOperation, stale))
	s.Require().Equal(stale, mgr.jobs[1])

	// The job retired a moment ago is still defended.
	mgr.handleWatcherEvent(jobEvent(dbCommon.UpdateOperation, s.job(3, params.JobStatusQueued, jobEventOlder)))
	s.Require().NotContains(mgr.jobs, int64(3))
}

// TestReapJobTombstonesEmpty checks the reaper is a no-op with nothing to reap.
func (s *WatcherJobCacheTestSuite) TestReapJobTombstonesEmpty() {
	mgr := s.newManager()

	s.Require().NoError(mgr.reapJobTombstones())
	s.Require().Empty(mgr.jobTombstones)
}

func TestWatcherJobCacheTestSuite(t *testing.T) {
	suite.Run(t, new(WatcherJobCacheTestSuite))
}
