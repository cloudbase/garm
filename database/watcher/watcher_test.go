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
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/suite"

	"github.com/cloudbase/garm/database"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
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
	}
}

func (s *WatcherTestSuite) TestRegisterConsumerTwiceWillError() {
	consumer, err := watcher.RegisterConsumer(s.ctx, "test")
	s.Require().NoError(err)
	s.Require().NotNil(consumer)

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

func (s *WatcherTestSuite) TestConsumetWithFilter() {
	producer, err := watcher.RegisterProducer(s.ctx, "test-producer")
	s.Require().NoError(err)
	s.Require().NotNil(producer)

	consumer, err := watcher.RegisterConsumer(
		s.ctx, "test-consumer",
		watcher.WithEntityTypeFilter(common.ControllerEntityType),
		watcher.WithOperationTypeFilter(common.UpdateOperation))
	s.Require().NoError(err)
	s.Require().NotNil(consumer)

	payload := common.ChangePayload{
		EntityType: common.ControllerEntityType,
		Operation:  common.UpdateOperation,
		Payload:    "test",
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	select {
	case receivedPayload := <-consumer.Watch():
		s.Require().Equal(payload, receivedPayload)
	case <-time.After(1 * time.Second):
		s.T().Fatal("expected payload not received")
	}

	payload = common.ChangePayload{
		EntityType: common.ControllerEntityType,
		Operation:  common.CreateOperation,
		Payload:    "test",
	}
	err = producer.Notify(payload)
	s.Require().NoError(err)

	select {
	case <-consumer.Watch():
		s.T().Fatal("unexpected payload received")
	case <-time.After(1 * time.Second):
	}
}

func maybeInitController(db common.Store) error {
	if _, err := db.ControllerInfo(); err == nil {
		return nil
	}

	if _, err := db.InitController(); err != nil {
		return errors.Wrap(err, "initializing controller")
	}

	return nil
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
