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
