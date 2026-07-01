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

package sql

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type ForgeInstanceTestFixtures struct {
	ForgeInstances       []params.ForgeInstance
	CreatePoolParams     params.CreatePoolParams
	CreateInstanceParams params.CreateInstanceParams
	UpdateEntityParams   params.UpdateEntityParams
	UpdatePoolParams     params.UpdatePoolParams
}

type ForgeInstanceTestSuite struct {
	suite.Suite
	Store    dbCommon.Store
	Fixtures *ForgeInstanceTestFixtures

	adminCtx      context.Context
	adminUserID   string
	giteaEndpoint params.ForgeEndpoint
	giteaCreds    params.ForgeCredentials
	giteaCreds2   params.ForgeCredentials
}

func (s *ForgeInstanceTestSuite) equalInstancesByName(expected, actual []params.Instance) {
	s.Require().Equal(len(expected), len(actual))
	sort.Slice(expected, func(i, j int) bool { return expected[i].Name > expected[j].Name })
	sort.Slice(actual, func(i, j int) bool { return actual[i].Name > actual[j].Name })
	for i := 0; i < len(expected); i++ {
		s.Require().Equal(expected[i].Name, actual[i].Name)
	}
}

func (s *ForgeInstanceTestSuite) TearDownTest() {
	watcher.CloseWatcher()
}

func (s *ForgeInstanceTestSuite) SetupTest() {
	ctx := context.Background()
	watcher.InitWatcher(ctx)

	db := newTestDB(s.T())
	s.Store = db

	adminCtx := garmTesting.ImpersonateAdminContext(ctx, db, s.T())
	s.adminCtx = adminCtx
	s.adminUserID = auth.UserID(adminCtx)
	s.Require().NotEmpty(s.adminUserID)

	s.giteaEndpoint = garmTesting.CreateDefaultGiteaEndpoint(adminCtx, db, s.T())
	s.giteaCreds = garmTesting.CreateTestGiteaCredentials(adminCtx, "gitea-creds", db, s.T(), s.giteaEndpoint)
	s.giteaCreds2 = garmTesting.CreateTestGiteaCredentials(adminCtx, "gitea-creds-2", db, s.T(), s.giteaEndpoint)

	// Create test forge instances
	forgeInstances := []params.ForgeInstance{}
	fi, err := db.CreateForgeInstance(
		adminCtx,
		s.giteaEndpoint.Name,
		s.giteaCreds,
		"test-webhook-secret-1",
		params.PoolBalancerTypeRoundRobin,
		false,
	)
	if err != nil {
		s.FailNow(fmt.Sprintf("failed to create forge instance: %q", err))
	}
	forgeInstances = append(forgeInstances, fi)

	var maxRunners uint = 30
	var minIdleRunners uint = 10
	s.Fixtures = &ForgeInstanceTestFixtures{
		ForgeInstances: forgeInstances,
		CreatePoolParams: params.CreatePoolParams{
			ProviderName:   "test-provider",
			MaxRunners:     3,
			MinIdleRunners: 1,
			Enabled:        true,
			Image:          "test-image",
			Flavor:         "test-flavor",
			OSType:         "linux",
			OSArch:         "amd64",
			Tags:           []string{"amd64-linux-runner"},
		},
		CreateInstanceParams: params.CreateInstanceParams{
			Name:   "test-instance-name",
			OSType: "linux",
		},
		UpdateEntityParams: params.UpdateEntityParams{
			CredentialsName: s.giteaCreds2.Name,
			WebhookSecret:   "updated-webhook-secret",
		},
		UpdatePoolParams: params.UpdatePoolParams{
			MaxRunners:     &maxRunners,
			MinIdleRunners: &minIdleRunners,
			Image:          "test-update-image",
			Flavor:         "test-update-flavor",
		},
	}
}

