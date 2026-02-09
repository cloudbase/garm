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

package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-github/v72/github"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

type githubClient struct {
	*github.ActionsService
	org        *github.OrganizationsService
	repo       *github.RepositoriesService
	enterprise *github.EnterpriseService
	rateLimit  *github.RateLimitService

	entity params.ForgeEntity
	cli    *github.Client
}

func (g *githubClient) ListEntityHooks(ctx context.Context, opts *github.ListOptions) (ret []*github.Hook, response *github.Response, err error) {
	metrics.GithubOperationCount.WithLabelValues(
		"ListHooks",           // label: operation
		g.entity.LabelScope(), // label: scope
	).Inc()
	defer func() {
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"ListHooks",           // label: operation
				g.entity.LabelScope(), // label: scope
			).Inc()
		}
	}()
	switch g.entity.EntityType {
	case params.ForgeEntityTypeRepository:
		ret, response, err = g.repo.ListHooks(ctx, g.entity.Owner, g.entity.Name, opts)
	case params.ForgeEntityTypeOrganization:
		ret, response, err = g.org.ListHooks(ctx, g.entity.Owner, opts)
	default:
		return nil, nil, fmt.Errorf("invalid entity type: %s", g.entity.EntityType)
	}
	return ret, response, err
}

func (g *githubClient) GetEntityHook(ctx context.Context, id int64) (ret *github.Hook, err error) {
	metrics.GithubOperationCount.WithLabelValues(
		"GetHook",             // label: operation
		g.entity.LabelScope(), // label: scope
	).Inc()
	defer func() {
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"GetHook",             // label: operation
				g.entity.LabelScope(), // label: scope
			).Inc()
		}
	}()
	switch g.entity.EntityType {
	case params.ForgeEntityTypeRepository:
		ret, _, err = g.repo.GetHook(ctx, g.entity.Owner, g.entity.Name, id)
	case params.ForgeEntityTypeOrganization:
		ret, _, err = g.org.GetHook(ctx, g.entity.Owner, id)
	default:
		return nil, errors.New("invalid entity type")
	}
	return ret, err
}

func (g *githubClient) createGithubEntityHook(ctx context.Context, hook *github.Hook) (ret *github.Hook, err error) {
	metrics.GithubOperationCount.WithLabelValues(
		"CreateHook",          // label: operation
		g.entity.LabelScope(), // label: scope
	).Inc()
	defer func() {
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"CreateHook",          // label: operation
				g.entity.LabelScope(), // label: scope
			).Inc()
		}
	}()
	switch g.entity.EntityType {
	case params.ForgeEntityTypeRepository:
		ret, _, err = g.repo.CreateHook(ctx, g.entity.Owner, g.entity.Name, hook)
	case params.ForgeEntityTypeOrganization:
		ret, _, err = g.org.CreateHook(ctx, g.entity.Owner, hook)
	default:
		return nil, errors.New("invalid entity type")
	}
	return ret, err
}

func (g *githubClient) CreateEntityHook(ctx context.Context, hook *github.Hook) (ret *github.Hook, err error) {
	switch g.entity.Credentials.ForgeType {
	case params.GithubEndpointType:
		return g.createGithubEntityHook(ctx, hook)
	case params.GiteaEndpointType:
		return g.createGiteaEntityHook(ctx, hook)
	default:
		return nil, errors.New("invalid entity type")
	}
}

func (g *githubClient) DeleteEntityHook(ctx context.Context, id int64) (ret *github.Response, err error) {
	metrics.GithubOperationCount.WithLabelValues(
		"DeleteHook",          // label: operation
		g.entity.LabelScope(), // label: scope
	).Inc()
	defer func() {
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"DeleteHook",          // label: operation
				g.entity.LabelScope(), // label: scope
			).Inc()
		}
	}()
	switch g.entity.EntityType {
	case params.ForgeEntityTypeRepository:
		ret, err = g.repo.DeleteHook(ctx, g.entity.Owner, g.entity.Name, id)
	case params.ForgeEntityTypeOrganization:
		ret, err = g.org.DeleteHook(ctx, g.entity.Owner, id)
	default:
		return nil, errors.New("invalid entity type")
	}
	return ret, err
}

