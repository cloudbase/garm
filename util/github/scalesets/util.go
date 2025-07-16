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
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
)

func (s *ScaleSetClient) newActionsRequest(ctx context.Context, method, uriPath string, body io.Reader) (*http.Request, error) {
	if err := s.ensureAdminInfo(ctx); err != nil {
		return nil, fmt.Errorf("failed to update token: %w", err)
	}

	actionsURI, err := s.actionsServiceInfo.GetURL()
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline URL: %w", err)
	}

	pathURI, err := url.Parse(uriPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path: %w", err)
	}
	pathQuery := pathURI.Query()
	baseQuery := actionsURI.Query()
	for k, values := range pathQuery {
		if baseQuery.Get(k) == "" {
			for _, val := range values {
				baseQuery.Add(k, val)
			}
		}
	}
	if baseQuery.Get("api-version") == "" {
		baseQuery.Set("api-version", "6.0-preview")
	}

	actionsURI.Path = path.Join(actionsURI.Path, pathURI.Path)
	actionsURI.RawQuery = baseQuery.Encode()

	req, err := http.NewRequestWithContext(ctx, method, actionsURI.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.actionsServiceInfo.Token))

	return req, nil
}
