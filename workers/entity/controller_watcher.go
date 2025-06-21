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
package entity

import (
	"fmt"
	"log/slog"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (c *Controller) handleWatcherEvent(event dbCommon.ChangePayload) {
	var entityGetter params.EntityGetter
	switch event.EntityType {
	case dbCommon.RepositoryEntityType:
		slog.DebugContext(c.ctx, "got repository payload event")
		repo, ok := event.Payload.(params.Repository)
		if !ok {
			slog.ErrorContext(c.ctx, "invalid payload for entity type", "entity_type", event.EntityType, "payload", event.Payload)
			return
		}
		entityGetter = repo
	case dbCommon.OrganizationEntityType:
		slog.DebugContext(c.ctx, "got organization payload event")
		org, ok := event.Payload.(params.Organization)
		if !ok {
			slog.ErrorContext(c.ctx, "invalid payload for entity type", "entity_type", event.EntityType, "payload", event.Payload)
			return
		}
		entityGetter = org
	case dbCommon.EnterpriseEntityType:
		slog.DebugContext(c.ctx, "got enterprise payload event")
		ent, ok := event.Payload.(params.Enterprise)
		if !ok {
			slog.ErrorContext(c.ctx, "invalid payload for entity type", "entity_type", event.EntityType, "payload", event.Payload)
			return
		}
		entityGetter = ent
	default:
		slog.ErrorContext(c.ctx, "invalid entity type", "entity_type", event.EntityType)
		return
	}

	entity, err := entityGetter.GetEntity()
	if err != nil {
		slog.ErrorContext(c.ctx, "getting entity from repository", "entity_type", event.EntityType, "payload", event.Payload, "error", err)
		return
	}

	switch event.Operation {
	case dbCommon.CreateOperation:
		slog.DebugContext(c.ctx, "got create operation")
		c.handleWatcherCreateOperation(entity)
	case dbCommon.DeleteOperation:
		slog.DebugContext(c.ctx, "got delete operation")
		c.handleWatcherDeleteOperation(entity)
	case dbCommon.UpdateOperation:
		slog.DebugContext(c.ctx, "got update operation")
		c.handleWatcherUpdateOperation(entity)
	default:
		slog.ErrorContext(c.ctx, "invalid operation type", "operation_type", event.Operation)
		return
	}
}

func (c *Controller) handleWatcherUpdateOperation(entity params.ForgeEntity) {
	c.mux.Lock()
	defer c.mux.Unlock()

	worker, ok := c.Entities[entity.ID]
	if !ok {
		slog.InfoContext(c.ctx, "entity not found in worker list", "entity_id", entity.ID)
		return
	}

	if worker.IsRunning() {
		// The worker is running. It watches for updates to its own entity. We only care about updates
		// in the controller, if for some reason, the worker is not running.
		slog.DebugContext(c.ctx, "worker is already running, skipping update", "entity_id", entity.ID)
		return
	}

	slog.InfoContext(c.ctx, "updating entity worker", "entity_id", entity.ID, "entity_type", entity.EntityType)
	worker.Entity = entity
	if err := worker.Start(); err != nil {
		slog.ErrorContext(c.ctx, "starting worker after update", "entity_id", entity.ID, "error", err)
		worker.addStatusEvent(fmt.Sprintf("failed to start worker for %s (%s) after update: %s", entity.ID, entity.ForgeURL(), err.Error()), params.EventError)
		return
	}
	slog.InfoContext(c.ctx, "entity worker updated and successfully started", "entity_id", entity.ID, "entity_type", entity.EntityType)
	worker.addStatusEvent(fmt.Sprintf("worker updated and successfully started for entity: %s (%s)", entity.ID, entity.ForgeURL()), params.EventInfo)
}

func (c *Controller) handleWatcherCreateOperation(entity params.ForgeEntity) {
	c.mux.Lock()
	defer c.mux.Unlock()

	worker, err := NewWorker(c.ctx, c.store, entity, c.providers)
	if err != nil {
		slog.ErrorContext(c.ctx, "creating worker from repository", "entity_type", entity.EntityType, "error", err)
		return
	}

	slog.InfoContext(c.ctx, "starting entity worker", "entity_id", entity.ID, "entity_type", entity.EntityType)
	if err := worker.Start(); err != nil {
		slog.ErrorContext(c.ctx, "starting worker", "entity_id", entity.ID, "error", err)
		return
	}

	c.Entities[entity.ID] = worker
}

func (c *Controller) handleWatcherDeleteOperation(entity params.ForgeEntity) {
	c.mux.Lock()
	defer c.mux.Unlock()

	worker, ok := c.Entities[entity.ID]
	if !ok {
		slog.InfoContext(c.ctx, "entity not found in worker list", "entity_id", entity.ID)
		return
	}
	slog.InfoContext(c.ctx, "stopping entity worker", "entity_id", entity.ID, "entity_type", entity.EntityType)
	if err := worker.Stop(); err != nil {
		slog.ErrorContext(c.ctx, "stopping worker", "entity_id", entity.ID, "error", err)
		return
	}
	delete(c.Entities, entity.ID)
}
