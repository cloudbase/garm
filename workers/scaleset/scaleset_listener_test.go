// Copyright 2026 Cloudbase Solutions SRL
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.
package scaleset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v84/github"
	"github.com/stretchr/testify/require"

	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common/mocks"
	"github.com/cloudbase/garm/util/github/scalesets"
)

type listenerTestHelper struct {
	client  *scalesets.ScaleSetClient
	calls   []string
	lastID  int64
	desired int
	fail    string
}

func (h *listenerTestHelper) GetScaleSet() params.ScaleSet {
	return params.ScaleSet{ID: 1, ScaleSetID: 42, LastMessageID: 10}
}

func (h *listenerTestHelper) GetScaleSetClient() (*scalesets.ScaleSetClient, error) {
	return h.client, nil
}
func (h *listenerTestHelper) Owner() string                     { return "test" }
func (h *listenerTestHelper) SetLastMessageID(id int64) error   { h.lastID = id; return nil }
func (h *listenerTestHelper) SetDesiredRunnerCount(n int) error { h.desired = n; return nil }
func (h *listenerTestHelper) record(jobs []params.ScaleSetJobMessage) error {
	for _, job := range jobs {
		h.calls = append(h.calls, job.MessageType)
	}
	if len(jobs) > 0 && jobs[0].MessageType == h.fail {
		return errors.New("handler failure")
	}
	return nil
}

func (h *listenerTestHelper) HandleJobsAvailable(jobs []params.ScaleSetJobMessage) error {
	return h.record(jobs)
}

func (h *listenerTestHelper) HandleJobsStarted(jobs []params.ScaleSetJobMessage) error {
	return h.record(jobs)
}

func (h *listenerTestHelper) HandleJobsCompleted(jobs []params.ScaleSetJobMessage) error {
	return h.record(jobs)
}

func TestHandleSessionMessageAcquisitionAndLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name           string
		failures       []string
		handlerFailure string
		wantCalls      []string
		wantAck        bool
	}{
		{"service unavailable", []string{"503"}, "", []string{params.MessageTypeJobAssigned, params.MessageTypeJobStarted, params.MessageTypeJobCompleted}, false},
		{"lost reply", []string{"lost"}, "", []string{params.MessageTypeJobAssigned, params.MessageTypeJobStarted, params.MessageTypeJobCompleted}, false},
		{"success", nil, "", []string{params.MessageTypeJobAvailable, params.MessageTypeJobAssigned, params.MessageTypeJobStarted, params.MessageTypeJobCompleted}, true},
		{"acquisition and started handler failure", []string{"503"}, params.MessageTypeJobStarted, []string{params.MessageTypeJobAssigned, params.MessageTypeJobStarted}, false},
		{"acquisition and completed handler failure", []string{"503"}, params.MessageTypeJobCompleted, []string{params.MessageTypeJobAssigned, params.MessageTypeJobStarted, params.MessageTypeJobCompleted}, false},
		{"started handler failure", nil, params.MessageTypeJobStarted, []string{params.MessageTypeJobAvailable, params.MessageTypeJobAssigned, params.MessageTypeJobStarted}, false},
		{"completed handler failure", nil, params.MessageTypeJobCompleted, []string{params.MessageTypeJobAvailable, params.MessageTypeJobAssigned, params.MessageTypeJobStarted, params.MessageTypeJobCompleted}, false},
		{"retry after unavailable", []string{"503", "success"}, "", []string{params.MessageTypeJobAssigned, params.MessageTypeJobStarted, params.MessageTypeJobCompleted, params.MessageTypeJobAvailable, params.MessageTypeJobAssigned, params.MessageTypeJobStarted, params.MessageTypeJobCompleted}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
					var ids []int64
					if err := json.NewDecoder(r.Body).Decode(&ids); err != nil || len(ids) != 1 || ids[0] != 123 {
						t.Errorf("invalid acquisition request: ids=%v, error=%v", ids, err)
						http.Error(w, "invalid request", http.StatusBadRequest)
						return
					}
					outcome := "success"
					if attempts <= len(tc.failures) {
						outcome = tc.failures[attempts-1]
					}
					switch outcome {
					case "503":
						http.Error(w, "unavailable", http.StatusServiceUnavailable)
					case "lost":
						hijacker, ok := w.(http.Hijacker)
						if !ok {
							t.Error("server does not support hijacking")
							http.Error(w, "cannot simulate lost reply", 500)
							return
						}
						conn, _, err := hijacker.Hijack()
						if err != nil {
							t.Errorf("hijacking connection: %v", err)
							return
						}
						_ = conn.Close()
					default:
						fmt.Fprint(w, `{"count":1,"value":[123]}`)
					}
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
			gh.On("CreateEntityRegistrationToken", context.Background()).Return(&github.RegistrationToken{Token: &token, ExpiresAt: &github.Timestamp{Time: time.Now().Add(time.Hour)}}, (*github.Response)(nil), nil).Maybe()
			client, err := scalesets.NewClient(gh)
			require.NoError(t, err)
			ctx := context.Background()
			session, err := client.CreateMessageSession(ctx, 42, "test")
			require.NoError(t, err)
			defer session.Close()
			helper := &listenerTestHelper{client: client, fail: tc.handlerFailure}
			listener := newListener(ctx, helper)
			listener.listenerCtx, listener.messageSession = ctx, session
			jobs := []params.ScaleSetJobMessage{{MessageType: params.MessageTypeJobAvailable, RunnerRequestID: 123}, {MessageType: params.MessageTypeJobAssigned}, {MessageType: params.MessageTypeJobStarted}, {MessageType: params.MessageTypeJobCompleted}}
			body, err := json.Marshal(jobs)
			require.NoError(t, err)
			msg := params.RunnerScaleSetMessage{MessageID: 11, MessageType: string(params.MessageTypeRunnerScaleSetJobMessages), Body: string(body), Statistics: &params.RunnerScaleSetStatistic{TotalAssignedJobs: 3}}
			listener.handleSessionMessage(msg)
			if len(tc.failures) > 1 {
				require.Equal(t, int64(10), listener.lastMessageID)
				require.Zero(t, helper.lastID)
				require.Zero(t, helper.desired)
				require.Zero(t, deletes)
				listener.handleSessionMessage(msg)
			}
			require.Equal(t, tc.wantCalls, helper.calls)
			if tc.wantAck {
				require.Equal(t, int64(11), listener.lastMessageID)
				require.Equal(t, int64(11), helper.lastID)
				require.Equal(t, 3, helper.desired)
				require.Equal(t, 1, deletes)
			} else {
				require.Equal(t, int64(10), listener.lastMessageID)
				require.Zero(t, helper.lastID)
				require.Zero(t, helper.desired)
				require.Zero(t, deletes)
			}
		})
	}
}
