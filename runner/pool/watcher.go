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
	"context"
	"log/slog"
	"time"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	runnerCommon "github.com/cloudbase/garm/runner/common"
	ghClient "github.com/cloudbase/garm/util/github"
)

// clientCreateTimeout bounds the creation of a new forge client after a
// credentials change. Creating a client issues a RateLimit probe (and, for
// github apps, may fetch an installation token); without a deadline a stalled
// forge endpoint would hang the rebuild indefinitely.
const clientCreateTimeout = 30 * time.Second

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

// triggerClientUpdate queues a forge client rebuild for clientUpdaterLoop
// without blocking the caller. The channel carries no payload — the rebuild
// snapshots r.entity when it runs — so a full channel already guarantees a
// future rebuild will see the newest credentials.
func (r *basePoolManager) triggerClientUpdate() {
	select {
	case r.clientUpdateCh <- struct{}{}:
	default:
	}
}

// clientUpdaterLoop is the only goroutine that rebuilds the forge client.
// Client creation involves network I/O and must never run on the watcher
// drain goroutine or under r.mux: a stalled forge endpoint would park event
// processing (and, via the watcher fan-out, global event dispatch) for the
// duration of the call.
func (r *basePoolManager) clientUpdaterLoop() {
	for {
		select {
		case <-r.quit:
			return
		case <-r.ctx.Done():
			return
		case <-r.clientUpdateCh:
			r.updateClient()
		}
	}
}

// updateClient rebuilds the forge client from the current entity credentials
// and refreshes the cached tools. If a newer credentials update lands while a
// rebuild is in flight, its queued trigger makes the loop run again with the
// fresh state, so a stale install is always overwritten.
func (r *basePoolManager) updateClient() {
	r.mux.Lock()
	entity := r.entity
	r.mux.Unlock()

	ctx, cancel := context.WithTimeout(r.ctx, clientCreateTimeout)
	defer cancel()

	var ghc runnerCommon.GithubClient
	ghc, err := ghClient.Client(ctx, entity)
	if err != nil {
		slog.WarnContext(r.ctx, "failed to create github client", "error", err)
		ghc = &stubGithubClient{
			err: runnerErrors.NewUnauthorizedError("failed to create github client; please update credentials"),
		}
	}

	r.mux.Lock()
	r.ghcli = ghc
	r.mux.Unlock()

	if err := r.updateTools(); err != nil {
		slog.ErrorContext(r.ctx, "failed to update tools", "error", err)
	}
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

	slog.DebugContext(r.ctx, "updating entity", "entity", entity.ID)
	r.mux.Lock()
	slog.DebugContext(r.ctx, "lock acquired", "entity", entity.ID)

	r.entity = entity
	if credentialsUpdate {
		if r.consumer != nil {
			filters := composeWatcherFilters(r.entity)
			r.consumer.SetFilters(filters)
		}
		slog.DebugContext(r.ctx, "credentials update; queueing client rebuild", "entity", entity.ID)
		r.triggerClientUpdate()
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
	r.mux.Lock()
	if r.entity.Credentials.GetID() != credentials.GetID() {
		slog.InfoContext(r.ctx, "credential ID mismatch; stale event?", "credentials_id", credentials.GetID())
		r.mux.Unlock()
		return
	}

	slog.DebugContext(r.ctx, "updating credentials; queueing client rebuild", "credentials_id", credentials.GetID())
	r.entity.Credentials = credentials
	r.triggerClientUpdate()
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
	case common.JobEntityType:
		slog.DebugContext(r.ctx, "new job via watcher")
		job, ok := event.Payload.(params.Job)
		if !ok {
			slog.ErrorContext(r.ctx, "failed to cast payload to job")
			return
		}
		if !job.BelongsTo(r.entity) {
			slog.InfoContext(r.ctx, "job does not belong to entity", "worklof_job_id", job.WorkflowJobID, "scaleset_job_id", job.ScaleSetJobID, "job_id", job.ID)
			return
		}
		slog.DebugContext(r.ctx, "recording job", "job_id", job.ID, "job_status", job.Status)
		r.mux.Lock()
		switch event.Operation {
		case common.CreateOperation, common.UpdateOperation:
			if params.JobStatus(job.Status) != params.JobStatusCompleted {
				slog.DebugContext(r.ctx, "adding job to map", "job_id", job.ID, "job_status", job.Status)
				r.jobs[job.ID] = job
				break
			}
			fallthrough
		case common.DeleteOperation:
			delete(r.jobs, job.ID)
		}
		r.mux.Unlock()
	}
}

func (r *basePoolManager) runWatcher() {
	defer r.consumer.Close()
	queued, err := r.store.ListEntityJobsByStatus(r.ctx, r.entity.EntityType, r.entity.ID, params.JobStatusQueued)
	if err != nil {
		slog.ErrorContext(r.ctx, "failed to list jobs", "error", err)
	}

	r.mux.Lock()
	for _, job := range queued {
		r.jobs[job.ID] = job
	}
	r.mux.Unlock()
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
			r.handleWatcherEvent(event)
		}
	}
}
