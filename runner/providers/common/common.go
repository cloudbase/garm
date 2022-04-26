package common

type InstanceStatus string
type RunnerStatus string

const (
	InstanceRunning       InstanceStatus = "running"
	InstanceStopped       InstanceStatus = "stopped"
	InstancePendingDelete InstanceStatus = "pending_delete"
	InstancePendingCreate InstanceStatus = "pending_create"
	InstanceStatusUnknown InstanceStatus = "unknown"

	RunnerIdle    RunnerStatus = "idle"
	RunnerPending RunnerStatus = "pending"
	RunnerActive  RunnerStatus = "active"
)
