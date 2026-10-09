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
package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
)

func TestForbiddenErrorIdentity(t *testing.T) {
	forbidden := NewForbiddenError("forbidden while calling %s", "https://example.com")

	tests := []struct {
		name   string
		target error
		want   bool
	}{
		{
			name:   "matches itself, so callers can single out a 403",
			target: &ForbiddenError{},
			want:   true,
		},
		{
			// The compatibility guarantee: callers that only ask "was this
			// refused?" must keep working after 403 stopped returning
			// ErrUnauthorized outright.
			name:   "still reports as unauthorized",
			target: runnerErrors.ErrUnauthorized,
			want:   true,
		},
		{
			name:   "is not a not-found",
			target: runnerErrors.ErrNotFound,
			want:   false,
		},
		{
			name:   "is not a bad request",
			target: runnerErrors.ErrBadRequest,
			want:   false,
		},
		{
			name:   "does not match a nil target",
			target: nil,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, errors.Is(forbidden, tt.target))
		})
	}
}

func TestForbiddenErrorKeepsItsMessage(t *testing.T) {
	err := NewForbiddenError("forbidden while calling %s: %q", "https://example.com", "rate limited")

	require.EqualError(t, err, `forbidden while calling https://example.com: "rate limited"`)
}

func TestForbiddenErrorSurvivesWrapping(t *testing.T) {
	// Callers report failures with %w up the stack, so the distinction has to
	// survive being wrapped or it is useless where it is actually read.
	wrapped := fmt.Errorf("removing runner from github: %w", NewForbiddenError("forbidden"))

	require.True(t, errors.Is(wrapped, &ForbiddenError{}))
	require.True(t, errors.Is(wrapped, runnerErrors.ErrUnauthorized))
}

func TestUnauthorizedErrorIsNotForbidden(t *testing.T) {
	// The inverse of the compatibility guarantee: a plain 401 must not be
	// mistaken for a 403, or the retry decision inverts.
	err := runnerErrors.NewUnauthorizedError("unauthorized while calling https://example.com")

	require.True(t, errors.Is(err, runnerErrors.ErrUnauthorized))
	require.False(t, errors.Is(err, &ForbiddenError{}))
}
