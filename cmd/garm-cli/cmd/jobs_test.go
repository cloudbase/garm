// Copyright 2026 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package cmd

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/cloudbase/garm/params"
)

func testJobListings() ([]params.Job, []params.ScaleSetJob) {
	now := time.Now()
	workflow := []params.Job{
		{
			WorkflowJobID:   100,
			Name:            "build",
			Status:          "queued",
			RepositoryOwner: "test-org",
			RepositoryName:  "test-repo",
			Labels:          []string{"self-hosted", "linux"},
			LockedBy:        uuid.New(),
			CreatedAt:       now.Add(-2 * time.Minute),
		},
		{
			WorkflowJobID:   101,
			Name:            "test",
			Status:          "completed",
			Conclusion:      "success",
			RunnerName:      "garm-pool-runner",
			RepositoryOwner: "test-org",
			RepositoryName:  "test-repo",
			CreatedAt:       now.Add(-4 * time.Minute),
		},
	}
	scaleSet := []params.ScaleSetJob{
		{
			ID:              7,
			ScaleSetJobID:   "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			Name:            "package",
			Status:          "in_progress",
			Result:          "succeeded",
			RunnerName:      "garm-scaleset-runner",
			RepositoryOwner: "test-org",
			RepositoryName:  "test-repo",
			RequestLabels:   []string{"ubuntu-noble"},
			CreatedAt:       now.Add(-3 * time.Minute),
		},
	}
	return workflow, scaleSet
}

func TestMergedJobRowsCoverBothSources(t *testing.T) {
	workflow, scaleSet := testJobListings()
	rows := mergedJobRows(workflow, scaleSet)
	require.Len(t, rows, 3)

	// Newest first, with the scale set job interleaved by age rather than
	// appended after the webhook jobs.
	require.Equal(t,
		[]string{jobSourceWebhook, jobSourceScaleSet, jobSourceWebhook},
		[]string{rows[0].source, rows[1].source, rows[2].source})

	require.Equal(t, "100", rows[0].id)
	require.Equal(t, "self-hosted linux", rows[0].labels)
	require.NotEmpty(t, rows[0].lockedBy, "a locked webhook job must show who holds it")

	// Scale set jobs carry a result instead of a conclusion, request labels
	// instead of labels, and are never locked.
	require.Equal(t, "7", rows[1].id)
	require.Equal(t, "succeeded", rows[1].conclusion)
	require.Equal(t, "ubuntu-noble", rows[1].labels)
	require.Empty(t, rows[1].lockedBy)
}

func TestMergedJobRowsOneSource(t *testing.T) {
	workflow, scaleSet := testJobListings()

	rows := mergedJobRows(workflow, nil)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Equal(t, jobSourceWebhook, row.source)
	}

	rows = mergedJobRows(nil, scaleSet)
	require.Len(t, rows, 1)
	require.Equal(t, jobSourceScaleSet, rows[0].source)

	require.Empty(t, mergedJobRows(nil, nil))
}

// TestRenderJobTables prints both listings so the layout can be eyeballed
// with go test -v.
func TestRenderJobTables(t *testing.T) {
	workflow, scaleSet := testJobListings()

	formatJobs(workflow, scaleSet)

	formatWebhookJobs(params.JobsPaginatedResponse{
		TotalCount:  31,
		Pages:       2,
		CurrentPage: 1,
		Results:     workflow,
	})
}

func TestNextPage(t *testing.T) {
	// The last page points nowhere.
	next, err := nextPage(3, nil)
	require.NoError(t, err)
	require.Zero(t, next)

	// Otherwise the listing says where to go, rather than the caller
	// assuming the page after this one.
	var page uint64 = 4
	next, err = nextPage(3, &page)
	require.NoError(t, err)
	require.EqualValues(t, 4, next)

	// A page that does not advance would be walked forever.
	page = 3
	_, err = nextPage(3, &page)
	require.ErrorContains(t, err, "points back at page 3")

	// A page the client cannot carry is refused rather than truncated.
	page = uint64(math.MaxInt64) + 1
	_, err = nextPage(3, &page)
	require.ErrorContains(t, err, "exceeds max int64 size")
}

func TestMergedJSONJobsKeepsOldShape(t *testing.T) {
	workflow, scaleSet := testJobListings()
	jobs := mergedJSONJobs(workflow, scaleSet)
	require.Len(t, jobs, 3)

	// One flat array of jobs, newest first, the way this listing printed
	// before scale set jobs moved to their own table.
	require.EqualValues(t, 100, jobs[0].WorkflowJobID)
	require.Equal(t, "package", jobs[1].Name)
	require.EqualValues(t, 101, jobs[2].WorkflowJobID)

	// The scale set job reads as a job. Its result lands in conclusion and
	// its request labels in labels, the pairs migration 0010 moved apart.
	ssJob := jobs[1]
	require.Equal(t, "succeeded", ssJob.Conclusion)
	require.Equal(t, []string{"ubuntu-noble"}, ssJob.Labels)
	require.Equal(t, "garm-scaleset-runner", ssJob.RunnerName)
	require.Equal(t, "test-org", ssJob.RepositoryOwner)

	// No workflow job ID and no record ID, which is what set scale set
	// jobs apart in this output and what keeps IDs from colliding.
	require.Zero(t, ssJob.WorkflowJobID)
	require.Zero(t, ssJob.ID)
}

func TestMergedJSONJobsEmptyIsNotNull(t *testing.T) {
	jobs := mergedJSONJobs(nil, nil)
	require.NotNil(t, jobs, "an empty listing must serialize as [] rather than null")
	require.Empty(t, jobs)
}
