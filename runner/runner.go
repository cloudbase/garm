package runner

import (
	"context"
	"runner-manager/config"
	"runner-manager/runner/common"

	"github.com/google/go-github/github"
)

func NewRunner(ctx context.Context, cfg *config.Config) (*Runner, error) {
	return &Runner{
		ctx:    ctx,
		config: cfg,
	}, nil
}

type Runner struct {
	ctx context.Context
	ghc *github.Client

	config *config.Config
	pools  []common.PoolManager
}
