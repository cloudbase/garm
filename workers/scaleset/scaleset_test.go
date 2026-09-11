// Copyright 2026 Cloudbase Solutions SRL
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

//go:build testing

package scaleset

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-github/v84/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/cache"
	storeMocks "github.com/cloudbase/garm/database/common/mocks"
	"github.com/cloudbase/garm/locking"
	"github.com/cloudbase/garm/params"
	runnerMocks "github.com/cloudbase/garm/runner/common/mocks"
)

const runnerRegistrationPath = "/actions/runner-registration"

var registerTestLocker = sync.OnceValue(func() error {
	locker, err := locking.NewLocalLocker(context.Background(), nil)
	if err != nil {
		return err
	}
	return locking.RegisterLocker(locker)
})

func TestDeletingInstancesDoNotCountAsRunnersButConsumeProviderSlots(t *testing.T) {
	for _, status := range []commonParams.InstanceStatus{
		commonParams.InstancePendingDelete,
		commonParams.InstancePendingForceDelete,
		commonParams.InstanceDeleting,
	} {
		t.Run(string(status), func(t *testing.T) {
			w := &Worker{
				scaleSet: params.ScaleSet{MinIdleRunners: 1, MaxRunners: 1},
				runners: map[string]params.Instance{
					"runner": {Status: status},
				},
			}

			assert.Zero(t, w.runnerCount())
			assert.Zero(t, w.runnersToAdd())

			w.runners["runner"] = params.Instance{Status: commonParams.InstanceDeleted}
			assert.Equal(t, 1, w.runnersToAdd())
		})
	}
}

func TestDeletingInstancesDoNotIncreaseScaleDownDelta(t *testing.T) {
	w := &Worker{
		scaleSet: params.ScaleSet{DesiredRunnerCount: 1, MaxRunners: 1},
		runners: map[string]params.Instance{
			"running-1": {Status: commonParams.InstanceRunning},
			"running-2": {Status: commonParams.InstanceRunning},
			"deleting":  {Status: commonParams.InstanceDeleting},
		},
	}

	assert.Equal(t, 1, w.runnerCount()-w.targetRunners())
}

func TestScaleDownRemovesExcessIdleRunnerDespitePendingDeletion(t *testing.T) {
	const entityID = "scale-down-repo"
	ctx := context.Background()
	require.NoError(t, registerTestLocker())

	var removals atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case runnerRegistrationPath:
			_, _ = fmt.Fprintf(w, `{"url":%q,"token":"eyJhbGciOiJub25lIn0.eyJleHAiOjQxNDk5MzYwMDB9."}`, server.URL)
		case "/_apis/distributedtask/pools/0/agents/1", "/_apis/distributedtask/pools/0/agents/2":
			assert.Equal(t, http.MethodDelete, r.Method)
			removals.Add(1)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	entity := params.ForgeEntity{ID: entityID, EntityType: params.ForgeEntityTypeRepository, Owner: "owner", Name: "repo"}
	cli := runnerMocks.NewGithubClient(t)
	cli.EXPECT().GetEntity().Return(entity).Maybe()
	cli.EXPECT().GithubBaseURL().Return(baseURL).Maybe()
	cli.EXPECT().CreateEntityRegistrationToken(mock.Anything).Return(&github.RegistrationToken{
		Token: github.Ptr("registration-token"), ExpiresAt: &github.Timestamp{Time: time.Now().Add(time.Hour)},
	}, nil, nil).Maybe()
	cache.SetGithubClient(entityID, cli)
	t.Cleanup(func() { cache.DeleteGithubClient(entityID) })
	store := storeMocks.NewStore(t)
	store.EXPECT().UpdateInstance(mock.Anything, mock.Anything, params.UpdateInstanceParams{Status: commonParams.InstancePendingDelete}).
		RunAndReturn(func(_ context.Context, name string, _ params.UpdateInstanceParams) (params.Instance, error) {
			return params.Instance{ID: name, Name: name, Status: commonParams.InstancePendingDelete}, nil
		}).Once()
	w := &Worker{
		ctx: ctx, store: store, consumerID: "scale-down-test",
		scaleSet: params.ScaleSet{RepoID: entityID, DesiredRunnerCount: 1, MaxRunners: 3},
		runners: map[string]params.Instance{
			"idle-1":   {ID: "idle-1", Name: "idle-1", AgentID: 1, Status: commonParams.InstanceRunning, RunnerStatus: params.RunnerIdle},
			"idle-2":   {ID: "idle-2", Name: "idle-2", AgentID: 2, Status: commonParams.InstanceRunning, RunnerStatus: params.RunnerIdle},
			"deleting": {ID: "deleting", Name: "deleting", Status: commonParams.InstanceDeleting},
		},
	}
	w.handleScaleDown()
	assert.EqualValues(t, 1, removals.Load())
	assert.Equal(t, 1, w.runnerCount())
}