func (g *githubClient) PingEntityHook(ctx context.Context, id int64) (ret *github.Response, err error) {
	metrics.GithubOperationCount.WithLabelValues(
		"PingHook",            // label: operation
		g.entity.LabelScope(), // label: scope
	).Inc()
	defer func() {
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"PingHook",            // label: operation
				g.entity.LabelScope(), // label: scope
			).Inc()
		}
	}()
	switch g.entity.EntityType {
	case params.ForgeEntityTypeRepository:
		ret, err = g.repo.PingHook(ctx, g.entity.Owner, g.entity.Name, id)
	case params.ForgeEntityTypeOrganization:
		ret, err = g.org.PingHook(ctx, g.entity.Owner, id)
	default:
		return nil, errors.New("invalid entity type")
	}
	return ret, err
}

func (g *githubClient) ListEntityRunners(ctx context.Context, opts *github.ListRunnersOptions) (*github.Runners, *github.Response, error) {
	var ret *github.Runners
	var response *github.Response
	var err error

	metrics.GithubOperationCount.WithLabelValues(
		"ListEntityRunners",   // label: operation
		g.entity.LabelScope(), // label: scope
	).Inc()
	defer func() {
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"ListEntityRunners",   // label: operation
				g.entity.LabelScope(), // label: scope
			).Inc()
		}
	}()

	switch g.entity.EntityType {
	case params.ForgeEntityTypeRepository:
		ret, response, err = g.ListRunners(ctx, g.entity.Owner, g.entity.Name, opts)
	case params.ForgeEntityTypeOrganization:
		ret, response, err = g.ListOrganizationRunners(ctx, g.entity.Owner, opts)
	case params.ForgeEntityTypeEnterprise:
		ret, response, err = g.enterprise.ListRunners(ctx, g.entity.Owner, opts)
	default:
		return nil, nil, errors.New("invalid entity type")
	}

	return ret, response, err
}

func (g *githubClient) ListEntityRunnerApplicationDownloads(ctx context.Context) ([]*github.RunnerApplicationDownload, *github.Response, error) {
	var ret []*github.RunnerApplicationDownload
	var response *github.Response
	var err error

	metrics.GithubOperationCount.WithLabelValues(
		"ListEntityRunnerApplicationDownloads", // label: operation
		g.entity.LabelScope(),                  // label: scope
	).Inc()
	defer func() {
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"ListEntityRunnerApplicationDownloads", // label: operation
				g.entity.LabelScope(),                  // label: scope
			).Inc()
		}
	}()

	switch g.entity.EntityType {
	case params.ForgeEntityTypeRepository:
		ret, response, err = g.ListRunnerApplicationDownloads(ctx, g.entity.Owner, g.entity.Name)
	case params.ForgeEntityTypeOrganization:
		ret, response, err = g.ListOrganizationRunnerApplicationDownloads(ctx, g.entity.Owner)
	case params.ForgeEntityTypeEnterprise:
		ret, response, err = g.enterprise.ListRunnerApplicationDownloads(ctx, g.entity.Owner)
	default:
		return nil, nil, errors.New("invalid entity type")
	}

	return ret, response, err
}

func parseError(response *github.Response, err error) error {
	var statusCode int
	if response != nil {
		statusCode = response.StatusCode
	}

	switch statusCode {
	case http.StatusNotFound:
		return runnerErrors.ErrNotFound
	case http.StatusUnauthorized:
		return runnerErrors.ErrUnauthorized
	case http.StatusUnprocessableEntity:
		return runnerErrors.ErrBadRequest
	default:
		if statusCode >= 100 && statusCode < 300 {
			return nil
		}
		if err != nil {
			errResp := &github.ErrorResponse{}
			if errors.As(err, &errResp) && errResp.Response != nil {
				switch errResp.Response.StatusCode {
				case http.StatusNotFound:
					return runnerErrors.ErrNotFound
				case http.StatusUnauthorized:
					return runnerErrors.ErrUnauthorized
				case http.StatusUnprocessableEntity:
					return runnerErrors.ErrBadRequest
				default:
					// ugly hack. Gitea returns 500 if we try to remove a runner that does not exist.
					if strings.Contains(err.Error(), "does not exist") {
						return runnerErrors.ErrNotFound
					}
					return err
				}
			}
			return err
		}
		return errors.New("unknown error")
	}
}

