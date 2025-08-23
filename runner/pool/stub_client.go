// Copyright 2025 Cloudbase Solutions SRL
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

package pool

import (
	"context"
	"net/url"

	"github.com/google/go-github/v72/github"

	"github.com/cloudbase/garm/params"
)

type stubGithubClient struct {
	err error
}

func (s *stubGithubClient) ListEntityHooks(_ context.Context, _ *github.ListOptions) ([]*github.Hook, *github.Response, error) {
	return nil, nil, s.err
}

func (s *stubGithubClient) GetEntityHook(_ context.Context, _ int64) (*github.Hook, error) {
	return nil, s.err
}

func (s *stubGithubClient) CreateEntityHook(_ context.Context, _ *github.Hook) (*github.Hook, error) {
	return nil, s.err
}

func (s *stubGithubClient) DeleteEntityHook(_ context.Context, _ int64) (*github.Response, error) {
	return nil, s.err
}

func (s *stubGithubClient) PingEntityHook(_ context.Context, _ int64) (*github.Response, error) {
	return nil, s.err
}

func (s *stubGithubClient) ListEntityRunners(_ context.Context, _ *github.ListRunnersOptions) (*github.Runners, *github.Response, error) {
	return nil, nil, s.err
}

func (s *stubGithubClient) ListEntityRunnerApplicationDownloads(_ context.Context) ([]*github.RunnerApplicationDownload, *github.Response, error) {
	return nil, nil, s.err
}

func (s *stubGithubClient) RemoveEntityRunner(_ context.Context, _ int64) error {
	return s.err
}

func (s *stubGithubClient) CreateEntityRegistrationToken(_ context.Context) (*github.RegistrationToken, *github.Response, error) {
	return nil, nil, s.err
}

func (s *stubGithubClient) GetEntityJITConfig(_ context.Context, _ string, _ params.Pool, _ []string) (map[string]string, *github.Runner, error) {
	return nil, nil, s.err
}

func (s *stubGithubClient) GetWorkflowJobByID(_ context.Context, _, _ string, _ int64) (*github.WorkflowJob, *github.Response, error) {
	return nil, nil, s.err
}

func (s *stubGithubClient) GetEntity() params.ForgeEntity {
	return params.ForgeEntity{}
}

func (s *stubGithubClient) GithubBaseURL() *url.URL {
	return nil
}

func (s *stubGithubClient) RateLimit(_ context.Context) (*github.RateLimits, error) {
	return nil, s.err
}

func (s *stubGithubClient) GetEntityRunnerGroupIDByName(_ context.Context, _ string) (int64, error) {
	return 0, s.err
}
