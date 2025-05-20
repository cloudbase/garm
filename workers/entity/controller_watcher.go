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
		}
		entityGetter = repo
	case dbCommon.OrganizationEntityType:
		slog.DebugContext(c.ctx, "got organization payload event")
		org, ok := event.Payload.(params.Organization)
		if !ok {
			slog.ErrorContext(c.ctx, "invalid payload for entity type", "entity_type", event.EntityType, "payload", event.Payload)
		}
		entityGetter = org
	case dbCommon.EnterpriseEntityType:
		slog.DebugContext(c.ctx, "got enterprise payload event")
		ent, ok := event.Payload.(params.Enterprise)
		if !ok {
			slog.ErrorContext(c.ctx, "invalid payload for entity type", "entity_type", event.EntityType, "payload", event.Payload)
		}
		entityGetter = ent
	default:
		slog.ErrorContext(c.ctx, "invalid entity type", "entity_type", event.EntityType)
		return
	}

	if entityGetter == nil {
		return
	}

	switch event.Operation {
	case dbCommon.CreateOperation:
		slog.DebugContext(c.ctx, "got create operation")
		c.handleWatcherCreateOperation(entityGetter, event)
	case dbCommon.DeleteOperation:
		slog.DebugContext(c.ctx, "got delete operation")
		c.handleWatcherDeleteOperation(entityGetter, event)
	default:
		slog.ErrorContext(c.ctx, "invalid operation type", "operation_type", event.Operation)
		return
	}
}

func (c *Controller) handleWatcherCreateOperation(entityGetter params.EntityGetter, event dbCommon.ChangePayload) {
	c.mux.Lock()
	defer c.mux.Unlock()
	entity, err := entityGetter.GetEntity()
	if err != nil {
		slog.ErrorContext(c.ctx, "getting entity from repository", "entity_type", event.EntityType, "payload", event.Payload, "error", err)
		return
	}
	worker, err := NewWorker(c.ctx, c.store, entity, c.providers)
	if err != nil {
		slog.ErrorContext(c.ctx, "creating worker from repository", "entity_type", event.EntityType, "payload", event.Payload, "error", err)
		return
	}

	slog.InfoContext(c.ctx, "starting entity worker", "entity_id", entity.ID, "entity_type", entity.EntityType)
	if err := worker.Start(); err != nil {
		slog.ErrorContext(c.ctx, "starting worker", "entity_id", entity.ID, "error", err)
		return
	}

	c.Entities[entity.ID] = worker
}

func (c *Controller) handleWatcherDeleteOperation(entityGetter params.EntityGetter, event dbCommon.ChangePayload) {
	c.mux.Lock()
	defer c.mux.Unlock()
	entity, err := entityGetter.GetEntity()
	if err != nil {
		slog.ErrorContext(c.ctx, "getting entity from repository", "entity_type", event.EntityType, "payload", event.Payload, "error", err)
		return
	}
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
