package common

import (
	"garm/params"
)

type PoolType string

const (
	RepositoryPool   PoolType = "repository"
	OrganizationPool PoolType = "organization"
)

type PoolManager interface {
	WebhookSecret() string
	HandleWorkflowJob(job params.WorkflowJob) error
	RefreshState(param params.UpdatePoolStateParams) error
	ID() string
	// AddPool(ctx context.Context, pool params.Pool) error

	// PoolManager lifecycle functions. Start/stop pool.
	Start() error
	Stop() error
	Wait() error
}
