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
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/apiserver/params"
)

func (a *APIController) InstanceMetadataHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	metadata, err := a.r.GetInstanceMetadata(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get instance metadata", "error", err)
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metadata); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /tools/garm-agent tools GarmAgentList
//
// List GARM agent tools.
//
//	Parameters:
//	  + name: page
//	    description: The page at which to list.
//	    type: integer
//	    in: query
//	    required: false
//	  + name: pageSize
//	    description: Number of items per page.
//	    type: integer
//	    in: query
//	    required: false
//
//	Responses:
//	  200: GARMAgentToolsPaginatedResponse
//	  400: APIErrorResponse
func (a *APIController) InstanceGARMToolsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var pageLocation int64
	var pageSize int64 = 25
	pageArg := r.URL.Query().Get("page")
	pageSizeArg := r.URL.Query().Get("pageSize")

	if pageArg != "" {
		pageInt, err := strconv.ParseInt(pageArg, 10, 64)
		if err == nil && pageInt >= 0 {
			pageLocation = pageInt
		}
	}
	if pageSizeArg != "" {
		pageSizeInt, err := strconv.ParseInt(pageSizeArg, 10, 64)
		if err == nil && pageSizeInt >= 0 {
			pageSize = pageSizeInt
		}
	}

	tools, err := a.r.GetGARMTools(ctx, uint64(pageLocation), uint64(pageSize))
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tools); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

func (a *APIController) InstanceShowGARMToolHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	objectID, err := getObjectIDFromVars(vars)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get object ID", "error", err)
		handleError(ctx, w, gErrors.NewBadRequestError("invalid objectID: %s", err))
		return
	}
	tools, err := a.r.ShowGARMTools(ctx, objectID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get garm tools", "error", err)
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tools); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

func (a *APIController) InstanceGARMToolDownloadHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	objectID, err := getObjectIDFromVars(vars)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get object ID", "error", err)
		handleError(ctx, w, gErrors.NewBadRequestError("invalid objectID: %s", err))
		return
	}

	reader, err := a.r.GetGARMToolsReadHandler(ctx, objectID)
	if err != nil {
		handleError(ctx, w, err)
		return
	}
	defer reader.Close()
	if _, err := io.Copy(w, reader); err != nil {
		slog.ErrorContext(ctx, "failed to stream data", "error", err)
	}
}

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

func (a *APIController) RunnerInstallScriptHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	installScript, err := a.r.GetRunnerInstallScript(ctx)
	if err != nil {
		slog.InfoContext(ctx, "failed to get runner install template", "error", err)
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(installScript); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}
