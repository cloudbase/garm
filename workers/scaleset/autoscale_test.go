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
	"github.com/cloudbase/garm/params"
	runnerMocks "github.com/cloudbase/garm/runner/common/mocks"
)

func TestAutoscaleRequiresFreshRunnerState(t *testing.T) {
	require.NoError(t, registerTestLocker())
	for _, direction := range []string{"up", "down"} {
		t.Run(direction, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			entityID := "refresh-" + direction
			entity := params.ForgeEntity{ID: entityID, EntityType: params.ForgeEntityTypeRepository, Owner: "owner", Name: "repo"}
			cache.SetGithubToolsCache(entity, nil)
			defer cache.DeleteGithubToolsCache(entityID)

			var jitRequests, removals, localCreates, localUpdates atomic.Int32
			encodedJIT := base64.StdEncoding.EncodeToString([]byte(`{"key":"value"}`))
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case runnerRegistrationPath:
					_, _ = fmt.Fprintf(w, `{"url":%q,"token":"eyJhbGciOiJub25lIn0.eyJleHAiOjQxNDk5MzYwMDB9."}`, server.URL)
				case "/_apis/runtime/runnerscalesets/42/generatejitconfig":
					assert.Equal(t, http.MethodPost, r.Method)
					jitRequests.Add(1)
					_, _ = fmt.Fprintf(w, `{"runner":{"id":99},"encodedJITConfig":%q}`, encodedJIT)
				case "/_apis/distributedtask/pools/0/agents/1":
					assert.Equal(t, http.MethodDelete, r.Method)
					removals.Add(1)
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			baseURL, err := url.Parse(server.URL)
			require.NoError(t, err)
			gh := runnerMocks.NewGithubClient(t)
			gh.EXPECT().GetEntity().Return(entity).Maybe()
			gh.EXPECT().GithubBaseURL().Return(baseURL).Maybe()
			gh.EXPECT().CreateEntityRegistrationToken(mock.Anything).Return(&github.RegistrationToken{
				Token: github.Ptr("registration-token"), ExpiresAt: &github.Timestamp{Time: time.Now().Add(time.Hour)},
			}, nil, nil).Maybe()
			cache.SetGithubClient(entityID, gh)
			defer cache.DeleteGithubClient(entityID)

			failedRefresh := make(chan struct{})
			freshRefresh := make(chan struct{})
			idle := params.Instance{ID: "idle", Name: "idle", AgentID: 1, Status: commonParams.InstanceRunning, RunnerStatus: params.RunnerIdle}
			fresh := []params.Instance{{ID: "deleted", Name: "deleted", Status: commonParams.InstanceDeleted}}
			if direction == "down" {
				fresh = append(fresh, idle)
			}
			store := storeMocks.NewStore(t)
			store.EXPECT().ListScaleSetInstances(mock.Anything, uint(4), false).
				Run(func(context.Context, uint, bool) { close(failedRefresh) }).Return(nil, errors.New("database unavailable")).Once()
			store.EXPECT().ListScaleSetInstances(mock.Anything, uint(4), false).
				Run(func(context.Context, uint, bool) { close(freshRefresh) }).Return(fresh, nil).Once()
			// Cleanup failure must not prevent scaling from the successful refresh.
			store.EXPECT().DeleteInstanceByName(mock.Anything, "deleted").Return(errors.New("row cleanup failed")).Once()
			store.EXPECT().ControllerInfo().Return(params.ControllerInfo{}, nil).Maybe()
			store.EXPECT().CreateScaleSetInstance(mock.Anything, uint(4), mock.Anything).
				RunAndReturn(func(_ context.Context, _ uint, p params.CreateInstanceParams) (params.Instance, error) {
					localCreates.Add(1)
					return params.Instance{ID: "new", Name: p.Name, Status: commonParams.InstancePendingCreate}, nil
				}).Maybe()
			store.EXPECT().UpdateInstance(mock.Anything, "idle", params.UpdateInstanceParams{Status: commonParams.InstancePendingDelete}).
				RunAndReturn(func(_ context.Context, _ string, _ params.UpdateInstanceParams) (params.Instance, error) {
					localUpdates.Add(1)
					return params.Instance{ID: "idle", Name: "idle", Status: commonParams.InstancePendingDelete}, nil
				}).Maybe()
			w := &Worker{
				ctx: ctx, store: store, consumerID: entityID, quit: make(chan struct{}),
				scaleSet: params.ScaleSet{ID: 4, RepoID: entityID, ScaleSetID: 42, Enabled: true, MaxRunners: 1},
				runners:  map[string]params.Instance{},
			}
			if direction == "up" {
				w.scaleSet.MinIdleRunners = 1
			} else {
				w.runners[idle.ID] = idle
			}
			done := make(chan struct{})
			go func() {
				defer close(done)
				w.handleAutoScale()
			}()
			defer func() { cancel(); <-done }()
			for tick, signal := range []chan struct{}{failedRefresh, freshRefresh} {
				select {
				case <-signal:
				case <-time.After(15 * time.Second):
					t.Fatal("autoscale did not reconcile")
				}
				// The list call runs under mux; acquiring it waits for the whole tick.
				w.mux.Lock()
				if tick == 0 {
					assert.Zero(t, jitRequests.Load(), "created a JIT runner from stale state")
					assert.Zero(t, removals.Load(), "removed a runner from stale state")
					assert.Zero(t, localCreates.Load())
					assert.Zero(t, localUpdates.Load())
				}
				w.mux.Unlock()
			}
			if direction == "up" {
				assert.EqualValues(t, 1, jitRequests.Load())
				assert.EqualValues(t, 1, localCreates.Load())
				assert.Zero(t, removals.Load())
			} else {
				assert.EqualValues(t, 1, removals.Load())
				assert.EqualValues(t, 1, localUpdates.Load())
				assert.Zero(t, jitRequests.Load())
			}
		})
	}
}
