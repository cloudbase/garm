package controllers

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// swagger:route GET /credentials credentials ListCredentials
//
// List all credentials.
//
//	Responses:
//	  200: Credentials
//	  400: APIErrorResponse
func (a *APIController) ListCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	creds, err := a.r.ListCredentials(ctx)
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(creds); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}
