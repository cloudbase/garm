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

//go:build testing
// +build testing

package sql

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/logger"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

// stmtRecorder captures the SQL gorm emits so a test can assert which tables a
// store call touches.
type stmtRecorder struct {
	logger.Interface
	mu   sync.Mutex
	stmt []string
}

func (r *stmtRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	r.mu.Lock()
	r.stmt = append(r.stmt, sql)
	r.mu.Unlock()
}

func (r *stmtRecorder) writes(table string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var hits []string
	for _, s := range r.stmt {
		upper := strings.ToUpper(s)
		if !strings.HasPrefix(upper, "INSERT") && !strings.HasPrefix(upper, "UPDATE") && !strings.HasPrefix(upper, "DELETE") {
			continue
		}
		if strings.Contains(s, "`"+table+"`") || strings.Contains(s, `"`+table+`"`) {
			hits = append(hits, s)
		}
	}
	return hits
}

// A runner status update must write the instances row and nothing else.
//
// getInstance preloads Pool and ScaleSet, and gorm saves belongs-to
// associations by default, so a plain Save also upserts the parent scale set.
// That turns every runner status write into a write on the scale_sets row.
// PostgreSQL blocks such a write behind any uncommitted update of that row,
// and UpdateEntityScaleSet updates it and then calls the forge inside the same
// transaction. The result is that a slow forge call stalls every runner status
// transition for that scale set, including pending_create to creating.
func TestUpdateInstanceWritesOnlyTheInstanceRow(t *testing.T) {
	ctx := context.Background()
	watcher.InitWatcher(ctx)
	t.Cleanup(func() { watcher.CloseWatcher() })

	store := newTestDB(t)
	adminCtx := garmTesting.ImpersonateAdminContext(ctx, store, t)
	db := store.(*sqlDatabase)

	ep := garmTesting.CreateDefaultGithubEndpoint(adminCtx, store, t)
	creds := garmTesting.CreateTestGithubCredentials(adminCtx, "assoc-creds", store, t, ep)
	repo, err := store.CreateRepository(adminCtx, "assoc-org", "assoc-repo", creds, "secret", params.PoolBalancerTypeRoundRobin, false)
	require.NoError(t, err)
	entity, err := repo.GetEntity()
	require.NoError(t, err)

	scaleSet, err := store.CreateEntityScaleSet(adminCtx, entity, params.CreateScaleSetParams{
		Name: "assoc-ss", ProviderName: "assoc-provider", MaxRunners: 10,
		Image: "img", Flavor: "flv", OSType: commonParams.Linux, OSArch: commonParams.Amd64,
	})
	require.NoError(t, err)

	instance, err := store.CreateScaleSetInstance(adminCtx, scaleSet.ID, params.CreateInstanceParams{
		Name:         "assoc-runner",
		Status:       commonParams.InstancePendingCreate,
		RunnerStatus: params.RunnerPending,
		OSType:       commonParams.Linux,
		OSArch:       commonParams.Amd64,
	})
	require.NoError(t, err)

	rec := &stmtRecorder{Interface: logger.Default.LogMode(logger.Silent)}
	db.conn.Logger = rec

	// Creating a runner must not write the parent either, or a new runner
	// cannot even be recorded while a scale set update is in flight.
	created, err := store.CreateScaleSetInstance(adminCtx, scaleSet.ID, params.CreateInstanceParams{
		Name:         "assoc-runner-2",
		Status:       commonParams.InstancePendingCreate,
		RunnerStatus: params.RunnerPending,
		OSType:       commonParams.Linux,
		OSArch:       commonParams.Amd64,
	})
	require.NoError(t, err)
	require.Equal(t, scaleSet.ID, created.ScaleSetID, "the foreign key must still be set")
	require.Equal(t, scaleSet.ProviderName, created.ProviderName,
		"ProviderName is read off the association, so it must stay populated")
	require.Empty(t, rec.writes("scale_sets"),
		"creating a runner must not write the scale_sets row")

	rec.mu.Lock()
	rec.stmt = nil
	rec.mu.Unlock()

	// The transition the instance manager performs first.
	updated, err := store.UpdateInstance(adminCtx, instance.Name, params.UpdateInstanceParams{
		Status: commonParams.InstanceCreating,
	})
	require.NoError(t, err)
	require.Equal(t, commonParams.InstanceCreating, updated.Status)

	require.Empty(t, rec.writes("scale_sets"),
		"updating a runner must not write the scale_sets row")
	require.Empty(t, rec.writes("pools"),
		"updating a runner must not write the pools row")
	require.NotEmpty(t, rec.writes("instances"),
		"the instance row itself must still be written")
}

