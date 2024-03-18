package common

import (
	"context"

	"github.com/google/go-github/v57/github"

	"github.com/cloudbase/garm/params"
)

type GithubEntityOperations interface {
	ListEntityHooks(ctx context.Context, opts *github.ListOptions) (ret []*github.Hook, response *github.Response, err error)
	GetEntityHook(ctx context.Context, id int64) (ret *github.Hook, err error)
	CreateEntityHook(ctx context.Context, hook *github.Hook) (ret *github.Hook, err error)
	DeleteEntityHook(ctx context.Context, id int64) (ret *github.Response, err error)
	PingEntityHook(ctx context.Context, id int64) (ret *github.Response, err error)
	ListEntityRunners(ctx context.Context, opts *github.ListOptions) (*github.Runners, *github.Response, error)
	ListEntityRunnerApplicationDownloads(ctx context.Context) ([]*github.RunnerApplicationDownload, *github.Response, error)
	RemoveEntityRunner(ctx context.Context, runnerID int64) (*github.Response, error)
	CreateEntityRegistrationToken(ctx context.Context) (*github.RegistrationToken, *github.Response, error)
	GetEntityJITConfig(ctx context.Context, instance string, pool params.Pool, labels []string) (jitConfigMap map[string]string, runner *github.Runner, err error)
}

// GithubClient that describes the minimum list of functions we need to interact with github.
// Allows for easier testing.
//
//go:generate mockery --all
type GithubClient interface {
	GithubEntityOperations

	// GetWorkflowJobByID gets details about a single workflow job.
	GetWorkflowJobByID(ctx context.Context, owner, repo string, jobID int64) (*github.WorkflowJob, *github.Response, error)
}
