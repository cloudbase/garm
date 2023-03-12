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

package runner

import (
	"context"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

type RepoPoolManager interface {
	CreateRepoPoolManager(ctx context.Context, repo params.Repository, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error)
	GetRepoPoolManager(repo params.Repository) (common.PoolManager, error)
	DeleteRepoPoolManager(repo params.Repository) error
	GetRepoPoolManagers() (map[string]common.PoolManager, error)
}

type OrgPoolManager interface {
	CreateOrgPoolManager(ctx context.Context, org params.Organization, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error)
	GetOrgPoolManager(org params.Organization) (common.PoolManager, error)
	DeleteOrgPoolManager(org params.Organization) error
	GetOrgPoolManagers() (map[string]common.PoolManager, error)
}

type EnterprisePoolManager interface {
	CreateEnterprisePoolManager(ctx context.Context, enterprise params.Enterprise, providers map[string]common.Provider, store dbCommon.Store) (common.PoolManager, error)
	GetEnterprisePoolManager(enterprise params.Enterprise) (common.PoolManager, error)
	DeleteEnterprisePoolManager(enterprise params.Enterprise) error
	GetEnterprisePoolManagers() (map[string]common.PoolManager, error)
}

//go:generate mockery --name=PoolManagerController

type PoolManagerController interface {
	RepoPoolManager
	OrgPoolManager
	EnterprisePoolManager
}
