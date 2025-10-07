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
package runner

import (
	"context"
	"fmt"
	"io"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

func (r *Runner) CreateFileObject(ctx context.Context, param params.CreateFileObjectParams, reader io.Reader) (params.FileObject, error) {
	if !auth.IsAdmin(ctx) {
		return params.FileObject{}, runnerErrors.ErrUnauthorized
	}

	fileObj, err := r.store.CreateFileObject(ctx, param, reader)
	if err != nil {
		return params.FileObject{}, fmt.Errorf("failed to create file object: %w", err)
	}
	return fileObj, nil
}

func (r *Runner) GetFileObject(ctx context.Context, objID uint) (params.FileObject, error) {
	if !auth.IsAdmin(ctx) {
		return params.FileObject{}, runnerErrors.ErrUnauthorized
	}

	fileObj, err := r.store.GetFileObject(ctx, objID)
	if err != nil {
		return params.FileObject{}, fmt.Errorf("failed to get file object: %w", err)
	}

	return fileObj, nil
}

func (r *Runner) DeleteFileObject(ctx context.Context, objID uint) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	if err := r.store.DeleteFileObject(ctx, objID); err != nil {
		return fmt.Errorf("failed to delete file object: %w", err)
	}
	return nil
}

func (r *Runner) ListFileObjects(ctx context.Context, page, pageSize uint64, tags []string) (params.FileObjectPaginatedResponse, error) {
	if !auth.IsAdmin(ctx) {
		return params.FileObjectPaginatedResponse{}, runnerErrors.ErrUnauthorized
	}
	var resp params.FileObjectPaginatedResponse
	var err error
	if len(tags) == 0 {
		resp, err = r.store.ListFileObjects(ctx, page, pageSize)
	} else {
		resp, err = r.store.SearchFileObjectByTags(ctx, tags, page, pageSize)
	}

	if err != nil {
		return params.FileObjectPaginatedResponse{}, fmt.Errorf("failed to list objects: %w", err)
	}
	return resp, nil
}

func (r *Runner) UpdateFileObject(ctx context.Context, objID uint, param params.UpdateFileObjectParams) (params.FileObject, error) {
	if !auth.IsAdmin(ctx) {
		return params.FileObject{}, runnerErrors.ErrUnauthorized
	}

	resp, err := r.store.UpdateFileObject(ctx, objID, param)
	if err != nil {
		return params.FileObject{}, fmt.Errorf("failed to update object: %w", err)
	}
	return resp, nil
}

func (r *Runner) GetFileObjectReader(ctx context.Context, objID uint) (io.ReadCloser, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	readCloser, err := r.store.OpenFileObjectContent(ctx, objID)
	if err != nil {
		return nil, fmt.Errorf("failed to open file object: %w", err)
	}
	return readCloser, nil
}
