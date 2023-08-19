package common

import (
	"context"

	"github.com/google/go-github/v55/github"
)

type OrganizationHooks interface {
	ListOrgHooks(ctx context.Context, org string, opts *github.ListOptions) ([]*github.Hook, *github.Response, error)
	GetOrgHook(ctx context.Context, org string, id int64) (*github.Hook, *github.Response, error)
	CreateOrgHook(ctx context.Context, org string, hook *github.Hook) (*github.Hook, *github.Response, error)
	DeleteOrgHook(ctx context.Context, org string, id int64) (*github.Response, error)
	PingOrgHook(ctx context.Context, org string, id int64) (*github.Response, error)
}

type RepositoryHooks interface {
	ListRepoHooks(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.Hook, *github.Response, error)
	GetRepoHook(ctx context.Context, owner, repo string, id int64) (*github.Hook, *github.Response, error)
	CreateRepoHook(ctx context.Context, owner, repo string, hook *github.Hook) (*github.Hook, *github.Response, error)
	DeleteRepoHook(ctx context.Context, owner, repo string, id int64) (*github.Response, error)
	PingRepoHook(ctx context.Context, owner, repo string, id int64) (*github.Response, error)
}

// GithubClient that describes the minimum list of functions we need to interact with github.
// Allows for easier testing.
//
//go:generate mockery --all
type GithubClient interface {
	OrganizationHooks
	RepositoryHooks

	// GetWorkflowJobByID gets details about a single workflow job.
	GetWorkflowJobByID(ctx context.Context, owner, repo string, jobID int64) (*github.WorkflowJob, *github.Response, error)
	// ListRunners lists all runners within a repository.
	ListRunners(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Runners, *github.Response, error)
	// ListRunnerApplicationDownloads returns a list of github runner application downloads for the
	// various supported operating systems and architectures.
	ListRunnerApplicationDownloads(ctx context.Context, owner, repo string) ([]*github.RunnerApplicationDownload, *github.Response, error)
	// RemoveRunner removes one runner from a repository.
	RemoveRunner(ctx context.Context, owner, repo string, runnerID int64) (*github.Response, error)
	// CreateRegistrationToken creates a runner registration token for one repository.
	CreateRegistrationToken(ctx context.Context, owner, repo string) (*github.RegistrationToken, *github.Response, error)
	// GenerateRepoJITConfig generates a just-in-time configuration for a repository.
	GenerateRepoJITConfig(ctx context.Context, owner, repo string, request *github.GenerateJITConfigRequest) (*github.JITRunnerConfig, *github.Response, error)

	// ListOrganizationRunners lists all runners within an organization.
	ListOrganizationRunners(ctx context.Context, owner string, opts *github.ListOptions) (*github.Runners, *github.Response, error)
	// ListOrganizationRunnerApplicationDownloads returns a list of github runner application downloads for the
	// various supported operating systems and architectures.
	ListOrganizationRunnerApplicationDownloads(ctx context.Context, owner string) ([]*github.RunnerApplicationDownload, *github.Response, error)
	// RemoveOrganizationRunner removes one github runner from an organization.
	RemoveOrganizationRunner(ctx context.Context, owner string, runnerID int64) (*github.Response, error)
	// CreateOrganizationRegistrationToken creates a runner registration token for an organization.
	CreateOrganizationRegistrationToken(ctx context.Context, owner string) (*github.RegistrationToken, *github.Response, error)
	// GenerateOrgJITConfig generate a just-in-time configuration for an organization.
	GenerateOrgJITConfig(ctx context.Context, owner string, request *github.GenerateJITConfigRequest) (*github.JITRunnerConfig, *github.Response, error)
}

type GithubEnterpriseClient interface {
	// ListRunners lists all runners within a repository.
	ListRunners(ctx context.Context, enterprise string, opts *github.ListOptions) (*github.Runners, *github.Response, error)
	// RemoveRunner removes one runner from an enterprise.
	RemoveRunner(ctx context.Context, enterprise string, runnerID int64) (*github.Response, error)
	// CreateRegistrationToken creates a runner registration token for an enterprise.
	CreateRegistrationToken(ctx context.Context, enterprise string) (*github.RegistrationToken, *github.Response, error)
	// ListRunnerApplicationDownloads returns a list of github runner application downloads for the
	// various supported operating systems and architectures.
	ListRunnerApplicationDownloads(ctx context.Context, enterprise string) ([]*github.RunnerApplicationDownload, *github.Response, error)
}
