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

type GithubEndpointStore interface {
	CreateGithubEndpoint(ctx context.Context, param params.CreateGithubEndpointParams) (params.GithubEndpoint, error)
	GetGithubEndpoint(ctx context.Context, name string) (params.GithubEndpoint, error)
	ListGithubEndpoints(ctx context.Context) ([]params.GithubEndpoint, error)
	UpdateGithubEndpoint(ctx context.Context, name string, param params.UpdateGithubEndpointParams) (params.GithubEndpoint, error)
	DeleteGithubEndpoint(ctx context.Context, name string) error
}

type GithubCredentialsStore interface {
	CreateGithubCredentials(ctx context.Context, param params.CreateGithubCredentialsParams) (params.GithubCredentials, error)
	GetGithubCredentials(ctx context.Context, id uint, detailed bool) (params.GithubCredentials, error)
	GetGithubCredentialsByName(ctx context.Context, name string, detailed bool) (params.GithubCredentials, error)
	ListGithubCredentials(ctx context.Context) ([]params.GithubCredentials, error)
	UpdateGithubCredentials(ctx context.Context, id uint, param params.UpdateGithubCredentialsParams) (params.GithubCredentials, error)
	DeleteGithubCredentials(ctx context.Context, id uint) error
}

type RepoStore interface {
	CreateRepository(ctx context.Context, owner, name, credentialsName, webhookSecret string, poolBalancerType params.PoolBalancerType) (params.Repository, error)
	GetRepository(ctx context.Context, owner, name, endpointName string) (params.Repository, error)
	GetRepositoryByID(ctx context.Context, repoID string) (params.Repository, error)
	ListRepositories(ctx context.Context) ([]params.Repository, error)
	DeleteRepository(ctx context.Context, repoID string) error
	UpdateRepository(ctx context.Context, repoID string, param params.UpdateEntityParams) (params.Repository, error)
}

type OrgStore interface {
	CreateOrganization(ctx context.Context, name, credentialsName, webhookSecret string, poolBalancerType params.PoolBalancerType) (params.Organization, error)
	GetOrganization(ctx context.Context, name, endpointName string) (params.Organization, error)
	GetOrganizationByID(ctx context.Context, orgID string) (params.Organization, error)
	ListOrganizations(ctx context.Context) ([]params.Organization, error)
	DeleteOrganization(ctx context.Context, orgID string) error
	UpdateOrganization(ctx context.Context, orgID string, param params.UpdateEntityParams) (params.Organization, error)
}

type EnterpriseStore interface {
	CreateEnterprise(ctx context.Context, name, credentialsName, webhookSecret string, poolBalancerType params.PoolBalancerType) (params.Enterprise, error)
	GetEnterprise(ctx context.Context, name, endpointName string) (params.Enterprise, error)
	GetEnterpriseByID(ctx context.Context, enterpriseID string) (params.Enterprise, error)
	ListEnterprises(ctx context.Context) ([]params.Enterprise, error)
	DeleteEnterprise(ctx context.Context, enterpriseID string) error
	UpdateEnterprise(ctx context.Context, enterpriseID string, param params.UpdateEntityParams) (params.Enterprise, error)
}

type PoolStore interface {
	// Probably a bad idea without some king of filter or at least pagination
	// nolint:golangci-lint,godox
	// TODO: add filter/pagination
	ListAllPools(ctx context.Context) ([]params.Pool, error)
	GetPoolByID(ctx context.Context, poolID string) (params.Pool, error)
	DeletePoolByID(ctx context.Context, poolID string) error

	ListPoolInstances(ctx context.Context, poolID string) ([]params.Instance, error)

	PoolInstanceCount(ctx context.Context, poolID string) (int64, error)
	GetPoolInstanceByName(ctx context.Context, poolID string, instanceName string) (params.Instance, error)
	FindPoolsMatchingAllTags(ctx context.Context, entityType params.GithubEntityType, entityID string, tags []string) ([]params.Pool, error)
}

type UserStore interface {
	GetUser(ctx context.Context, user string) (params.User, error)
	GetUserByID(ctx context.Context, userID string) (params.User, error)
	GetAdminUser(ctx context.Context) (params.User, error)

	CreateUser(ctx context.Context, user params.NewUserParams) (params.User, error)
	UpdateUser(ctx context.Context, user string, param params.UpdateUserParams) (params.User, error)
	HasAdminUser(ctx context.Context) bool
}

