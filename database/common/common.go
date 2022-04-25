package common

import (
	"context"
	"runner-manager/params"
)

type Store interface {
	CreateRepository(ctx context.Context, owner, name, webhookSecret string) (params.Repository, error)
	GetRepository(ctx context.Context, id string) (params.Repository, error)
	ListRepositories(ctx context.Context) ([]params.Repository, error)
	DeleteRepository(ctx context.Context, id string) error

	CreateOrganization(ctx context.Context, name, webhookSecret string) (params.Organization, error)
	GetOrganization(ctx context.Context, id string) (params.Organization, error)
	ListOrganizations(ctx context.Context) ([]params.Organization, error)
	DeleteOrganization(ctx context.Context, id string) error

	CreateRepositoryPool(ctx context.Context, repoId string, param params.CreatePoolParams) (params.Pool, error)
	CreateOrganizationPool(ctx context.Context, orgId string, param params.CreatePoolParams) (params.Pool, error)

	GetRepositoryPool(ctx context.Context, repoID, poolID string) (params.Pool, error)
	GetOrganizationPool(ctx context.Context, orgID, poolID string) (params.Pool, error)

	DeleteRepositoryPool(ctx context.Context, repoID, poolID string) error
	DeleteOrganizationPool(ctx context.Context, orgID, poolID string) error

	UpdateRepositoryPool(ctx context.Context, repoID, poolID string) (params.Pool, error)
	UpdateOrganizationPool(ctx context.Context, orgID, poolID string) (params.Pool, error)
}
