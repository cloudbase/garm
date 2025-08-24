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
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-github/v72/github"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/cache"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
)

func instanceInList(instanceName string, instances []commonParams.ProviderInstance) (commonParams.ProviderInstance, bool) {
	for _, val := range instances {
		if val.Name == instanceName {
			return val, true
		}
	}
	return commonParams.ProviderInstance{}, false
}

func controllerIDFromLabels(labels []string) string {
	for _, lbl := range labels {
		if strings.HasPrefix(lbl, controllerLabelPrefix) {
			trimLength := min(len(controllerLabelPrefix)+1, len(lbl))
			return lbl[trimLength:]
		}
	}
	return ""
}

func labelsFromRunner(runner forgeRunner) []string {
	if runner.Labels == nil {
		return []string{}
	}

	var labels []string
	for _, val := range runner.Labels {
		labels = append(labels, val.Name)
	}
	return labels
}

// isManagedRunner returns true if labels indicate the runner belongs to a pool
// this manager is responsible for.
func isManagedRunner(labels []string, controllerID string) bool {
	runnerControllerID := controllerIDFromLabels(labels)
	return runnerControllerID == controllerID
}

func composeWatcherFilters(entity params.ForgeEntity) dbCommon.PayloadFilterFunc {
	// We want to watch for changes in either the controller or the
	// entity itself.
	return watcher.WithAny(
		watcher.WithAll(
			// Updates to the controller
			watcher.WithEntityTypeFilter(dbCommon.ControllerEntityType),
			watcher.WithOperationTypeFilter(dbCommon.UpdateOperation),
		),
		// Any operation on the entity we're managing the pool for.
		watcher.WithEntityFilter(entity),
		// Watch for changes to the github credentials
		watcher.WithForgeCredentialsFilter(entity.Credentials),
	)
}

func (r *basePoolManager) waitForToolsOrCancel() (hasTools, stopped bool) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	select {
	case <-ticker.C:
		if _, err := cache.GetGithubToolsCache(r.entity.ID); err != nil {
			return false, false
		}
		return true, false
	case <-r.quit:
		return false, true
	case <-r.ctx.Done():
		return false, true
	}
}

func validateHookRequest(controllerID, baseURL string, allHooks []*github.Hook, req *github.Hook) error {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("error parsing webhook url: %w", err)
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
			return nil, fmt.Errorf("error fetching hooks: %w", err)
		}
		allHooks = append(allHooks, hooks...)
		if ghResp.NextPage == 0 {
			break
		}
		opts.Page = ghResp.NextPage
	}
	return allHooks, nil
}

func (r *basePoolManager) listRunnersWithPagination() ([]forgeRunner, error) {
	opts := github.ListRunnersOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	var allRunners []*github.Runner

	// Paginating like this can lead to a situation where if we have many pages of runners,
	// while we paginate, a particular runner can move from page n to page n-1 while we move
	// from page n-1 to page n. In situations such as that, we end up with a list of runners
	// that does not contain the runner that swapped pages while we were paginating.
	// Sadly, the GitHub API does not allow listing more than 100 runners per page.
	for {
		runners, ghResp, err := r.ghcli.ListEntityRunners(r.ctx, &opts)
		if err != nil {
			if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
				return nil, runnerErrors.NewUnauthorizedError("error fetching runners")
			}
			return nil, fmt.Errorf("error fetching runners: %w", err)
		}
		allRunners = append(allRunners, runners.Runners...)
		if ghResp.NextPage == 0 {
			break
		}
		opts.Page = ghResp.NextPage
	}

	ret := make([]forgeRunner, len(allRunners))
	for idx, val := range allRunners {
		ret[idx] = forgeRunner{
			ID:     val.GetID(),
			Name:   val.GetName(),
			Status: val.GetStatus(),
			Labels: make([]RunnerLabels, len(val.Labels)),
		}
		for labelIdx, label := range val.Labels {
			ret[idx].Labels[labelIdx] = RunnerLabels{
				Name: label.GetName(),
				Type: label.GetType(),
				ID:   label.GetID(),
			}
		}
	}

	return ret, nil
}

func (r *basePoolManager) listRunnersWithScaleSetAPI() ([]forgeRunner, error) {
	if r.scaleSetClient == nil {
		return nil, fmt.Errorf("scaleset client not initialized")
	}

	runners, err := r.scaleSetClient.ListAllRunners(r.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list runners through scaleset API: %w", err)
	}

	ret := []forgeRunner{}
	for _, runner := range runners.RunnerReferences {
		if runner.RunnerScaleSetID != 0 {
			// skip scale set runners.
			continue
		}
		run := forgeRunner{
			Name:   runner.Name,
			ID:     runner.ID,
			Status: string(runner.GetStatus()),
			Labels: make([]RunnerLabels, len(runner.Labels)),
		}
		for labelIDX, label := range runner.Labels {
			run.Labels[labelIDX] = RunnerLabels{
				Name: label.Name,
				Type: label.Type,
			}
		}
		ret = append(ret, run)
	}
	return ret, nil
}

func (r *basePoolManager) GetGithubRunners() ([]forgeRunner, error) {
	// Gitea has no scale sets API
	if r.scaleSetClient == nil {
		return r.listRunnersWithPagination()
	}

	// try the scale sets API for github
	runners, err := r.listRunnersWithScaleSetAPI()
	if err != nil {
		slog.WarnContext(r.ctx, "failed to list runners via scaleset API; falling back to pagination", "error", err)
		return r.listRunnersWithPagination()
	}

	entityInstances := cache.GetEntityInstances(r.entity.ID)
	if len(entityInstances) > 0 && len(runners) == 0 {
		// I have trust issues in the undocumented API. We seem to have runners for this
		// entity, but the scaleset API returned nothing and no error. Fall back to pagination.
		slog.DebugContext(r.ctx, "the scaleset api returned nothing, but we seem to have runners in the db; falling back to paginated API runner list")
		return r.listRunnersWithPagination()
	}
	slog.DebugContext(r.ctx, "Scaleset API runner list succeeded", "runners", runners)
	return runners, nil
}
