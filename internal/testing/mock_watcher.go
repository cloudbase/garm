//go:build testing
// +build testing

package testing

import "github.com/cloudbase/garm/database/common"

type MockWatcher struct{}

func (w *MockWatcher) RegisterProducer(_ string) (common.Producer, error) {
	return &MockProducer{}, nil
}

func (w *MockWatcher) RegisterConsumer(_ string, _ ...common.PayloadFilterFunc) (common.Consumer, error) {
	return &MockConsumer{}, nil
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
