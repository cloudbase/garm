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

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
)

type scaleSetJitRunnerConfig struct {
	Name       string `json:"name"`
	WorkFolder string `json:"workFolder"`
}

func (s *ScaleSetClient) GenerateJitRunnerConfig(ctx context.Context, runnerName string, scaleSet params.RunnerScaleSet) (params.RunnerScaleSetJitRunnerConfig, error) {
	runnerSettings := scaleSetJitRunnerConfig{
		Name:       runnerName,
		WorkFolder: "_work",
	}

	body, err := json.Marshal(runnerSettings)
	if err != nil {
		return params.RunnerScaleSetJitRunnerConfig{}, err
	}

	req, err := s.newActionsRequest(ctx, http.MethodPost, scaleSet.RunnerJitConfigUrl, bytes.NewBuffer(body))
	if err != nil {
		return params.RunnerScaleSetJitRunnerConfig{}, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.Do(req)
	if err != nil {
		return params.RunnerScaleSetJitRunnerConfig{}, fmt.Errorf("request failed for %s: %w", req.URL.String(), err)
	}
	defer resp.Body.Close()

	var runnerJitConfig params.RunnerScaleSetJitRunnerConfig
	if err := json.NewDecoder(resp.Body).Decode(&runnerJitConfig); err != nil {
		return params.RunnerScaleSetJitRunnerConfig{}, fmt.Errorf("failed to decode response: %w", err)
	}
	return runnerJitConfig, nil
}

func (s *ScaleSetClient) GetRunner(ctx context.Context, runnerId int64) (params.RunnerReference, error) {
	path := fmt.Sprintf("%s/%d", runnerEndpoint, runnerId)

	req, err := s.newActionsRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return params.RunnerReference{}, fmt.Errorf("failed to construct request: %w", err)
	}

	resp, err := s.Do(req)
	if err != nil {
		return params.RunnerReference{}, fmt.Errorf("request failed for %s: %w", req.URL.String(), err)
	}
	defer resp.Body.Close()

	var runnerReference params.RunnerReference
	if err := json.NewDecoder(resp.Body).Decode(&runnerReference); err != nil {
		return params.RunnerReference{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return runnerReference, nil
}

func (s *ScaleSetClient) GetRunnerByName(ctx context.Context, runnerName string) (params.RunnerReference, error) {
	path := fmt.Sprintf("%s?agentName=%s", runnerEndpoint, runnerName)

	req, err := s.newActionsRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return params.RunnerReference{}, fmt.Errorf("failed to construct request: %w", err)
	}

	resp, err := s.Do(req)
	if err != nil {
		return params.RunnerReference{}, fmt.Errorf("request failed for %s: %w", req.URL.String(), err)
	}
	defer resp.Body.Close()

	var runnerList params.RunnerReferenceList
	if err := json.NewDecoder(resp.Body).Decode(&runnerList); err != nil {
		return params.RunnerReference{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if runnerList.Count == 0 {
		return params.RunnerReference{}, fmt.Errorf("could not find runner with name %q: %w", runnerName, runnerErrors.ErrNotFound)
	}

	if runnerList.Count > 1 {
		return params.RunnerReference{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return runnerList.RunnerReferences[0], nil
}

func (s *ScaleSetClient) RemoveRunner(ctx context.Context, runnerId int64) error {
	path := fmt.Sprintf("%s/%d", runnerEndpoint, runnerId)

	req, err := s.newActionsRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("failed to construct request: %w", err)
	}

	resp, err := s.Do(req)
	if err != nil {
		return fmt.Errorf("request failed for %s: %w", req.URL.String(), err)
	}

	resp.Body.Close()
	return nil
}
