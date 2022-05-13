// Copyright 2022 Cloudbase Solutions SRL
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

import (
	"context"
	"garm/params"
)

type Store interface {
	CreateRepository(ctx context.Context, owner, name, credentialsName, webhookSecret string) (params.Repository, error)
	GetRepository(ctx context.Context, owner, name string) (params.Repository, error)
	GetRepositoryByID(ctx context.Context, repoID string) (params.Repository, error)
	ListRepositories(ctx context.Context) ([]params.Repository, error)
	DeleteRepository(ctx context.Context, repoID string) error
	UpdateRepository(ctx context.Context, repoID string, param params.UpdateRepositoryParams) (params.Repository, error)

	CreateOrganization(ctx context.Context, name, credentialsName, webhookSecret string) (params.Organization, error)
	GetOrganization(ctx context.Context, name string) (params.Organization, error)
	GetOrganizationByID(ctx context.Context, orgID string) (params.Organization, error)
	ListOrganizations(ctx context.Context) ([]params.Organization, error)
	DeleteOrganization(ctx context.Context, orgID string) error
	UpdateOrganization(ctx context.Context, orgID string, param params.UpdateRepositoryParams) (params.Organization, error)

	CreateRepositoryPool(ctx context.Context, repoId string, param params.CreatePoolParams) (params.Pool, error)
	CreateOrganizationPool(ctx context.Context, orgId string, param params.CreatePoolParams) (params.Pool, error)

	GetRepositoryPool(ctx context.Context, repoID, poolID string) (params.Pool, error)
	GetOrganizationPool(ctx context.Context, orgID, poolID string) (params.Pool, error)

	ListRepoPools(ctx context.Context, repoID string) ([]params.Pool, error)
	ListOrgPools(ctx context.Context, orgID string) ([]params.Pool, error)
	// Probably a bad idea without some king of filter or at least pagination
	// TODO: add filter/pagination
	ListAllPools(ctx context.Context) ([]params.Pool, error)
	GetPoolByID(ctx context.Context, poolID string) (params.Pool, error)
	DeletePoolByID(ctx context.Context, poolID string) error

	DeleteRepositoryPool(ctx context.Context, repoID, poolID string) error
	DeleteOrganizationPool(ctx context.Context, orgID, poolID string) error

	UpdateRepositoryPool(ctx context.Context, repoID, poolID string, param params.UpdatePoolParams) (params.Pool, error)
	UpdateOrganizationPool(ctx context.Context, orgID, poolID string, param params.UpdatePoolParams) (params.Pool, error)

	FindRepositoryPoolByTags(ctx context.Context, repoID string, tags []string) (params.Pool, error)
	FindOrganizationPoolByTags(ctx context.Context, orgID string, tags []string) (params.Pool, error)

	CreateInstance(ctx context.Context, poolID string, param params.CreateInstanceParams) (params.Instance, error)
	DeleteInstance(ctx context.Context, poolID string, instanceID string) error
	UpdateInstance(ctx context.Context, instanceID string, param params.UpdateInstanceParams) (params.Instance, error)

	ListPoolInstances(ctx context.Context, poolID string) ([]params.Instance, error)
	ListRepoInstances(ctx context.Context, repoID string) ([]params.Instance, error)
	ListOrgInstances(ctx context.Context, orgID string) ([]params.Instance, error)

	PoolInstanceCount(ctx context.Context, poolID string) (int64, error)

	// Probably a bad idea without some king of filter or at least pagination
	// TODO: add filter/pagination
	ListAllInstances(ctx context.Context) ([]params.Instance, error)

	GetPoolInstanceByName(ctx context.Context, poolID string, instanceName string) (params.Instance, error)
	GetInstanceByName(ctx context.Context, instanceName string) (params.Instance, error)
	AddInstanceStatusMessage(ctx context.Context, instanceID string, statusMessage string) error

	GetUser(ctx context.Context, user string) (params.User, error)
	GetUserByID(ctx context.Context, userID string) (params.User, error)

	CreateUser(ctx context.Context, user params.NewUserParams) (params.User, error)
	UpdateUser(ctx context.Context, user string, param params.UpdateUserParams) (params.User, error)
	HasAdminUser(ctx context.Context) bool

	ControllerInfo() (params.ControllerInfo, error)
	InitController() (params.ControllerInfo, error)
}
