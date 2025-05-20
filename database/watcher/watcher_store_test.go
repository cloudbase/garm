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

package watcher_test

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type WatcherStoreTestSuite struct {
	suite.Suite

	store common.Store
	ctx   context.Context
}

func (s *WatcherStoreTestSuite) TestJobWatcher() {
	consumer, err := watcher.RegisterConsumer(
		s.ctx, "job-test",
		watcher.WithEntityTypeFilter(common.JobEntityType),
		watcher.WithAny(
			watcher.WithOperationTypeFilter(common.CreateOperation),
			watcher.WithOperationTypeFilter(common.UpdateOperation),
			watcher.WithOperationTypeFilter(common.DeleteOperation)),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	s.T().Cleanup(func() { consumer.Close() })
	consumeEvents(consumer)

	jobParams := params.Job{
		ID:         1,
		RunID:      2,
		Action:     "test-action",
		Conclusion: "started",
		Status:     "in_progress",
		Name:       "test-job",
	}

	job, err := s.store.CreateOrUpdateJob(s.ctx, jobParams)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.JobEntityType,
			Operation:  common.CreateOperation,
			Payload:    job,
		}, event)
		asJob, ok := event.Payload.(params.Job)
		s.Require().True(ok)
		s.Require().Equal(job.ID, int64(1))
		s.Require().Equal(asJob.ID, int64(1))
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	jobParams.Conclusion = "success"
	updatedJob, err := s.store.CreateOrUpdateJob(s.ctx, jobParams)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.JobEntityType,
			Operation:  common.UpdateOperation,
			Payload:    updatedJob,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	entityID, err := uuid.NewUUID()
	s.Require().NoError(err)

	err = s.store.LockJob(s.ctx, updatedJob.ID, entityID.String())
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(event.Operation, common.UpdateOperation)
		s.Require().Equal(event.EntityType, common.JobEntityType)

		job, ok := event.Payload.(params.Job)
		s.Require().True(ok)
		s.Require().Equal(job.ID, updatedJob.ID)
		s.Require().Equal(job.LockedBy, entityID)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	err = s.store.UnlockJob(s.ctx, updatedJob.ID, entityID.String())
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(event.Operation, common.UpdateOperation)
		s.Require().Equal(event.EntityType, common.JobEntityType)

		job, ok := event.Payload.(params.Job)
		s.Require().True(ok)
		s.Require().Equal(job.ID, updatedJob.ID)
		s.Require().Equal(job.LockedBy, uuid.Nil)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	jobParams.Status = "queued"
	jobParams.LockedBy = entityID

	updatedJob, err = s.store.CreateOrUpdateJob(s.ctx, jobParams)
	s.Require().NoError(err)
	// We don't care about the update event here.
	consumeEvents(consumer)

	err = s.store.BreakLockJobIsQueued(s.ctx, updatedJob.ID)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(event.Operation, common.UpdateOperation)
		s.Require().Equal(event.EntityType, common.JobEntityType)

		job, ok := event.Payload.(params.Job)
		s.Require().True(ok)
		s.Require().Equal(job.ID, updatedJob.ID)
		s.Require().Equal(uuid.Nil, job.LockedBy)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}
}

