// Copyright 2026 Cloudbase Solutions SRL
// Licensed under the Apache License, Version 2.0.

//go:build testing

package scaleset

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-github/v84/github"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	dbCommon "github.com/cloudbase/garm/database/common"
	dbsql "github.com/cloudbase/garm/database/sql"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common/mocks"
	"github.com/cloudbase/garm/util/github/scalesets"
)

func TestJobAdmissionSQLRetainsUncertainAcquireAcrossWorkerRestart(t *testing.T) {
	for _, failure := range []string{"unavailable", "lost-reply", "partial-reply"} {
		t.Run(failure, func(t *testing.T) {
			ctx := context.Background()
			watcher.InitWatcher(ctx)
			t.Cleanup(func() { _ = watcher.CloseWatcher() })
			store, err := dbsql.NewSQLStore(ctx, garmTesting.GetTestSqliteDBConfig(t))
			require.NoError(t, err)
			ctx = garmTesting.ImpersonateAdminContext(ctx, store, t)
			_, err = store.InitController(ctx)
			require.NoError(t, err)
			endpoint := garmTesting.CreateDefaultGithubEndpoint(ctx, store, t)
			creds := garmTesting.CreateTestGithubCredentials(ctx, "job-admission-creds", store, t, endpoint)
			repo, err := store.CreateRepository(ctx, "test-org", "test-repo", creds, "test-secret", params.PoolBalancerTypeRoundRobin, false)
			require.NoError(t, err)
			entity, err := repo.GetEntity()
			require.NoError(t, err)
			set, err := store.CreateEntityScaleSet(ctx, entity, params.CreateScaleSetParams{Name: "job-admission", ProviderName: "synthetic", Enabled: true, MaxRunners: 1, Image: "test-image", Flavor: "test-flavor", OSType: commonParams.Linux, OSArch: commonParams.Amd64})
			require.NoError(t, err)
			admission := store.(dbCommon.AdmissionStore)
			vector := params.AdmissionVector{CPU: 1, RAM: 1, Disk: 1, VMs: 1, Jobs: 1}
			policy := params.AdmissionConfig{Domain: "test-domain", Provider: "synthetic", Protocol: jobAdmissionProtocol, Generation: 1, RequestedActivationGeneration: 1, Class: vector, Capacity: vector}
			require.NoError(t, admission.ConfigureAdmission(ctx, policy))
			policy.BackendIdentity = strings.Repeat("a", 64)
			policy.Ready, policy.HealthObservedAt, policy.HealthyUntil = true, time.Now().Add(-time.Second), time.Now().Add(time.Hour)
			require.NoError(t, admission.UpdateAdmissionHealth(ctx, policy))
			require.NoError(t, admission.ResumeAdmission(ctx, policy.Domain, policy.Generation))
			owner := fmt.Sprintf("scaleset:%d", set.ID)
			credit, err := admission.EnsureAdmissionCredit(ctx, policy.Domain, owner, vector, policy.Generation)
			require.NoError(t, err)

			var attempts atomic.Int64
			var base string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/actions/runner-registration"):
					fmt.Fprintf(w, `{"url":%q,"token":"eyJhbGciOiJub25lIn0.eyJleHAiOjQxMDI0NDQ4MDB9."}`, base)
				case strings.HasSuffix(r.URL.Path, "/acquirejobs"):
					attempts.Add(1)
					switch failure {
					case "unavailable":
						http.Error(w, "unavailable", http.StatusServiceUnavailable)
					case "lost-reply":
						conn, _, hijackErr := w.(http.Hijacker).Hijack()
						if hijackErr == nil {
							_ = conn.Close()
						}
					case "partial-reply":
						fmt.Fprint(w, `{"count":0,"value":[]}`)
					}
				default:
					t.Errorf("unexpected broker request: %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected", http.StatusNotFound)
				}
			}))
			t.Cleanup(server.Close)
			base = server.URL
			baseURL, err := url.Parse(base + "/")
			require.NoError(t, err)
			gh := mocks.NewGithubClient(t)
			gh.On("GetEntity").Return(params.ForgeEntity{}).Maybe()
			gh.On("GithubBaseURL").Return(baseURL).Maybe()
			token := "job-admission-registration-token"
			gh.On("CreateEntityRegistrationToken", mock.Anything).Return(&github.RegistrationToken{Token: &token, ExpiresAt: &github.Timestamp{Time: time.Now().Add(time.Hour)}}, (*github.Response)(nil), nil).Maybe()
			client, err := scalesets.NewClient(gh)
			require.NoError(t, err)
			job := params.ScaleSetJobMessage{MessageType: params.MessageTypeJobAvailable, RunnerRequestID: 123, JobID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}
			message := jobMessage(t, []params.ScaleSetJobMessage{job}, params.RunnerScaleSetStatistic{})
			worker, err := NewWorker(ctx, store, set, nil)
			require.NoError(t, err)
			require.Error(t, worker.handleJobAdmissionMessage(message, client, "test-token"))
			require.Equal(t, int64(1), attempts.Load())
			retained, jobs, err := admission.GetAdmissionCredit(ctx, policy.Domain, owner)
			require.NoError(t, err)
			require.Equal(t, credit.ID, retained.ID)
			require.Len(t, jobs, 1)
			require.Equal(t, "acquire_sending", jobs[0].State)
			worker, err = NewWorker(ctx, store, set, nil)
			require.NoError(t, err)
			_ = worker.handleJobAdmissionMessage(message, client, "test-token")
			require.Equal(t, int64(1), attempts.Load(), "replaying an uncertain acquisition must not repeat HTTP")
			job.MessageType = params.MessageTypeJobCompleted
			require.NoError(t, worker.handleJobAdmissionMessage(jobMessage(t, []params.ScaleSetJobMessage{job}, params.RunnerScaleSetStatistic{}), client, "test-token"))
			require.Equal(t, int64(1), attempts.Load())
			retained, _, err = admission.GetAdmissionCredit(ctx, policy.Domain, owner)
			require.NoError(t, err)
			require.Equal(t, credit.ID, retained.ID)
			require.NotEqual(t, "released", retained.State)
			require.Error(t, admission.ReleaseUnsentAdmission(ctx, credit.ID))
		})
	}
}
