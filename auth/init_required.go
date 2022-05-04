package auth

import (
	"encoding/json"
	"net/http"
	"garm/apiserver/params"
	"garm/database/common"
)

// NewjwtMiddleware returns a populated jwtMiddleware
func NewInitRequiredMiddleware(store common.Store) (Middleware, error) {
	return &initRequired{
		store: store,
	}, nil
}

type initRequired struct {
	store common.Store
}

// Middleware implements the middleware interface
func (i *initRequired) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctrlInfo, err := i.store.ControllerInfo()
		if err != nil || ctrlInfo.ControllerID.String() == "" {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(params.InitializationRequired)
			return
		}
		ctx := r.Context()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
