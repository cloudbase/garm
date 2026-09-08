//go:build testing

// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package watcher_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/database"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type WatcherTestSuite struct {
	suite.Suite
	store common.Store
	ctx   context.Context
}

func (s *WatcherTestSuite) SetupTest() {
	ctx := context.TODO()
	watcher.InitWatcher(ctx)
	store, err := database.NewDatabase(ctx, garmTesting.GetTestSqliteDBConfig(s.T()))
	if err != nil {
		s.T().Fatalf("failed to create db connection: %s", err)
	}
	s.store = store
}

func (s *WatcherTestSuite) TearDownTest() {
	s.store = nil
	currentWatcher := watcher.GetWatcher()
	if currentWatcher != nil {
		currentWatcher.Close()
		watcher.SetWatcher(nil)
	}
}

func (s *WatcherTestSuite) TestRegisterConsumerTwiceWillError() {
	consumer, err := watcher.RegisterConsumer(s.ctx, "test")
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	consumer, err = watcher.RegisterConsumer(s.ctx, "test")
	s.Require().ErrorIs(err, common.ErrConsumerAlreadyRegistered)
	s.Require().Nil(consumer)
}

func (s *WatcherTestSuite) TestRegisterProducerTwiceWillError() {
	producer, err := watcher.RegisterProducer(s.ctx, "test")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	producer, err = watcher.RegisterProducer(s.ctx, "test")
	s.Require().ErrorIs(err, common.ErrProducerAlreadyRegistered)
	s.Require().Nil(producer)
}

func (s *WatcherTestSuite) TestInitWatcherRanTwiceDoesNotReplaceWatcher() {
	ctx := context.TODO()
	currentWatcher := watcher.GetWatcher()
	s.Require().NotNil(currentWatcher)
	watcher.InitWatcher(ctx)
	newWatcher := watcher.GetWatcher()
	s.Require().Equal(currentWatcher, newWatcher)
}

func (s *WatcherTestSuite) TestRegisterConsumerFailsIfWatcherIsNotInitialized() {
	s.store = nil
	currentWatcher := watcher.GetWatcher()
	currentWatcher.Close()

	consumer, err := watcher.RegisterConsumer(s.ctx, "test")
	s.Require().Nil(consumer)
	s.Require().ErrorIs(err, common.ErrWatcherNotInitialized)
}

func (s *WatcherTestSuite) TestRegisterProducerFailsIfWatcherIsNotInitialized() {
	s.store = nil
	currentWatcher := watcher.GetWatcher()
	currentWatcher.Close()

	producer, err := watcher.RegisterProducer(s.ctx, "test")
	s.Require().Nil(producer)
	s.Require().ErrorIs(err, common.ErrWatcherNotInitialized)
}

func (s *WatcherTestSuite) TestProducerAndConsumer() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityTypeFilter(common.ControllerEntityType),
		watcher.WithOperationTypeFilter(common.UpdateOperation))
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.ControllerEntityType,
		Operation:  common.UpdateOperation,
		Payload:    "test",
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := <-consumer.Watch()
	s.Require().Equal(payload, receivedPayload)
}

