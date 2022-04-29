package common

import (
	"context"
	"runner-manager/params"
)

type PoolType string

const (
	RepositoryPool   PoolType = "repository"
	OrganizationPool PoolType = "organization"
)

type PoolManager interface {
	WebhookSecret() string
	HandleWorkflowJob(job params.WorkflowJob) error
	RefreshState(cfg params.Repository) error
	AddPool(ctx context.Context, pool params.Pool) error

	// PoolManager lifecycle functions. Start/stop pool.
	Start() error
	Stop() error
	Wait() error
}

type Pool interface {
	ListInstances() ([]params.Instance, error)
	GetInstance() (params.Instance, error)
	DeleteInstance() error
	StopInstance() error
	StartInstance() error

	// Pool lifecycle functions. Start/stop pool.
	Start() error
	Stop() error
	Wait() error
}
