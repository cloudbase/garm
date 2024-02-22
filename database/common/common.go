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

	"github.com/cloudbase/garm/params"
)

type RepoStore interface {
	CreateRepository(ctx context.Context, owner, name, credentialsName, webhookSecret string) (params.Repository, error)
	GetRepository(ctx context.Context, owner, name string) (params.Repository, error)
	GetRepositoryByID(ctx context.Context, repoID string) (params.Repository, error)
	ListRepositories(ctx context.Context) ([]params.Repository, error)
	DeleteRepository(ctx context.Context, repoID string) error
	UpdateRepository(ctx context.Context, repoID string, param params.UpdateEntityParams) (params.Repository, error)

	CreateRepositoryPool(ctx context.Context, repoID string, param params.CreatePoolParams) (params.Pool, error)

	GetRepositoryPool(ctx context.Context, repoID, poolID string) (params.Pool, error)
	DeleteRepositoryPool(ctx context.Context, repoID, poolID string) error
	UpdateRepositoryPool(ctx context.Context, repoID, poolID string, param params.UpdatePoolParams) (params.Pool, error)
	FindRepositoryPoolByTags(ctx context.Context, repoID string, tags []string) (params.Pool, error)

	ListRepoPools(ctx context.Context, repoID string) ([]params.Pool, error)
	ListRepoInstances(ctx context.Context, repoID string) ([]params.Instance, error)
}

type OrgStore interface {
	CreateOrganization(ctx context.Context, name, credentialsName, webhookSecret string) (params.Organization, error)
	GetOrganization(ctx context.Context, name string) (params.Organization, error)
	GetOrganizationByID(ctx context.Context, orgID string) (params.Organization, error)
	ListOrganizations(ctx context.Context) ([]params.Organization, error)
	DeleteOrganization(ctx context.Context, orgID string) error
	UpdateOrganization(ctx context.Context, orgID string, param params.UpdateEntityParams) (params.Organization, error)

	CreateOrganizationPool(ctx context.Context, orgID string, param params.CreatePoolParams) (params.Pool, error)
	GetOrganizationPool(ctx context.Context, orgID, poolID string) (params.Pool, error)
	DeleteOrganizationPool(ctx context.Context, orgID, poolID string) error
	UpdateOrganizationPool(ctx context.Context, orgID, poolID string, param params.UpdatePoolParams) (params.Pool, error)

	FindOrganizationPoolByTags(ctx context.Context, orgID string, tags []string) (params.Pool, error)
	ListOrgPools(ctx context.Context, orgID string) ([]params.Pool, error)
	ListOrgInstances(ctx context.Context, orgID string) ([]params.Instance, error)
}

type EnterpriseStore interface {
	CreateEnterprise(ctx context.Context, name, credentialsName, webhookSecret string) (params.Enterprise, error)
	GetEnterprise(ctx context.Context, name string) (params.Enterprise, error)
	GetEnterpriseByID(ctx context.Context, enterpriseID string) (params.Enterprise, error)
	ListEnterprises(ctx context.Context) ([]params.Enterprise, error)
	DeleteEnterprise(ctx context.Context, enterpriseID string) error
	UpdateEnterprise(ctx context.Context, enterpriseID string, param params.UpdateEntityParams) (params.Enterprise, error)

	CreateEnterprisePool(ctx context.Context, enterpriseID string, param params.CreatePoolParams) (params.Pool, error)
	GetEnterprisePool(ctx context.Context, enterpriseID, poolID string) (params.Pool, error)
	DeleteEnterprisePool(ctx context.Context, enterpriseID, poolID string) error
	UpdateEnterprisePool(ctx context.Context, enterpriseID, poolID string, param params.UpdatePoolParams) (params.Pool, error)

	FindEnterprisePoolByTags(ctx context.Context, enterpriseID string, tags []string) (params.Pool, error)
	ListEnterprisePools(ctx context.Context, enterpriseID string) ([]params.Pool, error)
	ListEnterpriseInstances(ctx context.Context, enterpriseID string) ([]params.Instance, error)
}

type PoolStore interface {
	// Probably a bad idea without some king of filter or at least pagination
	// TODO: add filter/pagination
	ListAllPools(ctx context.Context) ([]params.Pool, error)
	GetPoolByID(ctx context.Context, poolID string) (params.Pool, error)
	DeletePoolByID(ctx context.Context, poolID string) error

	ListPoolInstances(ctx context.Context, poolID string) ([]params.Instance, error)

	PoolInstanceCount(ctx context.Context, poolID string) (int64, error)
	GetPoolInstanceByName(ctx context.Context, poolID string, instanceName string) (params.Instance, error)
	FindPoolsMatchingAllTags(ctx context.Context, entityType params.PoolType, entityID string, tags []string) ([]params.Pool, error)
}

type UserStore interface {
	GetUser(ctx context.Context, user string) (params.User, error)
	GetUserByID(ctx context.Context, userID string) (params.User, error)

	CreateUser(ctx context.Context, user params.NewUserParams) (params.User, error)
	UpdateUser(ctx context.Context, user string, param params.UpdateUserParams) (params.User, error)
	HasAdminUser(ctx context.Context) bool
}

type InstanceStore interface {
	CreateInstance(ctx context.Context, poolID string, param params.CreateInstanceParams) (params.Instance, error)
	DeleteInstance(ctx context.Context, poolID string, instanceName string) error
	UpdateInstance(ctx context.Context, instanceID string, param params.UpdateInstanceParams) (params.Instance, error)

	// Probably a bad idea without some king of filter or at least pagination
	// TODO: add filter/pagination
	ListAllInstances(ctx context.Context) ([]params.Instance, error)

	GetInstanceByName(ctx context.Context, instanceName string) (params.Instance, error)
	AddInstanceEvent(ctx context.Context, instanceID string, event params.EventType, eventLevel params.EventLevel, eventMessage string) error
	ListInstanceEvents(ctx context.Context, instanceID string, eventType params.EventType, eventLevel params.EventLevel) ([]params.StatusMessage, error)
}

type JobsStore interface {
	CreateOrUpdateJob(ctx context.Context, job params.Job) (params.Job, error)
	ListEntityJobsByStatus(ctx context.Context, entityType params.PoolType, entityID string, status params.JobStatus) ([]params.Job, error)
	ListJobsByStatus(ctx context.Context, status params.JobStatus) ([]params.Job, error)
	ListAllJobs(ctx context.Context) ([]params.Job, error)

	GetJobByID(ctx context.Context, jobID int64) (params.Job, error)
	DeleteJob(ctx context.Context, jobID int64) error
	UnlockJob(ctx context.Context, jobID int64, entityID string) error
	LockJob(ctx context.Context, jobID int64, entityID string) error
	BreakLockJobIsQueued(ctx context.Context, jobID int64) error

	DeleteCompletedJobs(ctx context.Context) error
}

//go:generate mockery --name=Store
type Store interface {
	RepoStore
	OrgStore
	EnterpriseStore
	PoolStore
	UserStore
	InstanceStore
	JobsStore

	ControllerInfo() (params.ControllerInfo, error)
	InitController() (params.ControllerInfo, error)
}
