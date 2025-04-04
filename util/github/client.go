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
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/google/go-github/v57/github"
	"github.com/pkg/errors"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

type githubClient struct {
	*github.ActionsService
	org        *github.OrganizationsService
	repo       *github.RepositoriesService
	enterprise *github.EnterpriseService

	entity params.GithubEntity
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
	case params.GithubEntityTypeRepository:
		ret, response, err = g.repo.ListHooks(ctx, g.entity.Owner, g.entity.Name, opts)
	case params.GithubEntityTypeOrganization:
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
	case params.GithubEntityTypeRepository:
		ret, _, err = g.repo.GetHook(ctx, g.entity.Owner, g.entity.Name, id)
	case params.GithubEntityTypeOrganization:
		ret, _, err = g.org.GetHook(ctx, g.entity.Owner, id)
	default:
		return nil, errors.New("invalid entity type")
	}
	return ret, err
}

func (g *githubClient) CreateEntityHook(ctx context.Context, hook *github.Hook) (ret *github.Hook, err error) {
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
	case params.GithubEntityTypeRepository:
		ret, _, err = g.repo.CreateHook(ctx, g.entity.Owner, g.entity.Name, hook)
	case params.GithubEntityTypeOrganization:
		ret, _, err = g.org.CreateHook(ctx, g.entity.Owner, hook)
	default:
		return nil, errors.New("invalid entity type")
	}
	return ret, err
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
	case params.GithubEntityTypeRepository:
		ret, err = g.repo.DeleteHook(ctx, g.entity.Owner, g.entity.Name, id)
	case params.GithubEntityTypeOrganization:
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
	case params.GithubEntityTypeRepository:
		ret, err = g.repo.PingHook(ctx, g.entity.Owner, g.entity.Name, id)
	case params.GithubEntityTypeOrganization:
		ret, err = g.org.PingHook(ctx, g.entity.Owner, id)
	default:
		return nil, errors.New("invalid entity type")
	}
	return ret, err
}

func (g *githubClient) ListEntityRunners(ctx context.Context, opts *github.ListOptions) (*github.Runners, *github.Response, error) {
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
	case params.GithubEntityTypeRepository:
		ret, response, err = g.ListRunners(ctx, g.entity.Owner, g.entity.Name, opts)
	case params.GithubEntityTypeOrganization:
		ret, response, err = g.ListOrganizationRunners(ctx, g.entity.Owner, opts)
	case params.GithubEntityTypeEnterprise:
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
	case params.GithubEntityTypeRepository:
		ret, response, err = g.ListRunnerApplicationDownloads(ctx, g.entity.Owner, g.entity.Name)
	case params.GithubEntityTypeOrganization:
		ret, response, err = g.ListOrganizationRunnerApplicationDownloads(ctx, g.entity.Owner)
	case params.GithubEntityTypeEnterprise:
		ret, response, err = g.enterprise.ListRunnerApplicationDownloads(ctx, g.entity.Owner)
	default:
		return nil, nil, errors.New("invalid entity type")
	}

	return ret, response, err
}

func (g *githubClient) RemoveEntityRunner(ctx context.Context, runnerID int64) (*github.Response, error) {
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
	case params.GithubEntityTypeRepository:
		response, err = g.RemoveRunner(ctx, g.entity.Owner, g.entity.Name, runnerID)
	case params.GithubEntityTypeOrganization:
		response, err = g.RemoveOrganizationRunner(ctx, g.entity.Owner, runnerID)
	case params.GithubEntityTypeEnterprise:
		response, err = g.enterprise.RemoveRunner(ctx, g.entity.Owner, runnerID)
	default:
		return nil, errors.New("invalid entity type")
	}

	return response, err
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
	case params.GithubEntityTypeRepository:
		ret, response, err = g.CreateRegistrationToken(ctx, g.entity.Owner, g.entity.Name)
	case params.GithubEntityTypeOrganization:
		ret, response, err = g.CreateOrganizationRegistrationToken(ctx, g.entity.Owner)
	case params.GithubEntityTypeEnterprise:
		ret, response, err = g.enterprise.CreateRegistrationToken(ctx, g.entity.Owner)
	default:
		return nil, nil, errors.New("invalid entity type")
	}

	return ret, response, err
}

