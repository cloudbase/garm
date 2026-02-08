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

// swagger:route GET /pools/{poolID}/instances instances ListPoolInstances
//
// List runner instances in a pool.
//
//	Parameters:
//	  + name: poolID
//	    description: Runner pool ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: outdatedOnly
//	    description: List only instances that were created prior to a pool update that changed a setting which influences how instances are created (image, flavor, runner group, etc).
//	    type: boolean
//	    in: query
//	    required: false
//
//	Responses:
//	  200: Instances
//	  default: APIErrorResponse
func (a *APIController) ListPoolInstancesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	poolID, ok := vars["poolID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No pool ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	filterByOutdated, _ := strconv.ParseBool(r.URL.Query().Get("outdatedOnly"))
	instances, err := a.r.ListPoolInstances(ctx, poolID, filterByOutdated)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing pool instances")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(instances); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /scalesets/{scalesetID}/instances instances ListScaleSetInstances
//
// List runner instances in a scale set.
//
//	Parameters:
//	  + name: scalesetID
//	    description: Runner scale set ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: outdatedOnly
//	    description: List only instances that were created prior to a scaleset update that changed a setting which influences how instances are created (image, flavor, runner group, etc).
//	    type: boolean
//	    in: query
//	    required: false
//
//	Responses:
//	  200: Instances
//	  default: APIErrorResponse
func (a *APIController) ListScaleSetInstancesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	scalesetID, ok := vars["scalesetID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No pool ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}
	id, err := strconv.ParseUint(scalesetID, 10, 32)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to parse id")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	filterByOutdated, _ := strconv.ParseBool(r.URL.Query().Get("outdatedOnly"))
	instances, err := a.r.ListScaleSetInstances(ctx, uint(id), filterByOutdated)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing pool instances")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(instances); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /instances/{instanceName} instances GetInstance
//
// Get runner instance by name.
//
//	Parameters:
//	  + name: instanceName
//	    description: Runner instance name.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Instance
//	  default: APIErrorResponse
func (a *APIController) GetInstanceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	instanceName, ok := vars["instanceName"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No runner name specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	instance, err := a.r.GetInstance(ctx, instanceName)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing instances")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(instance); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /instances/{instanceName} instances DeleteInstance
//
// Delete runner instance by name.
//
//	Parameters:
//	  + name: instanceName
//	    description: Runner instance name.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: forceRemove
//	    description: If true GARM will ignore any provider error when removing the runner and will continue to remove the runner from github and the GARM database.
//	    type: boolean
//	    in: query
//	    required: false
//
//	  + name: bypassGHUnauthorized
//	    description: If true GARM will ignore unauthorized errors returned by GitHub when removing a runner. This is useful if you want to clean up runners and your credentials have expired.
//	    type: boolean
//	    in: query
//	    required: false
//
// Responses:
//
//	default: APIErrorResponse
func (a *APIController) DeleteInstanceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	instanceName, ok := vars["instanceName"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No instance name specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	forceRemove, _ := strconv.ParseBool(r.URL.Query().Get("forceRemove"))
	bypassGHUnauthorized, _ := strconv.ParseBool(r.URL.Query().Get("bypassGHUnauthorized"))
	if err := a.r.DeleteRunner(ctx, instanceName, forceRemove, bypassGHUnauthorized); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "removing runner")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route GET /repositories/{repoID}/instances repositories instances ListRepoInstances
//
// List repository instances.
//
//	Parameters:
//	  + name: repoID
//	    description: Repository ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Instances
//	  default: APIErrorResponse
func (a *APIController) ListRepoInstancesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	repoID, ok := vars["repoID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	instances, err := a.r.ListRepoInstances(ctx, repoID)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing pools")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(instances); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /organizations/{orgID}/instances organizations instances ListOrgInstances
//
// List organization instances.
//
//	Parameters:
//	  + name: orgID
//	    description: Organization ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Instances
//	  default: APIErrorResponse
func (a *APIController) ListOrgInstancesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	orgID, ok := vars["orgID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No org ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	instances, err := a.r.ListOrgInstances(ctx, orgID)
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

// swagger:route GET /enterprises/{enterpriseID}/instances enterprises instances ListEnterpriseInstances
//
// List enterprise instances.
//
//	Parameters:
//	  + name: enterpriseID
//	    description: Enterprise ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Instances
//	  default: APIErrorResponse
func (a *APIController) ListEnterpriseInstancesHandler(w http.ResponseWriter, r *http.Request) {
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

	instances, err := a.r.ListEnterpriseInstances(ctx, enterpriseID)
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

// swagger:route GET /instances instances ListInstances
//
// Get all runners' instances.
//
//	Responses:
//	  200: Instances
//	  default: APIErrorResponse
func (a *APIController) ListAllInstancesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	instances, err := a.r.ListAllInstances(ctx)
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

func (a *APIController) InstanceStatusMessageHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var updateMessage runnerParams.InstanceUpdateMessage
	if err := json.NewDecoder(r.Body).Decode(&updateMessage); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	if err := a.r.AddInstanceStatusMessage(ctx, updateMessage); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error saving status message")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (a *APIController) InstanceSystemInfoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var updateMessage runnerParams.UpdateSystemInfoParams
	if err := json.NewDecoder(r.Body).Decode(&updateMessage); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	if err := a.r.UpdateSystemInfo(ctx, updateMessage); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error saving status message")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}
