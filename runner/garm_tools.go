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

package runner

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

var (
	garmAgentFileTag          = "category=garm-agent"
	garmAgentOSTypeWindowsTag = "os_type=windows"
	garmAgentOSTypeLinuxTag   = "os_type=linux"
	garmAgentOSArchAMD64Tag   = "os_arch=amd64"
	garmAgentOSArchARM64Tag   = "os_arch=arm64"
)

func (r *Runner) ListGARMTools(ctx context.Context) ([]params.GARMAgentTool, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	ret := []params.GARMAgentTool{}
	var next uint64 = 1
	for {
		allAgentTools, err := r.store.SearchFileObjectByTags(r.ctx, []string{garmAgentFileTag}, next, 100)
		if err != nil {
			return nil, fmt.Errorf("failed to list files: %w", err)
		}
		if allAgentTools.TotalCount == 0 {
			return nil, nil
		}
		for _, tool := range allAgentTools.Results {
			parsed, err := fileObjectToGARMTool(tool, "")
			if err != nil {
				return nil, fmt.Errorf("failed to parse object with ID %d", tool.ID)
			}
			ret = append(ret, parsed)
		}
		if allAgentTools.NextPage == nil {
			break
		}
		next = *allAgentTools.NextPage
	}
	return ret, nil
}

func (r *Runner) CreateGARMTool(ctx context.Context, param params.CreateGARMToolParams, reader io.Reader) (params.FileObject, error) {
	if !auth.IsAdmin(ctx) {
		return params.FileObject{}, runnerErrors.ErrUnauthorized
	}

	// Validate version is provided
	if param.Version == "" {
		return params.FileObject{}, runnerErrors.NewBadRequestError("version is required")
	}

	// Build tags based on OS type and arch
	var osTypeTag, osArchTag string
	switch param.OSType {
	case "windows":
		osTypeTag = garmAgentOSTypeWindowsTag
	case "linux":
		osTypeTag = garmAgentOSTypeLinuxTag
	default:
		return params.FileObject{}, runnerErrors.NewBadRequestError("invalid os_type: must be 'windows' or 'linux'")
	}

	switch param.OSArch {
	case "amd64":
		osArchTag = garmAgentOSArchAMD64Tag
	case "arm64":
		osArchTag = garmAgentOSArchARM64Tag
	default:
		return params.FileObject{}, runnerErrors.NewBadRequestError("invalid os_arch: must be 'amd64' or 'arm64'")
	}

	// Build tags: category, os_type, os_arch, version
	tags := []string{
		garmAgentFileTag,
		osTypeTag,
		osArchTag,
		fmt.Sprintf("version=%s", param.Version),
	}

	// Create the file object params
	createParams := params.CreateFileObjectParams{
		Name:        param.Name,
		Description: param.Description,
		Size:        param.Size,
		Tags:        tags,
	}

	// Upload the new binary
	newTool, err := r.store.CreateFileObject(ctx, createParams, reader)
	if err != nil {
		return params.FileObject{}, fmt.Errorf("failed to upload garm-agent tool: %w", err)
	}
	slog.DebugContext(ctx, "uploaded new garm-agent tool",
		"tool_id", newTool.ID,
		"name", newTool.Name,
		"os_type", param.OSType,
		"os_arch", param.OSArch,
		"version", param.Version,
		"size", newTool.Size)

	// Clean up old versions (keep only the newly uploaded one)
	// Build tags to find all binaries with same OS/ARCH (excluding version)
	cleanupTags := []string{garmAgentFileTag, osTypeTag, osArchTag}

	// Delete all except the one we just uploaded
	// Paginate through all results to ensure we delete everything
	deletedCount := 0
	page := uint64(1)
	pageSize := uint64(100)

	for {
		allTools, err := r.store.SearchFileObjectByTags(ctx, cleanupTags, page, pageSize)
		if err != nil {
			slog.ErrorContext(ctx, "failed to search for old garm-agent versions during cleanup",
				"error", err,
				"os_type", param.OSType,
				"os_arch", param.OSArch,
				"new_tool_id", newTool.ID,
				"page", page)
			// Don't fail - upload succeeded
			break
		}

		for _, tool := range allTools.Results {
			if tool.ID != newTool.ID {
				// Delete old version directly via store (bypass API check since this is internal)
				if err := r.store.DeleteFileObject(ctx, tool.ID); err != nil {
					slog.WarnContext(ctx, "failed to delete old garm-agent version during cleanup",
						"error", err,
						"tool_id", tool.ID,
						"tool_name", tool.Name,
						"os_type", param.OSType,
						"os_arch", param.OSArch)
					continue
				}
				deletedCount++
				slog.DebugContext(ctx, "deleted old garm-agent version",
					"tool_id", tool.ID,
					"tool_name", tool.Name,
					"tags", tool.Tags)
			}
		}

		// Check if there's a next page
		if allTools.NextPage == nil {
			break
		}
		page = *allTools.NextPage
	}

	if deletedCount > 0 {
		slog.InfoContext(ctx, "cleaned up old garm-agent versions",
			"deleted_count", deletedCount,
			"os_type", param.OSType,
			"os_arch", param.OSArch,
			"kept_version", param.Version)
	}

	return newTool, nil
}

func (r *Runner) DeleteGarmTool(ctx context.Context, osType, osArch string) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	// Build tags based on OS type and arch
	tags := []string{garmAgentFileTag}

	switch osType {
	case "windows":
		tags = append(tags, garmAgentOSTypeWindowsTag)
	case "linux":
		tags = append(tags, garmAgentOSTypeLinuxTag)
	default:
		return runnerErrors.NewBadRequestError("invalid os_type: must be 'windows' or 'linux'")
	}

	switch osArch {
	case "amd64":
		tags = append(tags, garmAgentOSArchAMD64Tag)
	case "arm64":
		tags = append(tags, garmAgentOSArchARM64Tag)
	default:
		return runnerErrors.NewBadRequestError("invalid os_arch: must be 'amd64' or 'arm64'")
	}

	// Delete all tools matching these tags
	deletedCount, err := r.store.DeleteFileObjectsByTags(ctx, tags)
	if err != nil {
		return fmt.Errorf("failed to delete garm-agent tools: %w", err)
	}

	if deletedCount == 0 {
		return runnerErrors.NewNotFoundError("no garm-agent tools found for %s/%s", osType, osArch)
	}

	return nil
}
