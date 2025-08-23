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

package common

import (
	"context"
	"net/url"

	"github.com/google/go-github/v72/github"

	"github.com/cloudbase/garm/params"
)

type GithubEntityOperations interface {
	ListEntityHooks(ctx context.Context, opts *github.ListOptions) (ret []*github.Hook, response *github.Response, err error)
	GetEntityHook(ctx context.Context, id int64) (ret *github.Hook, err error)
	CreateEntityHook(ctx context.Context, hook *github.Hook) (ret *github.Hook, err error)
	DeleteEntityHook(ctx context.Context, id int64) (ret *github.Response, err error)
	PingEntityHook(ctx context.Context, id int64) (ret *github.Response, err error)
	ListEntityRunners(ctx context.Context, opts *github.ListRunnersOptions) (*github.Runners, *github.Response, error)
	ListEntityRunnerApplicationDownloads(ctx context.Context) ([]*github.RunnerApplicationDownload, *github.Response, error)
	RemoveEntityRunner(ctx context.Context, runnerID int64) error
	RateLimit(ctx context.Context) (*github.RateLimits, error)
	CreateEntityRegistrationToken(ctx context.Context) (*github.RegistrationToken, *github.Response, error)
	GetEntityJITConfig(ctx context.Context, instance string, pool params.Pool, labels []string) (jitConfigMap map[string]string, runner *github.Runner, err error)
	GetEntityRunnerGroupIDByName(ctx context.Context, runnerGroupName string) (int64, error)

	// GetEntity returns the GitHub entity for which the github client was instanciated.
	GetEntity() params.ForgeEntity
	// GithubBaseURL returns the base URL for the github or GHES API.
	GithubBaseURL() *url.URL
}

type RateLimitClient interface {
	RateLimit(ctx context.Context) (*github.RateLimits, error)
}

// GithubClient that describes the minimum list of functions we need to interact with github.
// Allows for easier testing.
//
//go:generate go run github.com/vektra/mockery/v2@latest
type GithubClient interface {
	GithubEntityOperations

	// GetWorkflowJobByID gets details about a single workflow job.
	GetWorkflowJobByID(ctx context.Context, owner, repo string, jobID int64) (*github.WorkflowJob, *github.Response, error)
}
