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
package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/cache"
	dbCommon "github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	garmUtil "github.com/cloudbase/garm/util"
)

func newInstanceManager(ctx context.Context, instance params.Instance, scaleSet params.ScaleSet, provider common.Provider, helper providerHelper) (*instanceManager, error) {
	ctx = garmUtil.WithSlogContext(ctx, slog.Any("worker", fmt.Sprintf("instance-worker-%s", instance.Name)))

	githubEntity, err := scaleSet.GetEntity()
	if err != nil {
		return nil, fmt.Errorf("getting github entity: %w", err)
	}
	return &instanceManager{
		ctx:            ctx,
		instance:       instance,
		provider:       provider,
		deleteBackoff:  time.Second * 0,
		scaleSet:       scaleSet,
		helper:         helper,
		scaleSetEntity: githubEntity,
	}, nil
}

// instanceManager handles the lifecycle of a single instance.
// When an instance is created, a new instance manager is created
// for it. When the instance is placed in pending_create, the manager
// will attempt to create a new compute resource in the designated
// provider. Finally, when an instance is marked as pending_delete, it is removed
// from the provider and on success the instance is marked as deleted. Failure to
// delete, will place the instance back in pending delete. The removal process is
// retried after a backoff period. Instances placed in force_pending_delete will
// ignore provider errors and exit.
type instanceManager struct {
	ctx context.Context

	instance params.Instance
	provider common.Provider
	helper   providerHelper

	scaleSet       params.ScaleSet
	scaleSetEntity params.ForgeEntity

	deleteBackoff time.Duration

	updates chan dbCommon.ChangePayload
	mux     sync.Mutex
	running bool
	quit    chan struct{}
}

func (i *instanceManager) Start() error {
	i.mux.Lock()
	defer i.mux.Unlock()

	slog.DebugContext(i.ctx, "starting instance manager", "instance", i.instance.Name)
	if i.running {
		return nil
	}

	i.running = true
	i.quit = make(chan struct{})
	i.updates = make(chan dbCommon.ChangePayload)

	go i.loop()
	go i.updatesLoop()
	return nil
}

func (i *instanceManager) Stop() error {
	i.mux.Lock()
	defer i.mux.Unlock()

	if !i.running {
		return nil
	}

	i.running = false
	close(i.quit)
	close(i.updates)
	return nil
}

func (i *instanceManager) sleepForBackOffOrCanceled() bool {
	timer := time.NewTimer(i.deleteBackoff)
	defer timer.Stop()

	slog.DebugContext(i.ctx, "sleeping for backoff", "duration", i.deleteBackoff, "instance", i.instance.Name)
	select {
	case <-timer.C:
		return false
	case <-i.quit:
		return true
	case <-i.ctx.Done():
		return true
	}
}

func (i *instanceManager) incrementBackOff() {
	if i.deleteBackoff == 0 {
		i.deleteBackoff = time.Second * 1
	} else {
		i.deleteBackoff *= 2
	}
	if i.deleteBackoff > time.Minute*5 {
		i.deleteBackoff = time.Minute * 5
	}
}

func (i *instanceManager) getEntity() (params.ForgeEntity, error) {
	entity, err := i.scaleSet.GetEntity()
	if err != nil {
		return params.ForgeEntity{}, fmt.Errorf("getting entity: %w", err)
	}
	ghEntity, err := i.helper.GetGithubEntity(entity)
	if err != nil {
		return params.ForgeEntity{}, fmt.Errorf("getting entity: %w", err)
	}
	return ghEntity, nil
}

func (i *instanceManager) pseudoPoolID() string {
	// This is temporary. We need to extend providers to know about scale sets.
	return fmt.Sprintf("%s-%s", i.scaleSet.Name, i.scaleSetEntity.ID)
}

