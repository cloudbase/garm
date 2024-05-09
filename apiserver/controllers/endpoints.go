package controllers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
)

// swagger:route POST /github/endpoints endpoints CreateGithubEndpoint
//
// Create a GitHub Endpoint.
//
//	Parameters:
//	  + name: Body
//	    description: Parameters used when creating a GitHub endpoint.
//	    type: CreateGithubEndpointParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: GithubEndpoint
//	  default: APIErrorResponse
func (a *APIController) CreateGithubEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var params params.CreateGithubEndpointParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	endpoint, err := a.r.CreateGithubEndpoint(ctx, params)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to create GitHub endpoint")
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(endpoint); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /github/endpoints endpoints ListGithubEndpoints
//
// List all GitHub Endpoints.
//
//	Responses:
//	  200: GithubEndpoints
//	  default: APIErrorResponse
func (a *APIController) ListGithubEndpoints(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	endpoints, err := a.r.ListGithubEndpoints(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to list GitHub endpoints")
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(endpoints); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /github/endpoints/{name} endpoints GetGithubEndpoint
//
// Get a GitHub Endpoint.
//
//	Parameters:
//	  + name: name
//	    description: The name of the GitHub endpoint.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: GithubEndpoint
//	  default: APIErrorResponse
func (a *APIController) GetGithubEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	name, ok := vars["name"]
	if !ok {
		slog.ErrorContext(ctx, "missing name in request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}
	endpoint, err := a.r.GetGithubEndpoint(ctx, name)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to get GitHub endpoint")
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(endpoint); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /github/endpoints/{name} endpoints DeleteGithubEndpoint
//
// Delete a GitHub Endpoint.
//
//	Parameters:
//	  + name: name
//	    description: The name of the GitHub endpoint.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteGithubEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	name, ok := vars["name"]
	if !ok {
		slog.ErrorContext(ctx, "missing name in request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}
	if err := a.r.DeleteGithubEndpoint(ctx, name); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to delete GitHub endpoint")
		handleError(ctx, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// swagger:route PUT /github/endpoints/{name} endpoints UpdateGithubEndpoint
//
// Update a GitHub Endpoint.
//
//	Parameters:
//	  + name: name
//	    description: The name of the GitHub endpoint.
//	    type: string
//	    in: path
//	    required: true
//	  + name: Body
//	    description: Parameters used when updating a GitHub endpoint.
//	    type: UpdateGithubEndpointParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: GithubEndpoint
//	  default: APIErrorResponse
func (a *APIController) UpdateGithubEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	name, ok := vars["name"]
	if !ok {
		slog.ErrorContext(ctx, "missing name in request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	var params params.UpdateGithubEndpointParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	endpoint, err := a.r.UpdateGithubEndpoint(ctx, name, params)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to update GitHub endpoint")
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(endpoint); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}
