// Copyright 2026 Cloudbase Solutions SRL
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
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database"
	dbCommon "github.com/cloudbase/garm/database/common"
	garmTesting "github.com/cloudbase/garm/internal/testing" //nolint:typecheck
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	runnerCommonMocks "github.com/cloudbase/garm/runner/common/mocks"
	runnerMocks "github.com/cloudbase/garm/runner/mocks"
)

type ForgeInstanceTestFixtures struct {
	AdminContext              context.Context
	Store                     dbCommon.Store
	StoreForgeInstances       map[string]params.ForgeInstance
	Providers                 map[string]common.Provider
	CreateForgeInstanceParams params.CreateForgeInstanceParams
	CreatePoolParams          params.CreatePoolParams
	UpdateEntityParams        params.UpdateEntityParams
	UpdatePoolParams          params.UpdatePoolParams
	ErrMock                   error
	ProviderMock              *runnerCommonMocks.Provider
	PoolMgrMock               *runnerCommonMocks.PoolManager
	PoolMgrCtrlMock           *runnerMocks.PoolManagerController
}

type ForgeInstanceTestSuite struct {
	suite.Suite
	Fixtures      *ForgeInstanceTestFixtures
	Runner        *Runner
	giteaEndpoint params.ForgeEndpoint
	giteaCreds    params.ForgeCredentials
	giteaCreds2   params.ForgeCredentials
}

func (s *ForgeInstanceTestSuite) SetupTest() {
	dbCfg := garmTesting.GetTestSqliteDBConfig(s.T())
	db, err := database.NewDatabase(context.Background(), dbCfg)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create db connection: %s", err))
	}

	adminCtx := garmTesting.ImpersonateAdminContext(context.Background(), db, s.T())
	s.giteaEndpoint = garmTesting.CreateDefaultGiteaEndpoint(adminCtx, db, s.T())
	s.giteaCreds = garmTesting.CreateTestGiteaCredentials(adminCtx, "gitea-creds", db, s.T(), s.giteaEndpoint)
	s.giteaCreds2 = garmTesting.CreateTestGiteaCredentials(adminCtx, "gitea-creds-2", db, s.T(), s.giteaEndpoint)

	forgeInstances := map[string]params.ForgeInstance{}
	fi, err := db.CreateForgeInstance(
		adminCtx,
		s.giteaEndpoint.Name,
		s.giteaCreds,
		"test-webhook-secret",
		params.PoolBalancerTypeRoundRobin,
		false,
	)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create forge instance: %v", err))
	}
	forgeInstances[s.giteaEndpoint.Name] = fi

	var maxRunners uint = 40
	var minIdleRunners uint = 20
	providerMock := runnerCommonMocks.NewProvider(s.T())
	fixtures := &ForgeInstanceTestFixtures{
		AdminContext:        adminCtx,
		Store:               db,
		StoreForgeInstances: forgeInstances,
		Providers: map[string]common.Provider{
			"test-provider": providerMock,
		},
		CreateForgeInstanceParams: params.CreateForgeInstanceParams{
			EndpointName:     s.giteaEndpoint.Name,
			CredentialsName:  s.giteaCreds.Name,
			WebhookSecret:    "new-webhook-secret",
			ForgeType:        params.GiteaEndpointType,
			PoolBalancerType: params.PoolBalancerTypeRoundRobin,
		},
		CreatePoolParams: params.CreatePoolParams{
			ProviderName:           "test-provider",
			MaxRunners:             4,
			MinIdleRunners:         2,
			Image:                  "test",
			Flavor:                 "test",
			OSType:                 "linux",
			OSArch:                 "arm64",
			Tags:                   []string{"arm64-linux-runner"},
			RunnerBootstrapTimeout: 0,
		},
		UpdateEntityParams: params.UpdateEntityParams{
			CredentialsName: s.giteaCreds2.Name,
			WebhookSecret:   "updated-webhook-secret",
		},
		UpdatePoolParams: params.UpdatePoolParams{
			MaxRunners:     &maxRunners,
			MinIdleRunners: &minIdleRunners,
			Image:          "test-images-updated",
			Flavor:         "test-flavor-updated",
		},
		ErrMock:         fmt.Errorf("mock error"),
		ProviderMock:    providerMock,
		PoolMgrMock:     runnerCommonMocks.NewPoolManager(s.T()),
		PoolMgrCtrlMock: runnerMocks.NewPoolManagerController(s.T()),
	}
	s.Fixtures = fixtures

	runner := &Runner{
		providers:       fixtures.Providers,
		ctx:             fixtures.AdminContext,
		store:           fixtures.Store,
		poolManagerCtrl: fixtures.PoolMgrCtrlMock,
	}
	s.Runner = runner
}

