package entity

import (
	"log/slog"

	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func (w *Worker) handleWorkerWatcherEvent(event dbCommon.ChangePayload) {
	// This worker may be for a repo, org or enterprise. React only to the entity type
	// that this worker is for.
	entityType := dbCommon.DatabaseEntityType(w.Entity.EntityType)
	switch event.EntityType {
	case entityType:
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
		w.handleEntityEventPayload(entity, event)
		return
	case dbCommon.GithubCredentialsEntityType:
		slog.DebugContext(w.ctx, "got github credentials payload event")
		credentials, ok := event.Payload.(params.GithubCredentials)
		if !ok {
			slog.ErrorContext(w.ctx, "invalid payload for entity type", "entity_type", event.EntityType, "payload", event.Payload)
			return
		}
		w.handleEntityCredentialsEventPayload(credentials, event)
	default:
		slog.DebugContext(w.ctx, "invalid entity type; ignoring", "entity_type", event.EntityType)
	}
}

func (w *Worker) handleEntityEventPayload(entity params.GithubEntity, event dbCommon.ChangePayload) {
	switch event.Operation {
	case dbCommon.UpdateOperation:
		slog.DebugContext(w.ctx, "got update operation")
		w.mux.Lock()
		defer w.mux.Unlock()

		credentials := entity.Credentials
		if w.Entity.Credentials.ID != credentials.ID {
			// credentials were swapped on the entity. We need to recompose the watcher
			// filters.
			w.consumer.SetFilters(composeWorkerWatcherFilters(entity))
		}
		w.Entity = entity
	default:
		slog.ErrorContext(w.ctx, "invalid operation type", "operation_type", event.Operation)
	}
}

func (w *Worker) handleEntityCredentialsEventPayload(credentials params.GithubCredentials, event dbCommon.ChangePayload) {
	switch event.Operation {
	case dbCommon.UpdateOperation:
		slog.DebugContext(w.ctx, "got delete operation")
		w.mux.Lock()
		defer w.mux.Unlock()
		if w.Entity.Credentials.ID != credentials.ID {
			// The channel is buffered. We may get an old update. If credentials get updated
			// immediately after they are swapped on the entity, we may still get an update
			// pushed to the channel before the filters are swapped. We can ignore the update.
			return
		}
		w.Entity.Credentials = credentials
	default:
		slog.ErrorContext(w.ctx, "invalid operation type", "operation_type", event.Operation)
	}
}