// Writing a scale set must touch only the scale_sets row.
//
// getScaleSetByID preloads Instances and the owning entity, and gorm saves
// associations even for a map based Updates, so the listener's per batch
// checkpoint also upserted every instance row and the owner row. Paired with
// updateInstance, which locks an instance row and then wrote the scale set
// row, that is a lock cycle. PostgreSQL reports it as SQLSTATE 40P01.
func TestScaleSetWritesOnlyTheScaleSetRow(t *testing.T) {
	ctx := context.Background()
	watcher.InitWatcher(ctx)
	t.Cleanup(func() { watcher.CloseWatcher() })

	store := newTestDB(t)
	adminCtx := garmTesting.ImpersonateAdminContext(ctx, store, t)
	db := store.(*sqlDatabase)

	ep := garmTesting.CreateDefaultGithubEndpoint(adminCtx, store, t)
	creds := garmTesting.CreateTestGithubCredentials(adminCtx, "ss-assoc-creds", store, t, ep)
	repo, err := store.CreateRepository(adminCtx, "ss-assoc-org", "ss-assoc-repo", creds, "secret", params.PoolBalancerTypeRoundRobin, false)
	require.NoError(t, err)
	entity, err := repo.GetEntity()
	require.NoError(t, err)

	scaleSet, err := store.CreateEntityScaleSet(adminCtx, entity, params.CreateScaleSetParams{
		Name: "ss-assoc", ProviderName: "p", MaxRunners: 10,
		Image: "img", Flavor: "flv", OSType: commonParams.Linux, OSArch: commonParams.Amd64,
	})
	require.NoError(t, err)

	_, err = store.CreateScaleSetInstance(adminCtx, scaleSet.ID, params.CreateInstanceParams{
		Name:   "ss-assoc-runner",
		Status: commonParams.InstancePendingCreate, RunnerStatus: params.RunnerPending,
		OSType: commonParams.Linux, OSArch: commonParams.Amd64,
	})
	require.NoError(t, err)

	rec := &stmtRecorder{Interface: logger.Default.LogMode(logger.Silent)}
	db.conn.Logger = rec

	// The listener writes both of these on every message batch.
	require.NoError(t, store.SetScaleSetLastMessageID(adminCtx, scaleSet.ID, 42))
	require.NoError(t, store.SetScaleSetDesiredRunnerCount(adminCtx, scaleSet.ID, 3))

	require.Empty(t, rec.writes("instances"),
		"checkpointing a scale set must not write runner rows, that is the other half of the deadlock")
	require.Empty(t, rec.writes("repositories"),
		"checkpointing a scale set must not write the owning entity row")
	require.NotEmpty(t, rec.writes("scale_sets"),
		"the scale set row itself must still be written")
}

// The same guard for pools. updatePool omitted the owner and tags but not
// Instances, which getEntityPool preloads.
func TestPoolUpdateWritesOnlyThePoolRow(t *testing.T) {
	ctx := context.Background()
	watcher.InitWatcher(ctx)
	t.Cleanup(func() { watcher.CloseWatcher() })

	store := newTestDB(t)
	adminCtx := garmTesting.ImpersonateAdminContext(ctx, store, t)
	db := store.(*sqlDatabase)

	ep := garmTesting.CreateDefaultGithubEndpoint(adminCtx, store, t)
	creds := garmTesting.CreateTestGithubCredentials(adminCtx, "pool-assoc-creds", store, t, ep)
	repo, err := store.CreateRepository(adminCtx, "pool-assoc-org", "pool-assoc-repo", creds, "secret", params.PoolBalancerTypeRoundRobin, false)
	require.NoError(t, err)
	entity, err := repo.GetEntity()
	require.NoError(t, err)

	pool, err := store.CreateEntityPool(adminCtx, entity, params.CreatePoolParams{
		ProviderName: "p", MaxRunners: 4, MinIdleRunners: 0,
		Image: "img", Flavor: "flv", OSType: commonParams.Linux, OSArch: commonParams.Amd64,
		Tags: []string{"linux"},
	})
	require.NoError(t, err)

	_, err = store.CreateInstance(adminCtx, pool.ID, params.CreateInstanceParams{
		Name:   "pool-assoc-runner",
		Status: commonParams.InstancePendingCreate, RunnerStatus: params.RunnerPending,
		OSType: commonParams.Linux, OSArch: commonParams.Amd64,
	})
	require.NoError(t, err)

	rec := &stmtRecorder{Interface: logger.Default.LogMode(logger.Silent)}
	db.conn.Logger = rec

	maxRunners := uint(7)
	_, err = store.UpdateEntityPool(adminCtx, entity, pool.ID, params.UpdatePoolParams{MaxRunners: &maxRunners})
	require.NoError(t, err)

	require.Empty(t, rec.writes("instances"),
		"updating a pool must not write runner rows")
	require.Empty(t, rec.writes("repositories"),
		"updating a pool must not write the owning entity row")
}