func (s *ForgeInstanceTestSuite) createSecondEndpoint() (params.ForgeEndpoint, params.ForgeCredentials) {
	ep, err := s.Fixtures.Store.CreateGiteaEndpoint(s.Fixtures.AdminContext, params.CreateGiteaEndpointParams{
		Name:       "second-gitea",
		APIBaseURL: "https://gitea2.example.com/api/v1",
		BaseURL:    "https://gitea2.example.com",
	})
	s.Require().Nil(err)
	creds := garmTesting.CreateTestGiteaCredentials(s.Fixtures.AdminContext, "creds-ep2", s.Fixtures.Store, s.T(), ep)
	return ep, creds
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstance() {
	s.Fixtures.PoolMgrMock.On("Start").Return(nil)
	s.Fixtures.PoolMgrCtrlMock.On("CreateForgeInstancePoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.ForgeInstance"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, nil)

	ep2, creds2 := s.createSecondEndpoint()
	createParams := params.CreateForgeInstanceParams{
		EndpointName:     ep2.Name,
		CredentialsName:  creds2.Name,
		WebhookSecret:    "new-secret",
		ForgeType:        params.GiteaEndpointType,
		PoolBalancerType: params.PoolBalancerTypeRoundRobin,
	}

	fi, err := s.Runner.CreateForgeInstance(s.Fixtures.AdminContext, createParams)

	s.Require().Nil(err)
	s.Require().Equal(ep2.Name, fi.Endpoint.Name)
	s.Require().Equal(creds2.Name, fi.Credentials.Name)
	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstanceErrUnauthorized() {
	_, err := s.Runner.CreateForgeInstance(context.Background(), s.Fixtures.CreateForgeInstanceParams)

	s.Require().Equal(runnerErrors.ErrUnauthorized, err)
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstanceEmptyParams() {
	_, err := s.Runner.CreateForgeInstance(s.Fixtures.AdminContext, params.CreateForgeInstanceParams{})

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "validating params")
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstanceMissingCredentials() {
	createParams := s.Fixtures.CreateForgeInstanceParams
	createParams.CredentialsName = notExistingCredentialsName

	_, err := s.Runner.CreateForgeInstance(s.Fixtures.AdminContext, createParams)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "not defined")
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstanceAlreadyExists() {
	s.Fixtures.PoolMgrCtrlMock.On("CreateForgeInstancePoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.ForgeInstance"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, nil).Maybe()
	s.Fixtures.PoolMgrMock.On("Start").Return(nil).Maybe()

	_, err := s.Runner.CreateForgeInstance(s.Fixtures.AdminContext, s.Fixtures.CreateForgeInstanceParams)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "already exists")
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstanceGithubRejected() {
	ghEndpoint := garmTesting.CreateDefaultGithubEndpoint(s.Fixtures.AdminContext, s.Fixtures.Store, s.T())
	ghCreds := garmTesting.CreateTestGithubCredentials(s.Fixtures.AdminContext, "gh-creds", s.Fixtures.Store, s.T(), ghEndpoint)

	createParams := params.CreateForgeInstanceParams{
		EndpointName:     ghEndpoint.Name,
		CredentialsName:  ghCreds.Name,
		WebhookSecret:    "secret",
		ForgeType:        params.GithubEndpointType,
		PoolBalancerType: params.PoolBalancerTypeRoundRobin,
	}

	_, err := s.Runner.CreateForgeInstance(s.Fixtures.AdminContext, createParams)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "does not support instance-level pools")
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstancePoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("CreateForgeInstancePoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.ForgeInstance"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, s.Fixtures.ErrMock)

	ep2, creds := s.createSecondEndpoint()
	createParams := params.CreateForgeInstanceParams{
		EndpointName:     ep2.Name,
		CredentialsName:  creds.Name,
		WebhookSecret:    "secret",
		ForgeType:        params.GiteaEndpointType,
		PoolBalancerType: params.PoolBalancerTypeRoundRobin,
	}

	_, err := s.Runner.CreateForgeInstance(s.Fixtures.AdminContext, createParams)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "error creating forge instance pool manager")
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstanceStartPoolMgrFailed() {
	s.Fixtures.PoolMgrMock.On("Start").Return(s.Fixtures.ErrMock)
	s.Fixtures.PoolMgrCtrlMock.On("CreateForgeInstancePoolManager", s.Fixtures.AdminContext, mock.AnythingOfType("params.ForgeInstance"), s.Fixtures.Providers, s.Fixtures.Store).Return(s.Fixtures.PoolMgrMock, nil)
	s.Fixtures.PoolMgrCtrlMock.On("DeleteForgeInstancePoolManager", mock.AnythingOfType("params.ForgeInstance")).Return(nil)

	ep2, creds := s.createSecondEndpoint()
	createParams := params.CreateForgeInstanceParams{
		EndpointName:     ep2.Name,
		CredentialsName:  creds.Name,
		WebhookSecret:    "secret",
		ForgeType:        params.GiteaEndpointType,
		PoolBalancerType: params.PoolBalancerTypeRoundRobin,
	}

	_, err := s.Runner.CreateForgeInstance(s.Fixtures.AdminContext, createParams)

	s.Fixtures.PoolMgrMock.AssertExpectations(s.T())
	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "error starting forge instance pool manager")
}

func (s *ForgeInstanceTestSuite) TestListForgeInstances() {
	instances, err := s.Runner.ListForgeInstances(s.Fixtures.AdminContext, params.ForgeInstanceFilter{})

	s.Require().Nil(err)
	s.Require().Len(instances, 1)
	s.Require().Equal(s.Fixtures.StoreForgeInstances[s.giteaEndpoint.Name].ID, instances[0].ID)
}

func (s *ForgeInstanceTestSuite) TestGetForgeInstanceByID() {
	fi := s.Fixtures.StoreForgeInstances[s.giteaEndpoint.Name]
	result, err := s.Runner.GetForgeInstanceByID(s.Fixtures.AdminContext, fi.ID)

	s.Require().Nil(err)
	s.Require().Equal(fi.ID, result.ID)
}

func (s *ForgeInstanceTestSuite) TestDeleteForgeInstance() {
	s.Fixtures.PoolMgrCtrlMock.On("DeleteForgeInstancePoolManager", mock.AnythingOfType("params.ForgeInstance")).Return(nil)

	fi := s.Fixtures.StoreForgeInstances[s.giteaEndpoint.Name]
	err := s.Runner.DeleteForgeInstance(s.Fixtures.AdminContext, fi.ID, false)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Nil(err)

	_, err = s.Fixtures.Store.GetForgeInstanceByID(s.Fixtures.AdminContext, fi.ID)
	s.Require().Contains(err.Error(), "not found")
}

func (s *ForgeInstanceTestSuite) TestDeleteForgeInstancePoolDefinedFailed() {
	fi := s.Fixtures.StoreForgeInstances[s.giteaEndpoint.Name]
	entity, err := fi.GetEntity()
	s.Require().Nil(err)

	pool, err := s.Fixtures.Store.CreateEntityPool(s.Fixtures.AdminContext, entity, s.Fixtures.CreatePoolParams)
	s.Require().Nil(err)

	err = s.Runner.DeleteForgeInstance(s.Fixtures.AdminContext, fi.ID, false)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), pool.ID)
}

