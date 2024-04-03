package watcher

import (
	"context"
	"sync"

	"github.com/cloudbase/garm/database/common"
)

var databaseWatcher common.Watcher

func InitWatcher(ctx context.Context) {
	if databaseWatcher != nil {
		return
	}
	w := &watcher{
		producers: make(map[string]*producer),
		consumers: make(map[string]*consumer),
		quit:      make(chan struct{}),
		ctx:       ctx,
	}

	go w.loop()
	databaseWatcher = w
}

func RegisterProducer(id string) (common.Producer, error) {
	if databaseWatcher == nil {
		return nil, common.ErrWatcherNotInitialized
	}
	return databaseWatcher.RegisterProducer(id)
}

func RegisterConsumer(id string, filters ...common.PayloadFilterFunc) (common.Consumer, error) {
	if databaseWatcher == nil {
		return nil, common.ErrWatcherNotInitialized
	}
	return databaseWatcher.RegisterConsumer(id, filters...)
}

type watcher struct {
	producers map[string]*producer
	consumers map[string]*consumer

	mux    sync.Mutex
	closed bool
	quit   chan struct{}
	ctx    context.Context
}

func (w *watcher) RegisterProducer(id string) (common.Producer, error) {
	if _, ok := w.producers[id]; ok {
		return nil, common.ErrProducerAlreadyRegistered
	}
	p := &producer{
		id:       id,
		messages: make(chan common.ChangePayload, 1),
		quit:     make(chan struct{}),
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
		delete(w.producers, prod.id)
	}()
	for {
		select {
		case <-w.quit:
			return
		case <-w.ctx.Done():
			return
		case payload := <-prod.messages:
			for _, c := range w.consumers {
				go c.Send(payload)
			}
		}
	}
}

func (w *watcher) RegisterConsumer(id string, filters ...common.PayloadFilterFunc) (common.Consumer, error) {
	if _, ok := w.consumers[id]; ok {
		return nil, common.ErrConsumerAlreadyRegistered
	}
	c := &consumer{
		messages: make(chan common.ChangePayload, 1),
		filters:  filters,
		quit:     make(chan struct{}),
		id:       id,
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
		delete(w.consumers, consumer.id)
	}()
	for {
		select {
		case <-consumer.quit:
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
