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

// swagger:route GET /credentials credentials ListCredentials
// swagger:route GET /github/credentials credentials ListCredentials
//
// List all credentials.
//
//	Responses:
//	  200: Credentials
//	  400: APIErrorResponse
func (a *APIController) ListCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	creds, err := a.r.ListCredentials(ctx)
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(creds); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route POST /github/credentials credentials CreateCredentials
//
// Create a GitHub credential.
//
//	Parameters:
//	  + name: Body
//	    description: Parameters used when creating a GitHub credential.
//	    type: CreateGithubCredentialsParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: GithubCredentials
//	  400: APIErrorResponse
func (a *APIController) CreateGithubCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var params params.CreateGithubCredentialsParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	cred, err := a.r.CreateGithubCredentials(ctx, params)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to create GitHub credential")
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cred); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /github/credentials/{id} credentials GetCredentials
//
// Get a GitHub credential.
//
//	Parameters:
//	  + name: id
//	    description: ID of the GitHub credential.
//	    type: integer
//	    in: path
//	    required: true
//
//	Responses:
//	  200: GithubCredentials
//	  400: APIErrorResponse
func (a *APIController) GetGithubCredential(w http.ResponseWriter, r *http.Request) {
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

	cred, err := a.r.GetGithubCredentials(ctx, uint(id))
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to get GitHub credential")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cred); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /github/credentials/{id} credentials DeleteCredentials
//
// Delete a GitHub credential.
//
//	Parameters:
//	  + name: id
//	    description: ID of the GitHub credential.
//	    type: integer
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteGithubCredential(w http.ResponseWriter, r *http.Request) {
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

	if err := a.r.DeleteGithubCredentials(ctx, uint(id)); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to delete GitHub credential")
		handleError(ctx, w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// swagger:route PUT /github/credentials/{id} credentials UpdateCredentials
//
// Update a GitHub credential.
//
//	Parameters:
//	  + name: id
//	    description: ID of the GitHub credential.
//	    type: integer
//	    in: path
//	    required: true
//	  + name: Body
//	    description: Parameters used when updating a GitHub credential.
//	    type: UpdateGithubCredentialsParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: GithubCredentials
//	  400: APIErrorResponse
func (a *APIController) UpdateGithubCredential(w http.ResponseWriter, r *http.Request) {
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

	var params params.UpdateGithubCredentialsParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	cred, err := a.r.UpdateGithubCredentials(ctx, uint(id), params)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to update GitHub credential")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cred); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}
