package pool

import (
	"context"
	"runner-manager/config"
	"runner-manager/params"
	"runner-manager/runner/common"

	"github.com/google/go-github/v43/github"
)

func NewRepositoryRunnerPool(ctx context.Context, cfg config.Repository, ghcli *github.Client, provider common.Provider) (common.PoolManager, error) {
	return &Repository{
		ctx:      ctx,
		cfg:      cfg,
		ghcli:    ghcli,
		provider: provider,
	}, nil
}

type Repository struct {
	ctx      context.Context
	cfg      config.Repository
	ghcli    *github.Client
	provider common.Provider
}

func (r *Repository) getGithubRunners() ([]github.Runner, error) {
	return nil, nil
}

func (r *Repository) getProviderInstances() ([]params.Instance, error) {
	return nil, nil
}

func (r *Repository) Start() error {
	return nil
}

func (r *Repository) Stop() error {
	return nil
}

func (r *Repository) loop() {

}
