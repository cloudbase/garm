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
package cache

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	"github.com/cloudbase/garm/params"
	garmUtil "github.com/cloudbase/garm/util"
	"github.com/cloudbase/garm/util/github"
)

func NewWorker(ctx context.Context, store common.Store) *Worker {
	consumerID := "cache"
	ctx = garmUtil.WithSlogContext(
		ctx,
		slog.Any("worker", consumerID))

	return &Worker{
		ctx:         ctx,
		store:       store,
		consumerID:  consumerID,
		toolsWorkes: make(map[string]*toolsUpdater),
		quit:        make(chan struct{}),
	}
}

type Worker struct {
	ctx        context.Context
	consumerID string

	consumer    common.Consumer
	store       common.Store
	toolsWorkes map[string]*toolsUpdater

	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func (w *Worker) setCacheForEntity(entityGetter params.EntityGetter, pools []params.Pool, scaleSets []params.ScaleSet) error {
	entity, err := entityGetter.GetEntity()
	if err != nil {
		return fmt.Errorf("getting entity: %w", err)
	}
	cache.SetEntity(entity)
	var entityPools []params.Pool
	var entityScaleSets []params.ScaleSet

	for _, pool := range pools {
		if pool.BelongsTo(entity) {
			entityPools = append(entityPools, pool)
		}
	}

	for _, scaleSet := range scaleSets {
		if scaleSet.BelongsTo(entity) {
			entityScaleSets = append(entityScaleSets, scaleSet)
		}
	}

	cache.ReplaceEntityPools(entity.ID, entityPools)
	cache.ReplaceEntityScaleSets(entity.ID, entityScaleSets)

	return nil
}

func (w *Worker) loadAllEntities() error {
	endpoints, err := w.store.ListGiteaEndpoints(w.ctx)
	if err != nil {
		slog.ErrorContext(w.ctx, "failed to load gitea endpoints", "error", err)
	} else {
		for _, ep := range endpoints {
			cache.SetEndpoint(ep)
		}
	}

	endpoints, err = w.store.ListGithubEndpoints(w.ctx)
	if err != nil {
		slog.ErrorContext(w.ctx, "failed to load github endpoints", "error", err)
	} else {
		for _, ep := range endpoints {
			cache.SetEndpoint(ep)
		}
	}

	pools, err := w.store.ListAllPools(w.ctx)
	if err != nil {
		return fmt.Errorf("listing pools: %w", err)
	}

	scaleSets, err := w.store.ListAllScaleSets(w.ctx)
	if err != nil {
		return fmt.Errorf("listing scale sets: %w", err)
	}

	repos, err := w.store.ListRepositories(w.ctx, params.RepositoryFilter{})
	if err != nil {
		return fmt.Errorf("listing repositories: %w", err)
	}

	orgs, err := w.store.ListOrganizations(w.ctx, params.OrganizationFilter{})
	if err != nil {
		return fmt.Errorf("listing organizations: %w", err)
	}

	enterprises, err := w.store.ListEnterprises(w.ctx, params.EnterpriseFilter{})
	if err != nil {
		return fmt.Errorf("listing enterprises: %w", err)
	}

	for _, repo := range repos {
		if err := w.setCacheForEntity(repo, pools, scaleSets); err != nil {
			return fmt.Errorf("setting cache for repo: %w", err)
		}
	}

	for _, org := range orgs {
		if err := w.setCacheForEntity(org, pools, scaleSets); err != nil {
			return fmt.Errorf("setting cache for org: %w", err)
		}
	}

	for _, enterprise := range enterprises {
		if err := w.setCacheForEntity(enterprise, pools, scaleSets); err != nil {
			return fmt.Errorf("setting cache for enterprise: %w", err)
		}
	}

	for _, entity := range cache.GetAllEntities() {
		worker := newToolsUpdater(w.ctx, entity, w.store)
		if err := worker.Start(); err != nil {
			return fmt.Errorf("starting tools updater: %w", err)
		}
		w.toolsWorkes[entity.ID] = worker
	}
	return nil
}

func (w *Worker) loadAllInstances() error {
	instances, err := w.store.ListAllInstances(w.ctx)
	if err != nil {
		return fmt.Errorf("listing instances: %w", err)
	}

	for _, instance := range instances {
		cache.SetInstanceCache(instance)
	}
	return nil
}

func (w *Worker) loadAllGithubCredentials() error {
	creds, err := w.store.ListGithubCredentials(w.ctx)
	if err != nil {
		return fmt.Errorf("listing github credentials: %w", err)
	}

	for _, cred := range creds {
		cache.SetGithubCredentials(cred)
	}
	return nil
}

func (w *Worker) loadAllGiteaCredentials() error {
	creds, err := w.store.ListGiteaCredentials(w.ctx)
	if err != nil {
		return fmt.Errorf("listing gitea credentials: %w", err)
	}

	for _, cred := range creds {
		cache.SetGiteaCredentials(cred)
	}
	return nil
}

func (w *Worker) waitForErrorGroupOrContextCancelled(g *errgroup.Group) error {
	if g == nil {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		waitErr := g.Wait()
		done <- waitErr
	}()

	select {
	case err := <-done:
		return err
	case <-w.ctx.Done():
		return w.ctx.Err()
	case <-w.quit:
		return nil
	}
}

func (w *Worker) Start() error {
	slog.DebugContext(w.ctx, "starting cache worker")
	w.mux.Lock()
	defer w.mux.Unlock()

	if w.running {
		return nil
	}

	g, _ := errgroup.WithContext(w.ctx)

	g.Go(func() error {
		ctrlInfo, err := w.store.ControllerInfo()
		if err != nil {
			return fmt.Errorf("failed to get controller info: %w", err)
		}
		cache.SetControllerCache(ctrlInfo)
		return nil
	})

	g.Go(func() error {
		if err := w.loadAllGithubCredentials(); err != nil {
			return fmt.Errorf("loading all github credentials: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		if err := w.loadAllGiteaCredentials(); err != nil {
			return fmt.Errorf("loading all gitea credentials: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		if err := w.loadAllEntities(); err != nil {
			return fmt.Errorf("loading all entities: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		if err := w.loadAllInstances(); err != nil {
			return fmt.Errorf("loading all instances: %w", err)
		}
		return nil
	})

	if err := w.waitForErrorGroupOrContextCancelled(g); err != nil {
		return fmt.Errorf("waiting for error group: %w", err)
	}

	consumer, err := watcher.RegisterConsumer(
		w.ctx, w.consumerID,
		watcher.WithAll())
	if err != nil {
		return fmt.Errorf("registering consumer: %w", err)
	}
	w.consumer = consumer
	w.running = true
	w.quit = make(chan struct{})

	go w.loop()
	go w.rateLimitLoop()
	return nil
}

func (w *Worker) Stop() error {
	slog.DebugContext(w.ctx, "stopping cache worker")
	w.mux.Lock()
	defer w.mux.Unlock()

	if !w.running {
		return nil
	}

	for _, worker := range w.toolsWorkes {
		if err := worker.Stop(); err != nil {
			slog.ErrorContext(w.ctx, "stopping tools updater", "error", err)
		}
	}
	w.consumer.Close()
	w.running = false
	close(w.quit)
	return nil
}

func (w *Worker) handleEntityEvent(entityGetter params.EntityGetter, op common.OperationType) {
	entity, err := entityGetter.GetEntity()
	if err != nil {
		slog.DebugContext(w.ctx, "getting entity from event", "error", err)
		return
	}
	switch op {
	case common.CreateOperation, common.UpdateOperation:
		old, hasOld := cache.GetEntity(entity.ID)
		cache.SetEntity(entity)
		worker, ok := w.toolsWorkes[entity.ID]
		if !ok {
			worker = newToolsUpdater(w.ctx, entity, w.store)
			if err := worker.Start(); err != nil {
				slog.ErrorContext(w.ctx, "starting tools updater", "error", err)
				return
			}
			w.toolsWorkes[entity.ID] = worker
		} else if hasOld {
			// probably an update operation
			if old.Credentials.GetID() != entity.Credentials.GetID() {
				worker.Reset()
			}
		}
	case common.DeleteOperation:
		cache.DeleteEntity(entity.ID)
		worker, ok := w.toolsWorkes[entity.ID]
		if ok {
			if err := worker.Stop(); err != nil {
				slog.ErrorContext(w.ctx, "stopping tools updater", "error", err)
			}
			delete(w.toolsWorkes, entity.ID)
		}
	}
}

func (w *Worker) handleRepositoryEvent(event common.ChangePayload) {
	repo, ok := event.Payload.(params.Repository)
	if !ok {
		slog.DebugContext(w.ctx, "invalid payload type for repository event", "payload", event.Payload)
		return
	}
	w.handleEntityEvent(repo, event.Operation)
}

func (w *Worker) handleOrgEvent(event common.ChangePayload) {
	org, ok := event.Payload.(params.Organization)
	if !ok {
		slog.DebugContext(w.ctx, "invalid payload type for org event", "payload", event.Payload)
		return
	}
	w.handleEntityEvent(org, event.Operation)
}

func (w *Worker) handleEnterpriseEvent(event common.ChangePayload) {
	enterprise, ok := event.Payload.(params.Enterprise)
	if !ok {
		slog.DebugContext(w.ctx, "invalid payload type for enterprise event", "payload", event.Payload)
		return
	}
	w.handleEntityEvent(enterprise, event.Operation)
}

func (w *Worker) handlePoolEvent(event common.ChangePayload) {
	pool, ok := event.Payload.(params.Pool)
	if !ok {
		slog.DebugContext(w.ctx, "invalid payload type for pool event", "payload", event.Payload)
		return
	}
	entity, err := pool.GetEntity()
	if err != nil {
		slog.DebugContext(w.ctx, "getting entity from pool", "error", err)
		return
	}

	switch event.Operation {
	case common.CreateOperation, common.UpdateOperation:
		cache.SetEntityPool(entity.ID, pool)
	case common.DeleteOperation:
		cache.DeleteEntityPool(entity.ID, pool.ID)
	}
}

func (w *Worker) handleScaleSetEvent(event common.ChangePayload) {
	scaleSet, ok := event.Payload.(params.ScaleSet)
	if !ok {
		slog.DebugContext(w.ctx, "invalid payload type for pool event", "payload", event.Payload)
		return
	}
	entity, err := scaleSet.GetEntity()
	if err != nil {
		slog.DebugContext(w.ctx, "getting entity from scale set", "error", err)
		return
	}

	switch event.Operation {
	case common.CreateOperation, common.UpdateOperation:
		cache.SetEntityScaleSet(entity.ID, scaleSet)
	case common.DeleteOperation:
		cache.DeleteEntityScaleSet(entity.ID, scaleSet.ID)
	}
}

func (w *Worker) handleInstanceEvent(event common.ChangePayload) {
	instance, ok := event.Payload.(params.Instance)
	if !ok {
		slog.DebugContext(w.ctx, "invalid payload type for instance event", "payload", event.Payload)
		return
	}
	switch event.Operation {
	case common.CreateOperation, common.UpdateOperation:
		cache.SetInstanceCache(instance)
	case common.DeleteOperation:
		cache.DeleteInstanceCache(instance.Name)
	}
}

func (w *Worker) handleTemplateEvent(event common.ChangePayload) {
	template, ok := event.Payload.(params.Template)
	if !ok {
		slog.DebugContext(w.ctx, "invalid payload type for template event", "payload", event.Payload)
		return
	}
	switch event.Operation {
	case common.CreateOperation, common.UpdateOperation:
		cache.SetTemplateCache(template)
	case common.DeleteOperation:
		cache.DeleteTemplate(template.ID)
	}
}

func (w *Worker) handleCredentialsEvent(event common.ChangePayload) {
	credentials, ok := event.Payload.(params.ForgeCredentials)
	if !ok {
		slog.DebugContext(w.ctx, "invalid payload type for credentials event", "payload", event.Payload)
		return
	}
	switch event.Operation {
	case common.CreateOperation, common.UpdateOperation:
		switch credentials.ForgeType {
		case params.GithubEndpointType:
			cache.SetGithubCredentials(credentials)
		case params.GiteaEndpointType:
			cache.SetGiteaCredentials(credentials)
		default:
			slog.DebugContext(w.ctx, "invalid credentials type", "credentials_type", credentials.ForgeType)
			return
		}
		entities := cache.GetEntitiesUsingCredentials(credentials)
		for _, entity := range entities {
			worker, ok := w.toolsWorkes[entity.ID]
			if ok {
				worker.Reset()
			}
		}
	case common.DeleteOperation:
		cache.DeleteGithubCredentials(credentials.ID)
	}
}

func (w *Worker) handleEndpointEvent(event common.ChangePayload) {
	endpoint, ok := event.Payload.(params.ForgeEndpoint)
	if !ok {
		slog.DebugContext(w.ctx, "invalid payload type for endpoint event", "payload", event.Payload)
		return
	}
	switch event.Operation {
	case common.UpdateOperation, common.CreateOperation:
		cache.SetEndpoint(endpoint)
		entities := cache.GetEntitiesUsingEndpoint(endpoint)
		for _, entity := range entities {
			worker, ok := w.toolsWorkes[entity.ID]
			if ok {
				worker.Reset()
			}
		}
	case common.DeleteOperation:
		cache.RemoveEndpoint(endpoint.Name)
	}
}

func (w *Worker) handleControllerInfoEvent(event common.ChangePayload) {
	ctrlInfo, ok := event.Payload.(params.ControllerInfo)
	if !ok {
		slog.DebugContext(w.ctx, "invalid payload type for controller info event event", "payload", event.Payload)
		return
	}
	cache.SetControllerCache(ctrlInfo)
}

func (w *Worker) handleEvent(event common.ChangePayload) {
	slog.DebugContext(w.ctx, "handling event", "event_entity_type", event.EntityType, "event_operation", event.Operation)
	switch event.EntityType {
	case common.PoolEntityType:
		w.handlePoolEvent(event)
	case common.ScaleSetEntityType:
		w.handleScaleSetEvent(event)
	case common.InstanceEntityType:
		w.handleInstanceEvent(event)
	case common.RepositoryEntityType:
		w.handleRepositoryEvent(event)
	case common.OrganizationEntityType:
		w.handleOrgEvent(event)
	case common.EnterpriseEntityType:
		w.handleEnterpriseEvent(event)
	case common.GithubCredentialsEntityType, common.GiteaCredentialsEntityType:
		w.handleCredentialsEvent(event)
	case common.ControllerEntityType:
		w.handleControllerInfoEvent(event)
	case common.TemplateEntityType:
		w.handleTemplateEvent(event)
	case common.GithubEndpointEntityType:
		w.handleEndpointEvent(event)
	default:
		slog.DebugContext(w.ctx, "unknown entity type", "entity_type", event.EntityType)
	}
}

func (w *Worker) loop() {
	defer w.Stop()
	for {
		select {
		case <-w.quit:
			return
		case event, ok := <-w.consumer.Watch():
			if !ok {
				slog.InfoContext(w.ctx, "consumer channel closed")
				return
			}
			w.handleEvent(event)
		case <-w.ctx.Done():
			slog.DebugContext(w.ctx, "context done")
			return
		}
	}
}

func (w *Worker) rateLimitLoop() {
	defer w.Stop()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.quit:
			return
		case <-w.ctx.Done():
			slog.DebugContext(w.ctx, "context done")
			return
		case <-ticker.C:
			// update credentials rate limits
			for _, creds := range cache.GetAllGithubCredentials() {
				rateCli, err := github.NewRateLimitClient(w.ctx, creds)
				if err != nil {
					slog.With(slog.Any("error", err)).ErrorContext(w.ctx, "failed to create rate limit client")
					continue
				}
				rateLimit, err := rateCli.RateLimit(w.ctx)
				if err != nil {
					slog.With(slog.Any("error", err)).ErrorContext(w.ctx, "failed to get rate limit")
					continue
				}
				if rateLimit != nil {
					core := rateLimit.GetCore()
					limit := params.GithubRateLimit{
						Limit:     core.Limit,
						Used:      core.Used,
						Remaining: core.Remaining,
						Reset:     core.Reset.Unix(),
					}
					cache.SetCredentialsRateLimit(creds.ID, limit)
				}
			}
		}
	}
}