func TestPendingCreatePreventsDuplicateReplacement(t *testing.T) {
	w := &Worker{
		scaleSet: params.ScaleSet{MinIdleRunners: 1, MaxRunners: 1},
		runners: map[string]params.Instance{
			"deleted":     {Status: commonParams.InstanceDeleted},
			"replacement": {Status: commonParams.InstancePendingCreate},
		},
	}

	assert.Equal(t, 1, w.runnerCount())
	assert.Equal(t, w.targetRunners(), w.runnerCount())
	assert.Zero(t, w.runnersToAdd())
}

func TestRunnersToAddHandlesLargeMaximum(t *testing.T) {
	w := &Worker{
		scaleSet: params.ScaleSet{MinIdleRunners: 1, MaxRunners: ^uint(0)},
		runners:  make(map[string]params.Instance),
	}

	assert.Equal(t, 1, w.runnersToAdd())
}

func TestRunnersToAddCapsDeficitAtRemainingProviderSlots(t *testing.T) {
	w := &Worker{
		scaleSet: params.ScaleSet{MinIdleRunners: 5, MaxRunners: 5},
		runners: map[string]params.Instance{
			"running":       {Status: commonParams.InstanceRunning},
			"pending":       {Status: commonParams.InstancePendingDelete},
			"pending-force": {Status: commonParams.InstancePendingForceDelete},
			"deleting":      {Status: commonParams.InstanceDeleting},
		},
	}
	assert.Equal(t, 1, w.runnerCount())
	assert.EqualValues(t, 4, w.providerSlotsInUse())
	assert.Equal(t, 4, w.targetRunners()-w.runnerCount())
	assert.Equal(t, 1, w.runnersToAdd())
}

func TestTargetRunnersHonorsMaximum(t *testing.T) {
	w := &Worker{
		scaleSet: params.ScaleSet{
			MinIdleRunners:     2,
			DesiredRunnerCount: 3,
			MaxRunners:         4,
		},
	}

	assert.Equal(t, 4, w.targetRunners())
}

func TestReconcileRunnersCleansDeletedRowsAndReplacesStaleCache(t *testing.T) {
	store := storeMocks.NewStore(t)
	store.EXPECT().
		ListScaleSetInstances(mock.Anything, uint(4), false).
		Return([]params.Instance{
			{ID: "deleted", Name: "deleted", Status: commonParams.InstanceDeleted, ScaleSetID: 4},
			{ID: "replacement", Name: "replacement", Status: commonParams.InstancePendingCreate, ScaleSetID: 4},
		}, nil)
	store.EXPECT().
		DeleteInstanceByName(mock.Anything, "deleted").
		Return(nil)

	w := &Worker{
		ctx:      context.Background(),
		store:    store,
		scaleSet: params.ScaleSet{ID: 4, MinIdleRunners: 1, MaxRunners: 1},
		runners: map[string]params.Instance{
			"stale": {ID: "stale", Name: "stale", Status: commonParams.InstanceRunning, ScaleSetID: 4},
		},
	}

	refreshed, err := w.reconcileRunners()
	assert.NoError(t, err)
	assert.True(t, refreshed)
	assert.Equal(t, 1, w.runnerCount())
	assert.Len(t, w.runners, 1)
	assert.Contains(t, w.runners, "replacement")
}

func TestReconcileRunnersContinuesAfterCleanupFailure(t *testing.T) {
	store := storeMocks.NewStore(t)
	store.EXPECT().
		ListScaleSetInstances(mock.Anything, uint(4), false).
		Return([]params.Instance{
			{ID: "failed", Name: "failed", Status: commonParams.InstanceDeleted, ScaleSetID: 4},
			{ID: "cleaned", Name: "cleaned", Status: commonParams.InstanceDeleted, ScaleSetID: 4},
		}, nil)
	store.EXPECT().
		DeleteInstanceByName(mock.Anything, "failed").
		Return(errors.New("database error"))
	store.EXPECT().
		DeleteInstanceByName(mock.Anything, "cleaned").
		Return(nil)

	w := &Worker{
		ctx:      context.Background(),
		store:    store,
		scaleSet: params.ScaleSet{ID: 4},
	}

	refreshed, err := w.reconcileRunners()
	assert.True(t, refreshed)
	assert.ErrorContains(t, err, "deleting instance failed")
	assert.NotContains(t, w.runners, "cleaned")
}

