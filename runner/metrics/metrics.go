package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/runner"
)

func CollectObjectMetric(ctx context.Context, r *runner.Runner, ticker *time.Ticker) {
	ctx = auth.GetAdminContext(ctx)

	controllerInfo, err := r.GetControllerInfo(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot get controller info")
	}

	go func() {
		// we wan't to initiate the collection immediately
		for ; true; <-ticker.C {
			select {
			case <-ctx.Done():
				return
			default:
				slog.InfoContext(ctx, "collecting metrics")

				var err error
				slog.DebugContext(ctx, "collecting organization metrics")
				err = CollectOrganizationMetric(ctx, r)
				if err != nil {
					slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot collect organization metrics")
				}

				slog.DebugContext(ctx, "collecting enterprise metrics")
				err = CollectEnterpriseMetric(ctx, r)
				if err != nil {
					slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot collect enterprise metrics")
				}

				slog.DebugContext(ctx, "collecting repository metrics")
				err = CollectRepositoryMetric(ctx, r)
				if err != nil {
					slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot collect repository metrics")
				}

				slog.DebugContext(ctx, "collecting provider metrics")
				err = CollectProviderMetric(ctx, r)
				if err != nil {
					slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot collect provider metrics")
				}

				slog.DebugContext(ctx, "collecting pool metrics")
				err = CollectPoolMetric(ctx, r)
				if err != nil {
					slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot collect pool metrics")
				}

				slog.DebugContext(ctx, "collecting health metrics")
				err = CollectHealthMetric(ctx, r, controllerInfo)
				if err != nil {
					slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot collect health metrics")
				}

				slog.DebugContext(ctx, "collecting instance metrics")
				err = CollectInstanceMetric(ctx, r, controllerInfo)
				if err != nil {
					slog.With(slog.Any("error", err)).ErrorContext(ctx, "cannot collect instance metrics")
				}
			}
		}
	}()
}
