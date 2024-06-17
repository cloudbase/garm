//go:build testing
// +build testing

package testing

import (
	"context"

	"github.com/cloudbase/garm/database/common"
)

type MockWatcher struct{}

func (w *MockWatcher) RegisterProducer(_ context.Context, _ string) (common.Producer, error) {
	return &MockProducer{}, nil
}

func (w *MockWatcher) RegisterConsumer(_ context.Context, _ string, _ ...common.PayloadFilterFunc) (common.Consumer, error) {
	return &MockConsumer{}, nil
}

func (w *MockWatcher) Close() {
}

type MockProducer struct{}

func (p *MockProducer) Notify(_ common.ChangePayload) error {
	return nil
}

func (p *MockProducer) IsClosed() bool {
	return false
}

func (p *MockProducer) Close() {
}

type MockConsumer struct{}

func (c *MockConsumer) Watch() <-chan common.ChangePayload {
	return nil
}

func (c *MockConsumer) SetFilters(_ ...common.PayloadFilterFunc) {
}

func (c *MockConsumer) Close() {
}

func (c *MockConsumer) IsClosed() bool {
	return false
}
