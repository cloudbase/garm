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
	c.mux.Lock()
	defer c.mux.Unlock()

	if !c.running {
		// Controller was stopped.
		return
	}

	switch event.EntityType {
	case dbCommon.GithubCredentialsEntityType, dbCommon.GiteaCredentialsEntityType:
		slog.DebugContext(c.ctx, "got credentials payload event", "entity_type", event.EntityType)
		c.handleCredentialsUpdateEvent(event)
		return
	}

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

// handleCredentialsUpdateEvent propagates credential updates to non-running workers.
// Running workers handle credential updates themselves via their own watcher.
func (c *Controller) handleCredentialsUpdateEvent(event dbCommon.ChangePayload) {
	creds, ok := event.Payload.(params.ForgeCredentials)
	if !ok {
		slog.ErrorContext(c.ctx, "invalid payload for credentials event", "entity_type", event.EntityType, "payload", event.Payload)
		return
	}

	c.Entities.Range(func(_, value any) bool {
		worker := value.(*Worker)
		if worker.IsRunning() {
			// Running workers handle credential updates via their own watcher.
			return true
		}

		worker.mux.Lock()
		defer worker.mux.Unlock()
		if worker.Entity.Credentials.GetID() != creds.GetID() {
			return true
		}
		slog.InfoContext(c.ctx, "propagating credential update to non-running worker", "entity_id", worker.Entity.ID)
		worker.Entity.Credentials = creds
		// Clear backoff so the retry loop picks up the fix immediately.
		c.backoff.RecordSuccess(worker.Entity.ID)
		return true
	})
}

func (c *Controller) handleWatcherUpdateOperation(entity params.ForgeEntity) {
	val, ok := c.Entities.Load(entity.ID)
	if !ok {
		slog.InfoContext(c.ctx, "entity not found in worker list", "entity_id", entity.ID)
		return
	}
	worker := val.(*Worker)

	if worker.IsRunning() {
		// The worker is running. It watches for updates to its own entity. We only care about updates
		// in the controller, if for some reason, the worker is not running.
		slog.DebugContext(c.ctx, "worker is already running, skipping update", "entity_id", entity.ID)
		return
	}

	slog.InfoContext(c.ctx, "updating entity worker", "entity_id", entity.ID, "entity_type", entity.EntityType)
	worker.mux.Lock()
	worker.Entity = entity
	worker.mux.Unlock()
	// Clear any previous backoff so the retry loop starts the worker
	// immediately. The user may have fixed the issue by updating the entity.
	c.backoff.RecordSuccess(entity.ID)
	// The retry loop will start the worker.
}

func (c *Controller) handleWatcherCreateOperation(entity params.ForgeEntity) {
	worker, err := NewWorker(c.ctx, c.store, entity, c.providers)
	if err != nil {
		slog.ErrorContext(c.ctx, "creating worker from repository", "entity_type", entity.EntityType, "error", err)
		return
	}

	if _, loaded := c.Entities.LoadOrStore(entity.ID, worker); loaded {
		slog.DebugContext(c.ctx, "entity already exists in worker list", "entity_id", entity.ID)
	}
	// The retry loop will start the worker.
}

func (c *Controller) handleWatcherDeleteOperation(entity params.ForgeEntity) {
	val, loaded := c.Entities.LoadAndDelete(entity.ID)
	if !loaded {
		slog.InfoContext(c.ctx, "entity not found in worker list", "entity_id", entity.ID)
		return
	}
	c.backoff.RecordSuccess(entity.ID)
	worker := val.(*Worker)
	slog.InfoContext(c.ctx, "stopping entity worker", "entity_id", entity.ID, "entity_type", entity.EntityType)
	if err := worker.Stop(); err != nil {
		// Re-store the worker so it can be retried.
		c.Entities.Store(entity.ID, worker)
		slog.ErrorContext(c.ctx, "stopping worker", "entity_id", entity.ID, "error", err)
	}
}
