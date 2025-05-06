package pool

import (
	"context"
	"net/url"

	"github.com/google/go-github/v71/github"

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

func (s *stubGithubClient) GetEntity() params.GithubEntity {
	return params.GithubEntity{}
}

func (s *stubGithubClient) GithubBaseURL() *url.URL {
	return nil
}

func (s *stubGithubClient) RateLimit(_ context.Context) (*github.RateLimits, error) {
	return nil, s.err
}
