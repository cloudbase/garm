package scaleset

import (
	"context"
	"sync"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util/github/scalesets"
)

func NewWorker(ctx context.Context, store dbCommon.Store, scaleSet params.ScaleSet, provider common.Provider) (*Worker, error) {
	return &Worker{
		ctx:      ctx,
		store:    store,
		provider: provider,
		Entity:   scaleSet,
	}, nil
}

type Worker struct {
	ctx context.Context

	provider common.Provider
	store    dbCommon.Store
	Entity   params.ScaleSet
	tools    []commonParams.RunnerApplicationDownload

	ghCli       common.GithubClient
	scaleSetCli *scalesets.ScaleSetClient

	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func (w *Worker) Stop() error {
	return nil
}

func (w *Worker) Start() error {
	w.mux.Lock()
	defer w.mux.Unlock()

	go w.loop()
	return nil
}

func (w *Worker) SetTools(tools []commonParams.RunnerApplicationDownload) {
	w.mux.Lock()
	defer w.mux.Unlock()

	w.tools = tools
}

func (w *Worker) SetGithubClient(client common.GithubClient, scaleSetCli *scalesets.ScaleSetClient) error {
	w.mux.Lock()
	defer w.mux.Unlock()

	// TODO:
	// * stop current listener if any

	w.ghCli = client
	w.scaleSetCli = scaleSetCli

	// TODO:
	// * start new listener

	return nil
}

func (w *Worker) loop() {

}
