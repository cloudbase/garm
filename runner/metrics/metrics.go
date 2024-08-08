package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner"
)

func CollectObjectMetric(ctx context.Context, r *runner.Runner, duration time.Duration) {
	ctx = auth.GetAdminContext(ctx)

	// get controller info for health metrics
	controllerInfo, err := r.GetControllerInfo(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot get controller info")
	}

	// we do not want to wait until the first ticker happens
	// for that we start an initial collection immediately
	slog.DebugContext(ctx, "collecting metrics")
	if err := collectMetrics(ctx, r, controllerInfo); err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot collect metrics")
	}

	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				slog.DebugContext(ctx, "collecting metrics")

				if err := collectMetrics(ctx, r, controllerInfo); err != nil {
					slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot collect metrics")
				}
			}
		}
	}()
}

func collectMetrics(ctx context.Context, r *runner.Runner, controllerInfo params.ControllerInfo) error {
	slog.DebugContext(ctx, "collecting organization metrics")
	err := CollectOrganizationMetric(ctx, r)
	if err != nil {
		return err
	}

	slog.DebugContext(ctx, "collecting enterprise metrics")
	err = CollectEnterpriseMetric(ctx, r)
	if err != nil {
		return err
	}

	slog.DebugContext(ctx, "collecting repository metrics")
	err = CollectRepositoryMetric(ctx, r)
	if err != nil {
		return err
	}

	slog.DebugContext(ctx, "collecting provider metrics")
	err = CollectProviderMetric(ctx, r)
	if err != nil {
		return err
	}

	slog.DebugContext(ctx, "collecting pool metrics")
	err = CollectPoolMetric(ctx, r)
	if err != nil {
		return err
	}

	slog.DebugContext(ctx, "collecting instance metrics")
	err = CollectInstanceMetric(ctx, r)
	if err != nil {
		return err
	}

	slog.DebugContext(ctx, "collecting health metrics")
	err = CollectHealthMetric(controllerInfo)
	if err != nil {
		return err
	}
	return nil
}
