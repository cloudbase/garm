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

	repo, err := s.store.CreateRepository(s.ctx, "test-owner", "test-repo", creds.Name, "test-secret", params.PoolBalancerTypeRoundRobin)
	s.Require().NoError(err)
	s.Require().NotEmpty(repo.ID)
	s.T().Cleanup(func() { s.store.DeleteRepository(s.ctx, repo.ID) })

	entity, err := repo.GetEntity()
	s.Require().NoError(err)

	createPoolParams := params.CreatePoolParams{
		ProviderName: "test-provider",
		Image:        "test-image",
		Flavor:       "test-flavor",
		MaxRunners:   100,
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

	repo, err := s.store.CreateRepository(s.ctx, "test-owner", "test-repo", creds.Name, "test-secret", params.PoolBalancerTypeRoundRobin)
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

	ent, err := s.store.CreateEnterprise(s.ctx, "test-enterprise", creds.Name, "test-secret", params.PoolBalancerTypeRoundRobin)
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

	org, err := s.store.CreateOrganization(s.ctx, "test-org", creds.Name, "test-secret", params.PoolBalancerTypeRoundRobin)
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

	repo, err := s.store.CreateRepository(s.ctx, "test-owner", "test-repo", creds.Name, "test-secret", params.PoolBalancerTypeRoundRobin)
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
		AuthType:    params.GithubAuthTypePAT,
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
			Payload: params.GithubCredentials{ID: ghCred.ID, Name: ghCred.Name},
		}, event)
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
			Payload: params.GithubEndpoint{Name: ghEp.Name},
		}, event)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}
}

func consumeEvents(consumer common.Consumer) {
consume:
	for {
		select {
		case <-consumer.Watch():
			// throw away event.
		case <-time.After(100 * time.Millisecond):
			break consume
		}
	}
}
