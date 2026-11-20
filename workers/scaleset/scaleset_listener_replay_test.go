// Copyright 2026 Cloudbase Solutions SRL
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-github/v84/github"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	dbsql "github.com/cloudbase/garm/database/sql"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/locking"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common/mocks"
	"github.com/cloudbase/garm/util/github/scalesets"
)

var (
	replayLockerOnce sync.Once
	replayLockerErr  error
)

type workerWithClient struct {
	*Worker
	client *scalesets.ScaleSetClient
}

func (w *workerWithClient) GetScaleSetClient() (*scalesets.ScaleSetClient, error) {
	return w.client, nil
}

func TestHandleSessionMessageReplayPreservesTerminalState(t *testing.T) {
	ctx := context.Background()
	watcher.InitWatcher(ctx)
	t.Cleanup(func() { _ = watcher.CloseWatcher() })
	store, err := dbsql.NewSQLStore(ctx, garmTesting.GetTestSqliteDBConfig(t))
	require.NoError(t, err)
	replayLockerOnce.Do(func() {
		var locker locking.Locker
		locker, replayLockerErr = locking.NewLocalLocker(ctx, store)
		if replayLockerErr == nil {
			replayLockerErr = locking.RegisterLocker(locker)
		}
	})
	require.NoError(t, replayLockerErr)
	ctx = garmTesting.ImpersonateAdminContext(ctx, store, t)
	_, err = store.InitController(ctx)
	require.NoError(t, err)
	endpoint := garmTesting.CreateDefaultGithubEndpoint(ctx, store, t)
	creds := garmTesting.CreateTestGithubCredentials(ctx, "replay-creds", store, t, endpoint)
	repo, err := store.CreateRepository(ctx, "test-org", "test-repo", creds, "test-secret", params.PoolBalancerTypeRoundRobin, false)
	require.NoError(t, err)
	entity, err := repo.GetEntity()
	require.NoError(t, err)
	scaleSet, err := store.CreateEntityScaleSet(ctx, entity, params.CreateScaleSetParams{Name: "replay", ProviderName: "test-provider", MaxRunners: 10, Image: "test-image", Flavor: "test-flavor", OSType: commonParams.Linux, OSArch: commonParams.Amd64})
	require.NoError(t, err)
	_, err = store.CreateScaleSetInstance(ctx, scaleSet.ID, params.CreateInstanceParams{Name: "test-runner", Status: commonParams.InstanceRunning, RunnerStatus: params.RunnerIdle, OSType: commonParams.Linux, OSArch: commonParams.Amd64})
	require.NoError(t, err)
	worker, err := NewWorker(ctx, store, scaleSet, nil)
	require.NoError(t, err)
	var base string
	attempts, deletes := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/actions/runner-registration"):
			fmt.Fprintf(w, `{"url":%q,"token":"eyJhbGciOiJub25lIn0.eyJleHAiOjQxMDI0NDQ4MDB9."}`, base)
		case strings.HasSuffix(r.URL.Path, "/sessions"):
			fmt.Fprintf(w, `{"sessionId":"00000000-0000-0000-0000-000000000001","messageQueueUrl":%q,"messageQueueAccessToken":"test-token"}`, base+"/queue")
		case strings.HasSuffix(r.URL.Path, "/acquirejobs"):
			attempts++
			if attempts <= 2 {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
				return
			}
			fmt.Fprint(w, `{"count":1,"value":[123]}`)
		case r.Method == http.MethodDelete && r.URL.Path == "/queue/11":
			deletes++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", 500)
		}
	}))
	defer srv.Close()
	base = srv.URL
	baseURL, err := url.Parse(base + "/")
	require.NoError(t, err)
	gh := mocks.NewGithubClient(t)
	gh.On("GetEntity").Return(params.ForgeEntity{}).Maybe()
	gh.On("GithubBaseURL").Return(baseURL).Maybe()
	token := "registration-token"
	gh.On("CreateEntityRegistrationToken", mock.Anything).Return(&github.RegistrationToken{Token: &token, ExpiresAt: &github.Timestamp{Time: time.Now().Add(time.Hour)}}, (*github.Response)(nil), nil).Maybe()
	client, err := scalesets.NewClient(gh)
	require.NoError(t, err)
	session, err := client.CreateMessageSession(ctx, scaleSet.ScaleSetID, "test")
	require.NoError(t, err)
	defer session.Close()
	listener := newListener(ctx, &workerWithClient{Worker: worker, client: client})
	listener.listenerCtx, listener.messageSession = ctx, session
	job := params.ScaleSetJobMessage{JobID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", RunnerRequestID: 123, RepositoryName: "test-repo", OwnerName: "test-org", RunnerName: "test-runner"}
	available, assigned, started, completed := job, job, job, job
	available.MessageType, assigned.MessageType, started.MessageType, completed.MessageType = params.MessageTypeJobAvailable, params.MessageTypeJobAssigned, params.MessageTypeJobStarted, params.MessageTypeJobCompleted
	completed.Result = "succeeded"
	body, err := json.Marshal([]params.ScaleSetJobMessage{available, assigned, started, completed})
	require.NoError(t, err)
	msg := params.RunnerScaleSetMessage{MessageID: 11, MessageType: string(params.MessageTypeRunnerScaleSetJobMessages), Body: string(body), Statistics: &params.RunnerScaleSetStatistic{TotalAssignedJobs: 1}}
	assertJob := func() {
		t.Helper()
		jobs, err := store.ListScaleSetJobs(ctx, scaleSet.ID)
		require.NoError(t, err)
		require.Len(t, jobs, 1)
		require.Equal(t, string(params.JobStatusCompleted), jobs[0].Status)
		require.Equal(t, "succeeded", jobs[0].Result)
	}
	for i := 0; i < 2; i++ {
		listener.handleSessionMessage(msg)
		require.Zero(t, listener.lastMessageID)
		require.Zero(t, deletes)
		instance, err := store.GetInstance(ctx, "test-runner")
		require.NoError(t, err)
		require.Equal(t, commonParams.InstancePendingDelete, instance.Status)
		require.Equal(t, params.RunnerTerminated, instance.RunnerStatus)
		assertJob()
		if i == 0 {
			// Check the started replay on its own, before a completion could mask a regression.
			require.NoError(t, worker.HandleJobsStarted([]params.ScaleSetJobMessage{started}))
			instance, err = store.GetInstance(ctx, "test-runner")
			require.NoError(t, err)
			require.Equal(t, commonParams.InstancePendingDelete, instance.Status)
			require.Equal(t, params.RunnerTerminated, instance.RunnerStatus)
			assertJob()
		}
	}
	require.NoError(t, store.DeleteInstanceByName(ctx, "test-runner"))
	listener.handleSessionMessage(msg)
	require.Equal(t, int64(11), listener.lastMessageID)
	require.Equal(t, 1, deletes)
	_, err = store.GetInstance(ctx, "test-runner")
	require.ErrorIs(t, err, runnerErrors.ErrNotFound)
	assertJob()
}
