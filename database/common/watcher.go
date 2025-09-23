// Copyright 2025 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

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
	GiteaCredentialsEntityType  DatabaseEntityType = "gitea_credentials"  // #nosec G101
	GithubEndpointEntityType    DatabaseEntityType = "github_endpoint"
	ScaleSetEntityType          DatabaseEntityType = "scaleset"
	TemplateEntityType          DatabaseEntityType = "template"
)

const (
	CreateOperation OperationType = "create"
	UpdateOperation OperationType = "update"
	DeleteOperation OperationType = "delete"
)

type ChangePayload struct {
	EntityType DatabaseEntityType `json:"entity-type"`
	Operation  OperationType      `json:"operation"`
	Payload    interface{}        `json:"payload"`
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