func (s *WatcherStoreTestSuite) TestInstanceWatcher() {
	consumer, err := watcher.RegisterConsumer(
		s.ctx, "instance-test",
		watcher.WithEntityTypeFilter(common.InstanceEntityType),
		watcher.WithAny(
			watcher.WithOperationTypeFilter(common.CreateOperation),
			watcher.WithOperationTypeFilter(common.UpdateOperation),
			watcher.WithOperationTypeFilter(common.DeleteOperation)),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	s.T().Cleanup(func() { consumer.Close() })
	consumeEvents(consumer)

	ep := garmTesting.CreateDefaultGithubEndpoint(s.ctx, s.store, s.T())
	creds := garmTesting.CreateTestGithubCredentials(s.ctx, "test-creds", s.store, s.T(), ep)
	s.T().Cleanup(func() { s.store.DeleteGithubCredentials(s.ctx, creds.ID) })

	repo, err := s.store.CreateRepository(s.ctx, "test-owner", "test-repo", creds, "test-secret", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotEmpty(repo.ID)
	s.T().Cleanup(func() { s.store.DeleteRepository(s.ctx, repo.ID) })

	entity, err := repo.GetEntity()
	s.Require().NoError(err)

	createPoolParams := params.CreatePoolParams{
		ProviderName: "test-provider",
		Image:        "test-image",
		Flavor:       "test-flavor",
		OSType:       commonParams.Linux,
		OSArch:       commonParams.Amd64,
		Tags:         []string{"test-tag"},
	}

	pool, err := s.store.CreateEntityPool(s.ctx, entity, createPoolParams)
	s.Require().NoError(err)
	s.Require().NotEmpty(pool.ID)
	s.T().Cleanup(func() { s.store.DeleteEntityPool(s.ctx, entity, pool.ID) })

	createInstanceParams := params.CreateInstanceParams{
		Name:   "test-instance",
		OSType: commonParams.Linux,
		OSArch: commonParams.Amd64,
		Status: commonParams.InstanceCreating,
	}
	instance, err := s.store.CreateInstance(s.ctx, pool.ID, createInstanceParams)
	s.Require().NoError(err)
	s.Require().NotEmpty(instance.ID)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.InstanceEntityType,
			Operation:  common.CreateOperation,
			Payload:    instance,
		}, event)
		asInstance, ok := event.Payload.(params.Instance)
		s.Require().True(ok)
		s.Require().Equal(instance.Name, "test-instance")
		s.Require().Equal(asInstance.Name, "test-instance")
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	updateParams := params.UpdateInstanceParams{
		RunnerStatus: params.RunnerActive,
	}

	updatedInstance, err := s.store.UpdateInstance(s.ctx, instance.Name, updateParams)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.InstanceEntityType,
			Operation:  common.UpdateOperation,
			Payload:    updatedInstance,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	err = s.store.DeleteInstance(s.ctx, pool.ID, updatedInstance.Name)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.InstanceEntityType,
			Operation:  common.DeleteOperation,
			Payload: params.Instance{
				ID:         updatedInstance.ID,
				Name:       updatedInstance.Name,
				ProviderID: updatedInstance.ProviderID,
				AgentID:    updatedInstance.AgentID,
				PoolID:     updatedInstance.PoolID,
			},
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}
}

func (s *WatcherStoreTestSuite) TestScaleSetInstanceWatcher() {
	consumer, err := watcher.RegisterConsumer(
		s.ctx, "instance-test",
		watcher.WithEntityTypeFilter(common.InstanceEntityType),
		watcher.WithAny(
			watcher.WithOperationTypeFilter(common.CreateOperation),
			watcher.WithOperationTypeFilter(common.UpdateOperation),
			watcher.WithOperationTypeFilter(common.DeleteOperation)),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	s.T().Cleanup(func() { consumer.Close() })
	consumeEvents(consumer)

	ep := garmTesting.CreateDefaultGithubEndpoint(s.ctx, s.store, s.T())
	creds := garmTesting.CreateTestGithubCredentials(s.ctx, "test-creds", s.store, s.T(), ep)
	s.T().Cleanup(func() { s.store.DeleteGithubCredentials(s.ctx, creds.ID) })

	repo, err := s.store.CreateRepository(s.ctx, "test-owner", "test-repo", creds, "test-secret", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotEmpty(repo.ID)
	s.T().Cleanup(func() { s.store.DeleteRepository(s.ctx, repo.ID) })

	entity, err := repo.GetEntity()
	s.Require().NoError(err)

	createScaleSetParams := params.CreateScaleSetParams{
		ProviderName:   "test-provider",
		Name:           "test-scaleset",
		Image:          "test-image",
		Flavor:         "test-flavor",
		MinIdleRunners: 0,
		MaxRunners:     1,
		OSType:         commonParams.Linux,
		OSArch:         commonParams.Amd64,
	}

	scaleSet, err := s.store.CreateEntityScaleSet(s.ctx, entity, createScaleSetParams)
	s.Require().NoError(err)
	s.Require().NotEmpty(scaleSet.ID)
	s.T().Cleanup(func() { s.store.DeleteScaleSetByID(s.ctx, scaleSet.ID) })

	createInstanceParams := params.CreateInstanceParams{
		Name:   "test-instance",
		OSType: commonParams.Linux,
		OSArch: commonParams.Amd64,
		Status: commonParams.InstanceCreating,
	}
	instance, err := s.store.CreateScaleSetInstance(s.ctx, scaleSet.ID, createInstanceParams)
	s.Require().NoError(err)
	s.Require().NotEmpty(instance.ID)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.InstanceEntityType,
			Operation:  common.CreateOperation,
			Payload:    instance,
		}, event)
		asInstance, ok := event.Payload.(params.Instance)
		s.Require().True(ok)
		s.Require().Equal(instance.Name, "test-instance")
		s.Require().Equal(asInstance.Name, "test-instance")
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	updateParams := params.UpdateInstanceParams{
		RunnerStatus: params.RunnerActive,
	}

	updatedInstance, err := s.store.UpdateInstance(s.ctx, instance.Name, updateParams)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.InstanceEntityType,
			Operation:  common.UpdateOperation,
			Payload:    updatedInstance,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	err = s.store.DeleteInstanceByName(s.ctx, updatedInstance.Name)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.InstanceEntityType,
			Operation:  common.DeleteOperation,
			Payload: params.Instance{
				ID:         updatedInstance.ID,
				Name:       updatedInstance.Name,
				ProviderID: updatedInstance.ProviderID,
				AgentID:    updatedInstance.AgentID,
				ScaleSetID: updatedInstance.ScaleSetID,
			},
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}
}

