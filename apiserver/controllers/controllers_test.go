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

package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
)

func TestHandleError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedError  string
		expectDetails  bool
	}{
		{
			name:           "missing webhook secret wrapped in fmt.Errorf",
			err:            fmt.Errorf("creating org: %w", gErrors.NewMissingSecretError("missing secret")),
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Bad Request",
			expectDetails:  true,
		},
		{
			name:           "unknown error yields 500 with blanked details",
			err:            errors.New("some unexpected internal failure"),
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Server error",
			expectDetails:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handleError(context.Background(), w, tc.err)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			var resp struct {
				Error   string `json:"error"`
				Details string `json:"details"`
			}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}

			if resp.Error != tc.expectedError {
				t.Errorf("expected error %q, got %q", tc.expectedError, resp.Error)
			}

			if tc.expectDetails && resp.Details == "" {
				t.Errorf("expected non-empty details, got empty")
			}
			if !tc.expectDetails && resp.Details != "" {
				t.Errorf("expected empty details, got %q", resp.Details)
			}
		})
	}
}
