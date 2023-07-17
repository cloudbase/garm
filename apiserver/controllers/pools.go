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

	"github.com/cloudbase/garm/apiserver/params"
	gErrors "github.com/cloudbase/garm/errors"
	runnerParams "github.com/cloudbase/garm/params"

	"github.com/gorilla/mux"
)

// swagger:route GET /pools pools ListPools
//
// List all pools.
//
//	Responses:
//	  200: Pools
//	  default: APIErrorResponse
func (a *APIController) ListAllPoolsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pools, err := a.r.ListAllPools(ctx)

	if err != nil {
		log.Printf("listing pools: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pools); err != nil {
		log.Printf("failed to encode response: %q", err)
	}
}

// swagger:route GET /pools/{poolID} pools GetPool
//
// Get pool by ID.
//
//	Parameters:
//	  + name: poolID
//	    description: ID of the pool to fetch.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Pool
//	  default: APIErrorResponse
func (a *APIController) GetPoolByIDHandler(w http.ResponseWriter, r *http.Request) {
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

	pool, err := a.r.GetPoolByID(ctx, poolID)
	if err != nil {
		log.Printf("fetching pool: %s", err)
		handleError(w, err)
		return
	}

	pool.RunnerBootstrapTimeout = pool.RunnerTimeout()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pool); err != nil {
		log.Printf("failed to encode response: %q", err)
	}
}

// swagger:route DELETE /pools/{poolID} pools DeletePool
//
// Delete pool by ID.
//
//	Parameters:
//	  + name: poolID
//	    description: ID of the pool to delete.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeletePoolByIDHandler(w http.ResponseWriter, r *http.Request) {
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

	if err := a.r.DeletePoolByID(ctx, poolID); err != nil {
		log.Printf("removing pool: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route PUT /pools/{poolID} pools UpdatePool
//
// Update pool by ID.
//
//	Parameters:
//	  + name: poolID
//	    description: ID of the pool to update.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters to update the pool with.
//	    type: UpdatePoolParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Pool
//	  default: APIErrorResponse
func (a *APIController) UpdatePoolByIDHandler(w http.ResponseWriter, r *http.Request) {
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

	var poolData runnerParams.UpdatePoolParams
	if err := json.NewDecoder(r.Body).Decode(&poolData); err != nil {
		log.Printf("failed to decode: %s", err)
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	pool, err := a.r.UpdatePoolByID(ctx, poolID, poolData)
	if err != nil {
		log.Printf("fetching pool: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pool); err != nil {
		log.Printf("failed to encode response: %q", err)
	}
}
