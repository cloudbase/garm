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

func (a *APIController) CreateRepoHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var repoData runnerParams.CreateRepoParams
	if err := json.NewDecoder(r.Body).Decode(&repoData); err != nil {
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	repo, err := a.r.CreateRepository(ctx, repoData)
	if err != nil {
		log.Printf("error creating repository: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(repo)
}

func (a *APIController) ListReposHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	repos, err := a.r.ListRepositories(ctx)
	if err != nil {
		log.Printf("listing repos: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(repos)
}

func (a *APIController) GetRepoByIDHandler(w http.ResponseWriter, r *http.Request) {
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

	repo, err := a.r.GetRepositoryByID(ctx, repoID)
	if err != nil {
		log.Printf("fetching repo: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(repo)
}

func (a *APIController) DeleteRepoHandler(w http.ResponseWriter, r *http.Request) {
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

	if err := a.r.DeleteRepository(ctx, repoID); err != nil {
		log.Printf("fetching repo: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

}

func (a *APIController) UpdateRepoHandler(w http.ResponseWriter, r *http.Request) {
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

	var updatePayload runnerParams.UpdateRepositoryParams
	if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	repo, err := a.r.UpdateRepository(ctx, repoID, updatePayload)
	if err != nil {
		log.Printf("error updating repository: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(repo)
}

func (a *APIController) CreateRepoPoolHandler(w http.ResponseWriter, r *http.Request) {
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

	var poolData runnerParams.CreatePoolParams
	if err := json.NewDecoder(r.Body).Decode(&poolData); err != nil {
		log.Printf("failed to decode: %s", err)
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	pool, err := a.r.CreateRepoPool(ctx, repoID, poolData)
	if err != nil {
		log.Printf("error creating repository pool: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pool)
}

func (a *APIController) ListRepoPoolsHandler(w http.ResponseWriter, r *http.Request) {
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

	pools, err := a.r.ListRepoPools(ctx, repoID)
	if err != nil {
		log.Printf("listing pools: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pools)
}

func (a *APIController) GetRepoPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	repoID, repoOk := vars["repoID"]
	poolID, poolOk := vars["poolID"]
	if !repoOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo or pool ID specified",
		})
		return
	}

	pool, err := a.r.GetRepoPoolByID(ctx, repoID, poolID)
	if err != nil {
		log.Printf("listing pools: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pool)
}

func (a *APIController) DeleteRepoPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	repoID, repoOk := vars["repoID"]
	poolID, poolOk := vars["poolID"]
	if !repoOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo or pool ID specified",
		})
		return
	}

	if err := a.r.DeleteRepoPool(ctx, repoID, poolID); err != nil {
		log.Printf("removing pool: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

}

func (a *APIController) UpdateRepoPoolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	repoID, repoOk := vars["repoID"]
	poolID, poolOk := vars["poolID"]
	if !repoOk || !poolOk {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo or pool ID specified",
		})
		return
	}

	var poolData runnerParams.UpdatePoolParams
	if err := json.NewDecoder(r.Body).Decode(&poolData); err != nil {
		log.Printf("failed to decode: %s", err)
		handleError(w, gErrors.ErrBadRequest)
		return
	}

	pool, err := a.r.UpdateRepoPool(ctx, repoID, poolID, poolData)
	if err != nil {
		log.Printf("error creating repository pool: %s", err)
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pool)
}
