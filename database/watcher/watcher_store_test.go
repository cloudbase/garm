package watcher_test

import (
	"context"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
)

type WatcherStoreTestSuite struct {
	suite.Suite

	store common.Store
	ctx   context.Context
}

func (s *WatcherStoreTestSuite) TestGithubEndpointWatcher() {
	consumer, err := watcher.RegisterConsumer(
		s.ctx, "gh-ep-test",
		watcher.WithEntityTypeFilter(common.GithubEndpointEntityType),
		watcher.WithAny(
			watcher.WithOperationTypeFilter(common.CreateOperation),
			watcher.WithOperationTypeFilter(common.UpdateOperation)),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
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
}
