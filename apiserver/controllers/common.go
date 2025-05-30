package controllers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	apiParams "github.com/cloudbase/garm/apiserver/params"
	"github.com/cloudbase/garm/params"
	"github.com/gorilla/mux"
)

func (a *APIController) GetRepository(w http.ResponseWriter, r *http.Request) (params.Repository, bool) {
	vars := mux.Vars(r)
	repoID, ok := vars["repoID"]
	owner, hasOwner := vars["owner"]
	repo, hasRepo := vars["repo"]
	if !ok && !(hasOwner && hasRepo) {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(apiParams.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(r.Context(), "failed to encode response")
		}
		return params.Repository{}, false
	}
	var repoObj params.Repository
	var err error
	if hasOwner && hasRepo {
		repoObj, err = a.r.ResolveRepository(r.Context(), owner, repo, r.URL.Query().Get("endpointName"))
	} else {
		repoObj, err = a.r.GetRepositoryByID(r.Context(), repoID)
	}
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(r.Context(), "listing pools")
		handleError(r.Context(), w, err)
		return params.Repository{}, false
	}
	return repoObj, true
}
