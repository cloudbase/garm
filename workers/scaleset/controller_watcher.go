// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
package scaleset

import (
	"fmt"
	"log/slog"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
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
	default:
		slog.ErrorContext(c.ctx, "invalid entity type", "entity_type", event.EntityType)
		return
	}
}

func (c *Controller) handleScaleSet(event dbCommon.ChangePayload) {
	scaleSet, ok := event.Payload.(params.ScaleSet)
	if !ok {
		slog.ErrorContext(c.ctx, "invalid scale set payload for entity type", "entity_type", event.EntityType, "payload", event)
		return
	}

	switch event.Operation {
	case dbCommon.CreateOperation:
		slog.DebugContext(c.ctx, "got create operation for scale set", "scale_set_id", scaleSet.ID, "scale_set_name", scaleSet.Name)
		if err := c.handleScaleSetCreateOperation(scaleSet); err != nil {
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

func (c *Controller) createScaleSetWorker(scaleSet params.ScaleSet) (*Worker, error) {
	provider, ok := c.providers[scaleSet.ProviderName]
	if !ok {
		// Providers are currently static, set in the config and cannot be updated without a restart.
		// ScaleSets and pools also do not allow updating the provider. This condition is not recoverable
		// without a restart, so we don't need to instantiate a worker for this scale set.
		return nil, fmt.Errorf("provider %s not found for scale set %s", scaleSet.ProviderName, scaleSet.Name)
	}

	worker, err := NewWorker(c.ctx, c.store, scaleSet, provider)
	if err != nil {
		return nil, fmt.Errorf("creating scale set worker: %w", err)
	}
	return worker, nil
}

func (c *Controller) handleScaleSetCreateOperation(sSet params.ScaleSet) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	if _, ok := c.ScaleSets[sSet.ID]; ok {
		slog.DebugContext(c.ctx, "scale set already exists in worker list", "scale_set_id", sSet.ID)
		return nil
	}

	worker, err := c.createScaleSetWorker(sSet)
	if err != nil {
		return fmt.Errorf("error creating scale set worker: %w", err)
	}
	if err := worker.Start(); err != nil {
		// The Start() function should only return an error if an unrecoverable error occurs.
		// For transient errors, it should mark the scale set as being in error, but continue
		// to retry fixing the condition. For example, not being able to retrieve tools due to bad
		// credentials should not stop the worker. The credentials can be fixed and the worker
		// can continue to work.
		return fmt.Errorf("error starting scale set worker: %w", err)
	}
	c.ScaleSets[sSet.ID] = &scaleSet{
		scaleSet: sSet,
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

	set, ok := c.ScaleSets[sSet.ID]
	if !ok {
		// Some error may have occurred when the scale set was first created, so we
		// attempt to create it after the user updated the scale set, hopefully
		// fixing the reason for the failure.
		return c.handleScaleSetCreateOperation(sSet)
	}
	if set.worker != nil && !set.worker.IsRunning() {
		worker, err := c.createScaleSetWorker(sSet)
		if err != nil {
			return fmt.Errorf("creating scale set worker: %w", err)
		}
		set.worker = worker
		defer func() {
			if err := worker.Start(); err != nil {
				slog.ErrorContext(c.ctx, "failed to start worker", "error", err, "scaleset", sSet.Name)
			}
		}()
	}

	set.scaleSet = sSet
	c.ScaleSets[sSet.ID] = set
	// We let the watcher in the scale set worker handle the update operation.
	return nil
}

func (c *Controller) handleEntityEvent(event dbCommon.ChangePayload) {
	var entityGetter params.EntityGetter
	var ok bool
	switch c.Entity.EntityType {
	case params.ForgeEntityTypeRepository:
		entityGetter, ok = event.Payload.(params.Repository)
	case params.ForgeEntityTypeOrganization:
		entityGetter, ok = event.Payload.(params.Organization)
	case params.ForgeEntityTypeEnterprise:
		entityGetter, ok = event.Payload.(params.Enterprise)
	}
	if !ok {
		slog.ErrorContext(c.ctx, "invalid entity payload for entity type", "entity_type", event.EntityType, "payload", event)
		return
	}

	entity, err := entityGetter.GetEntity()
	if err != nil {
		slog.ErrorContext(c.ctx, "invalid GitHub entity payload for entity type", "entity_type", event.EntityType, "payload", event)
		return
	}

	switch event.Operation {
	case dbCommon.UpdateOperation:
		slog.DebugContext(c.ctx, "got update operation")
		c.mux.Lock()
		defer c.mux.Unlock()
		c.Entity = entity
	default:
		slog.ErrorContext(c.ctx, "invalid operation type", "operation_type", event.Operation)
		return
	}
}
