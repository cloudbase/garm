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

	"github.com/cloudbase/garm/cache"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/github"
)

func (w *Worker) handleWorkerWatcherEvent(event dbCommon.ChangePayload) {
	// This worker may be for a repo, org or enterprise. React only to the entity type
	// that this worker is for.
	entityType := dbCommon.DatabaseEntityType(w.Entity.EntityType)
	switch event.EntityType {
	case entityType:
		w.handleEntityEventPayload(event)
	case dbCommon.GithubCredentialsEntityType, dbCommon.GiteaCredentialsEntityType:
		slog.DebugContext(w.ctx, "got github credentials payload event")
		w.handleEntityCredentialsEventPayload(event)
	default:
		slog.DebugContext(w.ctx, "invalid entity type; ignoring", "entity_type", event.EntityType)
	}
}

func (w *Worker) handleEntityEventPayload(event dbCommon.ChangePayload) {
	entityGetter, ok := event.Payload.(params.EntityGetter)
	if !ok {
		slog.ErrorContext(w.ctx, "invalid payload for entity type", "entity_type", event.EntityType, "payload", event.Payload)
		return
	}
	entity, err := entityGetter.GetEntity()
	if err != nil {
		slog.ErrorContext(w.ctx, "getting entity from repository", "entity_type", event.EntityType, "payload", event.Payload, "error", err)
		return
	}

	switch event.Operation {
	case dbCommon.UpdateOperation:
		slog.DebugContext(w.ctx, "got update operation")
		w.mux.Lock()
		defer w.mux.Unlock()

		credentials := entity.Credentials
		if w.Entity.Credentials.GetID() != credentials.GetID() {
			// credentials were swapped on the entity. We need to recompose the watcher
			// filters.
			w.consumer.SetFilters(composeWorkerWatcherFilters(entity))
			ghCli, err := github.Client(w.ctx, entity)
			if err != nil {
				slog.ErrorContext(w.ctx, "creating github client", "entity_id", entity.ID, "error", err)
				return
			}
			w.ghCli = ghCli
			cache.SetGithubClient(entity.ID, ghCli)
		}
		w.Entity = entity
	default:
		slog.ErrorContext(w.ctx, "invalid operation type", "operation_type", event.Operation)
	}
}

func (w *Worker) handleEntityCredentialsEventPayload(event dbCommon.ChangePayload) {
	var creds params.ForgeCredentials
	var ok bool
	switch event.EntityType {
	case dbCommon.GithubCredentialsEntityType, dbCommon.GiteaCredentialsEntityType:
		creds, ok = event.Payload.(params.ForgeCredentials)
	default:
		slog.ErrorContext(w.ctx, "invalid entity type", "entity_type", event.EntityType)
		return
	}
	if !ok {
		slog.ErrorContext(w.ctx, "invalid payload for entity type", "entity_type", event.EntityType, "payload", event.Payload)
		return
	}

	switch event.Operation {
	case dbCommon.UpdateOperation:
		slog.DebugContext(w.ctx, "got delete operation")
		w.mux.Lock()
		defer w.mux.Unlock()
		if w.Entity.Credentials.GetID() != creds.GetID() {
			// The channel is buffered. We may get an old update. If credentials get updated
			// immediately after they are swapped on the entity, we may still get an update
			// pushed to the channel before the filters are swapped. We can ignore the update.
			return
		}
		w.Entity.Credentials = creds
		ghCli, err := github.Client(w.ctx, w.Entity)
		if err != nil {
			slog.ErrorContext(w.ctx, "creating github client", "entity_id", w.Entity.ID, "error", err)
			return
		}
		w.ghCli = ghCli
		cache.SetGithubClient(w.Entity.ID, ghCli)
	default:
		slog.ErrorContext(w.ctx, "invalid operation type", "operation_type", event.Operation)
	}
}
