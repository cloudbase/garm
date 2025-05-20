//go:build testing
// +build testing

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
