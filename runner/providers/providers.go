// Copyright 2022 Cloudbase Solutions SRL
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

package providers

import (
	"context"
	"log"

	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/runner/providers/external"

	"github.com/pkg/errors"
)

// LoadProvidersFromConfig loads all providers from the config and populates
// a map with them.
func LoadProvidersFromConfig(ctx context.Context, cfg config.Config, controllerID string) (map[string]common.Provider, error) {
	providers := make(map[string]common.Provider, len(cfg.Providers))
	for _, providerCfg := range cfg.Providers {
		log.Printf("Loading provider %s", providerCfg.Name)
		switch providerCfg.ProviderType {
		case params.ExternalProvider:
			conf := providerCfg
			provider, err := external.NewProvider(ctx, &conf, controllerID)
			if err != nil {
				return nil, errors.Wrap(err, "creating provider")
			}
			providers[providerCfg.Name] = provider
		default:
			return nil, errors.Errorf("unknown provider type %s", providerCfg.ProviderType)
		}
	}
	return providers, nil
}
