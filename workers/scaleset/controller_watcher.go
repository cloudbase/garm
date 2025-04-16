package scaleset

import (
	"fmt"
	"log/slog"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util/github"
)

func (c *Controller) handleWatcherEvent(event dbCommon.ChangePayload) {
	entityType := dbCommon.DatabaseEntityType(c.Entity.EntityType)
	switch event.EntityType {
	case dbCommon.ScaleSetEntityType:
		slog.DebugContext(c.ctx, "got scale set payload event")
		c.handleScaleSet(event)
	case entityType:
		slog.DebugContext(c.ctx, "got entity payload event")
		c.handleEntityEvent(event)
	case dbCommon.GithubCredentialsEntityType:
		slog.DebugContext(c.ctx, "got github credentials payload event")
		c.handleCredentialsEvent(event)
	default:
		slog.ErrorContext(c.ctx, "invalid entity type", "entity_type", event.EntityType)
		return
	}
}

func (c *Controller) handleScaleSet(event dbCommon.ChangePayload) {
	scaleSet, ok := event.Payload.(params.ScaleSet)
	if !ok {
		slog.ErrorContext(c.ctx, "invalid payload for entity type", "entity_type", event.EntityType, "payload", event.Payload)
		return
	}

	switch event.Operation {
	case dbCommon.CreateOperation:
		slog.DebugContext(c.ctx, "got create operation for scale set", "scale_set_id", scaleSet.ID, "scale_set_name", scaleSet.Name)
		if err := c.handleScaleSetCreateOperation(scaleSet, c.ghCli); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(c.ctx, "failed to handle scale set create operation")
		}
	case dbCommon.UpdateOperation:
		slog.DebugContext(c.ctx, "got update operation for scale set", "scale_set_id", scaleSet.ID, "scale_set_name", scaleSet.Name)
		if err := c.handleScaleSetUpdateOperation(scaleSet); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(c.ctx, "failed to handle scale set update operation")
		}
	case dbCommon.DeleteOperation:
		slog.DebugContext(c.ctx, "got delete operation")
		if err := c.handleScaleSetDeleteOperation(scaleSet); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(c.ctx, "failed to handle scale set delete operation")
		}
	default:
		slog.ErrorContext(c.ctx, "invalid operation type", "operation_type", event.Operation)
		return
	}
}

func (c *Controller) handleScaleSetCreateOperation(sSet params.ScaleSet, ghCli common.GithubClient) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	if _, ok := c.ScaleSets[sSet.ID]; ok {
		slog.DebugContext(c.ctx, "scale set already exists in worker list", "scale_set_id", sSet.ID)
		return nil
	}

	provider, ok := c.providers[sSet.ProviderName]
	if !ok {
		// Providers are currently static, set in the config and cannot be updated without a restart.
		// ScaleSets and pools also do not allow updating the provider. This condition is not recoverable
		// without a restart, so we don't need to instantiate a worker for this scale set.
		return fmt.Errorf("provider %s not found for scale set %s", sSet.ProviderName, sSet.Name)
	}

	worker, err := NewWorker(c.ctx, c.store, sSet, provider, ghCli)
	if err != nil {
		return fmt.Errorf("creating scale set worker: %w", err)
	}
	if err := worker.Start(); err != nil {
		// The Start() function should only return an error if an unrecoverable error occurs.
		// For transient errors, it should mark the scale set as being in error, but continue
		// to retry fixing the condition. For example, not being able to retrieve tools due to bad
		// credentials should not stop the worker. The credentials can be fixed and the worker
		// can continue to work.
		return fmt.Errorf("starting scale set worker: %w", err)
	}
	c.ScaleSets[sSet.ID] = &scaleSet{
		scaleSet: sSet,
		status:   scaleSetStatus{},
		worker:   worker,
	}
	return nil
}

func (c *Controller) handleScaleSetDeleteOperation(sSet params.ScaleSet) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	set, ok := c.ScaleSets[sSet.ID]
	if !ok {
		slog.DebugContext(c.ctx, "scale set not found in worker list", "scale_set_id", sSet.ID)
		return nil
	}

	slog.DebugContext(c.ctx, "stopping scale set worker", "scale_set_id", sSet.ID)
	if err := set.worker.Stop(); err != nil {
		return fmt.Errorf("stopping scale set worker: %w", err)
	}
	delete(c.ScaleSets, sSet.ID)
	return nil
}

func (c *Controller) handleScaleSetUpdateOperation(sSet params.ScaleSet) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	if _, ok := c.ScaleSets[sSet.ID]; !ok {
		// Some error may have occured when the scale set was first created, so we
		// attempt to create it after the user updated the scale set, hopefully
		// fixing the reason for the failure.
		return c.handleScaleSetCreateOperation(sSet, c.ghCli)
	}
	// We let the watcher in the scale set worker handle the update operation.
	return nil
}

func (c *Controller) handleCredentialsEvent(event dbCommon.ChangePayload) {
	credentials, ok := event.Payload.(params.GithubCredentials)
	if !ok {
		slog.ErrorContext(c.ctx, "invalid payload for entity type", "entity_type", event.EntityType, "payload", event.Payload)
		return
	}

	switch event.Operation {
	case dbCommon.UpdateOperation:
		slog.DebugContext(c.ctx, "got update operation")
		c.mux.Lock()
		defer c.mux.Unlock()

		if c.Entity.Credentials.ID != credentials.ID {
			// stale update event.
			return
		}
		c.Entity.Credentials = credentials

		if err := c.updateAndBroadcastCredentials(c.Entity); err != nil {
			slog.With(slog.Any("error", err)).ErrorContext(c.ctx, "failed to update credentials")
			return
		}
	default:
		slog.ErrorContext(c.ctx, "invalid operation type", "operation_type", event.Operation)
		return
	}
}

func (c *Controller) handleEntityEvent(event dbCommon.ChangePayload) {
	entity, ok := event.Payload.(params.GithubEntity)
	if !ok {
		slog.ErrorContext(c.ctx, "invalid payload for entity type", "entity_type", event.EntityType, "payload", event.Payload)
		return
	}

	switch event.Operation {
	case dbCommon.UpdateOperation:
		slog.DebugContext(c.ctx, "got update operation")
		c.mux.Lock()
		defer c.mux.Unlock()

		if c.Entity.Credentials.ID != entity.Credentials.ID {
			// credentials were swapped on the entity. We need to recompose the watcher
			// filters.
			c.consumer.SetFilters(composeControllerWatcherFilters(entity))
			if err := c.updateAndBroadcastCredentials(c.Entity); err != nil {
				slog.With(slog.Any("error", err)).ErrorContext(c.ctx, "failed to update credentials")
			}
		}
		c.Entity = entity
	default:
		slog.ErrorContext(c.ctx, "invalid operation type", "operation_type", event.Operation)
		return
	}
}

func (c *Controller) updateAndBroadcastCredentials(entity params.GithubEntity) error {
	ghCli, err := github.Client(c.ctx, entity)
	if err != nil {
		return fmt.Errorf("creating github client: %w", err)
	}

	c.ghCli = ghCli

	for _, scaleSet := range c.ScaleSets {
		if err := scaleSet.worker.SetGithubClient(ghCli); err != nil {
			slog.ErrorContext(c.ctx, "setting github client on worker", "error", err)
			continue
		}
	}
	return nil
}
