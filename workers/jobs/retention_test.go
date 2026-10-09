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

package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

func newTestStore(t *testing.T) (dbCommon.Store, context.Context) {
	t.Helper()
	watcher.InitWatcher(context.Background())
	t.Cleanup(func() { watcher.CloseWatcher() })
	dbCfg := garmTesting.GetTestSqliteDBConfig(t)
	db, err := database.NewDatabase(context.Background(), dbCfg)
	require.NoError(t, err)
	adminCtx := garmTesting.ImpersonateAdminContext(context.Background(), db, t)
	return db, adminCtx
}

func TestPruneSweepsBothJobTables(t *testing.T) {
	store, adminCtx := newTestStore(t)

	githubEndpoint := garmTesting.CreateDefaultGithubEndpoint(adminCtx, store, t)
	creds := garmTesting.CreateTestGithubCredentials(adminCtx, "new-creds", store, t, githubEndpoint)
	repo, err := store.CreateRepository(adminCtx, "test-org", "test-repo", creds, "test-webhookSecret", params.PoolBalancerTypeRoundRobin, false)
	require.NoError(t, err)
	repoEntity, err := repo.GetEntity()
	require.NoError(t, err)
	scaleSet, err := store.CreateEntityScaleSet(adminCtx, repoEntity, params.CreateScaleSetParams{
		Name:         "test-scaleset",
		ProviderName: "test-provider",
		MaxRunners:   10,
		Image:        "test-image",
		Flavor:       "test-flavor",
		OSType:       commonParams.Linux,
		OSArch:       commonParams.Amd64,
	})
	require.NoError(t, err)

	// A completed and a queued record in each table.
	_, err = store.CreateOrUpdateScaleSetJob(adminCtx, params.ScaleSetJob{
		ScaleSetJobID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		ScaleSetID:    scaleSet.ID,
		Status:        string(params.JobStatusCompleted),
		Name:          "done",
	})
	require.NoError(t, err)
	_, err = store.CreateOrUpdateScaleSetJob(adminCtx, params.ScaleSetJob{
		ScaleSetJobID: "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
		ScaleSetID:    scaleSet.ID,
		Status:        string(params.JobStatusQueued),
		Name:          "waiting",
	})
	require.NoError(t, err)
	_, err = store.CreateOrUpdateJob(adminCtx, params.Job{
		WorkflowJobID: 100,
		Status:        string(params.JobStatusCompleted),
		Name:          "done",
	})
	require.NoError(t, err)
	_, err = store.CreateOrUpdateJob(adminCtx, params.Job{
		WorkflowJobID: 101,
		Status:        string(params.JobStatusQueued),
		Name:          "waiting",
	})
	require.NoError(t, err)

	w := NewWorker(context.Background(), store)

	// Within the retention age nothing is touched.
	w.prune(retentionAge)
	scaleSetJobs, err := store.ListAllScaleSetJobs(adminCtx)
	require.NoError(t, err)
	require.Len(t, scaleSetJobs, 2)
	jobs, err := store.ListAllJobs(adminCtx)
	require.NoError(t, err)
	require.Len(t, jobs, 2)

	// Past the retention age all scale set records go regardless of
	// status. Webhook jobs keep the queued record, those are still
	// actionable by pools.
	w.prune(-time.Minute)
	scaleSetJobs, err = store.ListAllScaleSetJobs(adminCtx)
	require.NoError(t, err)
	require.Empty(t, scaleSetJobs)
	jobs, err = store.ListAllJobs(adminCtx)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, string(params.JobStatusQueued), jobs[0].Status)
}
