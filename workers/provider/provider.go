package provider

import (
	"context"
	"fmt"
	"sync"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/runner/common"
)

func NewWorker(ctx context.Context, store dbCommon.Store, providers map[string]common.Provider) (*provider, error) {
	consumerID := "provider-worker"
	return &provider{
		ctx:        context.Background(),
		store:      store,
		consumerID: consumerID,
		providers:  providers,
	}, nil
}

type provider struct {
	ctx        context.Context
	consumerID string

	consumer dbCommon.Consumer
	// TODO: not all workers should have access to the store.
	// We need to implement way to RPC from workers to controllers
	// and abstract that into something we can use to eventually
	// scale out.
	store dbCommon.Store

	providers map[string]common.Provider

	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func (p *provider) Start() error {
	p.mux.Lock()
	defer p.mux.Unlock()

	if p.running {
		return nil
	}

	consumer, err := watcher.RegisterConsumer(
		p.ctx, p.consumerID, composeProviderWatcher())
	if err != nil {
		return fmt.Errorf("registering consumer: %w", err)
	}
	p.consumer = consumer

	p.quit = make(chan struct{})
	p.running = true
	return nil
}

func (p *provider) Stop() error {
	p.mux.Lock()
	defer p.mux.Unlock()

	if !p.running {
		return nil
	}

	p.consumer.Close()
	close(p.quit)
	p.running = false
	return nil
}
