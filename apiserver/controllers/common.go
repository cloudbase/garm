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
	owner, hasOwner := vars["owner"]
	repo, hasRepo := vars["repo"]
	if !(hasOwner && hasRepo) {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(apiParams.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(r.Context(), "failed to encode response")
		}
		return params.Repository{}, false
	}
	repoObj, err := a.r.ResolveRepository(r.Context(), owner, repo, r.URL.Query().Get("endpointName"))
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(r.Context(), "resolving repository")
		handleError(r.Context(), w, err)
		return params.Repository{}, false
	}
	return repoObj, true
}

func (a *APIController) GetRepositoryID(w http.ResponseWriter, r *http.Request) (string, bool) {
	vars := mux.Vars(r)
	owner, hasOwner := vars["owner"]
	repo, hasRepo := vars["repo"]
	if !(hasOwner && hasRepo) {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(apiParams.APIErrorResponse{
			Error:   "Bad Request",
			Details: "No repo ID specified",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(r.Context(), "failed to encode response")
		}
		return "", false
	}
	repoObj, err := a.r.ResolveRepository(r.Context(), owner, repo, r.URL.Query().Get("endpointName"))
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(r.Context(), "resolving repository")
		handleError(r.Context(), w, err)
		return "", false
	}
	return repoObj.ID, true
}
