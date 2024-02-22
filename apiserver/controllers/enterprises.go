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

	"github.com/gorilla/mux"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/apiserver/params"
	runnerParams "github.com/cloudbase/garm/params"
)

// swagger:route POST /enterprises enterprises CreateEnterprise
//
// Create enterprise with the given parameters.
//
//	Parameters:
//	  + name: Body
//	    description: Parameters used to create the enterprise.
//	    type: CreateEnterpriseParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Enterprise
//	  default: APIErrorResponse
func (a *APIController) CreateEnterpriseHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var enterpriseData runnerParams.CreateEnterpriseParams
	if err := json.NewDecoder(r.Body).Decode(&enterpriseData); err != nil {
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	enterprise, err := a.r.CreateEnterprise(ctx, enterpriseData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating enterprise")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(enterprise); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /enterprises enterprises ListEnterprises
//
// List all enterprises.
//
//	Responses:
//	  200: Enterprises
//	  default: APIErrorResponse
func (a *APIController) ListEnterprisesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	enterprise, err := a.r.ListEnterprises(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing enterprise")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(enterprise); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /enterprises/{enterpriseID} enterprises GetEnterprise
//
// Get enterprise by ID.
//
//	Parameters:
//	  + name: enterpriseID
//	    description: The ID of the enterprise to fetch.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Enterprise
//	  default: APIErrorResponse
func (a *APIController) GetEnterpriseByIDHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	enterpriseID, ok := vars["enterpriseID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No enterprise ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	enterprise, err := a.r.GetEnterpriseByID(ctx, enterpriseID)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "fetching enterprise")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(enterprise); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /enterprises/{enterpriseID} enterprises DeleteEnterprise
//
// Delete enterprise by ID.
//
//	Parameters:
//	  + name: enterpriseID
//	    description: ID of the enterprise to delete.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteEnterpriseHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	enterpriseID, ok := vars["enterpriseID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No enterprise ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	if err := a.r.DeleteEnterprise(ctx, enterpriseID); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "removing enterprise")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route PUT /enterprises/{enterpriseID} enterprises UpdateEnterprise
//
// Update enterprise with the given parameters.
//
//	Parameters:
//	  + name: enterpriseID
//	    description: The ID of the enterprise to update.
//	    type: string
//	    in: path
//	    required: true
//	  + name: Body
//	    description: Parameters used when updating the enterprise.
//	    type: UpdateEntityParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Enterprise
//	  default: APIErrorResponse
func (a *APIController) UpdateEnterpriseHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	enterpriseID, ok := vars["enterpriseID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No enterprise ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	var updatePayload runnerParams.UpdateEntityParams
	if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	enterprise, err := a.r.UpdateEnterprise(ctx, enterpriseID, updatePayload)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error updating enterprise: %s")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(enterprise); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route POST /enterprises/{enterpriseID}/pools enterprises pools CreateEnterprisePool
//
// Create enterprise pool with the parameters given.
//
//	Parameters:
//	  + name: enterpriseID
//	    description: Enterprise ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when creating the enterprise pool.
//	    type: CreatePoolParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Pool
//	  default: APIErrorResponse
func (a *APIController) CreateEnterprisePoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	enterpriseID, ok := vars["enterpriseID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No enterprise ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	var poolData runnerParams.CreatePoolParams
	if err := json.NewDecoder(r.Body).Decode(&poolData); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	pool, err := a.r.CreateEnterprisePool(ctx, enterpriseID, poolData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating enterprise pool")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pool); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /enterprises/{enterpriseID}/pools enterprises pools ListEnterprisePools
//
// List enterprise pools.
//
//	Parameters:
//	  + name: enterpriseID
//	    description: Enterprise ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Pools
//	  default: APIErrorResponse
func (a *APIController) ListEnterprisePoolsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	enterpriseID, ok := vars["enterpriseID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No enterprise ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	pools, err := a.r.ListEnterprisePools(ctx, enterpriseID)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing pools")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pools); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /enterprises/{enterpriseID}/pools/{poolID} enterprises pools GetEnterprisePool
//
// Get enterprise pool by ID.
//
//	Parameters:
//	  + name: enterpriseID
//	    description: Enterprise ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: poolID
//	    description: Pool ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Pool
//	  default: APIErrorResponse
func (a *APIController) GetEnterprisePoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	enterpriseID, enterpriseOk := vars["enterpriseID"]
	poolID, poolOk := vars["poolID"]
	if !enterpriseOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No enterprise or pool ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	pool, err := a.r.GetEnterprisePoolByID(ctx, enterpriseID, poolID)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing pools")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pool); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /enterprises/{enterpriseID}/pools/{poolID} enterprises pools DeleteEnterprisePool
//
// Delete enterprise pool by ID.
//
//	Parameters:
//	  + name: enterpriseID
//	    description: Enterprise ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: poolID
//	    description: ID of the enterprise pool to delete.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteEnterprisePoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	enterpriseID, enterpriseOk := vars["enterpriseID"]
	poolID, poolOk := vars["poolID"]
	if !enterpriseOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No enterprise or pool ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	if err := a.r.DeleteEnterprisePool(ctx, enterpriseID, poolID); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "removing pool")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route PUT /enterprises/{enterpriseID}/pools/{poolID} enterprises pools UpdateEnterprisePool
//
// Update enterprise pool with the parameters given.
//
//	Parameters:
//	  + name: enterpriseID
//	    description: Enterprise ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: poolID
//	    description: ID of the enterprise pool to update.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when updating the enterprise pool.
//	    type: UpdatePoolParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Pool
//	  default: APIErrorResponse
func (a *APIController) UpdateEnterprisePoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	enterpriseID, enterpriseOk := vars["enterpriseID"]
	poolID, poolOk := vars["poolID"]
	if !enterpriseOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No enterprise or pool ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	var poolData runnerParams.UpdatePoolParams
	if err := json.NewDecoder(r.Body).Decode(&poolData); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	pool, err := a.r.UpdateEnterprisePool(ctx, enterpriseID, poolID, poolData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating enterprise pool")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pool); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}
