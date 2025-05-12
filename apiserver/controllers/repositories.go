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

// swagger:route POST /repositories repositories CreateRepo
//
// Create repository with the parameters given.
//
//	Parameters:
//	  + name: Body
//	    description: Parameters used when creating the repository.
//	    type: CreateRepoParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Repository
//	  default: APIErrorResponse
func (a *APIController) CreateRepoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var repoData runnerParams.CreateRepoParams
	if err := json.NewDecoder(r.Body).Decode(&repoData); err != nil {
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	repo, err := a.r.CreateRepository(ctx, repoData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating repository")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(repo); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /repositories repositories ListRepos
//
// List repositories.
//
//	Responses:
//	  200: Repositories
//	  default: APIErrorResponse
func (a *APIController) ListReposHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	repos, err := a.r.ListRepositories(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing repositories")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(repos); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /repositories/{repoID} repositories GetRepo
//
// Get repository by ID.
//
//	Parameters:
//	  + name: repoID
//	    description: ID of the repository to fetch.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Repository
//	  default: APIErrorResponse
func (a *APIController) GetRepoByIDHandler(w http.ResponseWriter, r *http.Request) {
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

	repo, err := a.r.GetRepositoryByID(ctx, repoID)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "fetching repository")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(repo); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /repositories/{repoID} repositories DeleteRepo
//
// Delete repository by ID.
//
//	Parameters:
//	  + name: repoID
//	    description: ID of the repository to delete.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: keepWebhook
//	    description: If true and a webhook is installed for this repo, it will not be removed.
//	    type: boolean
//	    in: query
//	    required: false
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteRepoHandler(w http.ResponseWriter, r *http.Request) {
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

	keepWebhook, _ := strconv.ParseBool(r.URL.Query().Get("keepWebhook"))
	if err := a.r.DeleteRepository(ctx, repoID, keepWebhook); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "fetching repository")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route PUT /repositories/{repoID} repositories UpdateRepo
//
// Update repository with the parameters given.
//
//	Parameters:
//	  + name: repoID
//	    description: ID of the repository to update.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when updating the repository.
//	    type: UpdateEntityParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Repository
//	  default: APIErrorResponse
func (a *APIController) UpdateRepoHandler(w http.ResponseWriter, r *http.Request) {
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

	var updatePayload runnerParams.UpdateEntityParams
	if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	repo, err := a.r.UpdateRepository(ctx, repoID, updatePayload)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error updating repository")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(repo); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route POST /repositories/{repoID}/pools repositories pools CreateRepoPool
//
// Create repository pool with the parameters given.
//
//	Parameters:
//	  + name: repoID
//	    description: Repository ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when creating the repository pool.
//	    type: CreatePoolParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Pool
//	  default: APIErrorResponse
func (a *APIController) CreateRepoPoolHandler(w http.ResponseWriter, r *http.Request) {
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

	var poolData runnerParams.CreatePoolParams
	if err := json.NewDecoder(r.Body).Decode(&poolData); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	pool, err := a.r.CreateRepoPool(ctx, repoID, poolData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating repository pool")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pool); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route POST /repositories/{repoID}/scalesets repositories scalesets CreateRepoScaleSet
//
// Create repository scale set with the parameters given.
//
//	Parameters:
//	  + name: repoID
//	    description: Repository ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when creating the repository scale set.
//	    type: CreateScaleSetParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: ScaleSet
//	  default: APIErrorResponse
func (a *APIController) CreateRepoScaleSetHandler(w http.ResponseWriter, r *http.Request) {
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

	var scaleSetData runnerParams.CreateScaleSetParams
	if err := json.NewDecoder(r.Body).Decode(&scaleSetData); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	scaleSet, err := a.r.CreateEntityScaleSet(ctx, runnerParams.ForgeEntityTypeRepository, repoID, scaleSetData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating repository scale set")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(scaleSet); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /repositories/{repoID}/pools repositories pools ListRepoPools
//
// List repository pools.
//
//	Parameters:
//	  + name: repoID
//	    description: Repository ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Pools
//	  default: APIErrorResponse
func (a *APIController) ListRepoPoolsHandler(w http.ResponseWriter, r *http.Request) {
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

	pools, err := a.r.ListRepoPools(ctx, repoID)
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

// swagger:route GET /repositories/{repoID}/scalesets repositories scalesets ListRepoScaleSets
//
// List repository scale sets.
//
//	Parameters:
//	  + name: repoID
//	    description: Repository ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: ScaleSets
//	  default: APIErrorResponse
func (a *APIController) ListRepoScaleSetsHandler(w http.ResponseWriter, r *http.Request) {
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

	scaleSets, err := a.r.ListEntityScaleSets(ctx, runnerParams.ForgeEntityTypeRepository, repoID)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing scale sets")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(scaleSets); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /repositories/{repoID}/pools/{poolID} repositories pools GetRepoPool
//
// Get repository pool by ID.
//
//	Parameters:
//	  + name: repoID
//	    description: Repository ID.
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
func (a *APIController) GetRepoPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	repoID, repoOk := vars["repoID"]
	poolID, poolOk := vars["poolID"]
	if !repoOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo or pool ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	pool, err := a.r.GetRepoPoolByID(ctx, repoID, poolID)
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

// swagger:route DELETE /repositories/{repoID}/pools/{poolID} repositories pools DeleteRepoPool
//
// Delete repository pool by ID.
//
//	Parameters:
//	  + name: repoID
//	    description: Repository ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: poolID
//	    description: ID of the repository pool to delete.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteRepoPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	repoID, repoOk := vars["repoID"]
	poolID, poolOk := vars["poolID"]
	if !repoOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo or pool ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	if err := a.r.DeleteRepoPool(ctx, repoID, poolID); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "removing pool")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route PUT /repositories/{repoID}/pools/{poolID} repositories pools UpdateRepoPool
//
// Update repository pool with the parameters given.
//
//	Parameters:
//	  + name: repoID
//	    description: Repository ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: poolID
//	    description: ID of the repository pool to update.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when updating the repository pool.
//	    type: UpdatePoolParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Pool
//	  default: APIErrorResponse
func (a *APIController) UpdateRepoPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	repoID, repoOk := vars["repoID"]
	poolID, poolOk := vars["poolID"]
	if !repoOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo or pool ID specified",
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

	pool, err := a.r.UpdateRepoPool(ctx, repoID, poolID, poolData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating repository pool")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pool); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route POST /repositories/{repoID}/webhook repositories hooks InstallRepoWebhook
//
// Install the GARM webhook for an organization. The secret configured on the organization will
// be used to validate the requests.
//
//	Parameters:
//	  + name: repoID
//	    description: Repository ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when creating the repository webhook.
//	    type: InstallWebhookParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: HookInfo
//	  default: APIErrorResponse
func (a *APIController) InstallRepoWebhookHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	repoID, orgOk := vars["repoID"]
	if !orgOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repository ID specified",
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

	info, err := a.r.InstallRepoWebhook(ctx, repoID, hookParam)
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

// swagger:route DELETE /repositories/{repoID}/webhook repositories hooks UninstallRepoWebhook
//
// Uninstall organization webhook.
//
//	Parameters:
//	  + name: repoID
//	    description: Repository ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) UninstallRepoWebhookHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	repoID, orgOk := vars["repoID"]
	if !orgOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repository ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	if err := a.r.UninstallRepoWebhook(ctx, repoID); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "removing webhook")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route GET /repositories/{repoID}/webhook repositories hooks GetRepoWebhookInfo
//
// Get information about the GARM installed webhook on a repository.
//
//	Parameters:
//	  + name: repoID
//	    description: Repository ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: HookInfo
//	  default: APIErrorResponse
func (a *APIController) GetRepoWebhookInfoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	repoID, orgOk := vars["repoID"]
	if !orgOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repository ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	info, err := a.r.GetRepoWebhookInfo(ctx, repoID)
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
