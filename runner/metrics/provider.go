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

package metrics

import (
	"context"

	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/runner"
)

func CollectProviderMetric(ctx context.Context, r *runner.Runner) error {
	// reset metrics
	metrics.ProviderInfo.Reset()

	providers, err := r.ListProviders(ctx)
	if err != nil {
		return err
	}
	for _, provider := range providers {
		metrics.ProviderInfo.WithLabelValues(
			provider.Name,                 // label: name
			string(provider.ProviderType), // label: type
			provider.Description,          // label: description
		).Set(1)
	}
	return nil
}
