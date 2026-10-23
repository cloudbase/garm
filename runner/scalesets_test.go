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

package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

// fakeGithub answers with success for a zero actionsServiceStatus.
type fakeGithub struct {
	actionsServiceStatus int
	deleteStatus         int
}

func (f fakeGithub) serve(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("POST /api/v3/repos/test-owner/test-repo/actions/runners/registration-token", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"token": "registration-token"}`)
	})
	mux.HandleFunc("POST /actions/runner-registration", func(w http.ResponseWriter, _ *http.Request) {
		if f.actionsServiceStatus != 0 {
			w.WriteHeader(f.actionsServiceStatus)
			return
		}
		fmt.Fprintf(w, `{"url": %q, "token": "actions-token"}`, srv.URL+"/pipeline")
	})
	mux.HandleFunc("DELETE /pipeline/_apis/runtime/runnerscalesets/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(f.deleteStatus)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestDeleteScaleSetByID(t *testing.T) {
	tests := []struct {
		name    string
		github  fakeGithub
		bypass  bool
		wantErr bool
	}{
		{
			name:   "deleted on github",
			github: fakeGithub{deleteStatus: http.StatusNoContent},
		},
		{
			name:   "already gone from github",
			github: fakeGithub{deleteStatus: http.StatusNotFound},
		},
		{
			name:    "actions service does not resolve the entity",
			github:  fakeGithub{actionsServiceStatus: http.StatusNotFound},
			wantErr: true,
		},
		{
			name:    "github fails the delete",
			github:  fakeGithub{deleteStatus: http.StatusInternalServerError},
			wantErr: true,
		},
		{
			name:   "bypass with an unresolved entity",
			github: fakeGithub{actionsServiceStatus: http.StatusNotFound},
			bypass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := tt.github.serve(t)

			db, err := database.NewDatabase(context.Background(), garmTesting.GetTestSqliteDBConfig(t))
			require.NoError(t, err)
			ctx := garmTesting.ImpersonateAdminContext(context.Background(), db, t)

			endpoint, err := db.CreateGithubEndpoint(ctx, params.CreateGithubEndpointParams{
				Name:          "fake-github",
				APIBaseURL:    srv.URL + "/api/v3/",
				UploadBaseURL: srv.URL + "/api/uploads/",
				BaseURL:       srv.URL,
			})
			require.NoError(t, err)
			creds := garmTesting.CreateTestGithubCredentials(ctx, "fake-github-creds", db, t, endpoint)
			repo, err := db.CreateRepository(ctx, "test-owner", "test-repo", creds, "secret", params.PoolBalancerTypeRoundRobin, false)
			require.NoError(t, err)
			entity, err := repo.GetEntity()
			require.NoError(t, err)
			scaleSet, err := db.CreateEntityScaleSet(ctx, entity, params.CreateScaleSetParams{
				Name:         "test-scaleset",
				ScaleSetID:   1,
				ProviderName: "test-provider",
				MaxRunners:   1,
				Image:        "test-image",
				Flavor:       "test-flavor",
			})
			require.NoError(t, err)

			r := &Runner{ctx: ctx, store: db}
			err = r.DeleteScaleSetByID(ctx, scaleSet.ID, tt.bypass)
			_, getErr := db.GetScaleSetByID(ctx, scaleSet.ID)
			if tt.wantErr {
				require.Error(t, err)
				require.NoError(t, getErr)
			} else {
				require.NoError(t, err)
				require.ErrorIs(t, getErr, runnerErrors.ErrNotFound)
			}
		})
	}
}