func (s *ForgeInstanceTestSuite) TestDeleteForgeInstancePoolMgrFailed() {
	s.Fixtures.PoolMgrCtrlMock.On("DeleteForgeInstancePoolManager", mock.AnythingOfType("params.ForgeInstance")).Return(s.Fixtures.ErrMock)

	fi := s.Fixtures.StoreForgeInstances[s.giteaEndpoint.Name]
	err := s.Runner.DeleteForgeInstance(s.Fixtures.AdminContext, fi.ID, false)

	s.Fixtures.PoolMgrCtrlMock.AssertExpectations(s.T())
	s.Require().Contains(err.Error(), "error deleting forge instance pool manager")
}

func (s *ForgeInstanceTestSuite) TestUpdateForgeInstance() {
	fi := s.Fixtures.StoreForgeInstances[s.giteaEndpoint.Name]
	param := s.Fixtures.UpdateEntityParams
	param.PoolBalancerType = params.PoolBalancerTypePack

	result, err := s.Runner.UpdateForgeInstance(s.Fixtures.AdminContext, fi.ID, param)

	s.Require().Nil(err)
	s.Require().Equal(s.giteaCreds2.Name, result.Credentials.Name)
	s.Require().Equal("updated-webhook-secret", result.WebhookSecret)
	s.Require().Equal(params.PoolBalancerTypePack, result.PoolBalancerType)
}

