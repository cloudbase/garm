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
	"time"

	"github.com/google/go-github/v84/github"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	internalErrors "github.com/cloudbase/garm/internal/errors"
	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

// requestTimeout bounds every scale set API call except the message queue
// long poll. Without it, a stalled endpoint hangs the request
// forever. Some call sites (like listener teardown) cannot rely on their
// context for cancellation.
const requestTimeout = 60 * time.Second

// longPollRequestTimeout bounds a single message queue long poll. The broker
// holds the request open for up to ~50 seconds before returning an empty
// response, so this must stay comfortably above that; it only exists so a
// black-holed connection cannot hold a poll open forever. GitHub's own
// runner polls the same broker with a 100 second client timeout.
const longPollRequestTimeout = 100 * time.Second

func NewClient(cli common.GithubClient) (*ScaleSetClient, error) {
	// Use separate clients for regular API calls against the scaleset API
	// and the scaleset long poll message queue. The long poll is held open
	// by the broker for up to ~50 seconds by design, so it cannot share the
	// tighter timeout every other call gets.
	return &ScaleSetClient{
		ghCli: cli,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
		longPollClient: &http.Client{
			Timeout: longPollRequestTimeout,
		},
	}, nil
}

type ScaleSetClient struct {
	ghCli          common.GithubClient
	httpClient     *http.Client
	longPollClient *http.Client

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

func (s *ScaleSetClient) endpointLabel() string {
	creds := s.ghCli.GetEntity().Credentials
	endpoint := creds.Endpoint.Name
	if endpoint == "" {
		endpoint = creds.BaseURL
	}
	return endpoint
}

func (s *ScaleSetClient) recordOperation(operation string) {
	metrics.GithubOperationCount.WithLabelValues(
		operation,
		s.ghCli.GetEntity().LabelScope(),
		s.endpointLabel(),
	).Inc()
}

func (s *ScaleSetClient) recordFailedOperation(operation string) {
	metrics.GithubOperationFailedCount.WithLabelValues(
		operation,
		s.ghCli.GetEntity().LabelScope(),
		s.endpointLabel(),
	).Inc()
}

func (s *ScaleSetClient) SetGithubClient(cli common.GithubClient) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.ghCli = cli
}

func (s *ScaleSetClient) GetGithubClient() (common.GithubClient, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.ghCli == nil {
		return nil, fmt.Errorf("github client is not set in scaleset client")
	}
	return s.ghCli, nil
}

func (s *ScaleSetClient) Do(req *http.Request) (*http.Response, error) {
	return s.doWithClient(s.httpClient, req)
}

// DoLongPoll dispatches a request through the long poll client, whose
// timeout accommodates the broker holding the request open for up to ~50
// seconds. Use it only for requests that legitimately block server side
// (the message queue long poll).
func (s *ScaleSetClient) DoLongPoll(req *http.Request) (*http.Response, error) {
	return s.doWithClient(s.longPollClient, req)
}

func (s *ScaleSetClient) doWithClient(client *http.Client, req *http.Request) (*http.Response, error) {
	if client == nil {
		return nil, fmt.Errorf("http client is not initialized")
	}

	resp, err := client.Do(req) //nolint:gosec // G704 - URL is constructed from GitHub API endpoints
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
	case 401:
		// The credentials were rejected outright. Keep the body: it is the only
		// thing that says whether the token expired, was revoked, or was never
		// valid for this resource.
		return nil, runnerErrors.NewUnauthorizedError(
			fmt.Sprintf("unauthorized while calling %s: %q", req.URL.String(), string(body)))
	case 403:
		// Not the same as 401. GitHub answers 403 for a secondary rate limit and
		// for SSO enforcement as well as for a genuine permission problem, so
		// collapsing it into ErrUnauthorized tells callers the credentials are
		// dead when they are usually fine and the refusal is temporary.
		return nil, internalErrors.NewForbiddenError(
			"forbidden while calling %s: %q", req.URL.String(), string(body))
	default:
		return nil, fmt.Errorf("request to %s failed with status code %d: %q", req.URL.String(), resp.StatusCode, string(body))
	}
}