func (s *WatcherStoreTestSuite) TestPoolWatcher() {
	consumer, err := watcher.RegisterConsumer(
		s.ctx, "pool-test",
		watcher.WithEntityTypeFilter(common.PoolEntityType),
		watcher.WithAny(
			watcher.WithOperationTypeFilter(common.CreateOperation),
			watcher.WithOperationTypeFilter(common.UpdateOperation),
			watcher.WithOperationTypeFilter(common.DeleteOperation)),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	s.T().Cleanup(func() { consumer.Close() })
	consumeEvents(consumer)

	ep := garmTesting.CreateDefaultGithubEndpoint(s.ctx, s.store, s.T())
	creds := garmTesting.CreateTestGithubCredentials(s.ctx, "test-creds", s.store, s.T(), ep)
	s.T().Cleanup(func() {
		if err := s.store.DeleteGithubCredentials(s.ctx, creds.ID); err != nil {
			s.T().Logf("failed to delete Github credentials: %v", err)
		}
	})

	repo, err := s.store.CreateRepository(s.ctx, "test-owner", "test-repo", creds, "test-secret", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotEmpty(repo.ID)
	s.T().Cleanup(func() { s.store.DeleteRepository(s.ctx, repo.ID) })

	entity, err := repo.GetEntity()
	s.Require().NoError(err)

	createPoolParams := params.CreatePoolParams{
		ProviderName: "test-provider",
		Image:        "test-image",
		Flavor:       "test-flavor",
		OSType:       commonParams.Linux,
		OSArch:       commonParams.Amd64,
		Tags:         []string{"test-tag"},
	}
	pool, err := s.store.CreateEntityPool(s.ctx, entity, createPoolParams)
	s.Require().NoError(err)
	s.Require().NotEmpty(pool.ID)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.PoolEntityType,
			Operation:  common.CreateOperation,
			Payload:    pool,
		}, event)
		asPool, ok := event.Payload.(params.Pool)
		s.Require().True(ok)
		s.Require().Equal(pool.Image, "test-image")
		s.Require().Equal(asPool.Image, "test-image")
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	updateParams := params.UpdatePoolParams{
		Tags: []string{"updated-tag"},
	}

	updatedPool, err := s.store.UpdateEntityPool(s.ctx, entity, pool.ID, updateParams)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.PoolEntityType,
			Operation:  common.UpdateOperation,
			Payload:    updatedPool,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	err = s.store.DeleteEntityPool(s.ctx, entity, pool.ID)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.PoolEntityType,
			Operation:  common.DeleteOperation,
			Payload:    params.Pool{ID: pool.ID},
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	// Also test DeletePoolByID
	pool, err = s.store.CreateEntityPool(s.ctx, entity, createPoolParams)
	s.Require().NoError(err)
	s.Require().NotEmpty(pool.ID)

	// Consume the create event
	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.PoolEntityType,
			Operation:  common.CreateOperation,
			Payload:    pool,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	err = s.store.DeletePoolByID(s.ctx, pool.ID)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.PoolEntityType,
			Operation:  common.DeleteOperation,
			Payload:    params.Pool{ID: pool.ID},
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}
}