func TestReconcileDeletedRunnerCreatesOneReplacement(t *testing.T) {
	const (
		entityID   = "repo-id"
		scaleSetID = 4
	)

	var registrationRequests atomic.Int32
	var jitRequests atomic.Int32
	encodedJIT := base64.StdEncoding.EncodeToString([]byte(`{"key":"value"}`))

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case runnerRegistrationPath:
			registrationRequests.Add(1)
			assert.Equal(t, http.MethodPost, r.Method)
			_, _ = fmt.Fprintf(w, `{"url":%q,"token":"eyJhbGciOiJub25lIn0.eyJleHAiOjQxNDk5MzYwMDB9."}`, server.URL)
		case "/_apis/runtime/runnerscalesets/42/generatejitconfig":
			jitRequests.Add(1)
			assert.Equal(t, http.MethodPost, r.Method)
			_, _ = fmt.Fprintf(w, `{"runner":{"id":99},"encodedJITConfig":%q}`, encodedJIT)
		default:
			http.NotFound(w, r)
			t.Errorf("unexpected GitHub request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	entity := params.ForgeEntity{
		ID:         entityID,
		EntityType: params.ForgeEntityTypeRepository,
		Owner:      "owner",
		Name:       "repo",
	}
	githubClient := runnerMocks.NewGithubClient(t)
	githubClient.EXPECT().
		CreateEntityRegistrationToken(mock.Anything).
		Return(&github.RegistrationToken{
			Token:     github.Ptr("registration-token"),
			ExpiresAt: &github.Timestamp{Time: time.Now().Add(time.Hour)},
		}, nil, nil).
		Once()
	githubClient.EXPECT().GithubBaseURL().Return(baseURL).Once()
	githubClient.EXPECT().GetEntity().Return(entity).Maybe()
	cache.SetGithubClient(entityID, githubClient)
	t.Cleanup(func() { cache.DeleteGithubClient(entityID) })

	store := storeMocks.NewStore(t)
	store.EXPECT().
		ListScaleSetInstances(mock.Anything, uint(scaleSetID), false).
		Return([]params.Instance{{
			ID:           "deleted",
			Name:         "deleted",
			Status:       commonParams.InstanceDeleted,
			RunnerStatus: params.RunnerIdle,
			ScaleSetID:   scaleSetID,
		}}, nil).
		Once()
	store.EXPECT().DeleteInstanceByName(mock.Anything, "deleted").Return(nil).Once()
	store.EXPECT().ControllerInfo().Return(params.ControllerInfo{
		CallbackURL: "https://garm.example/callback",
		MetadataURL: "https://garm.example/metadata",
	}, nil).Once()
	store.EXPECT().
		CreateScaleSetInstance(
			mock.Anything,
			uint(scaleSetID),
			mock.MatchedBy(func(create params.CreateInstanceParams) bool {
				return create.Status == commonParams.InstancePendingCreate &&
					create.RunnerStatus == params.RunnerPending &&
					create.CallbackURL == "https://garm.example/callback" &&
					create.MetadataURL == "https://garm.example/metadata" &&
					create.AgentID == 99 &&
					create.JitConfiguration["key"] == "value"
			}),
		).
		Return(params.Instance{
			ID:           "replacement",
			Name:         "replacement",
			Status:       commonParams.InstancePendingCreate,
			RunnerStatus: params.RunnerPending,
			ScaleSetID:   scaleSetID,
		}, nil).
		Once()

	w := &Worker{
		ctx:   context.Background(),
		store: store,
		scaleSet: params.ScaleSet{
			ID:             scaleSetID,
			ScaleSetID:     42,
			RepoID:         entityID,
			Enabled:        true,
			MinIdleRunners: 1,
			MaxRunners:     1,
		},
		runners: map[string]params.Instance{
			"stale": {ID: "stale", Status: commonParams.InstanceRunning},
		},
	}

	refreshed, err := w.reconcileRunners()
	require.NoError(t, err)
	require.True(t, refreshed)
	w.handleScaleUp()
	w.handleScaleUp()

	assert.EqualValues(t, 1, registrationRequests.Load())
	assert.EqualValues(t, 1, jitRequests.Load())
	assert.Len(t, w.runners, 1)
	assert.Equal(t, commonParams.InstancePendingCreate, w.runners["replacement"].Status)
}

func TestMarkScaleSetCreated(t *testing.T) {
	store := storeMocks.NewStore(t)
	entity := params.ForgeEntity{ID: "repo-id", EntityType: params.ForgeEntityTypeRepository}
	store.EXPECT().
		UpdateEntityScaleSet(
			mock.Anything,
			entity,
			uint(4),
			mock.MatchedBy(func(update params.UpdateScaleSetParams) bool {
				return update.State != nil && *update.State == params.ScaleSetCreated
			}),
			mock.Anything,
		).
		Return(params.ScaleSet{ID: 4, RepoID: entity.ID, State: params.ScaleSetCreated}, nil)

	w := &Worker{
		ctx:   context.Background(),
		store: store,
		scaleSet: params.ScaleSet{
			ID:     4,
			RepoID: entity.ID,
			State:  params.ScaleSetPendingCreate,
		},
	}

	assert.NoError(t, w.markScaleSetCreated())
	assert.Equal(t, params.ScaleSetCreated, w.scaleSet.State)
}
