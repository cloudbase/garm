package providers

import (
	"context"
	"log"
	"garm/config"
	"garm/runner/common"
	"garm/runner/providers/lxd"

	"github.com/pkg/errors"
)

// LoadProvidersFromConfig loads all providers from the config and populates
// a map with them.
func LoadProvidersFromConfig(ctx context.Context, cfg config.Config, controllerID string) (map[string]common.Provider, error) {
	providers := make(map[string]common.Provider, len(cfg.Providers))
	for _, providerCfg := range cfg.Providers {
		log.Printf("Loading provider %s", providerCfg.Name)
		switch providerCfg.ProviderType {
		case config.LXDProvider:
			conf := providerCfg
			provider, err := lxd.NewProvider(ctx, &conf, controllerID)
			if err != nil {
				return nil, errors.Wrap(err, "creating provider")
			}
			providers[providerCfg.Name] = provider
		}
	}
	return providers, nil
}
