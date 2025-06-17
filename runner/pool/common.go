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

package pool

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-github/v72/github"
	"github.com/pkg/errors"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/params"
)

func validateHookRequest(controllerID, baseURL string, allHooks []*github.Hook, req *github.Hook) error {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return errors.Wrap(err, "parsing webhook url")
	}

	partialMatches := []string{}
	for _, hook := range allHooks {
		hookURL := strings.ToLower(hook.Config.GetURL())
		if hookURL == "" {
			continue
		}

		if hook.Config.GetURL() == req.Config.GetURL() {
			return runnerErrors.NewConflictError("hook already installed")
		} else if strings.Contains(hookURL, controllerID) || strings.Contains(hookURL, parsed.Hostname()) {
			partialMatches = append(partialMatches, hook.Config.GetURL())
		}
	}

	if len(partialMatches) > 0 {
		return runnerErrors.NewConflictError("a webhook containing the controller ID or hostname of this contreoller is already installed on this repository")
	}

	return nil
}

func hookToParamsHookInfo(hook *github.Hook) params.HookInfo {
	hookURL := hook.Config.GetURL()

	insecureSSLConfig := hook.Config.GetInsecureSSL()
	insecureSSL := insecureSSLConfig == "1"

	return params.HookInfo{
		ID:          *hook.ID,
		URL:         hookURL,
		Events:      hook.Events,
		Active:      *hook.Active,
		InsecureSSL: insecureSSL,
	}
}

func (r *basePoolManager) listHooks(ctx context.Context) ([]*github.Hook, error) {
	opts := github.ListOptions{
		PerPage: 100,
	}
	var allHooks []*github.Hook
	for {
		hooks, ghResp, err := r.ghcli.ListEntityHooks(ctx, &opts)
		if err != nil {
			if ghResp != nil && ghResp.StatusCode == http.StatusNotFound {
				return nil, runnerErrors.NewBadRequestError("repository not found or your PAT does not have access to manage webhooks")
			}
			return nil, errors.Wrap(err, "fetching hooks")
		}
		allHooks = append(allHooks, hooks...)
		if ghResp.NextPage == 0 {
			break
		}
		opts.Page = ghResp.NextPage
	}
	return allHooks, nil
}
