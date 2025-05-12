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

// swagger:route POST /organizations organizations CreateOrg
//
// Create organization with the parameters given.
//
//	Parameters:
//	  + name: Body
//	    description: Parameters used when creating the organization.
//	    type: CreateOrgParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Organization
//	  default: APIErrorResponse
func (a *APIController) CreateOrgHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var orgData runnerParams.CreateOrgParams
	if err := json.NewDecoder(r.Body).Decode(&orgData); err != nil {
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	org, err := a.r.CreateOrganization(ctx, orgData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating organization")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(org); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /organizations organizations ListOrgs
//
// List organizations.
//
//	Responses:
//	  200: Organizations
//	  default: APIErrorResponse
func (a *APIController) ListOrgsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	orgs, err := a.r.ListOrganizations(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing orgs")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(orgs); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /organizations/{orgID} organizations GetOrg
//
// Get organization by ID.
//
//	Parameters:
//	  + name: orgID
//	    description: ID of the organization to fetch.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Organization
//	  default: APIErrorResponse
func (a *APIController) GetOrgByIDHandler(w http.ResponseWriter, r *http.Request) {
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

	org, err := a.r.GetOrganizationByID(ctx, orgID)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "fetching org")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(org); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /organizations/{orgID} organizations DeleteOrg
//
// Delete organization by ID.
//
//	Parameters:
//	  + name: orgID
//	    description: ID of the organization to delete.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: keepWebhook
//	    description: If true and a webhook is installed for this organization, it will not be removed.
//	    type: boolean
//	    in: query
//	    required: false
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteOrgHandler(w http.ResponseWriter, r *http.Request) {
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

	keepWebhook, _ := strconv.ParseBool(r.URL.Query().Get("keepWebhook"))

	if err := a.r.DeleteOrganization(ctx, orgID, keepWebhook); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "removing org")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route PUT /organizations/{orgID} organizations UpdateOrg
//
// Update organization with the parameters given.
//
//	Parameters:
//	  + name: orgID
//	    description: ID of the organization to update.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when updating the organization.
//	    type: UpdateEntityParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Organization
//	  default: APIErrorResponse
func (a *APIController) UpdateOrgHandler(w http.ResponseWriter, r *http.Request) {
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

	var updatePayload runnerParams.UpdateEntityParams
	if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	org, err := a.r.UpdateOrganization(ctx, orgID, updatePayload)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error updating organization")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(org); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route POST /organizations/{orgID}/pools organizations pools CreateOrgPool
//
// Create organization pool with the parameters given.
//
//	Parameters:
//	  + name: orgID
//	    description: Organization ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when creating the organization pool.
//	    type: CreatePoolParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Pool
//	  default: APIErrorResponse
func (a *APIController) CreateOrgPoolHandler(w http.ResponseWriter, r *http.Request) {
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

	var poolData runnerParams.CreatePoolParams
	if err := json.NewDecoder(r.Body).Decode(&poolData); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	pool, err := a.r.CreateOrgPool(ctx, orgID, poolData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating organization pool")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pool); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route POST /organizations/{orgID}/scalesets organizations scalesets CreateOrgScaleSet
//
// Create organization scale set with the parameters given.
//
//	Parameters:
//	  + name: orgID
//	    description: Organization ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when creating the organization scale set.
//	    type: CreateScaleSetParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: ScaleSet
//	  default: APIErrorResponse
func (a *APIController) CreateOrgScaleSetHandler(w http.ResponseWriter, r *http.Request) {
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

	var scalesetData runnerParams.CreateScaleSetParams
	if err := json.NewDecoder(r.Body).Decode(&scalesetData); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	scaleSet, err := a.r.CreateEntityScaleSet(ctx, runnerParams.ForgeEntityTypeOrganization, orgID, scalesetData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating organization scale set")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(scaleSet); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /organizations/{orgID}/pools organizations pools ListOrgPools
//
// List organization pools.
//
//	Parameters:
//	  + name: orgID
//	    description: Organization ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Pools
//	  default: APIErrorResponse
func (a *APIController) ListOrgPoolsHandler(w http.ResponseWriter, r *http.Request) {
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

	pools, err := a.r.ListOrgPools(ctx, orgID)
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

// swagger:route GET /organizations/{orgID}/scalesets organizations scalesets ListOrgScaleSets
//
// List organization scale sets.
//
//	Parameters:
//	  + name: orgID
//	    description: Organization ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: ScaleSets
//	  default: APIErrorResponse
func (a *APIController) ListOrgScaleSetsHandler(w http.ResponseWriter, r *http.Request) {
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

	scaleSets, err := a.r.ListEntityScaleSets(ctx, runnerParams.ForgeEntityTypeOrganization, orgID)
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

// swagger:route GET /organizations/{orgID}/pools/{poolID} organizations pools GetOrgPool
//
// Get organization pool by ID.
//
//	Parameters:
//	  + name: orgID
//	    description: Organization ID.
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
func (a *APIController) GetOrgPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	orgID, orgOk := vars["orgID"]
	poolID, poolOk := vars["poolID"]
	if !orgOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No org or pool ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	pool, err := a.r.GetOrgPoolByID(ctx, orgID, poolID)
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

// swagger:route DELETE /organizations/{orgID}/pools/{poolID} organizations pools DeleteOrgPool
//
// Delete organization pool by ID.
//
//	Parameters:
//	  + name: orgID
//	    description: Organization ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: poolID
//	    description: ID of the organization pool to delete.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteOrgPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	orgID, orgOk := vars["orgID"]
	poolID, poolOk := vars["poolID"]
	if !orgOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No org or pool ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	if err := a.r.DeleteOrgPool(ctx, orgID, poolID); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "removing pool")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route PUT /organizations/{orgID}/pools/{poolID} organizations pools UpdateOrgPool
//
// Update organization pool with the parameters given.
//
//	Parameters:
//	  + name: orgID
//	    description: Organization ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: poolID
//	    description: ID of the organization pool to update.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when updating the organization pool.
//	    type: UpdatePoolParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Pool
//	  default: APIErrorResponse
func (a *APIController) UpdateOrgPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	orgID, orgOk := vars["orgID"]
	poolID, poolOk := vars["poolID"]
	if !orgOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No org or pool ID specified",
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

	pool, err := a.r.UpdateOrgPool(ctx, orgID, poolID, poolData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating organization pool")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pool); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route POST /organizations/{orgID}/webhook organizations hooks InstallOrgWebhook
//
// Install the GARM webhook for an organization. The secret configured on the organization will
// be used to validate the requests.
//
//	Parameters:
//	  + name: orgID
//	    description: Organization ID.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when creating the organization webhook.
//	    type: InstallWebhookParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: HookInfo
//	  default: APIErrorResponse
func (a *APIController) InstallOrgWebhookHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	orgID, orgOk := vars["orgID"]
	if !orgOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No org ID specified",
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

	info, err := a.r.InstallOrgWebhook(ctx, orgID, hookParam)
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

// swagger:route DELETE /organizations/{orgID}/webhook organizations hooks UninstallOrgWebhook
//
// Uninstall organization webhook.
//
//	Parameters:
//	  + name: orgID
//	    description: Organization ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) UninstallOrgWebhookHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	orgID, orgOk := vars["orgID"]
	if !orgOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No org ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	if err := a.r.UninstallOrgWebhook(ctx, orgID); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "removing webhook")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route GET /organizations/{orgID}/webhook organizations hooks GetOrgWebhookInfo
//
// Get information about the GARM installed webhook on an organization.
//
//	Parameters:
//	  + name: orgID
//	    description: Organization ID.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: HookInfo
//	  default: APIErrorResponse
func (a *APIController) GetOrgWebhookInfoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	orgID, orgOk := vars["orgID"]
	if !orgOk {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No org ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	info, err := a.r.GetOrgWebhookInfo(ctx, orgID)
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