func (g *githubClient) RemoveEntityRunner(ctx context.Context, runnerID int64) error {
	var response *github.Response
	var err error

	metrics.GithubOperationCount.WithLabelValues(
		"RemoveEntityRunner",  // label: operation
		g.entity.LabelScope(), // label: scope
	).Inc()
	defer func() {
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"RemoveEntityRunner",  // label: operation
				g.entity.LabelScope(), // label: scope
			).Inc()
		}
	}()

	switch g.entity.EntityType {
	case params.ForgeEntityTypeRepository:
		response, err = g.RemoveRunner(ctx, g.entity.Owner, g.entity.Name, runnerID)
	case params.ForgeEntityTypeOrganization:
		response, err = g.RemoveOrganizationRunner(ctx, g.entity.Owner, runnerID)
	case params.ForgeEntityTypeEnterprise:
		response, err = g.enterprise.RemoveRunner(ctx, g.entity.Owner, runnerID)
	default:
		return errors.New("invalid entity type")
	}

	if err := parseError(response, err); err != nil {
		return fmt.Errorf("error removing runner %d: %w", runnerID, err)
	}

	return nil
}

func (g *githubClient) CreateEntityRegistrationToken(ctx context.Context) (*github.RegistrationToken, *github.Response, error) {
	var ret *github.RegistrationToken
	var response *github.Response
	var err error

	metrics.GithubOperationCount.WithLabelValues(
		"CreateEntityRegistrationToken", // label: operation
		g.entity.LabelScope(),           // label: scope
	).Inc()
	defer func() {
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"CreateEntityRegistrationToken", // label: operation
				g.entity.LabelScope(),           // label: scope
			).Inc()
		}
	}()

	switch g.entity.EntityType {
	case params.ForgeEntityTypeRepository:
		ret, response, err = g.CreateRegistrationToken(ctx, g.entity.Owner, g.entity.Name)
	case params.ForgeEntityTypeOrganization:
		ret, response, err = g.CreateOrganizationRegistrationToken(ctx, g.entity.Owner)
	case params.ForgeEntityTypeEnterprise:
		ret, response, err = g.enterprise.CreateRegistrationToken(ctx, g.entity.Owner)
	default:
		return nil, nil, errors.New("invalid entity type")
	}

	return ret, response, err
}

func (g *githubClient) getOrganizationRunnerGroupIDByName(ctx context.Context, entity params.ForgeEntity, rgName string) (int64, error) {
	opts := github.ListOrgRunnerGroupOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		metrics.GithubOperationCount.WithLabelValues(
			"ListOrganizationRunnerGroups", // label: operation
			entity.LabelScope(),            // label: scope
		).Inc()
		runnerGroups, ghResp, err := g.ListOrganizationRunnerGroups(ctx, entity.Owner, &opts)
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"ListOrganizationRunnerGroups", // label: operation
				entity.LabelScope(),            // label: scope
			).Inc()
			if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
				return 0, fmt.Errorf("error fetching runners: %w", runnerErrors.ErrUnauthorized)
			}
			return 0, fmt.Errorf("error fetching runners: %w", err)
		}
		for _, runnerGroup := range runnerGroups.RunnerGroups {
			if runnerGroup.Name != nil && *runnerGroup.Name == rgName {
				return *runnerGroup.ID, nil
			}
		}
		if ghResp.NextPage == 0 {
			break
		}
		opts.Page = ghResp.NextPage
	}
	return 0, runnerErrors.NewNotFoundError("runner group %s not found", rgName)
}

