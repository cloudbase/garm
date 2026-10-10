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

package github

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-github/v84/github"
	"github.com/stretchr/testify/require"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
)

func ghResponse(status int) *github.Response {
	return &github.Response{Response: &http.Response{StatusCode: status}}
}

func TestParseErrorMapsStatuses(t *testing.T) {
	apiErr := errors.New("GET https://api.github.com/some/endpoint: boom")

	tests := []struct {
		name   string
		status int
		target error
	}{
		{name: "not found", status: http.StatusNotFound, target: runnerErrors.ErrNotFound},
		{name: "unauthorized", status: http.StatusUnauthorized, target: runnerErrors.ErrUnauthorized},
		{name: "forbidden", status: http.StatusForbidden, target: runnerErrors.ErrForbidden},
		{name: "unprocessable", status: http.StatusUnprocessableEntity, target: runnerErrors.ErrBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseError(ghResponse(tt.status), apiErr)
			require.ErrorIs(t, err, tt.target)
		})
	}
}

func TestParseErrorRefusalsStayDistinct(t *testing.T) {
	// Only a 401 means the credentials were rejected.
	unauthorized := parseError(ghResponse(http.StatusUnauthorized), errors.New("bad credentials"))
	require.NotErrorIs(t, unauthorized, runnerErrors.ErrForbidden)

	forbidden := parseError(ghResponse(http.StatusForbidden), errors.New("secondary rate limit"))
	require.NotErrorIs(t, forbidden, runnerErrors.ErrUnauthorized)
	require.ErrorContains(t, forbidden, "secondary rate limit")
}

func TestParseErrorMapsWrappedErrorResponse(t *testing.T) {
	// Some call sites only have the status on the error itself.
	apiErr := &github.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusForbidden},
		Message:  "secondary rate limit",
	}

	err := parseError(nil, apiErr)
	require.ErrorIs(t, err, runnerErrors.ErrForbidden)
	require.ErrorContains(t, err, "secondary rate limit")
}

func TestRunnersDisabledErrorConvertsNotFound(t *testing.T) {
	g := &githubClient{entity: params.ForgeEntity{
		EntityType: params.ForgeEntityTypeRepository,
		Owner:      "test-org",
		Name:       "test-repo",
	}}

	err := g.runnersDisabledError(runnerErrors.ErrNotFound)
	require.ErrorIs(t, err, &runnerErrors.ConflictError{})
	require.ErrorContains(t, err, "self hosted runners are not enabled")
	require.ErrorContains(t, err, "test-org/test-repo")

	// Anything else passes through untouched.
	passthrough := errors.New("boom")
	require.Equal(t, passthrough, g.runnersDisabledError(passthrough))
	require.NoError(t, g.runnersDisabledError(nil))
}
