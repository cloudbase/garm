// Copyright 2025 Cloudbase Solutions SRL
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

package external

import (
	"context"
	"fmt"

	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/runner/common"
	v010 "github.com/cloudbase/garm/runner/providers/v0.1.0"
	v011 "github.com/cloudbase/garm/runner/providers/v0.1.1"
)

// NewProvider selects the provider based on the interface version
func NewProvider(ctx context.Context, cfg *config.Provider, controllerID string) (common.Provider, error) {
	switch cfg.External.InterfaceVersion {
	case common.Version010, "":
		return v010.NewProvider(ctx, cfg, controllerID)
	case common.Version011:
		return v011.NewProvider(ctx, cfg, controllerID)
	default:
		return nil, fmt.Errorf("unsupported interface version: %s", cfg.External.InterfaceVersion)
	}
}