func (i *instanceManager) handleCreateInstanceInProvider(instance params.Instance) error {
	entity, err := i.getEntity()
	if err != nil {
		return fmt.Errorf("getting entity: %w", err)
	}

	token, err := i.helper.InstanceTokenGetter().NewInstanceJWTToken(
		instance, entity, entity.EntityType, i.scaleSet.RunnerBootstrapTimeout)
	if err != nil {
		return fmt.Errorf("creating instance token: %w", err)
	}
	tools, err := cache.GetGithubToolsCache(entity.ID)
	if err != nil {
		return fmt.Errorf("tools not found in cache for entity %s: %w", entity.String(), err)
	}

	bootstrapArgs := commonParams.BootstrapInstance{
		Name:          instance.Name,
		Tools:         tools,
		RepoURL:       entity.ForgeURL(),
		MetadataURL:   instance.MetadataURL,
		CallbackURL:   instance.CallbackURL,
		InstanceToken: token,
		OSArch:        i.scaleSet.OSArch,
		OSType:        i.scaleSet.OSType,
		Flavor:        i.scaleSet.Flavor,
		Image:         i.scaleSet.Image,
		ExtraSpecs:    i.scaleSet.ExtraSpecs,
		// This is temporary. We need to extend providers to know about scale sets.
		PoolID:            i.pseudoPoolID(),
		CACertBundle:      entity.Credentials.CABundle,
		GitHubRunnerGroup: i.scaleSet.GitHubRunnerGroup,
		JitConfigEnabled:  true,
	}

	var instanceIDToDelete string
	baseParams, err := i.getProviderBaseParams()
	if err != nil {
		return fmt.Errorf("getting provider base params: %w", err)
	}

	defer func() {
		if instanceIDToDelete != "" {
			deleteInstanceParams := common.DeleteInstanceParams{
				DeleteInstanceV011: common.DeleteInstanceV011Params{
					ProviderBaseParams: baseParams,
				},
			}
			if err := i.provider.DeleteInstance(i.ctx, instanceIDToDelete, deleteInstanceParams); err != nil {
				if !errors.Is(err, runnerErrors.ErrNotFound) {
					slog.With(slog.Any("error", err)).ErrorContext(
						i.ctx, "failed to cleanup instance",
						"provider_id", instanceIDToDelete)
				}
			}
		}
	}()

	createInstanceParams := common.CreateInstanceParams{
		CreateInstanceV011: common.CreateInstanceV011Params{
			ProviderBaseParams: baseParams,
		},
	}

	providerInstance, err := i.provider.CreateInstance(i.ctx, bootstrapArgs, createInstanceParams)
	if err != nil {
		instanceIDToDelete = instance.Name
		return fmt.Errorf("creating instance in provider: %w", err)
	}

	if providerInstance.Status == commonParams.InstanceError {
		instanceIDToDelete = instance.ProviderID
		if instanceIDToDelete == "" {
			instanceIDToDelete = instance.Name
		}
	}

	updated, err := i.helper.updateArgsFromProviderInstance(instance.Name, providerInstance)
	if err != nil {
		return fmt.Errorf("updating instance args: %w", err)
	}
	i.instance = updated

	return nil
}

func (i *instanceManager) getProviderBaseParams() (common.ProviderBaseParams, error) {
	info, err := i.helper.GetControllerInfo()
	if err != nil {
		return common.ProviderBaseParams{}, fmt.Errorf("getting controller info: %w", err)
	}

	return common.ProviderBaseParams{
		ControllerInfo: info,
	}, nil
}

func (i *instanceManager) handleDeleteInstanceInProvider(instance params.Instance) error {
	slog.InfoContext(i.ctx, "deleting instance in provider", "runner_name", instance.Name)
	identifier := instance.ProviderID
	if identifier == "" {
		// provider did not return a provider ID?
		// try with name
		identifier = instance.Name
	}

	baseParams, err := i.getProviderBaseParams()
	if err != nil {
		return fmt.Errorf("getting provider base params: %w", err)
	}

	slog.DebugContext(
		i.ctx, "calling delete instance on provider",
		"runner_name", instance.Name,
		"provider_id", identifier)

	deleteInstanceParams := common.DeleteInstanceParams{
		DeleteInstanceV011: common.DeleteInstanceV011Params{
			ProviderBaseParams: baseParams,
		},
	}
	if err := i.provider.DeleteInstance(i.ctx, identifier, deleteInstanceParams); err != nil {
		return fmt.Errorf("deleting instance in provider: %w", err)
	}
	return nil
}

