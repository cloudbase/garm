package providers

import (
	"context"
	"runner-manager/config"
	"runner-manager/runner/common"
	"runner-manager/runner/providers/lxd"

	"github.com/pkg/errors"
)

// LoadProvidersFromConfig loads all providers from the config and populates
// a map with them.
func LoadProvidersFromConfig(ctx context.Context, cfg config.Config, controllerID string) (map[string]common.Provider, error) {
	providers := map[string]common.Provider{}
	for _, providerCfg := range cfg.Providers {
		switch providerCfg.ProviderType {
		case config.LXDProvider:
			provider, err := lxd.NewProvider(ctx, &providerCfg, controllerID)
			if err != nil {
				return nil, errors.Wrap(err, "creating provider")
			}
			providers[providerCfg.Name] = provider
		}
	}
	return providers, nil
}
