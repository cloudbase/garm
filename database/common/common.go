package common

import (
	"context"
	"runner-manager/params"
)

type Store interface {
	CreateRepository(ctx context.Context, owner, name, credentialsName, webhookSecret string) (params.Repository, error)
	GetRepository(ctx context.Context, owner, name string) (params.Repository, error)
	ListRepositories(ctx context.Context) ([]params.Repository, error)
	DeleteRepository(ctx context.Context, owner, name string) error

	CreateOrganization(ctx context.Context, name, credentialsName, webhookSecret string) (params.Organization, error)
	GetOrganization(ctx context.Context, name string) (params.Organization, error)
	ListOrganizations(ctx context.Context) ([]params.Organization, error)
	DeleteOrganization(ctx context.Context, name string) error

	CreateRepositoryPool(ctx context.Context, repoId string, param params.CreatePoolParams) (params.Pool, error)
	CreateOrganizationPool(ctx context.Context, orgId string, param params.CreatePoolParams) (params.Pool, error)

	GetRepositoryPool(ctx context.Context, repoID, poolID string) (params.Pool, error)
	GetOrganizationPool(ctx context.Context, orgID, poolID string) (params.Pool, error)

	DeleteRepositoryPool(ctx context.Context, repoID, poolID string) error
	DeleteOrganizationPool(ctx context.Context, orgID, poolID string) error

	UpdateRepositoryPool(ctx context.Context, repoID, poolID string, param params.UpdatePoolParams) (params.Pool, error)
	UpdateOrganizationPool(ctx context.Context, orgID, poolID string, param params.UpdatePoolParams) (params.Pool, error)

	FindRepositoryPoolByTags(ctx context.Context, repoID string, tags []string) (params.Pool, error)
	FindOrganizationPoolByTags(ctx context.Context, orgID string, tags []string) (params.Pool, error)

	CreateInstance(ctx context.Context, poolID string, param params.CreateInstanceParams) (params.Instance, error)
	DeleteInstance(ctx context.Context, poolID string, instanceID string) error
	UpdateInstance(ctx context.Context, instanceID string, param params.UpdateInstanceParams) (params.Instance, error)

	ListInstances(ctx context.Context, poolID string) ([]params.Instance, error)
	ListRepoInstances(ctx context.Context, repoID string) ([]params.Instance, error)
	ListOrgInstances(ctx context.Context, orgID string) ([]params.Instance, error)

	// GetInstance(ctx context.Context, poolID string, instanceID string) (params.Instance, error)
	GetInstanceByName(ctx context.Context, poolID string, instanceName string) (params.Instance, error)

	CreateUser(ctx context.Context, user params.NewUserParams) (params.User, error)
	GetUser(ctx context.Context, user string) (params.User, error)
	UpdateUser(ctx context.Context, user string, param params.UpdateUserParams) (params.User, error)
	HasAdminUser(ctx context.Context) bool

	ControllerInfo() (params.ControllerInfo, error)
	InitController() (params.ControllerInfo, error)
}