type InstanceStore interface {
	CreateInstance(ctx context.Context, poolID string, param params.CreateInstanceParams) (params.Instance, error)
	DeleteInstance(ctx context.Context, poolID string, instanceName string) error
	UpdateInstance(ctx context.Context, instanceName string, param params.UpdateInstanceParams) (params.Instance, error)

	// Probably a bad idea without some king of filter or at least pagination
	//
	// nolint:golangci-lint,godox
	// TODO: add filter/pagination
	ListAllInstances(ctx context.Context) ([]params.Instance, error)

	GetInstanceByName(ctx context.Context, instanceName string) (params.Instance, error)
	AddInstanceEvent(ctx context.Context, instanceName string, event params.EventType, eventLevel params.EventLevel, eventMessage string) error
}

type JobsStore interface {
	CreateOrUpdateJob(ctx context.Context, job params.Job) (params.Job, error)
	ListEntityJobsByStatus(ctx context.Context, entityType params.GithubEntityType, entityID string, status params.JobStatus) ([]params.Job, error)
	ListJobsByStatus(ctx context.Context, status params.JobStatus) ([]params.Job, error)
	ListAllJobs(ctx context.Context) ([]params.Job, error)

	GetJobByID(ctx context.Context, jobID int64) (params.Job, error)
	DeleteJob(ctx context.Context, jobID int64) error
	UnlockJob(ctx context.Context, jobID int64, entityID string) error
	LockJob(ctx context.Context, jobID int64, entityID string) error
	BreakLockJobIsQueued(ctx context.Context, jobID int64) error

	DeleteCompletedJobs(ctx context.Context) error
}

type EntityPoolStore interface {
	CreateEntityPool(ctx context.Context, entity params.GithubEntity, param params.CreatePoolParams) (params.Pool, error)
	GetEntityPool(ctx context.Context, entity params.GithubEntity, poolID string) (params.Pool, error)
	DeleteEntityPool(ctx context.Context, entity params.GithubEntity, poolID string) error
	UpdateEntityPool(ctx context.Context, entity params.GithubEntity, poolID string, param params.UpdatePoolParams) (params.Pool, error)

	ListEntityPools(ctx context.Context, entity params.GithubEntity) ([]params.Pool, error)
	ListEntityInstances(ctx context.Context, entity params.GithubEntity) ([]params.Instance, error)
}

type ControllerStore interface {
	ControllerInfo() (params.ControllerInfo, error)
	InitController() (params.ControllerInfo, error)
	UpdateController(info params.UpdateControllerParams) (params.ControllerInfo, error)
}

type ScaleSetsStore interface {
	ListAllScaleSets(ctx context.Context) ([]params.ScaleSet, error)
	CreateEntityScaleSet(_ context.Context, entity params.GithubEntity, param params.CreateScaleSetParams) (scaleSet params.ScaleSet, err error)
	ListEntityScaleSets(_ context.Context, entity params.GithubEntity) ([]params.ScaleSet, error)
	UpdateEntityScaleSet(_ context.Context, entity params.GithubEntity, scaleSetID uint, param params.UpdateScaleSetParams, callback func(old, new params.ScaleSet) error) (updatedScaleSet params.ScaleSet, err error)
	GetScaleSetByID(ctx context.Context, scaleSet uint) (params.ScaleSet, error)
	DeleteScaleSetByID(ctx context.Context, scaleSetID uint) (err error)
	ListScaleSetInstances(_ context.Context, scalesetID uint) ([]params.Instance, error)
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
	GithubEndpointStore
	GithubCredentialsStore
	ControllerStore
	EntityPoolStore
	ScaleSetsStore

	ControllerInfo() (params.ControllerInfo, error)
	InitController() (params.ControllerInfo, error)
	GetGithubEntity(_ context.Context, entityType params.GithubEntityType, entityID string) (params.GithubEntity, error)
	AddEntityEvent(ctx context.Context, entity params.GithubEntity, event params.EventType, eventLevel params.EventLevel, statusMessage string, maxEvents int) error
}
