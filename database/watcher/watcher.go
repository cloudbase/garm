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

package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cloudbase/garm/database/common"
	garmUtil "github.com/cloudbase/garm/util"
)

var databaseWatcher common.Watcher

func InitWatcher(ctx context.Context) {
	if databaseWatcher != nil {
		return
	}
	ctx = garmUtil.WithSlogContext(ctx, slog.Any("watcher", "database"))
	w := &watcher{
		producers: make(map[string]*producer),
		consumers: make(map[string]*consumer),
		quit:      make(chan struct{}),
		ctx:       ctx,
	}

	go w.loop()
	databaseWatcher = w
}

func CloseWatcher() error {
	if databaseWatcher == nil {
		return nil
	}
	databaseWatcher.Close()
	databaseWatcher = nil
	return nil
}

func RegisterProducer(ctx context.Context, id string) (common.Producer, error) {
	if databaseWatcher == nil {
		return nil, common.ErrWatcherNotInitialized
	}
	ctx = garmUtil.WithSlogContext(ctx, slog.Any("producer_id", id))
	return databaseWatcher.RegisterProducer(ctx, id)
}

func RegisterConsumer(ctx context.Context, id string, filters ...common.PayloadFilterFunc) (common.Consumer, error) {
	if databaseWatcher == nil {
		return nil, common.ErrWatcherNotInitialized
	}
	ctx = garmUtil.WithSlogContext(ctx, slog.Any("consumer_id", id))
	return databaseWatcher.RegisterConsumer(ctx, id, filters...)
}

type watcher struct {
	producers map[string]*producer
	consumers map[string]*consumer

	mux    sync.Mutex
	closed bool
	quit   chan struct{}
	ctx    context.Context
}

func (w *watcher) RegisterProducer(ctx context.Context, id string) (common.Producer, error) {
	w.mux.Lock()
	defer w.mux.Unlock()

	if _, ok := w.producers[id]; ok {
		return nil, fmt.Errorf("producer_id %s: %w", id, common.ErrProducerAlreadyRegistered)
	}
	p := &producer{
		id:       id,
		messages: make(chan common.ChangePayload, 1),
		quit:     make(chan struct{}),
		ctx:      ctx,
	}
	w.producers[id] = p
	go w.serviceProducer(p)
	return p, nil
}

func (w *watcher) serviceProducer(prod *producer) {
	defer func() {
		w.mux.Lock()
		defer w.mux.Unlock()
		prod.Close()
		slog.InfoContext(w.ctx, "removing producer from watcher", "consumer_id", prod.id)
		delete(w.producers, prod.id)
	}()
	for {
		select {
		case <-w.quit:
			slog.InfoContext(w.ctx, "shutting down watcher")
			return
		case <-w.ctx.Done():
			slog.InfoContext(w.ctx, "shutting down watcher")
			return
		case <-prod.quit:
			slog.InfoContext(w.ctx, "closing producer")
			return
		case <-prod.ctx.Done():
			slog.InfoContext(w.ctx, "closing producer")
			return
		case payload := <-prod.messages:
			w.mux.Lock()
			for _, c := range w.consumers {
				go c.Send(payload)
			}
			w.mux.Unlock()
		}
	}
}

func (w *watcher) RegisterConsumer(ctx context.Context, id string, filters ...common.PayloadFilterFunc) (common.Consumer, error) {
	w.mux.Lock()
	defer w.mux.Unlock()
	if _, ok := w.consumers[id]; ok {
		return nil, common.ErrConsumerAlreadyRegistered
	}
	c := &consumer{
		messages: make(chan common.ChangePayload, 1),
		filters:  filters,
		quit:     make(chan struct{}),
		id:       id,
		ctx:      ctx,
	}
	w.consumers[id] = c
	go w.serviceConsumer(c)
	return c, nil
}

func (w *watcher) serviceConsumer(consumer *consumer) {
	defer func() {
		w.mux.Lock()
		defer w.mux.Unlock()
		consumer.Close()
		slog.InfoContext(w.ctx, "removing consumer from watcher", "consumer_id", consumer.id)
		delete(w.consumers, consumer.id)
	}()
	slog.InfoContext(w.ctx, "starting consumer", "consumer_id", consumer.id)
	for {
		select {
		case <-consumer.quit:
			return
		case <-consumer.ctx.Done():
			return
		case <-w.quit:
			return
		case <-w.ctx.Done():
			return
		}
	}
}

func (w *watcher) Close() {
	w.mux.Lock()
	defer w.mux.Unlock()
	if w.closed {
		return
	}

	close(w.quit)
	w.closed = true

	for _, p := range w.producers {
		p.Close()
	}

	for _, c := range w.consumers {
		c.Close()
	}

	databaseWatcher = nil
}

func (w *watcher) loop() {
	defer func() {
		w.Close()
	}()
	for {
		select {
		case <-w.quit:
			return
		case <-w.ctx.Done():
			return
		}
	}
}
