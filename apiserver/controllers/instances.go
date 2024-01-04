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
	"log"
	"net/http"
	"strconv"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/apiserver/params"
	runnerParams "github.com/cloudbase/garm/params"

	"github.com/gorilla/mux"
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
			log.Printf("failed to encode response: %q", err)
		}
		return
	}

	instances, err := a.r.ListPoolInstances(ctx, poolID)
	if err != nil {
		log.Printf("listing pool instances: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(instances); err != nil {
		log.Printf("failed to encode response: %q", err)
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
			log.Printf("failed to encode response: %q", err)
		}
		return
	}

	instance, err := a.r.GetInstance(ctx, instanceName)
	if err != nil {
		log.Printf("listing instances: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(instance); err != nil {
		log.Printf("failed to encode response: %q", err)
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
//	Responses:
//	  default: APIErrorResponse
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
			log.Printf("failed to encode response: %q", err)
		}
		return
	}

	forceRemove, _ := strconv.ParseBool(r.URL.Query().Get("forceRemove"))
	if err := a.r.DeleteRunner(ctx, instanceName, forceRemove); err != nil {
		log.Printf("removing runner: %s", err)
		handleError(w, err)
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
			log.Printf("failed to encode response: %q", err)
		}
		return
	}

	instances, err := a.r.ListRepoInstances(ctx, repoID)
	if err != nil {
		log.Printf("listing pools: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(instances); err != nil {
		log.Printf("failed to encode response: %q", err)
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
			log.Printf("failed to encode response: %q", err)
		}
		return
	}

	instances, err := a.r.ListOrgInstances(ctx, orgID)
	if err != nil {
		log.Printf("listing instances: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(instances); err != nil {
		log.Printf("failed to encode response: %q", err)
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
			log.Printf("failed to encode response: %q", err)
		}
		return
	}

	instances, err := a.r.ListEnterpriseInstances(ctx, enterpriseID)
	if err != nil {
		log.Printf("listing instances: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(instances); err != nil {
		log.Printf("failed to encode response: %q", err)
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
		log.Printf("listing instances: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(instances); err != nil {
		log.Printf("failed to encode response: %q", err)
	}
}

func (a *APIController) InstanceStatusMessageHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var updateMessage runnerParams.InstanceUpdateMessage
	if err := json.NewDecoder(r.Body).Decode(&updateMessage); err != nil {
		log.Printf("failed to decode: %s", err)
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	if err := a.r.AddInstanceStatusMessage(ctx, updateMessage); err != nil {
		log.Printf("error saving status message: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (a *APIController) InstanceSystemInfoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var updateMessage runnerParams.UpdateSystemInfoParams
	if err := json.NewDecoder(r.Body).Decode(&updateMessage); err != nil {
		log.Printf("failed to decode: %s", err)
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	if err := a.r.UpdateSystemInfo(ctx, updateMessage); err != nil {
		log.Printf("error saving status message: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}
