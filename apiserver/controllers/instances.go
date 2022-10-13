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

	"garm/apiserver/params"
	gErrors "garm/errors"
	runnerParams "garm/params"

	"github.com/gorilla/mux"
)

func (a *APIController) ListPoolInstancesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	poolID, ok := vars["poolID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No pool ID specified",
		})
		return
	}

	instances, err := a.r.ListPoolInstances(ctx, poolID)
	if err != nil {
		log.Printf("listing pool instances: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instances)
}

func (a *APIController) GetInstanceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	instanceName, ok := vars["instanceName"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No runner name specified",
		})
		return
	}

	instance, err := a.r.GetInstance(ctx, instanceName)
	if err != nil {
		log.Printf("listing instances: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instance)
}

func (a *APIController) DeleteInstanceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	instanceName, ok := vars["instanceName"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No instance name specified",
		})
		return
	}

	if err := a.r.ForceDeleteRunner(ctx, instanceName); err != nil {
		log.Printf("removing runner: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (a *APIController) ListRepoInstancesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	repoID, ok := vars["repoID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo ID specified",
		})
		return
	}

	instances, err := a.r.ListRepoInstances(ctx, repoID)
	if err != nil {
		log.Printf("listing pools: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instances)
}

func (a *APIController) ListOrgInstancesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	orgID, ok := vars["orgID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No org ID specified",
		})
		return
	}

	instances, err := a.r.ListOrgInstances(ctx, orgID)
	if err != nil {
		log.Printf("listing instances: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instances)
}

func (a *APIController) ListEnterpriseInstancesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	enterpriseID, ok := vars["enterpriseID"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No enterprise ID specified",
		})
		return
	}

	instances, err := a.r.ListEnterpriseInstances(ctx, enterpriseID)
	if err != nil {
		log.Printf("listing instances: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instances)
}

func (a *APIController) ListAllInstancesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	instances, err := a.r.ListAllInstances(ctx)
	if err != nil {
		log.Printf("listing instances: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instances)
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
