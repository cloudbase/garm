package controllers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
)

// swagger:route POST /gitea/endpoints endpoints CreateGiteaEndpoint
//
// Create a Gitea Endpoint.
//
//	Parameters:
//	  + name: Body
//	    description: Parameters used when creating a Gitea endpoint.
//	    type: CreateGiteaEndpointParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: ForgeEndpoint
//	  default: APIErrorResponse
func (a *APIController) CreateGiteaEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var params params.CreateGiteaEndpointParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	endpoint, err := a.r.CreateGiteaEndpoint(ctx, params)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to create Gitea endpoint")
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(endpoint); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /gitea/endpoints endpoints ListGiteaEndpoints
//
// List all Gitea Endpoints.
//
//	Responses:
//	  200: ForgeEndpoints
//	  default: APIErrorResponse
func (a *APIController) ListGiteaEndpoints(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	endpoints, err := a.r.ListGiteaEndpoints(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to list Gitea endpoints")
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(endpoints); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /gitea/endpoints/{name} endpoints GetGiteaEndpoint
//
// Get a Gitea Endpoint.
//
//	Parameters:
//	  + name: name
//	    description: The name of the Gitea endpoint.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: ForgeEndpoint
//	  default: APIErrorResponse
func (a *APIController) GetGiteaEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	name, ok := vars["name"]
	if !ok {
		slog.ErrorContext(ctx, "missing name in request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}
	endpoint, err := a.r.GetGiteaEndpoint(ctx, name)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to get Gitea endpoint")
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(endpoint); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /gitea/endpoints/{name} endpoints DeleteGiteaEndpoint
//
// Delete a Gitea Endpoint.
//
//	Parameters:
//	  + name: name
//	    description: The name of the Gitea endpoint.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteGiteaEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	name, ok := vars["name"]
	if !ok {
		slog.ErrorContext(ctx, "missing name in request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}
	if err := a.r.DeleteGiteaEndpoint(ctx, name); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to delete Gitea endpoint")
		handleError(ctx, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// swagger:route PUT /gitea/endpoints/{name} endpoints UpdateGiteaEndpoint
//
// Update a Gitea Endpoint.
//
//	Parameters:
//	  + name: name
//	    description: The name of the Gitea endpoint.
//	    type: string
//	    in: path
//	    required: true
//	  + name: Body
//	    description: Parameters used when updating a Gitea endpoint.
//	    type: UpdateGiteaEndpointParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: ForgeEndpoint
//	  default: APIErrorResponse
func (a *APIController) UpdateGiteaEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	name, ok := vars["name"]
	if !ok {
		slog.ErrorContext(ctx, "missing name in request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	var params params.UpdateGiteaEndpointParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	endpoint, err := a.r.UpdateGiteaEndpoint(ctx, name, params)
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