func TestForgeInstanceTestSuite(t *testing.T) {
	suite.Run(t, new(ForgeInstanceTestSuite))
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstance() {
	// Use a second endpoint to avoid unique constraint collision
	ep2, err := s.Store.CreateGiteaEndpoint(s.adminCtx, params.CreateGiteaEndpointParams{
		Name:       "second-gitea",
		APIBaseURL: "https://gitea2.example.com/api/v1",
		BaseURL:    "https://gitea2.example.com",
	})
	s.Require().Nil(err)
	creds2 := garmTesting.CreateTestGiteaCredentials(s.adminCtx, "creds-for-ep2", s.Store, s.T(), ep2)

	fi, err := s.Store.CreateForgeInstance(
		s.adminCtx,
		ep2.Name,
		creds2,
		"new-webhook-secret",
		params.PoolBalancerTypeRoundRobin,
		true,
	)
	s.Require().Nil(err)
	s.Require().NotEmpty(fi.ID)
	s.Require().Equal(ep2.Name, fi.Endpoint.Name)
	s.Require().Equal(creds2.Name, fi.Credentials.Name)
	s.Require().Equal("new-webhook-secret", fi.WebhookSecret)
	s.Require().True(fi.AgentMode)
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstanceMissingSecret() {
	_, err := s.Store.CreateForgeInstance(
		s.adminCtx,
		s.giteaEndpoint.Name,
		s.giteaCreds,
		"",
		params.PoolBalancerTypeRoundRobin,
		false,
	)
	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "missing secret")
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstanceGithubNotSupported() {
	ghEndpoint := garmTesting.CreateDefaultGithubEndpoint(s.adminCtx, s.Store, s.T())
	ghCreds := garmTesting.CreateTestGithubCredentials(s.adminCtx, "gh-creds", s.Store, s.T(), ghEndpoint)

	_, err := s.Store.CreateForgeInstance(
		s.adminCtx,
		ghEndpoint.Name,
		ghCreds,
		"test-secret",
		params.PoolBalancerTypeRoundRobin,
		false,
	)
	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "only supported for gitea")
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstanceEndpointMismatch() {
	ep2, err := s.Store.CreateGiteaEndpoint(s.adminCtx, params.CreateGiteaEndpointParams{
		Name:       "mismatch-gitea",
		APIBaseURL: "https://other-gitea.example.com/api/v1",
		BaseURL:    "https://other-gitea.example.com",
	})
	s.Require().Nil(err)

	// Credentials are for s.giteaEndpoint, but we pass ep2.Name
	_, err = s.Store.CreateForgeInstance(
		s.adminCtx,
		ep2.Name,
		s.giteaCreds,
		"test-secret",
		params.PoolBalancerTypeRoundRobin,
		false,
	)
	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "does not match")
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstanceDuplicateEndpoint() {
	// First one already exists from SetupTest; creating another should fail
	_, err := s.Store.CreateForgeInstance(
		s.adminCtx,
		s.giteaEndpoint.Name,
		s.giteaCreds,
		"another-secret",
		params.PoolBalancerTypeRoundRobin,
		false,
	)
	s.Require().NotNil(err)
}

func (s *ForgeInstanceTestSuite) TestGetForgeInstance() {
	fi, err := s.Store.GetForgeInstance(s.adminCtx, s.giteaEndpoint.Name)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.ForgeInstances[0].ID, fi.ID)
	s.Require().Equal(s.giteaEndpoint.Name, fi.Endpoint.Name)
}

func (s *ForgeInstanceTestSuite) TestGetForgeInstanceNotFound() {
	_, err := s.Store.GetForgeInstance(s.adminCtx, "nonexistent-endpoint")

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "not found")
}

func (s *ForgeInstanceTestSuite) TestGetForgeInstanceByID() {
	fi, err := s.Store.GetForgeInstanceByID(s.adminCtx, s.Fixtures.ForgeInstances[0].ID)

	s.Require().Nil(err)
	s.Require().Equal(s.Fixtures.ForgeInstances[0].ID, fi.ID)
	s.Require().Equal(s.giteaEndpoint.Name, fi.Endpoint.Name)
	s.Require().Equal(s.giteaCreds.Name, fi.Credentials.Name)
}

func (s *ForgeInstanceTestSuite) TestGetForgeInstanceByIDInvalidID() {
	_, err := s.Store.GetForgeInstanceByID(s.adminCtx, "not-a-uuid")

	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "invalid request")
}

func (s *ForgeInstanceTestSuite) TestListForgeInstances() {
	instances, err := s.Store.ListForgeInstances(s.adminCtx, params.ForgeInstanceFilter{})

	s.Require().Nil(err)
	s.Require().Len(instances, 1)
	s.Require().Equal(s.Fixtures.ForgeInstances[0].ID, instances[0].ID)
}

func (s *ForgeInstanceTestSuite) TestListForgeInstancesWithFilter() {
	instances, err := s.Store.ListForgeInstances(s.adminCtx, params.ForgeInstanceFilter{
		Endpoint: s.giteaEndpoint.Name,
	})
	s.Require().Nil(err)
	s.Require().Len(instances, 1)

	instances, err = s.Store.ListForgeInstances(s.adminCtx, params.ForgeInstanceFilter{
		Endpoint: "nonexistent",
	})
	s.Require().Nil(err)
	s.Require().Len(instances, 0)
}

