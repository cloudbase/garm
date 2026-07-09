// Copyright 2026 Cloudbase Solutions SRL
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

	"github.com/gorilla/mux"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	runnerParams "github.com/cloudbase/garm/params"
)

// swagger:route GET /proxies proxies ListProxies
//
// List proxies.
//
//	Responses:
//	  200: Proxies
//	  default: APIErrorResponse
func (a *APIController) ListProxiesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	proxies, err := a.r.ListProxies(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing proxies")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(proxies); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /proxies/{proxyID} proxies GetProxy
//
// Get proxy by ID.
//
//	Parameters:
//	  + name: proxyID
//	    description: ID of the proxy to fetch.
//	    type: number
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Proxy
//	  default: APIErrorResponse
func (a *APIController) GetProxyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	proxyID, err := getValueFromVarsAsUint(vars, "proxyID")
	if err != nil {
		handleError(ctx, w, err)
		return
	}
	proxy, err := a.r.GetProxy(ctx, proxyID)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "fetching proxy")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(proxy); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route POST /proxies proxies CreateProxy
//
// Create proxy with the parameters given.
//
//	Parameters:
//	  + name: Body
//	    description: Parameters used when creating the proxy.
//	    type: CreateProxyParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Proxy
//	  default: APIErrorResponse
func (a *APIController) CreateProxyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var proxyData runnerParams.CreateProxyParams
	if err := json.NewDecoder(r.Body).Decode(&proxyData); err != nil {
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	proxy, err := a.r.CreateProxy(ctx, proxyData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating proxy")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(proxy); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route PUT /proxies/{proxyID} proxies UpdateProxy
//
// Update proxy with the parameters given.
//
//	Parameters:
//	  + name: proxyID
//	    description: ID of the proxy to update.
//	    type: number
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when updating the proxy.
//	    type: UpdateProxyParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Proxy
//	  default: APIErrorResponse
func (a *APIController) UpdateProxyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	proxyID, err := getValueFromVarsAsUint(vars, "proxyID")
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	var updatePayload runnerParams.UpdateProxyParams
	if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	proxy, err := a.r.UpdateProxy(ctx, proxyID, updatePayload)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error updating proxy")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(proxy); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /proxies/{proxyID} proxies DeleteProxy
//
// Delete proxy by ID.
//
//	Parameters:
//	  + name: proxyID
//	    description: ID of the proxy to delete.
//	    type: number
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteProxyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	proxyID, err := getValueFromVarsAsUint(vars, "proxyID")
	if err != nil {
		handleError(ctx, w, err)
		return
	}
	if err := a.r.DeleteProxy(ctx, proxyID); err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}
