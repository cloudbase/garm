// Copyright 2023 Cloudbase Solutions SRL
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
	"fmt"
	"log/slog"
	"net/http"

	"github.com/cloudbase/garm/apiserver/params"
	"github.com/gorilla/mux"
)

func (a *APIController) InstanceGithubRegistrationTokenHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token, err := a.r.GetInstanceGithubRegistrationToken(ctx)
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(token)); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

func (a *APIController) JITCredentialsFileHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	fileName, ok := vars["fileName"]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(params.APIErrorResponse{
			Error:   "Not Found",
			Details: "Not Found",
		}); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
		}
		return
	}

	dotFileName := fmt.Sprintf(".%s", fileName)

	data, err := a.r.GetJITConfigFile(ctx, dotFileName)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "getting JIT config file")
		handleError(ctx, w, err)
		return
	}

	// Note the leading dot in the filename
	name := fmt.Sprintf("attachment; filename=%s", dotFileName)
	w.Header().Set("Content-Disposition", name)
	w.Header().Set("Content-Type", "octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

func (a *APIController) SystemdServiceNameHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	serviceName, err := a.r.GetRunnerServiceName(ctx)
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(serviceName)); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

func (a *APIController) SystemdUnitFileHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	runAsUser := r.URL.Query().Get("runAsUser")

	data, err := a.r.GenerateSystemdUnitFile(ctx, runAsUser)
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

func (a *APIController) RootCertificateBundleHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	bundle, err := a.r.GetRootCertificateBundle(ctx)
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(bundle); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}
