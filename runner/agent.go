package runner

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

func (r *Runner) RecordAgentHeartbeat(ctx context.Context) error {
	instance, err := auth.InstanceParams(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get instance params")
		return runnerErrors.ErrUnauthorized
	}
	now := time.Now().UTC()
	updateParams := params.UpdateInstanceParams{
		Heartbeat: &now,
	}

	if _, err := r.store.UpdateInstance(ctx, instance.Name, updateParams); err != nil {
		return fmt.Errorf("failed to record heartbeat: %w", err)
	}
	return nil
}

func (r *Runner) SetInstanceCapabilities(ctx context.Context, caps params.AgentCapabilities) error {
	instance, err := auth.InstanceParams(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get instance params")
		return runnerErrors.ErrUnauthorized
	}

	updateParams := params.UpdateInstanceParams{
		Capabilities: &caps,
	}

	if _, err := r.store.UpdateInstance(ctx, instance.ID, updateParams); err != nil {
		return fmt.Errorf("failed to update capabilities: %w", err)
	}
	return nil
}

func (r *Runner) SetInstanceToPendingDelete(ctx context.Context) error {
	instance, err := auth.InstanceParams(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get instance params")
		return runnerErrors.ErrUnauthorized
	}

	updateParams := params.UpdateInstanceParams{
		Status: commonParams.InstancePendingDelete,
	}

	if _, err := r.store.UpdateInstance(r.ctx, instance.ID, updateParams); err != nil {
		return fmt.Errorf("failed to set instance to pending_delete: %w", err)
	}
	return nil
}

func (r *Runner) GetAgentJWTToken(ctx context.Context, runnerName string) (string, error) {
	var instance params.Instance
	var err error
	if !auth.IsAdmin(ctx) {
		instance, err = auth.InstanceParams(ctx)
		if err != nil {
			return "", runnerErrors.ErrUnauthorized
		}

		// A runner bootstrap token can get an agent token for itself.
		if instance.Name != runnerName || auth.InstanceIsAgent(ctx) {
			return "", runnerErrors.ErrUnauthorized
		}
	} else {
		instance, err = r.GetInstance(ctx, runnerName)
		if err != nil {
			return "", fmt.Errorf("failed to get runner: %w", err)
		}
	}

	var entityGetter params.EntityGetter
	switch {
	case instance.PoolID != "":
		entityGetter, err = r.GetPoolByID(ctx, instance.PoolID)
	case instance.ScaleSetID != 0:
		entityGetter, err = r.GetScaleSetByID(ctx, instance.ScaleSetID)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get entity: %w", err)
	}

	entity, err := entityGetter.GetEntity()
	if err != nil {
		return "", fmt.Errorf("failed to get entity: %w", err)
	}

	dbEntity, err := r.store.GetForgeEntity(ctx, entity.EntityType, entity.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get entity from DB: %w", err)
	}

	agentToken, err := r.tokenGetter.NewAgentJWTToken(instance, dbEntity)
	if err != nil {
		return "", fmt.Errorf("failed to get agent token: %w", err)
	}
	return agentToken, nil
}