func (s *WatcherStoreTestSuite) TestScaleSetWatcher() {
	consumer, err := watcher.RegisterConsumer(
		s.ctx, "scaleset-test",
		watcher.WithEntityTypeFilter(common.ScaleSetEntityType),
		watcher.WithAny(
			watcher.WithOperationTypeFilter(common.CreateOperation),
			watcher.WithOperationTypeFilter(common.UpdateOperation),
			watcher.WithOperationTypeFilter(common.DeleteOperation)),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	s.T().Cleanup(func() { consumer.Close() })
	consumeEvents(consumer)

	ep := garmTesting.CreateDefaultGithubEndpoint(s.ctx, s.store, s.T())
	creds := garmTesting.CreateTestGithubCredentials(s.ctx, "test-creds", s.store, s.T(), ep)
	s.T().Cleanup(func() {
		if err := s.store.DeleteGithubCredentials(s.ctx, creds.ID); err != nil {
			s.T().Logf("failed to delete Github credentials: %v", err)
		}
	})

	repo, err := s.store.CreateRepository(s.ctx, "test-owner", "test-repo", creds, "test-secret", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotEmpty(repo.ID)
	s.T().Cleanup(func() { s.store.DeleteRepository(s.ctx, repo.ID) })

	entity, err := repo.GetEntity()
	s.Require().NoError(err)

	createScaleSetParams := params.CreateScaleSetParams{
		ProviderName:   "test-provider",
		Name:           "test-scaleset",
		Image:          "test-image",
		Flavor:         "test-flavor",
		MinIdleRunners: 0,
		MaxRunners:     1,
		OSType:         commonParams.Linux,
		OSArch:         commonParams.Amd64,
		Tags:           []string{"test-tag"},
	}
	scaleSet, err := s.store.CreateEntityScaleSet(s.ctx, entity, createScaleSetParams)
	s.Require().NoError(err)
	s.Require().NotEmpty(scaleSet.ID)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.ScaleSetEntityType,
			Operation:  common.CreateOperation,
			Payload:    scaleSet,
		}, event)
		asScaleSet, ok := event.Payload.(params.ScaleSet)
		s.Require().True(ok)
		s.Require().Equal(scaleSet.Image, "test-image")
		s.Require().Equal(asScaleSet.Image, "test-image")
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	updateParams := params.UpdateScaleSetParams{
		Flavor: "updated-flavor",
	}

	callbackFn := func(old, newScaleSet params.ScaleSet) error {
		s.Require().Equal(old.ID, newScaleSet.ID)
		s.Require().Equal(old.Flavor, "test-flavor")
		s.Require().Equal(newScaleSet.Flavor, "updated-flavor")
		return nil
	}
	updatedScaleSet, err := s.store.UpdateEntityScaleSet(s.ctx, entity, scaleSet.ID, updateParams, callbackFn)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.ScaleSetEntityType,
			Operation:  common.UpdateOperation,
			Payload:    updatedScaleSet,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	err = s.store.SetScaleSetLastMessageID(s.ctx, updatedScaleSet.ID, 99)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		asScaleSet, ok := event.Payload.(params.ScaleSet)
		s.Require().True(ok)
		s.Require().Equal(asScaleSet.ID, updatedScaleSet.ID)
		s.Require().Equal(asScaleSet.LastMessageID, int64(99))
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	err = s.store.SetScaleSetDesiredRunnerCount(s.ctx, updatedScaleSet.ID, 5)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		asScaleSet, ok := event.Payload.(params.ScaleSet)
		s.Require().True(ok)
		s.Require().Equal(asScaleSet.ID, updatedScaleSet.ID)
		s.Require().Equal(asScaleSet.DesiredRunnerCount, 5)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	err = s.store.DeleteScaleSetByID(s.ctx, scaleSet.ID)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		// We updated last message ID and desired runner count above.
		updatedScaleSet.DesiredRunnerCount = 5
		updatedScaleSet.LastMessageID = 99
		s.Require().Equal(common.ChangePayload{
			EntityType: common.ScaleSetEntityType,
			Operation:  common.DeleteOperation,
			Payload:    updatedScaleSet,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}
}

func (s *WatcherStoreTestSuite) TestControllerWatcher() {
	consumer, err := watcher.RegisterConsumer(
		s.ctx, "controller-test",
		watcher.WithEntityTypeFilter(common.ControllerEntityType),
		watcher.WithOperationTypeFilter(common.UpdateOperation),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	s.T().Cleanup(func() { consumer.Close() })
	consumeEvents(consumer)

	metadataURL := "http://metadata.example.com"
	updateParams := params.UpdateControllerParams{
		MetadataURL: &metadataURL,
	}

	controller, err := s.store.UpdateController(updateParams)
	s.Require().NoError(err)
	s.Require().Equal(metadataURL, controller.MetadataURL)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.ControllerEntityType,
			Operation:  common.UpdateOperation,
			Payload:    controller,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}
}

func (s *WatcherStoreTestSuite) TestEnterpriseWatcher() {
	consumer, err := watcher.RegisterConsumer(
		s.ctx, "enterprise-test",
		watcher.WithEntityTypeFilter(common.EnterpriseEntityType),
		watcher.WithAny(
			watcher.WithOperationTypeFilter(common.CreateOperation),
			watcher.WithOperationTypeFilter(common.UpdateOperation),
			watcher.WithOperationTypeFilter(common.DeleteOperation)),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	s.T().Cleanup(func() { consumer.Close() })
	consumeEvents(consumer)

	ep := garmTesting.CreateDefaultGithubEndpoint(s.ctx, s.store, s.T())
	creds := garmTesting.CreateTestGithubCredentials(s.ctx, "test-creds", s.store, s.T(), ep)
	s.T().Cleanup(func() { s.store.DeleteGithubCredentials(s.ctx, creds.ID) })

	ent, err := s.store.CreateEnterprise(s.ctx, "test-enterprise", creds, "test-secret", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotEmpty(ent.ID)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.EnterpriseEntityType,
			Operation:  common.CreateOperation,
			Payload:    ent,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	updateParams := params.UpdateEntityParams{
		WebhookSecret: "updated",
	}

	updatedEnt, err := s.store.UpdateEnterprise(s.ctx, ent.ID, updateParams)
	s.Require().NoError(err)
	s.Require().Equal("updated", updatedEnt.WebhookSecret)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.EnterpriseEntityType,
			Operation:  common.UpdateOperation,
			Payload:    updatedEnt,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	err = s.store.DeleteEnterprise(s.ctx, ent.ID)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.EnterpriseEntityType,
			Operation:  common.DeleteOperation,
			Payload:    updatedEnt,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}
}

func (s *WatcherStoreTestSuite) TestOrgWatcher() {
	consumer, err := watcher.RegisterConsumer(
		s.ctx, "org-test",
		watcher.WithEntityTypeFilter(common.OrganizationEntityType),
		watcher.WithAny(
			watcher.WithOperationTypeFilter(common.CreateOperation),
			watcher.WithOperationTypeFilter(common.UpdateOperation),
			watcher.WithOperationTypeFilter(common.DeleteOperation)),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	s.T().Cleanup(func() { consumer.Close() })
	consumeEvents(consumer)

	ep := garmTesting.CreateDefaultGithubEndpoint(s.ctx, s.store, s.T())
	creds := garmTesting.CreateTestGithubCredentials(s.ctx, "test-creds", s.store, s.T(), ep)
	s.T().Cleanup(func() { s.store.DeleteGithubCredentials(s.ctx, creds.ID) })

	org, err := s.store.CreateOrganization(s.ctx, "test-org", creds, "test-secret", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotEmpty(org.ID)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.OrganizationEntityType,
			Operation:  common.CreateOperation,
			Payload:    org,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	updateParams := params.UpdateEntityParams{
		WebhookSecret: "updated",
	}

	updatedOrg, err := s.store.UpdateOrganization(s.ctx, org.ID, updateParams)
	s.Require().NoError(err)
	s.Require().Equal("updated", updatedOrg.WebhookSecret)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.OrganizationEntityType,
			Operation:  common.UpdateOperation,
			Payload:    updatedOrg,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	err = s.store.DeleteOrganization(s.ctx, org.ID)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.OrganizationEntityType,
			Operation:  common.DeleteOperation,
			Payload:    updatedOrg,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}
}

func (s *WatcherStoreTestSuite) TestRepoWatcher() {
	consumer, err := watcher.RegisterConsumer(
		s.ctx, "repo-test",
		watcher.WithEntityTypeFilter(common.RepositoryEntityType),
		watcher.WithAny(
			watcher.WithOperationTypeFilter(common.CreateOperation),
			watcher.WithOperationTypeFilter(common.UpdateOperation),
			watcher.WithOperationTypeFilter(common.DeleteOperation)),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	s.T().Cleanup(func() { consumer.Close() })
	consumeEvents(consumer)

	ep := garmTesting.CreateDefaultGithubEndpoint(s.ctx, s.store, s.T())
	creds := garmTesting.CreateTestGithubCredentials(s.ctx, "test-creds", s.store, s.T(), ep)
	s.T().Cleanup(func() { s.store.DeleteGithubCredentials(s.ctx, creds.ID) })

	repo, err := s.store.CreateRepository(s.ctx, "test-owner", "test-repo", creds, "test-secret", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotEmpty(repo.ID)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.RepositoryEntityType,
			Operation:  common.CreateOperation,
			Payload:    repo,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	newSecret := "updated"
	updateParams := params.UpdateEntityParams{
		WebhookSecret: newSecret,
	}

	updatedRepo, err := s.store.UpdateRepository(s.ctx, repo.ID, updateParams)
	s.Require().NoError(err)
	s.Require().Equal(newSecret, updatedRepo.WebhookSecret)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.RepositoryEntityType,
			Operation:  common.UpdateOperation,
			Payload:    updatedRepo,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	err = s.store.DeleteRepository(s.ctx, repo.ID)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.RepositoryEntityType,
			Operation:  common.DeleteOperation,
			Payload:    updatedRepo,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}
}

func (s *WatcherStoreTestSuite) TestGithubCredentialsWatcher() {
	consumer, err := watcher.RegisterConsumer(
		s.ctx, "gh-cred-test",
		watcher.WithEntityTypeFilter(common.GithubCredentialsEntityType),
		watcher.WithAny(
			watcher.WithOperationTypeFilter(common.CreateOperation),
			watcher.WithOperationTypeFilter(common.UpdateOperation),
			watcher.WithOperationTypeFilter(common.DeleteOperation)),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	s.T().Cleanup(func() { consumer.Close() })
	consumeEvents(consumer)

	ghCredParams := params.CreateGithubCredentialsParams{
		Name:        "test-creds",
		Description: "test credentials",
		Endpoint:    "github.com",
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "bogus",
		},
	}

	ghCred, err := s.store.CreateGithubCredentials(s.ctx, ghCredParams)
	s.Require().NoError(err)
	s.Require().NotEmpty(ghCred.ID)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.GithubCredentialsEntityType,
			Operation:  common.CreateOperation,
			Payload:    ghCred,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	newDesc := "updated description"
	updateParams := params.UpdateGithubCredentialsParams{
		Description: &newDesc,
	}

	updatedGhCred, err := s.store.UpdateGithubCredentials(s.ctx, ghCred.ID, updateParams)
	s.Require().NoError(err)
	s.Require().Equal(newDesc, updatedGhCred.Description)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.GithubCredentialsEntityType,
			Operation:  common.UpdateOperation,
			Payload:    updatedGhCred,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	err = s.store.DeleteGithubCredentials(s.ctx, ghCred.ID)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.GithubCredentialsEntityType,
			Operation:  common.DeleteOperation,
			// We only get the ID and Name of the deleted entity
			Payload: params.ForgeCredentials{ID: ghCred.ID, Name: ghCred.Name},
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}
}

func (s *WatcherStoreTestSuite) TestGiteaCredentialsWatcher() {
	consumer, err := watcher.RegisterConsumer(
		s.ctx, "gitea-cred-test",
		watcher.WithEntityTypeFilter(common.GiteaCredentialsEntityType),
		watcher.WithAny(
			watcher.WithOperationTypeFilter(common.CreateOperation),
			watcher.WithOperationTypeFilter(common.UpdateOperation),
			watcher.WithOperationTypeFilter(common.DeleteOperation)),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	s.T().Cleanup(func() { consumer.Close() })
	consumeEvents(consumer)

	testEndpointParams := params.CreateGiteaEndpointParams{
		Name:        "test",
		Description: "test endpoint",
		APIBaseURL:  "https://api.gitea.example.com",
		BaseURL:     "https://gitea.example.com",
	}

	testEndpoint, err := s.store.CreateGiteaEndpoint(s.ctx, testEndpointParams)
	s.Require().NoError(err)
	s.Require().NotEmpty(testEndpoint.Name)

	s.T().Cleanup(func() {
		if err := s.store.DeleteGiteaEndpoint(s.ctx, testEndpoint.Name); err != nil {
			s.T().Logf("failed to delete Gitea endpoint: %v", err)
		}
		consumeEvents(consumer)
	})

	giteaCredParams := params.CreateGiteaCredentialsParams{
		Name:        "test-creds",
		Description: "test credentials",
		Endpoint:    testEndpoint.Name,
		AuthType:    params.ForgeAuthTypePAT,
		PAT: params.GithubPAT{
			OAuth2Token: "bogus",
		},
	}

	giteaCred, err := s.store.CreateGiteaCredentials(s.ctx, giteaCredParams)
	s.Require().NoError(err)
	s.Require().NotEmpty(giteaCred.ID)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.GiteaCredentialsEntityType,
			Operation:  common.CreateOperation,
			Payload:    giteaCred,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	newDesc := "updated test description"
	updateParams := params.UpdateGiteaCredentialsParams{
		Description: &newDesc,
	}

	updatedGiteaCred, err := s.store.UpdateGiteaCredentials(s.ctx, giteaCred.ID, updateParams)
	s.Require().NoError(err)
	s.Require().Equal(newDesc, updatedGiteaCred.Description)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.GiteaCredentialsEntityType,
			Operation:  common.UpdateOperation,
			Payload:    updatedGiteaCred,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	err = s.store.DeleteGiteaCredentials(s.ctx, giteaCred.ID)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		asCreds, ok := event.Payload.(params.ForgeCredentials)
		s.Require().True(ok)
		s.Require().Equal(event.Operation, common.DeleteOperation)
		s.Require().Equal(event.EntityType, common.GiteaCredentialsEntityType)
		s.Require().Equal(asCreds.ID, updatedGiteaCred.ID)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}
}

func (s *WatcherStoreTestSuite) TestGithubEndpointWatcher() {
	consumer, err := watcher.RegisterConsumer(
		s.ctx, "gh-ep-test",
		watcher.WithEntityTypeFilter(common.GithubEndpointEntityType),
		watcher.WithAny(
			watcher.WithOperationTypeFilter(common.CreateOperation),
			watcher.WithOperationTypeFilter(common.UpdateOperation),
			watcher.WithOperationTypeFilter(common.DeleteOperation)),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	s.T().Cleanup(func() { consumer.Close() })
	consumeEvents(consumer)

	ghEpParams := params.CreateGithubEndpointParams{
		Name:          "test",
		Description:   "test endpoint",
		APIBaseURL:    "https://api.ghes.example.com",
		UploadBaseURL: "https://upload.ghes.example.com",
		BaseURL:       "https://ghes.example.com",
	}

	ghEp, err := s.store.CreateGithubEndpoint(s.ctx, ghEpParams)
	s.Require().NoError(err)
	s.Require().NotEmpty(ghEp.Name)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.GithubEndpointEntityType,
			Operation:  common.CreateOperation,
			Payload:    ghEp,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	newDesc := "updated description"
	updateParams := params.UpdateGithubEndpointParams{
		Description: &newDesc,
	}

	updatedGhEp, err := s.store.UpdateGithubEndpoint(s.ctx, ghEp.Name, updateParams)
	s.Require().NoError(err)
	s.Require().Equal(newDesc, updatedGhEp.Description)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.GithubEndpointEntityType,
			Operation:  common.UpdateOperation,
			Payload:    updatedGhEp,
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	err = s.store.DeleteGithubEndpoint(s.ctx, ghEp.Name)
	s.Require().NoError(err)

	select {
	case event := <-consumer.Watch():
		s.Require().Equal(common.ChangePayload{
			EntityType: common.GithubEndpointEntityType,
			Operation:  common.DeleteOperation,
			// We only get the name of the deleted entity
			Payload: params.ForgeEndpoint{Name: ghEp.Name},
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}
}

func consumeEvents(consumer common.Consumer) {
consume:
	for {
		select {
		case _, ok := <-consumer.Watch():
			// throw away event.
			if !ok {
				return
			}
		case <-time.After(20 * time.Millisecond):
			break consume
		}
	}
}
