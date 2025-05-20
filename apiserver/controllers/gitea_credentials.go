// Copyright 2025 Cloudbase Solutions SRL
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
package controllers

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
)

// swagger:route GET /gitea/credentials credentials ListGiteaCredentials
//
// List all credentials.
//
//	Responses:
//	  200: Credentials
//	  400: APIErrorResponse
func (a *APIController) ListGiteaCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	creds, err := a.r.ListGiteaCredentials(ctx)
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(creds); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route POST /gitea/credentials credentials CreateGiteaCredentials
//
// Create a Gitea credential.
//
//	Parameters:
//	  + name: Body
//	    description: Parameters used when creating a Gitea credential.
//	    type: CreateGiteaCredentialsParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: ForgeCredentials
//	  400: APIErrorResponse
func (a *APIController) CreateGiteaCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var params params.CreateGiteaCredentialsParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	cred, err := a.r.CreateGiteaCredentials(ctx, params)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to create Gitea credential")
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cred); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /gitea/credentials/{id} credentials GetGiteaCredentials
//
// Get a Gitea credential.
//
//	Parameters:
//	  + name: id
//	    description: ID of the Gitea credential.
//	    type: integer
//	    in: path
//	    required: true
//
//	Responses:
//	  200: ForgeCredentials
//	  400: APIErrorResponse
func (a *APIController) GetGiteaCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	idParam, ok := vars["id"]
	if !ok {
		slog.ErrorContext(ctx, "missing id in request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to parse id")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	if id > math.MaxUint {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "id is too large")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	cred, err := a.r.GetGiteaCredentials(ctx, uint(id))
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to get Gitea credential")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cred); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /gitea/credentials/{id} credentials DeleteGiteaCredentials
//
// Delete a Gitea credential.
//
//	Parameters:
//	  + name: id
//	    description: ID of the Gitea credential.
//	    type: integer
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteGiteaCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	idParam, ok := vars["id"]
	if !ok {
		slog.ErrorContext(ctx, "missing id in request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to parse id")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	if id > math.MaxUint {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "id is too large")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	if err := a.r.DeleteGiteaCredentials(ctx, uint(id)); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to delete Gitea credential")
		handleError(ctx, w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// swagger:route PUT /gitea/credentials/{id} credentials UpdateGiteaCredentials
//
// Update a Gitea credential.
//
//	Parameters:
//	  + name: id
//	    description: ID of the Gitea credential.
//	    type: integer
//	    in: path
//	    required: true
//	  + name: Body
//	    description: Parameters used when updating a Gitea credential.
//	    type: UpdateGiteaCredentialsParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: ForgeCredentials
//	  400: APIErrorResponse
func (a *APIController) UpdateGiteaCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	idParam, ok := vars["id"]
	if !ok {
		slog.ErrorContext(ctx, "missing id in request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to parse id")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	if id > math.MaxUint {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "id is too large")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	var params params.UpdateGiteaCredentialsParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	cred, err := a.r.UpdateGiteaCredentials(ctx, uint(id), params)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to update Gitea credential")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cred); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}
