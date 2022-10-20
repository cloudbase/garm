package common

import (
	"context"

	"github.com/google/go-github/v48/github"
)

// GithubClient that describes the minimum list of functions we need to interact with github.
// Allows for easier testing.
//
//go:generate mockery --all
type GithubClient interface {
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

	// ListOrganizationRunners lists all runners within an organization.
	ListOrganizationRunners(ctx context.Context, owner string, opts *github.ListOptions) (*github.Runners, *github.Response, error)
	// ListOrganizationRunnerApplicationDownloads returns a list of github runner application downloads for the
	// various supported operating systems and architectures.
	ListOrganizationRunnerApplicationDownloads(ctx context.Context, owner string) ([]*github.RunnerApplicationDownload, *github.Response, error)
	// RemoveOrganizationRunner removes one github runner from an organization.
	RemoveOrganizationRunner(ctx context.Context, owner string, runnerID int64) (*github.Response, error)
	// CreateOrganizationRegistrationToken creates a runner registration token for an organization.
	CreateOrganizationRegistrationToken(ctx context.Context, owner string) (*github.RegistrationToken, *github.Response, error)
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
