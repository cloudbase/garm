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
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/apiserver/params"
	runnerParams "github.com/cloudbase/garm/params"
)

// swagger:route POST /forge-instances forge-instances CreateForgeInstance
//
// Create forge instance with the given parameters.
//
//	Parameters:
//	  + name: Body
//	    description: Parameters used to create the forge instance.
//	    type: CreateForgeInstanceParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: ForgeInstance
//	  default: APIErrorResponse
func (a *APIController) CreateForgeInstanceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var createData runnerParams.CreateForgeInstanceParams
	if err := json.NewDecoder(r.Body).Decode(&createData); err != nil {
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	forgeInstance, err := a.r.CreateForgeInstance(ctx, createData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating forge instance")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(forgeInstance); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /forge-instances forge-instances ListForgeInstances
//
// List all forge instances.
//
//	Parameters:
//	  + name: endpoint
//	    description: Exact endpoint name to filter by
//	    type: string
//	    in: query
//	    required: false
//
//	Responses:
//	  200: ForgeInstances
//	  default: APIErrorResponse
func (a *APIController) ListForgeInstancesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filter := runnerParams.ForgeInstanceFilter{
		Endpoint: r.URL.Query().Get("endpoint"),
	}
	forgeInstances, err := a.r.ListForgeInstances(ctx, filter)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing forge instances")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(forgeInstances); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /forge-instances/{forgeInstanceID} forge-instances GetForgeInstance
//
// Get forge instance by ID.
//
//	Parameters:
//	  + name: forgeInstanceID
//	    description: The ID of the forge instance to fetch.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: ForgeInstance
//	  default: APIErrorResponse
func (a *APIController) GetForgeInstanceByIDHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	forgeInstanceID, ok := vars["forgeInstanceID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No forge instance ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	forgeInstance, err := a.r.GetForgeInstanceByID(ctx, forgeInstanceID)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "fetching forge instance")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(forgeInstance); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /forge-instances/{forgeInstanceID} forge-instances DeleteForgeInstance
//
// Delete forge instance by ID.
//
//	Parameters:
//	  + name: forgeInstanceID
//	    description: ID of the forge instance to delete.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: keepWebhook
//	    description: If true and a webhook is installed for this forge instance, it will not be removed.
//	    type: boolean
//	    in: query
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteForgeInstanceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	forgeInstanceID, ok := vars["forgeInstanceID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No forge instance ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	keepWebhook, _ := strconv.ParseBool(r.URL.Query().Get("keepWebhook"))

	if err := a.r.DeleteForgeInstance(ctx, forgeInstanceID, keepWebhook); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "removing forge instance")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route PUT /forge-instances/{forgeInstanceID} forge-instances UpdateForgeInstance
//
// Update forge instance with the given parameters.
//
//	Parameters:
//	  + name: forgeInstanceID
//	    description: The ID of the forge instance to update.
//	    type: string
//	    in: path
//	    required: true
//	  + name: Body
//	    description: Parameters used when updating the forge instance.
//	    type: UpdateEntityParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: ForgeInstance
//	  default: APIErrorResponse
func (a *APIController) UpdateForgeInstanceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	forgeInstanceID, ok := vars["forgeInstanceID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No forge instance ID specified",
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

	forgeInstance, err := a.r.UpdateForgeInstance(ctx, forgeInstanceID, updatePayload)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error updating forge instance")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(forgeInstance); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route POST /forge-instances/{forgeInstanceID}/pools forge-instances pools CreateForgeInstancePool
//
// Create forge instance pool with the parameters given.
//
//	Parameters:
//	  + name: forgeInstanceID
//	    description: Forge instance ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when creating the forge instance pool.
//	    type: CreatePoolParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Pool
//	  default: APIErrorResponse
func (a *APIController) CreateForgeInstancePoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	forgeInstanceID, ok := vars["forgeInstanceID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No forge instance ID specified",
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

	pool, err := a.r.CreateForgeInstancePool(ctx, forgeInstanceID, poolData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating forge instance pool")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pool); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /forge-instances/{forgeInstanceID}/pools forge-instances pools ListForgeInstancePools
//
// List forge instance pools.
//
//	Parameters:
//	  + name: forgeInstanceID
//	    description: Forge instance ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Pools
//	  default: APIErrorResponse
func (a *APIController) ListForgeInstancePoolsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	forgeInstanceID, ok := vars["forgeInstanceID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No forge instance ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	pools, err := a.r.ListForgeInstancePools(ctx, forgeInstanceID)
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

// swagger:route GET /forge-instances/{forgeInstanceID}/pools/{poolID} forge-instances pools GetForgeInstancePool
//
// Get forge instance pool by ID.
//
//	Parameters:
//	  + name: forgeInstanceID
//	    description: Forge instance ID.
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
func (a *APIController) GetForgeInstancePoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	forgeInstanceID, fiOk := vars["forgeInstanceID"]
	poolID, poolOk := vars["poolID"]
	if !fiOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No forge instance or pool ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	pool, err := a.r.GetForgeInstancePoolByID(ctx, forgeInstanceID, poolID)
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

// swagger:route DELETE /forge-instances/{forgeInstanceID}/pools/{poolID} forge-instances pools DeleteForgeInstancePool
//
// Delete forge instance pool by ID.
//
//	Parameters:
//	  + name: forgeInstanceID
//	    description: Forge instance ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: poolID
//	    description: ID of the forge instance pool to delete.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteForgeInstancePoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	forgeInstanceID, fiOk := vars["forgeInstanceID"]
	poolID, poolOk := vars["poolID"]
	if !fiOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No forge instance or pool ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	if err := a.r.DeleteForgeInstancePool(ctx, forgeInstanceID, poolID); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "removing pool")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route PUT /forge-instances/{forgeInstanceID}/pools/{poolID} forge-instances pools UpdateForgeInstancePool
//
// Update forge instance pool with the parameters given.
//
//	Parameters:
//	  + name: forgeInstanceID
//	    description: Forge instance ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: poolID
//	    description: ID of the forge instance pool to update.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when updating the forge instance pool.
//	    type: UpdatePoolParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Pool
//	  default: APIErrorResponse
func (a *APIController) UpdateForgeInstancePoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	forgeInstanceID, fiOk := vars["forgeInstanceID"]
	poolID, poolOk := vars["poolID"]
	if !fiOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No forge instance or pool ID specified",
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

	pool, err := a.r.UpdateForgeInstancePool(ctx, forgeInstanceID, poolID, poolData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error updating forge instance pool")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pool); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /forge-instances/{forgeInstanceID}/instances forge-instances ListForgeInstanceInstances
//
// List forge instance runner instances.
//
//	Parameters:
//	  + name: forgeInstanceID
//	    description: Forge instance ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Instances
//	  default: APIErrorResponse
func (a *APIController) ListForgeInstanceInstancesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	forgeInstanceID, ok := vars["forgeInstanceID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No forge instance ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	instances, err := a.r.ListForgeInstanceInstances(ctx, forgeInstanceID)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing instances")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(instances); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route POST /forge-instances/{forgeInstanceID}/webhook forge-instances hooks InstallForgeInstanceWebhook
//
// Install the GARM webhook for a forge instance. The secret configured on the forge instance will
// be used to validate the requests.
//
//	Parameters:
//	  + name: forgeInstanceID
//	    description: Forge instance ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when creating the forge instance webhook.
//	    type: InstallWebhookParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: HookInfo
//	  default: APIErrorResponse
func (a *APIController) InstallForgeInstanceWebhookHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	forgeInstanceID, ok := vars["forgeInstanceID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No forge instance ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	var hookParam runnerParams.InstallWebhookParams
	if err := json.NewDecoder(r.Body).Decode(&hookParam); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	info, err := a.r.InstallForgeInstanceWebhook(ctx, forgeInstanceID, hookParam)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "installing webhook")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /forge-instances/{forgeInstanceID}/webhook forge-instances hooks UninstallForgeInstanceWebhook
//
// Uninstall forge instance webhook.
//
//	Parameters:
//	  + name: forgeInstanceID
//	    description: Forge instance ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) UninstallForgeInstanceWebhookHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	forgeInstanceID, ok := vars["forgeInstanceID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No forge instance ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	if err := a.r.UninstallForgeInstanceWebhook(ctx, forgeInstanceID); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "removing webhook")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route GET /forge-instances/{forgeInstanceID}/webhook forge-instances hooks GetForgeInstanceWebhookInfo
//
// Get information about the GARM installed webhook on a forge instance.
//
//	Parameters:
//	  + name: forgeInstanceID
//	    description: Forge instance ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: HookInfo
//	  default: APIErrorResponse
func (a *APIController) GetForgeInstanceWebhookInfoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	forgeInstanceID, ok := vars["forgeInstanceID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No forge instance ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	info, err := a.r.GetForgeInstanceWebhookInfo(ctx, forgeInstanceID)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "getting webhook info")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}