func (g *githubClient) getOrganizationRunnerGroupIDByName(ctx context.Context, entity params.GithubEntity, rgName string) (int64, error) {
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
				return 0, errors.Wrap(runnerErrors.ErrUnauthorized, "fetching runners")
			}
			return 0, errors.Wrap(err, "fetching runners")
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

func (g *githubClient) getEnterpriseRunnerGroupIDByName(ctx context.Context, entity params.GithubEntity, rgName string) (int64, error) {
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
				return 0, errors.Wrap(runnerErrors.ErrUnauthorized, "fetching runners")
			}
			return 0, errors.Wrap(err, "fetching runners")
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

func (g *githubClient) GetEntityJITConfig(ctx context.Context, instance string, pool params.Pool, labels []string) (jitConfigMap map[string]string, runner *github.Runner, err error) {
	// If no runner group is set, use the default runner group ID. This is also the default for
	// repository level runners.
	var rgID int64 = 1

	if pool.GitHubRunnerGroup != "" {
		switch g.entity.EntityType {
		case params.GithubEntityTypeOrganization:
			rgID, err = g.getOrganizationRunnerGroupIDByName(ctx, g.entity, pool.GitHubRunnerGroup)
		case params.GithubEntityTypeEnterprise:
			rgID, err = g.getEnterpriseRunnerGroupIDByName(ctx, g.entity, pool.GitHubRunnerGroup)
		}

		if err != nil {
			return nil, nil, fmt.Errorf("getting runner group ID: %w", err)
		}
	}

	req := github.GenerateJITConfigRequest{
		Name:          instance,
		RunnerGroupID: rgID,
		Labels:        labels,
		// nolint:golangci-lint,godox
		// TODO(gabriel-samfira): Should we make this configurable?
		WorkFolder: github.String("_work"),
	}

	metrics.GithubOperationCount.WithLabelValues(
		"GetEntityJITConfig",  // label: operation
		g.entity.LabelScope(), // label: scope
	).Inc()

	var ret *github.JITRunnerConfig
	var response *github.Response

	switch g.entity.EntityType {
	case params.GithubEntityTypeRepository:
		ret, response, err = g.GenerateRepoJITConfig(ctx, g.entity.Owner, g.entity.Name, &req)
	case params.GithubEntityTypeOrganization:
		ret, response, err = g.GenerateOrgJITConfig(ctx, g.entity.Owner, &req)
	case params.GithubEntityTypeEnterprise:
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
			_, innerErr := g.RemoveEntityRunner(ctx, run.GetID())
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

func (g *githubClient) GetEntity() params.GithubEntity {
	return g.entity
}

func (g *githubClient) GithubBaseURL() *url.URL {
	return g.cli.BaseURL
}

func GithubClient(ctx context.Context, entity params.GithubEntity) (common.GithubClient, error) {
	// func GithubClient(ctx context.Context, entity params.GithubEntity) (common.GithubClient, error) {
	httpClient, err := entity.Credentials.GetHTTPClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fetching http client")
	}

	ghClient, err := github.NewClient(httpClient).WithEnterpriseURLs(
		entity.Credentials.APIBaseURL, entity.Credentials.UploadBaseURL)
	if err != nil {
		return nil, errors.Wrap(err, "fetching github client")
	}

	cli := &githubClient{
		ActionsService: ghClient.Actions,
		org:            ghClient.Organizations,
		repo:           ghClient.Repositories,
		enterprise:     ghClient.Enterprise,
		cli:            ghClient,
		entity:         entity,
	}

	return cli, nil
}
