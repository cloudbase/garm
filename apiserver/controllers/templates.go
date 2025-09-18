// Copyright 2025 Cloudbase Solutions SRL
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
	"strconv"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	runnerParams "github.com/cloudbase/garm/params"
	"github.com/gorilla/mux"
)

// swagger:route GET /templates templates ListTemplates
//
// List templates.
//
//	Parameters:
//	  + name: osType
//	    description: OS type of the templates.
//	    type: string
//	    in: query
//	    required: false
//
//	  + name: partialName
//	    description: Partial or full name of the template.
//	    type: string
//	    in: query
//	    required: false
//
//	  + name: forgeType
//	    description: Forge type of the templates.
//	    type: string
//	    in: query
//	    required: false
//
//	Responses:
//	  200: Templates
//	  default: APIErrorResponse
func (a *APIController) ListTemplatesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var partialName *string
	var osType *commonParams.OSType
	var forgeType *runnerParams.EndpointType

	queryName := r.URL.Query().Get("name")
	queryOSType := r.URL.Query().Get("osType")
	queryForgeType := r.URL.Query().Get("forgeType")
	if queryName != "" {
		partialName = &queryName
	}
	if queryOSType != "" {
		asOsType := commonParams.OSType(queryOSType)
		osType = &asOsType
	}

	if queryForgeType != "" {
		asForgeType := runnerParams.EndpointType(queryForgeType)
		forgeType = &asForgeType
	}

	templates, err := a.r.ListTemplates(ctx, osType, forgeType, partialName)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "listing templates")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(templates); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route GET /templates/{templateID} templates GetTemplate
//
// Get template by ID.
//
//	Parameters:
//	  + name: templateID
//	    description: ID of the template to fetch.
//	    type: number
//	    in: path
//	    required: true
//
//	Responses:
//	  200: Template
//	  default: APIErrorResponse
func (a *APIController) GetTemplateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	templateID, err := getValueFromVarsAsUint64(vars, "templateID")
	if err != nil {
		handleError(ctx, w, err)
		return
	}
	template, err := a.r.GetTemplate(ctx, uint(templateID))
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "fetching template")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(template); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /templates/{templateID} templates DeleteTemplate
//
// Get template by ID.
//
//	Parameters:
//	  + name: templateID
//	    description: ID of the template to delete.
//	    type: number
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteTemplateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	templateID, err := getValueFromVarsAsUint64(vars, "templateID")
	if err != nil {
		handleError(ctx, w, err)
		return
	}
	if err := a.r.DeleteTemplate(ctx, uint(templateID)); err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// swagger:route POST /templates templates CreateTemplate
//
// Create template with the parameters given.
//
//	Parameters:
//	  + name: Body
//	    description: Parameters used when creating the template.
//	    type: CreateTemplateParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Template
//	  default: APIErrorResponse
func (a *APIController) CreateTemplateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var templateData runnerParams.CreateTemplateParams
	if err := json.NewDecoder(r.Body).Decode(&templateData); err != nil {
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	template, err := a.r.CreateTemplate(ctx, templateData)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error creating template")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(template); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route PUT /templates/{templateID} templates UpdateTemplate
//
// Update template with the parameters given.
//
//	Parameters:
//	  + name: templateID
//	    description: ID of the template to update.
//	    type: string
//	    in: path
//	    required: true
//
//	  + name: Body
//	    description: Parameters used when updating the template.
//	    type: UpdateTemplateParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: Template
//	  default: APIErrorResponse
func (a *APIController) UpdateTemplateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	templateID, err := getValueFromVarsAsUint64(vars, "templateID")
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	var updatePayload runnerParams.UpdateTemplateParams
	if err := json.NewDecoder(r.Body).Decode(&updatePayload); err != nil {
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	template, err := a.r.UpdateTemplate(ctx, uint(templateID), updatePayload)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "error updating template")
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(template); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

func getValueFromVarsAsUint64(vars map[string]string, key string) (uint64, error) {
	ret, ok := vars[key]
	if !ok {
		return 0, gErrors.NewBadRequestError("no %s specified", key)
	}
	asUint, err := strconv.ParseUint(ret, 10, 64)
	if err != nil {
		return 0, gErrors.NewBadRequestError("invalid value for %q: %q", key, ret)
	}

	return asUint, nil
}
