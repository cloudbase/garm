// Copyright 2022 Cloudbase Solutions SRL
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
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/apiserver/params"
	runnerParams "github.com/cloudbase/garm/params"
)

// swagger:route GET /scalesets scalesets ListScalesets
//
// List all scalesets.
//
//	Responses:
//	  200: ScaleSets
//	  default: APIErrorResponse
func (a *APIController) ListAllScaleSetsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scalesets, err := a.r.ListAllScaleSets(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing scale sets")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(scalesets); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /scalesets/{scalesetID} scalesets GetScaleSet
//
// Get scale set by ID.
//
//	Parameters:
//	  + name: scalesetID
//	    description: ID of the scale set to fetch.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: ScaleSet
//	  default: APIErrorResponse
func (a *APIController) GetScaleSetByIDHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	scaleSetID, ok := vars["scalesetID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No scale set ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}
	id, err := strconv.ParseUint(scaleSetID, 10, 64)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to parse id")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	scaleSet, err := a.r.GetScaleSetByID(ctx, uint(id))
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "fetching scale set")
		handleError(ctx, w, err)
		return
	}

	scaleSet.RunnerBootstrapTimeout = scaleSet.RunnerTimeout()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(scaleSet); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /scalesets/{scalesetID} scalesets DeleteScaleSet
//
// Delete scale set by ID.
//
//	Parameters:
//	  + name: scalesetID
//	    description: ID of the scale set to delete.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteScaleSetByIDHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	scalesetID, ok := vars["scalesetID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No scale set ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	id, err := strconv.ParseUint(scalesetID, 10, 64)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to parse id")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	if err := a.r.DeleteScaleSetByID(ctx, uint(id)); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "removing scale set")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route PUT /scalesets/{scalesetID} scalesets UpdateScaleSet
//
// Update scale set by ID.
//
//	Parameters:
//	  + name: scalesetID
//	    description: ID of the scale set to update.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters to update the scale set with.
//	    type: UpdateScaleSetParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: ScaleSet
//	  default: APIErrorResponse
func (a *APIController) UpdateScaleSetByIDHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	scalesetID, ok := vars["scalesetID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No scale set ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	id, err := strconv.ParseUint(scalesetID, 10, 64)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to parse id")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	var scaleSetData runnerParams.UpdateScaleSetParams
	if err := json.NewDecoder(r.Body).Decode(&scaleSetData); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	scaleSet, err := a.r.UpdateScaleSetByID(ctx, uint(id), scaleSetData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "updating scale set")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(scaleSet); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}
