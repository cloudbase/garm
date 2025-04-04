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
	"io"
	"net/http"
	"time"

	"github.com/cloudbase/garm/params"
)

func (s *ScaleSetClient) getActionServiceInfo(ctx context.Context) (params.ActionsServiceAdminInfoResponse, error) {
	regPath := "/actions/runner-registration"
	baseURL := s.ghCli.GithubBaseURL()
	url, err := baseURL.Parse(regPath)
	if err != nil {
		return params.ActionsServiceAdminInfoResponse{}, fmt.Errorf("failed to parse url: %w", err)
	}

	entity := s.ghCli.GetEntity()
	body := params.ActionsServiceAdminInfoRequest{
		URL:         entity.GithubURL(),
		RunnerEvent: "register",
	}

	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(body); err != nil {
		return params.ActionsServiceAdminInfoResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), buf)
	if err != nil {
		return params.ActionsServiceAdminInfoResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("RemoteAuth %s", *s.runnerRegistrationToken.Token))

	resp, err := s.Do(req)
	if err != nil {
		return params.ActionsServiceAdminInfoResponse{}, fmt.Errorf("failed to get actions service admin info: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return params.ActionsServiceAdminInfoResponse{}, fmt.Errorf("failed to read response body: %w", err)
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	var info params.ActionsServiceAdminInfoResponse
	if err := json.Unmarshal(data, &info); err != nil {
		return params.ActionsServiceAdminInfoResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return info, nil
}

func (s *ScaleSetClient) ensureAdminInfo(ctx context.Context) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	var expiresAt time.Time
	if s.runnerRegistrationToken != nil {
		expiresAt = s.runnerRegistrationToken.GetExpiresAt().Time
	}

	now := time.Now().UTC().Add(2 * time.Minute)
	if now.After(expiresAt) || s.runnerRegistrationToken == nil {
		token, _, err := s.ghCli.CreateEntityRegistrationToken(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch runner registration token: %w", err)
		}
		s.runnerRegistrationToken = token
	}

	if s.actionsServiceInfo == nil || s.actionsServiceInfo.ExpiresIn(2*time.Minute) {
		info, err := s.getActionServiceInfo(ctx)
		if err != nil {
			return fmt.Errorf("failed to get action service info: %w", err)
		}
		s.actionsServiceInfo = &info
	}

	return nil
}
