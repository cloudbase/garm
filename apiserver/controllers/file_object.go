// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
)

// swagger:route GET /objects objects ListFileObjects
//
// List file objects.
//
//	Parameters:
//	  + name: tags
//	    description: List of tags to filter by.
//	    type: array
//	    items:
//	      type: string
//	    in: query
//	    required: false
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
//	  200: FileObjectPaginatedResponse
//	  400: APIErrorResponse
func (a *APIController) ListFileObjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var pageLocation int64
	var pageSize int64 = 25
	tags := r.URL.Query().Get("tags")
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
	parsedTags := parseTagsArg(tags)
	files, err := a.r.ListFileObjects(ctx, uint64(pageLocation), uint64(pageSize), parsedTags)
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(files); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route DELETE /objects/{objectID} objects DeleteFileObject
//
// Delete a file object.
//
//	Parameters:
//	  + name: objectID
//	    description: The ID of the file object.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  default: APIErrorResponse
func (a *APIController) DeleteFileObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	objectID, err := getObjectIDFromVars(vars)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get object ID", "error", err)
		handleError(ctx, w, gErrors.NewBadRequestError("invalid objectID: %s", err))
		return
	}

	if err := a.r.DeleteFileObject(ctx, objectID); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to delete file object")
		handleError(ctx, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// swagger:route GET /objects/{objectID} objects GetFileObject
//
// Get a file object.
//
//	Parameters:
//	  + name: objectID
//	    description: The ID of the file object.
//	    type: string
//	    in: path
//	    required: true
//
//	Responses:
//	  200: FileObject
//	  400: APIErrorResponse
func (a *APIController) GetFileObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	objectID, err := getObjectIDFromVars(vars)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get object ID", "error", err)
		handleError(ctx, w, gErrors.NewBadRequestError("invalid objectID: %s", err))
		return
	}

	file, err := a.r.GetFileObject(ctx, objectID)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to get file object")
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(file); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

// swagger:route PUT /objects/{objectID} objects UpdateFileObject
//
// Update a file object.
//
//	Parameters:
//	  + name: objectID
//	    description: The ID of the file object.
//	    type: string
//	    in: path
//	    required: true
//	  + name: Body
//	    description: Parameters used when updating a file object.
//	    type: UpdateFileObjectParams
//	    in: body
//	    required: true
//
//	Responses:
//	  200: FileObject
//	  400: APIErrorResponse
func (a *APIController) UpdateFileObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	objectID, err := getObjectIDFromVars(vars)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get object ID", "error", err)
		handleError(ctx, w, gErrors.NewBadRequestError("invalid objectID: %s", err))
		return
	}

	var param params.UpdateFileObjectParams
	if err := json.NewDecoder(r.Body).Decode(&param); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to decode request")
		handleError(ctx, w, gErrors.ErrBadRequest)
		return
	}

	if len(param.Tags) > 0 {
		for idx, val := range param.Tags {
			param.Tags[idx] = strings.ToLower(strings.TrimSpace(val))
		}
	}

	file, err := a.r.UpdateFileObject(ctx, objectID, param)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to get file object")
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(file); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

func (a *APIController) CreateFileObject(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ctx := r.Context()
	fileName := r.Header.Get("X-File-Name")
	if fileName == "" {
		handleError(ctx, w, gErrors.NewBadRequestError("missing X-File-Name header"))
		return
	}
	description := r.Header.Get("X-File-Description")
	contentLengthStr := r.Header.Get("Content-Length")
	if contentLengthStr == "" {
		handleError(ctx, w, gErrors.NewBadRequestError("missing Content-Length header in request"))
		return
	}

	tags := r.Header.Get("X-Tags")
	parsedTags := parseTagsArg(tags)

	fileSize, err := strconv.ParseInt(contentLengthStr, 10, 64)
	if err != nil {
		handleError(ctx, w, gErrors.NewBadRequestError("invalid Content-Length"))
		return
	}

	param := params.CreateFileObjectParams{
		Name: fileName,
		Size: fileSize,
		Tags: parsedTags,
	}
	if len(description) > 0 {
		param.Description = description
	}

	fileObj, err := a.r.CreateFileObject(ctx, param, r.Body)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create blob", "error", err)
		handleError(ctx, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(fileObj); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to encode response")
	}
}

func (a *APIController) DownloadFileObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	objectID, err := getObjectIDFromVars(vars)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get object ID", "error", err)
		handleError(ctx, w, gErrors.NewBadRequestError("invalid objectID: %s", err))
		return
	}

	objectDetails, err := a.r.GetFileObject(ctx, objectID)
	if err != nil {
		handleError(ctx, w, err)
		return
	}

	objectHandle, err := a.r.GetFileObjectReader(ctx, objectID)
	if err != nil {
		handleError(ctx, w, err)
		return
	}
	defer objectHandle.Close()
	w.Header().Set("Content-Type", objectDetails.FileType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", objectDetails.Name))
	w.Header().Set("Content-Length", strconv.FormatInt(objectDetails.Size, 10))

	if r.Method == http.MethodHead {
		return
	}

	copied, err := io.Copy(w, objectHandle)
	if err != nil {
		slog.ErrorContext(ctx, "failed to stream data", "error", err)
	}
	if copied < objectDetails.Size {
		slog.WarnContext(ctx, "some data was not streamed", "object_id", objectDetails.ID, "object_size", objectDetails.Size, "streamed_bytes", copied)
	}
}

func parseTagsArg(tags string) []string {
	var parsedTags []string
	foundTag := make(map[string]struct{})
	if tags != "" {
		tagList := strings.SplitSeq(tags, ",")
		for val := range tagList {
			if val == "" {
				continue
			}
			low := strings.ToLower(strings.TrimSpace(val))
			if _, ok := foundTag[low]; ok {
				continue
			}
			parsedTags = append(parsedTags, low)
			foundTag[low] = struct{}{}
		}
	}
	return parsedTags
}

func parseAsUint(val string) (uint, error) {
	parsedObjID, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid object ID; must be a number")
	}
	if parsedObjID > math.MaxUint {
		return 0, fmt.Errorf("the object ID is too large")
	}
	return uint(parsedObjID), nil
}

func getObjectIDFromVars(vars map[string]string) (uint, error) {
	objectID, ok := vars["objectID"]
	if !ok {
		return 0, fmt.Errorf("no objectID specified")
	}
	return parseAsUint(objectID)
}
