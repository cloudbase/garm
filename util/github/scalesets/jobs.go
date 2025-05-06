// Copyright 2024 Cloudbase Solutions SRL
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

package scalesets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cloudbase/garm/params"
)

type acquireJobsResult struct {
	Count int     `json:"count"`
	Value []int64 `json:"value"`
}

func (s *ScaleSetClient) AcquireJobs(ctx context.Context, runnerScaleSetID int, messageQueueAccessToken string, requestIDs []int64) ([]int64, error) {
	u := fmt.Sprintf("%s/%d/acquirejobs?api-version=6.0-preview", scaleSetEndpoint, runnerScaleSetID)

	body, err := json.Marshal(requestIDs)
	if err != nil {
		return nil, err
	}

	req, err := s.newActionsRequest(ctx, http.MethodPost, u, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", messageQueueAccessToken))

	resp, err := s.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed for %s: %w", req.URL.String(), err)
	}
	defer resp.Body.Close()

	var acquiredJobs acquireJobsResult
	err = json.NewDecoder(resp.Body).Decode(&acquiredJobs)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return acquiredJobs.Value, nil
}

func (s *ScaleSetClient) GetAcquirableJobs(ctx context.Context, runnerScaleSetID int) (params.AcquirableJobList, error) {
	path := fmt.Sprintf("%d/acquirablejobs", runnerScaleSetID)

	req, err := s.newActionsRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return params.AcquirableJobList{}, fmt.Errorf("failed to construct request: %w", err)
	}

	resp, err := s.Do(req)
	if err != nil {
		return params.AcquirableJobList{}, fmt.Errorf("request failed for %s: %w", req.URL.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return params.AcquirableJobList{Count: 0, Jobs: []params.AcquirableJob{}}, nil
	}

	var acquirableJobList params.AcquirableJobList
	err = json.NewDecoder(resp.Body).Decode(&acquirableJobList)
	if err != nil {
		return params.AcquirableJobList{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return acquirableJobList, nil
}
