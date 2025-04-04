// Copyright 2024 Cloudbase Solutions SRL
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

package scalesets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
)

const (
	runnerEndpoint   = "_apis/distributedtask/pools/0/agents"
	scaleSetEndpoint = "_apis/runtime/runnerscalesets"
)

const (
	HeaderActionsActivityID = "ActivityId"
	HeaderGitHubRequestID   = "X-GitHub-Request-Id"
)

func (s *ScaleSetClient) GetRunnerScaleSetByNameAndRunnerGroup(ctx context.Context, runnerGroupId int, name string) (params.RunnerScaleSet, error) {
	path := fmt.Sprintf("%s?runnerGroupId=%d&name=%s", scaleSetEndpoint, runnerGroupId, name)
	req, err := s.newActionsRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return params.RunnerScaleSet{}, err
	}

	resp, err := s.Do(req)
	if err != nil {
		return params.RunnerScaleSet{}, err
	}

	var runnerScaleSetList *params.RunnerScaleSetsResponse
	if err := json.NewDecoder(resp.Body).Decode(&runnerScaleSetList); err != nil {
		return params.RunnerScaleSet{}, fmt.Errorf("failed to decode response: %w", err)
	}
	if runnerScaleSetList.Count == 0 {
		return params.RunnerScaleSet{}, runnerErrors.NewNotFoundError("runner scale set with name %s and runner group ID %d was not found", name, runnerGroupId)
	}

	// Runner scale sets must have a uniqe name. Attempting to create a runner scale set with the same name as
	// an existing scale set will result in a Bad Request (400) error.
	return runnerScaleSetList.RunnerScaleSets[0], nil
}

func (s *ScaleSetClient) GetRunnerScaleSetById(ctx context.Context, runnerScaleSetId int) (params.RunnerScaleSet, error) {
	path := fmt.Sprintf("%s/%d", scaleSetEndpoint, runnerScaleSetId)
	req, err := s.newActionsRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return params.RunnerScaleSet{}, err
	}

	resp, err := s.Do(req)
	if err != nil {
		return params.RunnerScaleSet{}, fmt.Errorf("failed to get runner scaleset with ID %d: %w", runnerScaleSetId, err)
	}

	var runnerScaleSet params.RunnerScaleSet
	if err := json.NewDecoder(resp.Body).Decode(&runnerScaleSet); err != nil {
		return params.RunnerScaleSet{}, fmt.Errorf("failed to decode response: %w", err)
	}
	return runnerScaleSet, nil
}

// ListRunnerScaleSets lists all runner scale sets in a github entity.
func (s *ScaleSetClient) ListRunnerScaleSets(ctx context.Context) (*params.RunnerScaleSetsResponse, error) {
	req, err := s.newActionsRequest(ctx, http.MethodGet, scaleSetEndpoint, nil)
	if err != nil {
		return nil, err
	}
	data, err := httputil.DumpRequest(req, false)
	if err == nil {
		fmt.Println(string(data))
	}
	resp, err := s.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list runner scale sets: %w", err)
	}

	var runnerScaleSetList params.RunnerScaleSetsResponse
	if err := json.NewDecoder(resp.Body).Decode(&runnerScaleSetList); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &runnerScaleSetList, nil
}

// CreateRunnerScaleSet creates a new runner scale set in the target GitHub entity.
func (s *ScaleSetClient) CreateRunnerScaleSet(ctx context.Context, runnerScaleSet *params.RunnerScaleSet) (params.RunnerScaleSet, error) {
	body, err := json.Marshal(runnerScaleSet)
	if err != nil {
		return params.RunnerScaleSet{}, err
	}

	req, err := s.newActionsRequest(ctx, http.MethodPost, scaleSetEndpoint, bytes.NewReader(body))
	if err != nil {
		return params.RunnerScaleSet{}, err
	}

	resp, err := s.Do(req)
	if err != nil {
		return params.RunnerScaleSet{}, fmt.Errorf("failed to create runner scale set: %w", err)
	}

	var createdRunnerScaleSet params.RunnerScaleSet
	if err := json.NewDecoder(resp.Body).Decode(&createdRunnerScaleSet); err != nil {
		return params.RunnerScaleSet{}, fmt.Errorf("failed to decode response: %w", err)
	}
	return createdRunnerScaleSet, nil
}

func (s *ScaleSetClient) UpdateRunnerScaleSet(ctx context.Context, runnerScaleSetId int, runnerScaleSet params.RunnerScaleSet) (params.RunnerScaleSet, error) {
	path := fmt.Sprintf("%s/%d", scaleSetEndpoint, runnerScaleSetId)

	body, err := json.Marshal(runnerScaleSet)
	if err != nil {
		return params.RunnerScaleSet{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := s.newActionsRequest(ctx, http.MethodPatch, path, bytes.NewReader(body))
	if err != nil {
		return params.RunnerScaleSet{}, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.Do(req)
	if err != nil {
		return params.RunnerScaleSet{}, fmt.Errorf("failed to make request: %w", err)
	}

	var ret params.RunnerScaleSet
	if err := json.NewDecoder(resp.Body).Decode(&ret); err != nil {
		return params.RunnerScaleSet{}, fmt.Errorf("failed to decode response: %w", err)
	}
	return ret, nil
}

func (s *ScaleSetClient) DeleteRunnerScaleSet(ctx context.Context, runnerScaleSetId int) error {
	path := fmt.Sprintf("%s/%d", scaleSetEndpoint, runnerScaleSetId)
	req, err := s.newActionsRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to delete scale set with code %d", resp.StatusCode)
	}

	resp.Body.Close()
	return nil
}

func (s *ScaleSetClient) GetRunnerGroupByName(ctx context.Context, runnerGroup string) (params.RunnerGroup, error) {
	path := fmt.Sprintf("_apis/runtime/runnergroups/?groupName=%s", runnerGroup)
	req, err := s.newActionsRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return params.RunnerGroup{}, err
	}

	resp, err := s.Do(req)
	if err != nil {
		return params.RunnerGroup{}, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	var runnerGroupList params.RunnerGroupList
	err = json.NewDecoder(resp.Body).Decode(&runnerGroupList)
	if err != nil {
		return params.RunnerGroup{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if runnerGroupList.Count == 0 {
		return params.RunnerGroup{}, runnerErrors.NewNotFoundError("runner group %s does not exist", runnerGroup)
	}

	if runnerGroupList.Count > 1 {
		return params.RunnerGroup{}, runnerErrors.NewConflictError("multiple runner groups exist with the same name (%s)", runnerGroup)
	}

	return runnerGroupList.RunnerGroups[0], nil
}
