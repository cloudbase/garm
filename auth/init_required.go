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

package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/cloudbase/garm/apiserver/params"
	"github.com/cloudbase/garm/database/common"
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
		ctx := r.Context()

		if !i.store.HasAdminUser(ctx) {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(params.InitializationRequired); err != nil {
				slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
			}
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func NewUrlsRequiredMiddleware(store common.Store) (Middleware, error) {
	return &urlsRequired{
		store: store,
	}, nil
}

type urlsRequired struct {
	store common.Store
}

func (u *urlsRequired) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctrlInfo, err := u.store.ControllerInfo()
		if err != nil || ctrlInfo.MetadataURL == "" || ctrlInfo.CallbackURL == "" {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(params.URLsRequired); err != nil {
				slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
			}
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
