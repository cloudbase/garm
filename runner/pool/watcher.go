// Copyright 2025 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package pool

import (
	"log/slog"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	runnerCommon "github.com/cloudbase/garm/runner/common"
	ghClient "github.com/cloudbase/garm/util/github"
)

// entityGetter is implemented by all github entities (repositories, organizations and enterprises)
type entityGetter interface {
	GetEntity() (params.ForgeEntity, error)
}

func (r *basePoolManager) handleControllerUpdateEvent(controllerInfo params.ControllerInfo) {
	r.mux.Lock()
	defer r.mux.Unlock()

	slog.DebugContext(r.ctx, "updating controller info", "controller_info", controllerInfo)
	r.controllerInfo = controllerInfo
}

func (r *basePoolManager) getClientOrStub() runnerCommon.GithubClient {
	var err error
	var ghc runnerCommon.GithubClient
	ghc, err = ghClient.Client(r.ctx, r.entity)
	if err != nil {
		slog.WarnContext(r.ctx, "failed to create github client", "error", err)
		ghc = &stubGithubClient{
			err: runnerErrors.NewUnauthorizedError("failed to create github client; please update credentials"),
		}
	}
	return ghc
}

func (r *basePoolManager) handleEntityUpdate(entity params.ForgeEntity, operation common.OperationType) {
	slog.DebugContext(r.ctx, "received entity operation", "entity", entity.ID, "operation", operation)
	if r.entity.ID != entity.ID {
		slog.WarnContext(r.ctx, "entity ID mismatch; stale event? refusing to update", "entity", entity.ID)
		return
	}

	if operation == common.DeleteOperation {
		slog.InfoContext(r.ctx, "entity deleted; closing db consumer", "entity", entity.ID)
		r.consumer.Close()
		return
	}

	if operation != common.UpdateOperation {
		slog.DebugContext(r.ctx, "operation not update; ignoring", "entity", entity.ID, "operation", operation)
		return
	}

	credentialsUpdate := r.entity.Credentials.GetID() != entity.Credentials.GetID()
	defer func() {
		slog.DebugContext(r.ctx, "deferred tools update", "credentials_update", credentialsUpdate)
		if !credentialsUpdate {
			return
		}
		slog.DebugContext(r.ctx, "updating tools", "entity", entity.ID)
		if err := r.updateTools(); err != nil {
			slog.ErrorContext(r.ctx, "failed to update tools", "error", err)
		}
	}()

	slog.DebugContext(r.ctx, "updating entity", "entity", entity.ID)
	r.mux.Lock()
	slog.DebugContext(r.ctx, "lock acquired", "entity", entity.ID)

	r.entity = entity
	if credentialsUpdate {
		if r.consumer != nil {
			filters := composeWatcherFilters(r.entity)
			r.consumer.SetFilters(filters)
		}
		slog.DebugContext(r.ctx, "credentials update", "entity", entity.ID)
		r.ghcli = r.getClientOrStub()
	}
	r.mux.Unlock()
	slog.DebugContext(r.ctx, "lock released", "entity", entity.ID)
}

func (r *basePoolManager) handleCredentialsUpdate(credentials params.ForgeCredentials) {
	// when we switch credentials on an entity (like from one app to another or from an app
	// to a PAT), we may still get events for the previous credentials as the channel is buffered.
	// The watcher will watch for changes to the entity itself, which includes events that
	// change the credentials name on the entity, but we also watch for changes to the credentials
	// themselves, like an updated PAT token set on existing credentials entity.
	// The handleCredentialsUpdate function handles situations where we have changes on the
	// credentials entity itself, not on the entity that the credentials are set on.
	// For example, we may have a credentials entity called org_pat set on a repo called
	// test-repo. This function would handle situations where "org_pat" is updated.
	// If "test-repo" is updated with new credentials, that event is handled above in
	// handleEntityUpdate.
	shouldUpdateTools := r.entity.Credentials.GetID() == credentials.GetID()
	defer func() {
		if !shouldUpdateTools {
			return
		}
		slog.DebugContext(r.ctx, "deferred tools update", "credentials_id", credentials.GetID())
		if err := r.updateTools(); err != nil {
			slog.ErrorContext(r.ctx, "failed to update tools", "error", err)
		}
	}()

	r.mux.Lock()
	if !shouldUpdateTools {
		slog.InfoContext(r.ctx, "credential ID mismatch; stale event?", "credentials_id", credentials.GetID())
		r.mux.Unlock()
		return
	}

	slog.DebugContext(r.ctx, "updating credentials", "credentials_id", credentials.GetID())
	r.entity.Credentials = credentials
	r.ghcli = r.getClientOrStub()
	r.mux.Unlock()
}

func (r *basePoolManager) handleWatcherEvent(event common.ChangePayload) {
	dbEntityType := common.DatabaseEntityType(r.entity.EntityType)
	switch event.EntityType {
	case common.GithubCredentialsEntityType, common.GiteaCredentialsEntityType:
		credentials, ok := event.Payload.(params.ForgeCredentials)
		if !ok {
			slog.ErrorContext(r.ctx, "failed to cast payload to github credentials")
			return
		}
		r.handleCredentialsUpdate(credentials)
	case common.ControllerEntityType:
		controllerInfo, ok := event.Payload.(params.ControllerInfo)
		if !ok {
			slog.ErrorContext(r.ctx, "failed to cast payload to controller info")
			return
		}
		r.handleControllerUpdateEvent(controllerInfo)
	case dbEntityType:
		entity, ok := event.Payload.(entityGetter)
		if !ok {
			slog.ErrorContext(r.ctx, "failed to cast payload to entity")
			return
		}
		entityInfo, err := entity.GetEntity()
		if err != nil {
			slog.ErrorContext(r.ctx, "failed to get entity", "error", err)
			return
		}
		r.handleEntityUpdate(entityInfo, event.Operation)
	}
}

func (r *basePoolManager) runWatcher() {
	defer r.consumer.Close()
	for {
		select {
		case <-r.quit:
			return
		case <-r.ctx.Done():
			return
		case event, ok := <-r.consumer.Watch():
			if !ok {
				return
			}
			go r.handleWatcherEvent(event)
		}
	}
}