func (s *WatcherTestSuite) TestConsumeWithFilter() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityTypeFilter(common.ControllerEntityType),
		watcher.WithOperationTypeFilter(common.UpdateOperation))
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.ControllerEntityType,
		Operation:  common.UpdateOperation,
		Payload:    "test",
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.ControllerEntityType,
		Operation:  common.CreateOperation,
		Payload:    "test",
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithAnyFilter() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithAny(
			watcher.WithEntityTypeFilter(common.ControllerEntityType),
			watcher.WithEntityFilter(params.ForgeEntity{
				EntityType: params.ForgeEntityTypeRepository,
				Owner:      "test",
				Name:       "test",
				ID:         "test",
			}),
		))
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.ControllerEntityType,
		Operation:  common.UpdateOperation,
		Payload:    "test",
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.RepositoryEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Repository{
			Owner: "test",
			Name:  "test",
			ID:    "test",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	// We're not watching for this repo
	payload = common.ChangePayload{
		EntityType: common.RepositoryEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Repository{
			Owner: "test",
			Name:  "test",
			ID:    "test2",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)

	// We're not watching for orgs
	payload = common.ChangePayload{
		EntityType: common.OrganizationEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Repository{
			Owner: "test",
			Name:  "test",
			ID:    "test2",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithAllFilter() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithAll(
			watcher.WithEntityFilter(params.ForgeEntity{
				EntityType: params.ForgeEntityTypeRepository,
				Owner:      "test",
				Name:       "test",
				ID:         "test",
			}),
			watcher.WithOperationTypeFilter(common.CreateOperation),
		))
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.RepositoryEntityType,
		Operation:  common.CreateOperation,
		Payload: params.Repository{
			Owner: "test",
			Name:  "test",
			ID:    "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.RepositoryEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Repository{
			Owner: "test",
			Name:  "test",
			ID:    "test",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func maybeInitController(db common.Store) error {
	if _, err := db.ControllerInfo(); err == nil {
		return nil
	}

	if _, err := db.InitController(); err != nil {
		return fmt.Errorf("error initializing controller: %w", err)
	}

	return nil
}

func (s *WatcherTestSuite) TestWithEntityPoolFilterRepository() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		EntityType: params.ForgeEntityTypeRepository,
		Owner:      "test",
		Name:       "test",
		ID:         "test",
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityPoolFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.PoolEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Pool{
			ID:     "test",
			RepoID: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.PoolEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Pool{
			ID:     "test",
			RepoID: "test2",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityPoolFilterOrg() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test",
		ID:         "test",
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityPoolFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.PoolEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Pool{
			ID:    "test",
			OrgID: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.PoolEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Pool{
			ID:    "test",
			OrgID: "test2",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityPoolFilterEnterprise() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		EntityType: params.ForgeEntityTypeEnterprise,
		Name:       "test",
		ID:         "test",
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityPoolFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.PoolEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Pool{
			ID:           "test",
			EnterpriseID: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.PoolEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Pool{
			ID:           "test",
			EnterpriseID: "test2",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)

	// Invalid payload for declared entity type
	payload = common.ChangePayload{
		EntityType: common.PoolEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.ScaleSet{
			ID:           1,
			EnterpriseID: "test2",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityPoolFilterBogusEntityType() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		// This should trigger the default branch in the filter and
		// return false
		EntityType: params.ForgeEntityType("bogus"),
		Name:       "test",
		ID:         "test",
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityPoolFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.PoolEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Pool{
			ID:           "test",
			EnterpriseID: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.PoolEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Pool{
			ID:           "test",
			EnterpriseID: "test2",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityScaleSetFilterRepository() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		EntityType: params.ForgeEntityTypeRepository,
		Owner:      "test",
		Name:       "test",
		ID:         "test",
		Credentials: params.ForgeCredentials{
			ForgeType: params.GithubEndpointType,
		},
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityScaleSetFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.ScaleSetEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.ScaleSet{
			ID:     1,
			RepoID: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.ScaleSetEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.ScaleSet{
			ID:     1,
			RepoID: "test2",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityScaleSetFilterOrg() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test",
		ID:         "test",
		Credentials: params.ForgeCredentials{
			ForgeType: params.GithubEndpointType,
		},
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityScaleSetFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.ScaleSetEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.ScaleSet{
			ID:    1,
			OrgID: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.ScaleSetEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.ScaleSet{
			ID:    1,
			OrgID: "test2",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityScaleSetFilterEnterprise() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		EntityType: params.ForgeEntityTypeEnterprise,
		Name:       "test",
		ID:         "test",
		Credentials: params.ForgeCredentials{
			ForgeType: params.GithubEndpointType,
		},
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityScaleSetFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.ScaleSetEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.ScaleSet{
			ID:           1,
			EnterpriseID: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.ScaleSetEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.ScaleSet{
			ID:           1,
			EnterpriseID: "test2",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityScaleSetFilterBogusEntityType() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		// This should trigger the default branch in the filter and
		// return false
		EntityType: params.ForgeEntityType("bogus"),
		Name:       "test",
		ID:         "test",
		Credentials: params.ForgeCredentials{
			ForgeType: params.GithubEndpointType,
		},
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityScaleSetFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.ScaleSetEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.ScaleSet{
			ID:           1,
			EnterpriseID: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.ScaleSetEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.ScaleSet{
			ID:           1,
			EnterpriseID: "test2",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityScaleSetFilterReturnsFalseForGiteaEndpoints() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		EntityType: params.ForgeEntityTypeRepository,
		Owner:      "test",
		Name:       "test",
		ID:         "test",
		Credentials: params.ForgeCredentials{
			ForgeType: params.GiteaEndpointType,
		},
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityScaleSetFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.ScaleSetEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.ScaleSet{
			ID:     1,
			RepoID: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityFilterRepository() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		EntityType: params.ForgeEntityTypeRepository,
		Owner:      "test",
		Name:       "test",
		ID:         "test",
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.RepositoryEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Repository{
			ID:    "test",
			Name:  "test",
			Owner: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.RepositoryEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Repository{
			ID:    "test2",
			Name:  "test",
			Owner: "test",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityFilterOrg() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test",
		ID:         "test",
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.OrganizationEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Organization{
			ID:   "test",
			Name: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.OrganizationEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Organization{
			ID:   "test2",
			Name: "test",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityFilterEnterprise() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		EntityType: params.ForgeEntityTypeEnterprise,
		Name:       "test",
		ID:         "test",
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.EnterpriseEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Enterprise{
			ID:   "test",
			Name: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.EnterpriseEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Enterprise{
			ID:   "test2",
			Name: "test",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityJobFilterRepository() {
	repoUUID, err := uuid.NewUUID()
	s.Require().NoError(err)

	repoUUID2, err := uuid.NewUUID()
	s.Require().NoError(err)
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		EntityType: params.ForgeEntityTypeRepository,
		Owner:      "test",
		Name:       "test",
		ID:         repoUUID.String(),
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityJobFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.JobEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Job{
			ID:     1,
			Name:   "test",
			RepoID: &repoUUID,
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.JobEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Job{
			ID:     1,
			Name:   "test",
			RepoID: &repoUUID2,
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityJobFilterOrg() {
	orgUUID, err := uuid.NewUUID()
	s.Require().NoError(err)

	orgUUID2, err := uuid.NewUUID()
	s.Require().NoError(err)

	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test",
		ID:         orgUUID.String(),
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityJobFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.JobEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Job{
			ID:    1,
			Name:  "test",
			OrgID: &orgUUID,
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.JobEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Job{
			ID:    1,
			Name:  "test",
			OrgID: &orgUUID2,
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityJobFilterEnterprise() {
	entUUID, err := uuid.NewUUID()
	s.Require().NoError(err)

	entUUID2, err := uuid.NewUUID()
	s.Require().NoError(err)

	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		EntityType: params.ForgeEntityTypeEnterprise,
		Name:       "test",
		ID:         entUUID.String(),
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityJobFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.JobEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Job{
			ID:           1,
			Name:         "test",
			EnterpriseID: &entUUID,
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.JobEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Job{
			ID:           1,
			Name:         "test",
			EnterpriseID: &entUUID2,
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithEntityJobFilterBogusEntityType() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	entity := params.ForgeEntity{
		// This should trigger the default branch in the filter and
		// return false
		EntityType: params.ForgeEntityType("bogus"),
		Name:       "test",
		ID:         "test",
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityJobFilter(entity),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.JobEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Job{
			ID:           1,
			Name:         "test",
			EnterpriseID: nil,
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.JobEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Job{
			ID:           1,
			Name:         "test",
			EnterpriseID: nil,
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithNone() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithNone(),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.RepositoryEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Repository{
			ID:    "test",
			Name:  "test",
			Owner: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithUserIDFilter() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	userID, err := uuid.NewUUID()
	s.Require().NoError(err)

	userID2, err := uuid.NewUUID()
	s.Require().NoError(err)

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithUserIDFilter(userID.String()),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.UserEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.User{
			ID: userID.String(),
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.UserEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.User{
			ID: userID2.String(),
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.UserEntityType,
		Operation:  common.UpdateOperation,
		// Declare as user, but payload is a pool. Filter should return false.
		Payload: params.Pool{},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithForgeCredentialsGithub() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	creds := params.ForgeCredentials{
		ForgeType: params.GithubEndpointType,
		ID:        1,
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithForgeCredentialsFilter(creds),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.GithubCredentialsEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.ForgeCredentials{
			ForgeType: params.GithubEndpointType,
			ID:        1,
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.GiteaCredentialsEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.ForgeCredentials{
			ForgeType: params.GiteaEndpointType,
			ID:        1,
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.GiteaCredentialsEntityType,
		Operation:  common.UpdateOperation,
		Payload:    params.Pool{},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithcaleSetFilter() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	scaleSet := params.ScaleSet{
		ID: 1,
	}

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithScaleSetFilter(scaleSet),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.ScaleSetEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.ScaleSet{
			ID:   1,
			Name: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.ScaleSetEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.ScaleSet{
			ID:   2,
			Name: "test",
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.ScaleSetEntityType,
		Operation:  common.UpdateOperation,
		Payload:    params.Pool{},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)
}

func (s *WatcherTestSuite) TestWithExcludeEntityTypeFilter() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithExcludeEntityTypeFilter(common.RepositoryEntityType),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.RepositoryEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Repository{
			ID:    "test",
			Name:  "test",
			Owner: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.OrganizationEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Repository{
			ID:   "test",
			Name: "test",
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)
}

func (s *WatcherTestSuite) TestWithInstanceStatusFilter() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithInstanceStatusFilter(
			commonParams.InstanceCreating,
			commonParams.InstanceDeleting),
	)
	s.Require().NoError(err)
	s.Require().NotNil(consumer)
	consumeEvents(consumer)

	payload := common.ChangePayload{
		EntityType: common.InstanceEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Instance{
			ID:     "test-instance",
			Status: commonParams.InstanceCreating,
		},
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	receivedPayload := waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.InstanceEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Instance{
			ID:     "test-instance",
			Status: commonParams.InstanceDeleted,
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().Nil(receivedPayload)

	payload = common.ChangePayload{
		EntityType: common.InstanceEntityType,
		Operation:  common.UpdateOperation,
		Payload: params.Instance{
			ID:     "test-instance",
			Status: commonParams.InstanceDeleting,
		},
	}

	err = producer.Notify(payload)
	s.Require().NoError(err)
	receivedPayload = waitForPayload(consumer.Watch(), 100*time.Millisecond)
	s.Require().NotNil(receivedPayload)
	s.Require().Equal(payload, *receivedPayload)
}

func (s *WatcherTestSuite) TestBlockedConsumerDoesNotBlockWatcher() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	// This consumer is deliberately wedged: nothing reads from its Watch()
	// channel until the flood below is over.
	blocked, err := watcher.RegisterConsumer(
		s.ctx, "test-blocked-consumer",
		watcher.WithEntityTypeFilter(common.ControllerEntityType),
		watcher.WithOperationTypeFilter(common.UpdateOperation))
	s.Require().NoError(err)
	s.Require().NotNil(blocked)

	healthy, err := watcher.RegisterConsumer(
		s.ctx, "test-healthy-consumer",
		watcher.WithEntityTypeFilter(common.ControllerEntityType),
		watcher.WithOperationTypeFilter(common.UpdateOperation))
	s.Require().NoError(err)
	s.Require().NotNil(healthy)
	consumeEvents(healthy)

	// Send more events than the producer buffer, the consumer hand-off
	// buffer and the messages buffer combined can absorb, so delivery to the
	// wedged consumer must spill into its dispatch queue.
	const eventCount = 1000

	expected := make([]common.ChangePayload, eventCount)
	for i := range eventCount {
		expected[i] = common.ChangePayload{
			EntityType: common.ControllerEntityType,
			Operation:  common.UpdateOperation,
			Payload:    fmt.Sprintf("event-%d", i),
		}
	}

	// The producer must never be throttled by the wedged consumer. With a
	// blocking dispatch design, a full consumer buffer costs up to 1 second
	// per event, so a flood this size would take many minutes; enqueue-based
	// dispatch completes in milliseconds. The generous bound only guards
	// against regressions, not scheduling jitter.
	start := time.Now()
	for i := range eventCount {
		s.Require().NoError(producer.Notify(expected[i]))
	}
	s.Require().Less(time.Since(start), 10*time.Second, "producer was throttled by a blocked consumer")

	// The healthy consumer receives every event, in emission order, while
	// the other consumer is still wedged.
	for i := range eventCount {
		received := waitForPayload(healthy.Watch(), 1*time.Second)
		s.Require().NotNil(received, "healthy consumer did not receive event %d", i)
		s.Require().Equal(expected[i], *received)
	}

	// The wedged consumer lost nothing: once it starts reading, every event
	// is there, in order.
	for i := range eventCount {
		received := waitForPayload(blocked.Watch(), 1*time.Second)
		s.Require().NotNil(received, "blocked consumer lost event %d", i)
		s.Require().Equal(expected[i], *received)
	}
}

func TestWatcherTestSuite(t *testing.T) {
	// Watcher tests
	watcherSuite := &WatcherTestSuite{
		ctx: context.TODO(),
	}
	suite.Run(t, watcherSuite)

	ctx := context.Background()
	watcher.InitWatcher(ctx)

	store, err := database.NewDatabase(ctx, garmTesting.GetTestSqliteDBConfig(t))
	if err != nil {
		t.Fatalf("failed to create db connection: %s", err)
	}

	err = maybeInitController(store)
	if err != nil {
		t.Fatalf("failed to init controller: %s", err)
	}

	adminCtx := garmTesting.ImpersonateAdminContext(ctx, store, t)
	watcherStoreSuite := &WatcherStoreTestSuite{
		ctx:   adminCtx,
		store: store,
	}
	suite.Run(t, watcherStoreSuite)
}
