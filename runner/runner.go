package runner

import (
	"runner-manager/config"
	"runner-manager/runner/common"

	"github.com/google/go-github/github"
)

type Runner struct {
	ghc *github.Client

	config *config.Config
	pools  []common.PoolManager
}
