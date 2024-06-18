package watcher_test

import (
	"context"
	"time"

	"github.com/stretchr/testify/suite"

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
