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
