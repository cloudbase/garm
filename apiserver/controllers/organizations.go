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

func (a *APIController) CreateOrgHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var repoData runnerParams.CreateOrgParams
	if err := json.NewDecoder(r.Body).Decode(&repoData); err != nil {
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	repo, err := a.r.CreateOrganization(ctx, repoData)
	if err != nil {
		log.Printf("error creating repository: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(repo)
}

func (a *APIController) ListOrgsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	orgs, err := a.r.ListOrganizations(ctx)
	if err != nil {
		log.Printf("listing orgs: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orgs)
}

func (a *APIController) GetOrgByIDHandler(w http.ResponseWriter, r *http.Request) {
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

	org, err := a.r.GetOrganizationByID(ctx, orgID)
	if err != nil {
		log.Printf("fetching org: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(org)
}

func (a *APIController) DeleteOrgHandler(w http.ResponseWriter, r *http.Request) {
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

	if err := a.r.DeleteOrganization(ctx, orgID); err != nil {
		log.Printf("fetching org: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

}

func (a *APIController) UpdateOrgHandler(w http.ResponseWriter, r *http.Request) {
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

	var updatePayload runnerParams.UpdateRepositoryParams
	if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	org, err := a.r.UpdateOrganization(ctx, orgID, updatePayload)
	if err != nil {
		log.Printf("error updating organization: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(org)
}

func (a *APIController) CreateOrgPoolHandler(w http.ResponseWriter, r *http.Request) {
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

	var poolData runnerParams.CreatePoolParams
	if err := json.NewDecoder(r.Body).Decode(&poolData); err != nil {
		log.Printf("failed to decode: %s", err)
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	pool, err := a.r.CreateOrgPool(ctx, orgID, poolData)
	if err != nil {
		log.Printf("error creating organization pool: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pool)
}

func (a *APIController) ListOrgPoolsHandler(w http.ResponseWriter, r *http.Request) {
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

	pools, err := a.r.ListOrgPools(ctx, orgID)
	if err != nil {
		log.Printf("listing pools: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pools)
}

func (a *APIController) GetOrgPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	orgID, repoOk := vars["orgID"]
	poolID, poolOk := vars["poolID"]
	if !repoOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No org or pool ID specified",
		})
		return
	}

	pool, err := a.r.GetOrgPoolByID(ctx, orgID, poolID)
	if err != nil {
		log.Printf("listing pools: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pool)
}

func (a *APIController) DeleteOrgPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	orgID, orgOk := vars["orgID"]
	poolID, poolOk := vars["poolID"]
	if !orgOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No org or pool ID specified",
		})
		return
	}

	if err := a.r.DeleteOrgPool(ctx, orgID, poolID); err != nil {
		log.Printf("removing pool: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

}

func (a *APIController) UpdateOrgPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	orgID, orgOk := vars["orgID"]
	poolID, poolOk := vars["poolID"]
	if !orgOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No org or pool ID specified",
		})
		return
	}

	var poolData runnerParams.UpdatePoolParams
	if err := json.NewDecoder(r.Body).Decode(&poolData); err != nil {
		log.Printf("failed to decode: %s", err)
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	pool, err := a.r.UpdateOrgPool(ctx, orgID, poolID, poolData)
	if err != nil {
		log.Printf("error creating organization pool: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pool)
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
		log.Printf("listing pools: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instances)
}