func (g *githubClient) getEnterpriseRunnerGroupIDByName(ctx context.Context, entity params.ForgeEntity, rgName string) (int64, error) {
	opts := github.ListEnterpriseRunnerGroupOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		metrics.GithubOperationCount.WithLabelValues(
			"ListRunnerGroups",  // label: operation
			entity.LabelScope(), // label: scope
		).Inc()
		runnerGroups, ghResp, err := g.enterprise.ListRunnerGroups(ctx, entity.Owner, &opts)
		if err != nil {
			metrics.GithubOperationFailedCount.WithLabelValues(
				"ListRunnerGroups",  // label: operation
				entity.LabelScope(), // label: scope
			).Inc()
			if ghResp != nil && ghResp.StatusCode == http.StatusUnauthorized {
				return 0, fmt.Errorf("error fetching runners: %w", runnerErrors.ErrUnauthorized)
			}
			return 0, fmt.Errorf("error fetching runners: %w", err)
		}
		for _, runnerGroup := range runnerGroups.RunnerGroups {
			if runnerGroup.Name != nil && *runnerGroup.Name == rgName {
				return *runnerGroup.ID, nil
			}
		}
		if ghResp.NextPage == 0 {
			break
		}
		opts.Page = ghResp.NextPage
	}
	return 0, runnerErrors.NewNotFoundError("runner group not found")
}

func (g *githubClient) GetEntityRunnerGroupIDByName(ctx context.Context, runnerGroupName string) (int64, error) {
	var rgID int64 = 1

	if g.entity.EntityType == params.ForgeEntityTypeRepository {
		// This is a repository. Runner groups are supported at the org and
		// enterprise levels. Return the default runner group id, early.
		return rgID, nil
	}

	var ok bool
	var err error
	// attempt to get the runner group ID from cache. Cache will invalidate after 1 hour.
	if runnerGroupName != "" && !strings.EqualFold(runnerGroupName, "default") {
		rgID, ok = cache.GetEntityRunnerGroup(g.entity.ID, runnerGroupName)
		if !ok || rgID == 0 {
			switch g.entity.EntityType {
			case params.ForgeEntityTypeOrganization:
				rgID, err = g.getOrganizationRunnerGroupIDByName(ctx, g.entity, runnerGroupName)
			case params.ForgeEntityTypeEnterprise:
				rgID, err = g.getEnterpriseRunnerGroupIDByName(ctx, g.entity, runnerGroupName)
			}

			if err != nil {
				return 0, fmt.Errorf("getting runner group ID: %w", err)
			}
		}
		// set cache. Avoid getting the same runner group for more than once an hour.
		cache.SetEntityRunnerGroup(g.entity.ID, runnerGroupName, rgID)
	}
	return rgID, nil
}

func (g *githubClient) GetEntityJITConfig(ctx context.Context, instance string, pool params.Pool, labels []string) (jitConfigMap map[string]string, runner *github.Runner, err error) {
	rgID, err := g.GetEntityRunnerGroupIDByName(ctx, pool.GitHubRunnerGroup)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get runner group: %w", err)
	}
	slog.DebugContext(ctx, "using runner group", "group_name", pool.GitHubRunnerGroup, "runner_group_id", rgID)
	req := github.GenerateJITConfigRequest{
		Name:          instance,
		RunnerGroupID: rgID,
		Labels:        labels,
		// nolint:golangci-lint,godox
		// TODO(gabriel-samfira): Should we make this configurable?
		WorkFolder: github.Ptr("_work"),
	}

	metrics.GithubOperationCount.WithLabelValues(
		"GetEntityJITConfig",  // label: operation
		g.entity.LabelScope(), // label: scope
	).Inc()

	var ret *github.JITRunnerConfig
	var response *github.Response

	switch g.entity.EntityType {
	case params.ForgeEntityTypeRepository:
		ret, response, err = g.GenerateRepoJITConfig(ctx, g.entity.Owner, g.entity.Name, &req)
	case params.ForgeEntityTypeOrganization:
		ret, response, err = g.GenerateOrgJITConfig(ctx, g.entity.Owner, &req)
	case params.ForgeEntityTypeEnterprise:
		ret, response, err = g.enterprise.GenerateEnterpriseJITConfig(ctx, g.entity.Owner, &req)
	}
	if err != nil {
		metrics.GithubOperationFailedCount.WithLabelValues(
			"GetEntityJITConfig",  // label: operation
			g.entity.LabelScope(), // label: scope
		).Inc()
		if response != nil && response.StatusCode == http.StatusUnauthorized {
			return nil, nil, fmt.Errorf("failed to get JIT config: %w", err)
		}
		return nil, nil, fmt.Errorf("failed to get JIT config: %w", err)
	}

	defer func(run *github.Runner) {
		if err != nil && run != nil {
			innerErr := g.RemoveEntityRunner(ctx, run.GetID())
			slog.With(slog.Any("error", innerErr)).ErrorContext(
				ctx, "failed to remove runner",
				"runner_id", run.GetID(), string(g.entity.EntityType), g.entity.String())
		}
	}(ret.Runner)

	decoded, err := base64.StdEncoding.DecodeString(*ret.EncodedJITConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode JIT config: %w", err)
	}

	var jitConfig map[string]string
	if err := json.Unmarshal(decoded, &jitConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal JIT config: %w", err)
	}

	return jitConfig, ret.Runner, nil
}