func (s *ForgeInstanceTestSuite) TestDeleteForgeInstance() {
	err := s.Store.DeleteForgeInstance(s.adminCtx, s.Fixtures.ForgeInstances[0].ID)
	s.Require().Nil(err)

	_, err = s.Store.GetForgeInstanceByID(s.adminCtx, s.Fixtures.ForgeInstances[0].ID)
	s.Require().NotNil(err)
	s.Require().Contains(err.Error(), "not found")
}

func (s *ForgeInstanceTestSuite) TestDeleteForgeInstanceInvalidID() {
	err := s.Store.DeleteForgeInstance(s.adminCtx, "not-a-uuid")
	s.Require().NotNil(err)
}

func (s *ForgeInstanceTestSuite) TestUpdateForgeInstance() {
	fi, err := s.Store.UpdateForgeInstance(
		s.adminCtx,
		s.Fixtures.ForgeInstances[0].ID,
		s.Fixtures.UpdateEntityParams,
	)
	s.Require().Nil(err)
	s.Require().Equal(s.giteaCreds2.Name, fi.Credentials.Name)
	s.Require().Equal("updated-webhook-secret", fi.WebhookSecret)
}

func (s *ForgeInstanceTestSuite) TestUpdateForgeInstanceInvalidID() {
	_, err := s.Store.UpdateForgeInstance(s.adminCtx, "not-a-uuid", s.Fixtures.UpdateEntityParams)
	s.Require().NotNil(err)
}

func (s *ForgeInstanceTestSuite) TestCreateForgeInstancePool() {
	entity, err := s.Fixtures.ForgeInstances[0].GetEntity()
	s.Require().Nil(err)

	pool, err := s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)
	s.Require().Nil(err)
	s.Require().NotEmpty(pool.ID)
	s.Require().Equal(s.Fixtures.ForgeInstances[0].ID, pool.ForgeInstanceID)

	fi, err := s.Store.GetForgeInstanceByID(s.adminCtx, s.Fixtures.ForgeInstances[0].ID)
	s.Require().Nil(err)
	s.Require().Len(fi.Pools, 1)
	s.Require().Equal(pool.ID, fi.Pools[0].ID)
}

func (s *ForgeInstanceTestSuite) TestListForgeInstancePools() {
	entity, err := s.Fixtures.ForgeInstances[0].GetEntity()
	s.Require().Nil(err)

	_, err = s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)
	s.Require().Nil(err)

	pools, err := s.Store.ListEntityPools(s.adminCtx, entity)
	s.Require().Nil(err)
	s.Require().Len(pools, 1)
}

func (s *ForgeInstanceTestSuite) TestGetForgeInstancePool() {
	entity, err := s.Fixtures.ForgeInstances[0].GetEntity()
	s.Require().Nil(err)

	pool, err := s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)
	s.Require().Nil(err)

	fetchedPool, err := s.Store.GetEntityPool(s.adminCtx, entity, pool.ID)
	s.Require().Nil(err)
	s.Require().Equal(pool.ID, fetchedPool.ID)
	s.Require().Equal(s.Fixtures.ForgeInstances[0].ID, fetchedPool.ForgeInstanceID)
}

func (s *ForgeInstanceTestSuite) TestDeleteForgeInstancePool() {
	entity, err := s.Fixtures.ForgeInstances[0].GetEntity()
	s.Require().Nil(err)

	pool, err := s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)
	s.Require().Nil(err)

	err = s.Store.DeleteEntityPool(s.adminCtx, entity, pool.ID)
	s.Require().Nil(err)

	pools, err := s.Store.ListEntityPools(s.adminCtx, entity)
	s.Require().Nil(err)
	s.Require().Len(pools, 0)
}

func (s *ForgeInstanceTestSuite) TestUpdateForgeInstancePool() {
	entity, err := s.Fixtures.ForgeInstances[0].GetEntity()
	s.Require().Nil(err)

	pool, err := s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)
	s.Require().Nil(err)

	updatedPool, err := s.Store.UpdateEntityPool(s.adminCtx, entity, pool.ID, s.Fixtures.UpdatePoolParams)
	s.Require().Nil(err)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MaxRunners, updatedPool.MaxRunners)
	s.Require().Equal(*s.Fixtures.UpdatePoolParams.MinIdleRunners, updatedPool.MinIdleRunners)
	s.Require().Equal(s.Fixtures.UpdatePoolParams.Image, updatedPool.Image)
}

