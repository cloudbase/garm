package watcher

import (
	"context"
	"log/slog"
	"sync"

	"github.com/pkg/errors"

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
		return nil, errors.Wrapf(common.ErrProducerAlreadyRegistered, "producer_id: %s", id)
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
