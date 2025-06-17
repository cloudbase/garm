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
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/google/go-github/v72/github"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

func NewClient(cli common.GithubClient) (*ScaleSetClient, error) {
	return &ScaleSetClient{
		ghCli:      cli,
		httpClient: &http.Client{},
	}, nil
}

type ScaleSetClient struct {
	ghCli      common.GithubClient
	httpClient *http.Client

	// scale sets are aparently available through the same security
	// contex that a normal runner would use. We connect to the same
	// API endpoint a runner would connect to, in order to fetch jobs.
	// To do this, we use a runner registration token.
	runnerRegistrationToken *github.RegistrationToken
	// actionsServiceInfo holds the pipeline URL and the JWT token to
	// access it. The pipeline URL is the base URL where we can access
	// the scale set endpoints.
	actionsServiceInfo *params.ActionsServiceAdminInfoResponse

	mux sync.Mutex
}

func (s *ScaleSetClient) SetGithubClient(cli common.GithubClient) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.ghCli = cli
}

func (s *ScaleSetClient) Do(req *http.Request) (*http.Response, error) {
	if s.httpClient == nil {
		return nil, fmt.Errorf("http client is not initialized")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to dispatch HTTP request: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}

	var body []byte
	if resp != nil {
		defer resp.Body.Close()
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}
	}

	switch resp.StatusCode {
	case 404:
		return nil, runnerErrors.NewNotFoundError("resource %s not found: %q", req.URL.String(), string(body))
	case 400:
		return nil, runnerErrors.NewBadRequestError("bad request while calling %s: %q", req.URL.String(), string(body))
	case 409:
		return nil, runnerErrors.NewConflictError("conflict while calling %s: %q", req.URL.String(), string(body))
	case 401, 403:
		return nil, runnerErrors.ErrUnauthorized
	default:
		return nil, fmt.Errorf("request to %s failed with status code %d: %q", req.URL.String(), resp.StatusCode, string(body))
	}
}