func (s *ForgeInstanceTestSuite) TestListForgeInstanceInstances() {
	entity, err := s.Fixtures.ForgeInstances[0].GetEntity()
	s.Require().Nil(err)

	pool, err := s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)
	s.Require().Nil(err)

	createParams := []params.CreateInstanceParams{}
	for i := 0; i < 3; i++ {
		createParams = append(createParams, params.CreateInstanceParams{
			Name:         fmt.Sprintf("test-fi-instance-%d", i),
			OSType:       "linux",
			OSArch:       "amd64",
			Status:       commonParams.InstanceRunning,
			RunnerStatus: params.RunnerIdle,
		})
	}

	expectedInstances := []params.Instance{}
	for _, cp := range createParams {
		inst, err := s.Store.CreateInstance(s.adminCtx, pool.ID, cp)
		s.Require().Nil(err)
		expectedInstances = append(expectedInstances, inst)
	}

	instances, err := s.Store.ListEntityInstances(s.adminCtx, entity)
	s.Require().Nil(err)
	s.equalInstancesByName(expectedInstances, instances)
}

func (s *ForgeInstanceTestSuite) TestJobRecordsForgeInstanceID() {
	entity, err := s.Fixtures.ForgeInstances[0].GetEntity()
	s.Require().Nil(err)

	entityUUID, err := entity.GetIDAsUUID()
	s.Require().Nil(err)

	job := params.Job{
		WorkflowJobID:   12345,
		Action:          "queued",
		Status:          "queued",
		Name:            "test-job",
		Labels:          []string{"ubuntu"},
		RepositoryName:  "test-repo",
		RepositoryOwner: "test-owner",
		ForgeInstanceID: &entityUUID,
	}

	createdJob, err := s.Store.CreateOrUpdateJob(s.adminCtx, job)
	s.Require().Nil(err)
	s.Require().NotNil(createdJob.ForgeInstanceID)
	s.Require().Equal(entityUUID.String(), createdJob.ForgeInstanceID.String())

	// Verify it appears in entity job listing
	jobs, err := s.Store.ListEntityJobsByStatus(s.adminCtx, params.ForgeEntityTypeInstance, entity.ID, params.JobStatusQueued)
	s.Require().Nil(err)
	s.Require().Len(jobs, 1)
	s.Require().Equal(int64(12345), jobs[0].WorkflowJobID)
}

func (s *ForgeInstanceTestSuite) TestJobBelongsToForgeInstance() {
	entity, err := s.Fixtures.ForgeInstances[0].GetEntity()
	s.Require().Nil(err)

	entityUUID, err := entity.GetIDAsUUID()
	s.Require().Nil(err)

	job := params.Job{
		ForgeInstanceID: &entityUUID,
	}
	s.Require().True(job.BelongsTo(entity))

	otherEntity := params.ForgeEntity{
		ID:         "00000000-0000-0000-0000-000000000000",
		EntityType: params.ForgeEntityTypeInstance,
	}
	s.Require().False(job.BelongsTo(otherEntity))
}

func (s *ForgeInstanceTestSuite) TestAddForgeInstanceEvent() {
	entity, err := s.Fixtures.ForgeInstances[0].GetEntity()
	s.Require().Nil(err)

	err = s.Store.AddEntityEvent(s.adminCtx, entity, params.StatusEvent, params.EventInfo, "test event message", 10)
	s.Require().Nil(err)

	fi, err := s.Store.GetForgeInstanceByID(s.adminCtx, s.Fixtures.ForgeInstances[0].ID)
	s.Require().Nil(err)
	s.Require().Len(fi.Events, 1)
	s.Require().Equal("test event message", fi.Events[0].Message)
}

func (s *ForgeInstanceTestSuite) TestGetForgeEntity() {
	entity, err := s.Store.GetForgeEntity(s.adminCtx, params.ForgeEntityTypeInstance, s.Fixtures.ForgeInstances[0].ID)
	s.Require().Nil(err)
	s.Require().Equal(params.ForgeEntityTypeInstance, entity.EntityType)
	s.Require().Equal(s.Fixtures.ForgeInstances[0].ID, entity.ID)
}

func (s *ForgeInstanceTestSuite) TestPoolBelongsToForgeInstance() {
	entity, err := s.Fixtures.ForgeInstances[0].GetEntity()
	s.Require().Nil(err)

	pool, err := s.Store.CreateEntityPool(s.adminCtx, entity, s.Fixtures.CreatePoolParams)
	s.Require().Nil(err)

	s.Require().Equal(params.ForgeEntityTypeInstance, pool.PoolType())
	s.Require().True(pool.BelongsTo(entity))

	otherEntity := params.ForgeEntity{
		ID:         "00000000-0000-0000-0000-000000000000",
		EntityType: params.ForgeEntityTypeInstance,
	}
	s.Require().False(pool.BelongsTo(otherEntity))
}
