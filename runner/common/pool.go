package common

import "runner-manager/params"

type PoolManager interface {
	WebhookSecret() string
	HandleWorkflowJob(job params.WorkflowJob) error
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
