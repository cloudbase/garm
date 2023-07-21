package execution

type ExecutionCommand string

const (
	CreateInstanceCommand     ExecutionCommand = "CreateInstance"
	DeleteInstanceCommand     ExecutionCommand = "DeleteInstance"
	GetInstanceCommand        ExecutionCommand = "GetInstance"
	ListInstancesCommand      ExecutionCommand = "ListInstances"
	StartInstanceCommand      ExecutionCommand = "StartInstance"
	StopInstanceCommand       ExecutionCommand = "StopInstance"
	RemoveAllInstancesCommand ExecutionCommand = "RemoveAllInstances"
)