func (i *instanceManager) consolidateState() error {
	i.mux.Lock()
	defer i.mux.Unlock()

	if !i.running {
		return nil
	}

	switch i.instance.Status {
	case commonParams.InstancePendingCreate:
		// kick off the creation process
		if err := i.helper.SetInstanceStatus(i.instance.Name, commonParams.InstanceCreating, nil); err != nil {
			return fmt.Errorf("setting instance status to creating: %w", err)
		}
		if err := i.handleCreateInstanceInProvider(i.instance); err != nil {
			slog.ErrorContext(i.ctx, "creating instance in provider", "error", err)
			if err := i.helper.SetInstanceStatus(i.instance.Name, commonParams.InstanceError, []byte(err.Error())); err != nil {
				return fmt.Errorf("setting instance status to error: %w", err)
			}
		}
	case commonParams.InstanceRunning:
		// Nothing to do. The provider finished creating the instance.
	case commonParams.InstancePendingDelete, commonParams.InstancePendingForceDelete:
		// Remove or force remove the runner. When force remove is specified, we ignore
		// IaaS errors.
		if i.instance.Status == commonParams.InstancePendingDelete {
			// invoke backoff sleep. We only do this for non forced removals,
			// as force delete will always return, regardless of whether or not
			// the remove operation succeeded in the provider. A user may decide
			// to force delete a runner if GARM fails to remove it normally.
			if canceled := i.sleepForBackOffOrCanceled(); canceled {
				// the worker is shutting down. Return here.
				return nil
			}
		}

		prevStatus := i.instance.Status
		if err := i.helper.SetInstanceStatus(i.instance.Name, commonParams.InstanceDeleting, nil); err != nil {
			if errors.Is(err, runnerErrors.ErrNotFound) {
				return nil
			}
			return fmt.Errorf("setting instance status to deleting: %w", err)
		}

		if err := i.handleDeleteInstanceInProvider(i.instance); err != nil {
			slog.ErrorContext(i.ctx, "deleting instance in provider", "error", err, "forced", i.instance.Status == commonParams.InstancePendingForceDelete)
			if prevStatus == commonParams.InstancePendingDelete {
				i.incrementBackOff()
				if err := i.helper.SetInstanceStatus(i.instance.Name, commonParams.InstancePendingDelete, []byte(err.Error())); err != nil {
					return fmt.Errorf("setting instance status to error: %w", err)
				}

				return fmt.Errorf("error removing instance. Will retry: %w", err)
			}
		}
		if err := i.helper.SetInstanceStatus(i.instance.Name, commonParams.InstanceDeleted, nil); err != nil {
			if !errors.Is(err, runnerErrors.ErrNotFound) {
				return fmt.Errorf("setting instance status to deleted: %w", err)
			}
		}
		return ErrInstanceDeleted
	case commonParams.InstanceError:
		// Instance is in error state. We wait for next status or potentially retry
		// spawning the instance with a backoff timer.
		if err := i.helper.SetInstanceStatus(i.instance.Name, commonParams.InstancePendingDelete, nil); err != nil {
			return fmt.Errorf("setting instance status to error: %w", err)
		}
	case commonParams.InstanceDeleted:
		return ErrInstanceDeleted
	}
	return nil
}

func (i *instanceManager) handleUpdate(update dbCommon.ChangePayload) error {
	// We need a better way to handle instance state. Database updates may fail, and we
	// end up with an inconsistent state between what we know about the instance and what
	// is reflected in the database.
	if !i.running {
		return nil
	}

	instance, ok := update.Payload.(params.Instance)
	if !ok {
		return runnerErrors.NewBadRequestError("invalid payload type")
	}

	i.instance = instance
	return nil
}

func (i *instanceManager) Update(instance dbCommon.ChangePayload) error {
	if !i.running {
		return runnerErrors.NewBadRequestError("instance manager is not running")
	}

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	slog.DebugContext(i.ctx, "sending update to instance manager")
	select {
	case i.updates <- instance:
	case <-i.quit:
		return nil
	case <-i.ctx.Done():
		return nil
	case <-timer.C:
		return fmt.Errorf("timeout while sending update to instance manager")
	}
	return nil
}

func (i *instanceManager) updatesLoop() {
	defer i.Stop()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-i.quit:
			return
		case <-i.ctx.Done():
			return
		case update, ok := <-i.updates:
			if !ok {
				slog.InfoContext(i.ctx, "updates channel closed")
				return
			}
			slog.DebugContext(i.ctx, "received update")
			if err := i.handleUpdate(update); err != nil {
				if errors.Is(err, ErrInstanceDeleted) {
					// instance had been deleted, we can exit the loop.
					return
				}
				slog.ErrorContext(i.ctx, "handling update", "error", err)
			}
		}
	}
}

func (i *instanceManager) loop() {
	defer i.Stop()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-i.quit:
			return
		case <-i.ctx.Done():
			return
		case <-ticker.C:
			if err := i.consolidateState(); err != nil {
				if errors.Is(err, ErrInstanceDeleted) {
					// instance had been deleted, we can exit the loop.
					return
				}
				slog.ErrorContext(i.ctx, "consolidating state", "error", err)
			}
		}
	}
}