func (g *githubClient) RateLimit(ctx context.Context) (*github.RateLimits, error) {
	limits, resp, err := g.rateLimit.Get(ctx)
	if err != nil {
		metrics.GithubOperationFailedCount.WithLabelValues(
			"GetRateLimit",        // label: operation
			g.entity.LabelScope(), // label: scope
		).Inc()
	}
	if err := parseError(resp, err); err != nil {
		return nil, fmt.Errorf("getting rate limit: %w", err)
	}
	return limits, nil
}

func (g *githubClient) GetEntity() params.ForgeEntity {
	return g.entity
}

func (g *githubClient) GithubBaseURL() *url.URL {
	return g.cli.BaseURL
}

func NewRateLimitClient(ctx context.Context, credentials params.ForgeCredentials) (common.RateLimitClient, error) {
	httpClient, err := credentials.GetHTTPClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching http client: %w", err)
	}

	slog.DebugContext(
		ctx, "creating rate limit client",
		"base_url", credentials.APIBaseURL,
		"upload_url", credentials.UploadBaseURL)

	ghClient, err := github.NewClient(httpClient).WithEnterpriseURLs(
		credentials.APIBaseURL, credentials.UploadBaseURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching github client: %w", err)
	}
	cli := &githubClient{
		rateLimit: ghClient.RateLimit,
		cli:       ghClient,
	}

	return cli, nil
}

func withGiteaURLs(client *github.Client, apiBaseURL string) (*github.Client, error) {
	if client == nil {
		return nil, errors.New("client is nil")
	}

	if apiBaseURL == "" {
		return nil, errors.New("invalid gitea URLs")
	}

	parsedBaseURL, err := url.ParseRequestURI(apiBaseURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing gitea base URL: %w", err)
	}

	if !strings.HasSuffix(parsedBaseURL.Path, "/") {
		parsedBaseURL.Path += "/"
	}

	if !strings.HasSuffix(parsedBaseURL.Path, "/api/v1/") {
		parsedBaseURL.Path += "api/v1/"
	}

	client.BaseURL = parsedBaseURL
	client.UploadURL = parsedBaseURL

	return client, nil
}

func Client(ctx context.Context, entity params.ForgeEntity) (common.GithubClient, error) {
	httpClient, err := entity.Credentials.GetHTTPClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching http client: %w", err)
	}

	slog.DebugContext(
		ctx, "creating client for entity",
		"entity", entity.String(), "base_url", entity.Credentials.APIBaseURL,
		"upload_url", entity.Credentials.UploadBaseURL)

	ghClient := github.NewClient(httpClient)
	switch entity.Credentials.ForgeType {
	case params.GithubEndpointType:
		ghClient, err = ghClient.WithEnterpriseURLs(entity.Credentials.APIBaseURL, entity.Credentials.UploadBaseURL)
	case params.GiteaEndpointType:
		ghClient, err = withGiteaURLs(ghClient, entity.Credentials.APIBaseURL)
	}

	if err != nil {
		return nil, fmt.Errorf("error fetching github client: %w", err)
	}

	cli := &githubClient{
		ActionsService: ghClient.Actions,
		org:            ghClient.Organizations,
		repo:           ghClient.Repositories,
		enterprise:     ghClient.Enterprise,
		rateLimit:      ghClient.RateLimit,
		cli:            ghClient,
		entity:         entity,
	}

	return cli, nil
}
