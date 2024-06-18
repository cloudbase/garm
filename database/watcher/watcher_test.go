//go:build testing

package watcher_test

import (
	"context"
	"fmt"
	"testing"
	"time"

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
	fmt.Printf("creating store: %v\n", s.store)
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

func TestWatcherTestSuite(t *testing.T) {
	// Watcher tests
	watcherSuite := &WatcherTestSuite{
		ctx: context.TODO(),
	}
	suite.Run(t, watcherSuite)

	// These tests run store changes and make sure that the store properly
	// triggers watcher notifications.
	ctx := context.TODO()
	watcher.InitWatcher(ctx)

	store, err := database.NewDatabase(ctx, garmTesting.GetTestSqliteDBConfig(t))
	if err != nil {
		t.Fatalf("failed to create db connection: %s", err)
	}
	watcherStoreSuite := &WatcherStoreTestSuite{
		ctx:   context.TODO(),
		store: store,
	}
	suite.Run(t, watcherStoreSuite)
}