func (s *ForgeInstanceTestSuite) TestUpdateForgeInstanceInvalidBalancerType() {
	fi := s.Fixtures.StoreForgeInstances[s.giteaEndpoint.Name]
	param := params.UpdateEntityParams{
		PoolBalancerType: "invalid-balancer",
	}

	_, err := s.Runner.UpdateForgeInstance(s.Fixtures.AdminContext, fi.ID, param)

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "invalid pool balancer type")
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstancePool() {
	fi := s.Fixtures.StoreForgeInstances[s.giteaEndpoint.Name]
	pool, err := s.Runner.CreateForgeInstancePool(s.Fixtures.AdminContext, fi.ID, s.Fixtures.CreatePoolParams)

	s.Require().Nil(err)
	s.Require().Equal(fi.ID, pool.ForgeInstanceID)

	result, err := s.Fixtures.Store.GetForgeInstanceByID(s.Fixtures.AdminContext, fi.ID)
	s.Require().Nil(err)
	s.Require().Len(result.Pools, 1)
	s.Require().Equal(pool.ID, result.Pools[0].ID)
}

func (s *ForgeInstanceTestSuite) TestGetForgeInstancePoolByID() {
	fi := s.Fixtures.StoreForgeInstances[s.giteaEndpoint.Name]
	pool, err := s.Runner.CreateForgeInstancePool(s.Fixtures.AdminContext, fi.ID, s.Fixtures.CreatePoolParams)
	s.Require().Nil(err)

	result, err := s.Runner.GetForgeInstancePoolByID(s.Fixtures.AdminContext, fi.ID, pool.ID)

	s.Require().Nil(err)
	s.Require().Equal(pool.ID, result.ID)
}

func (s *ForgeInstanceTestSuite) TestDeleteForgeInstancePool() {
	fi := s.Fixtures.StoreForgeInstances[s.giteaEndpoint.Name]
	pool, err := s.Runner.CreateForgeInstancePool(s.Fixtures.AdminContext, fi.ID, s.Fixtures.CreatePoolParams)
	s.Require().Nil(err)

	err = s.Runner.DeleteForgeInstancePool(s.Fixtures.AdminContext, fi.ID, pool.ID)

	s.Require().Nil(err)
	pools, err := s.Runner.ListForgeInstancePools(s.Fixtures.AdminContext, fi.ID)
	s.Require().Nil(err)
	s.Require().Len(pools, 0)
}

func (s *ForgeInstanceTestSuite) TestUpdateForgeInstancePool() {
	fi := s.Fixtures.StoreForgeInstances[s.giteaEndpoint.Name]
	pool, err := s.Runner.CreateForgeInstancePool(s.Fixtures.AdminContext, fi.ID, s.Fixtures.CreatePoolParams)
	s.Require().Nil(err)

	result, err := s.Runner.UpdateForgeInstancePool(s.Fixtures.AdminContext, fi.ID, pool.ID, s.Fixtures.UpdatePoolParams)

	s.Require().Nil(err)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MaxRunners, result.MaxRunners)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MinIdleRunners, result.MinIdleRunners)
	s.Require().Equal(s.Fixtures.UpdatePoolParams.Image, result.Image)
}

func (s *ForgeInstanceTestSuite) TestListForgeInstancePools() {
	fi := s.Fixtures.StoreForgeInstances[s.giteaEndpoint.Name]
	_, err := s.Runner.CreateForgeInstancePool(s.Fixtures.AdminContext, fi.ID, s.Fixtures.CreatePoolParams)
	s.Require().Nil(err)

	pools, err := s.Runner.ListForgeInstancePools(s.Fixtures.AdminContext, fi.ID)

	s.Require().Nil(err)
	s.Require().Len(pools, 1)
}

func TestForgeInstanceTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ForgeInstanceTestSuite))
}
