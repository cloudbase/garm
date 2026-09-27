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
package scalesets

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	internalErrors "github.com/cloudbase/garm/internal/errors"
)

// doAgainstStatus dispatches one request through ScaleSetClient.Do against a
// server that answers with the given status and body.
func doAgainstStatus(t *testing.T, status int, body string) error {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodDelete, srv.URL, nil)
	require.NoError(t, err)

	cli := &ScaleSetClient{httpClient: srv.Client()}
	resp, err := cli.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	return err
}

func TestDoMapsRefusalStatuses(t *testing.T) {
	tests := []struct {
		name string
		// status GitHub answered with.
		status int
		// isForbidden is whether the caller can tell this apart as a 403.
		isForbidden bool
	}{
		{
			// 401 is the only status that says the credentials themselves were
			// rejected.
			name:        "unauthorized is not forbidden",
			status:      http.StatusUnauthorized,
			isForbidden: false,
		},
		{
			// GitHub answers 403 for a secondary rate limit or SSO enforcement,
			// which say nothing about the credentials.
			name:        "forbidden is distinguishable",
			status:      http.StatusForbidden,
			isForbidden: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := doAgainstStatus(t, tt.status, "some detail")

			require.Error(t, err)
			// Both remain unauthorized, so callers that only ask whether the
			// request was refused are unaffected by the split.
			require.True(t, errors.Is(err, runnerErrors.ErrUnauthorized))
			require.Equal(t, tt.isForbidden, errors.Is(err, &internalErrors.ForbiddenError{}))
		})
	}
}

func TestDoKeepsRefusalDetail(t *testing.T) {
	// Both refusals used to return a bare sentinel, so an operator reading the
	// log had no way to tell a dead credential from a rate limit. Every other
	// status branch reports the URL and body; these now do too.
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, body: "bad credentials"},
		{name: "forbidden", status: http.StatusForbidden, body: "secondary rate limit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := doAgainstStatus(t, tt.status, tt.body)

			require.ErrorContains(t, err, tt.body)
		})
	}
}

func TestDoLeavesOtherStatusesAlone(t *testing.T) {
	tests := []struct {
		name   string
		status int
		target error
	}{
		{name: "not found", status: http.StatusNotFound, target: runnerErrors.ErrNotFound},
		{name: "bad request", status: http.StatusBadRequest, target: runnerErrors.ErrBadRequest},
		{name: "conflict", status: http.StatusConflict, target: &runnerErrors.ConflictError{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := doAgainstStatus(t, tt.status, "detail")

			require.ErrorIs(t, err, tt.target)
			require.False(t, errors.Is(err, runnerErrors.ErrUnauthorized))
		})
	}
}

func TestDoPassesSuccessThrough(t *testing.T) {
	require.NoError(t, doAgainstStatus(t, http.StatusOK, "ok"))
}
