package common

import "context"

type (
	DatabaseEntityType string
	OperationType      string
	PayloadFilterFunc  func(ChangePayload) bool
)

const (
	RepositoryEntityType        DatabaseEntityType = "repository"
	OrganizationEntityType      DatabaseEntityType = "organization"
	EnterpriseEntityType        DatabaseEntityType = "enterprise"
	PoolEntityType              DatabaseEntityType = "pool"
	UserEntityType              DatabaseEntityType = "user"
	InstanceEntityType          DatabaseEntityType = "instance"
	JobEntityType               DatabaseEntityType = "job"
	ControllerEntityType        DatabaseEntityType = "controller"
	GithubCredentialsEntityType DatabaseEntityType = "github_credentials" // #nosec G101
	GithubEndpointEntityType    DatabaseEntityType = "github_endpoint"
)

const (
	CreateOperation OperationType = "create"
	UpdateOperation OperationType = "update"
	DeleteOperation OperationType = "delete"
)

type ChangePayload struct {
	EntityType DatabaseEntityType
	Operation  OperationType
	Payload    interface{}
}

type Consumer interface {
	Watch() <-chan ChangePayload
	IsClosed() bool
	Close()
	SetFilters(filters ...PayloadFilterFunc)
}

type Producer interface {
	Notify(ChangePayload) error
	IsClosed() bool
	Close()
}

type Watcher interface {
	RegisterProducer(ctx context.Context, ID string) (Producer, error)
	RegisterConsumer(ctx context.Context, ID string, filters ...PayloadFilterFunc) (Consumer, error)
	Close()
}
