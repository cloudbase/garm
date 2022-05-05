package pool

import (
	"garm/params"

	"github.com/google/go-github/v43/github"
)

type poolHelper interface {
	GetGithubToken() string
	GetGithubRunners() ([]*github.Runner, error)
	FetchTools() ([]*github.RunnerApplicationDownload, error)
	FetchDbInstances() ([]params.Instance, error)
	RemoveGithubRunner(runnerID int64) error
	ListPools() ([]params.Pool, error)
	GithubURL() string
	JwtToken() string
	GetGithubRegistrationToken() (string, error)
	String() string
	GetCallbackURL() string
	FindPoolByTags(labels []string) (params.Pool, error)
	GetPoolByID(poolID string) (params.Pool, error)
	ValidateOwner(job params.WorkflowJob) error
	UpdateState(param params.UpdatePoolStateParams) error
	WebhookSecret() string
	ID() string
}
