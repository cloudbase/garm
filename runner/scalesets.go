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

package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/appdefaults"
	"github.com/cloudbase/garm/util/github"
	"github.com/cloudbase/garm/util/github/scalesets"
)

func (r *Runner) ListAllScaleSets(ctx context.Context) ([]params.ScaleSet, error) {
	if !auth.IsAdmin(ctx) {
		return []params.ScaleSet{}, runnerErrors.ErrUnauthorized
	}

	scalesets, err := r.store.ListAllScaleSets(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching pools: %w", err)
	}
	return scalesets, nil
}

func (r *Runner) GetScaleSetByID(ctx context.Context, scaleSet uint) (params.ScaleSet, error) {
	if !auth.IsAdmin(ctx) {
		return params.ScaleSet{}, runnerErrors.ErrUnauthorized
	}

	set, err := r.store.GetScaleSetByID(ctx, scaleSet)
	if err != nil {
		return params.ScaleSet{}, fmt.Errorf("error fetching scale set: %w", err)
	}
	return set, nil
}

func (r *Runner) DeleteScaleSetByID(ctx context.Context, scaleSetID uint) error {
	if !auth.IsAdmin(ctx) {
		return runnerErrors.ErrUnauthorized
	}

	scaleSet, err := r.store.GetScaleSetByID(ctx, scaleSetID)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return fmt.Errorf("error fetching scale set: %w", err)
		}
		return nil
	}

	if len(scaleSet.Instances) > 0 {
		return runnerErrors.NewBadRequestError("scale set has runners")
	}

	if scaleSet.Enabled {
		return runnerErrors.NewBadRequestError("scale set is enabled; disable it first")
	}

	paramEntity, err := scaleSet.GetEntity()
	if err != nil {
		return fmt.Errorf("error getting entity: %w", err)
	}

	entity, err := r.store.GetForgeEntity(ctx, paramEntity.EntityType, paramEntity.ID)
	if err != nil {
		return fmt.Errorf("error getting entity: %w", err)
	}

	ghCli, err := github.Client(ctx, entity)
	if err != nil {
		return fmt.Errorf("error creating github client: %w", err)
	}

	scalesetCli, err := scalesets.NewClient(ghCli)
	if err != nil {
		return fmt.Errorf("error getting scaleset client: %w", err)
	}

	slog.DebugContext(ctx, "deleting scale set", "scale_set_id", scaleSet.ScaleSetID)
	if err := scalesetCli.DeleteRunnerScaleSet(ctx, scaleSet.ScaleSetID); err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			slog.InfoContext(ctx, "scale set not found", "scale_set_id", scaleSet.ScaleSetID)
			return nil
		}
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to delete scale set from github")
		return fmt.Errorf("error deleting scale set from github: %w", err)
	}
	if err := r.store.DeleteScaleSetByID(ctx, scaleSetID); err != nil {
		return fmt.Errorf("error deleting scale set: %w", err)
	}
	return nil
}

func (r *Runner) UpdateScaleSetByID(ctx context.Context, scaleSetID uint, param params.UpdateScaleSetParams) (params.ScaleSet, error) {
	if !auth.IsAdmin(ctx) {
		return params.ScaleSet{}, runnerErrors.ErrUnauthorized
	}

	scaleSet, err := r.store.GetScaleSetByID(ctx, scaleSetID)
	if err != nil {
		return params.ScaleSet{}, fmt.Errorf("error fetching scale set: %w", err)
	}

	maxRunners := scaleSet.MaxRunners
	minIdleRunners := scaleSet.MinIdleRunners

	if param.MaxRunners != nil {
		maxRunners = *param.MaxRunners
	}
	if param.MinIdleRunners != nil {
		minIdleRunners = *param.MinIdleRunners
	}

	if param.RunnerBootstrapTimeout != nil && *param.RunnerBootstrapTimeout == 0 {
		return params.ScaleSet{}, runnerErrors.NewBadRequestError("runner_bootstrap_timeout cannot be 0")
	}

	if minIdleRunners > maxRunners {
		return params.ScaleSet{}, runnerErrors.NewBadRequestError("min_idle_runners cannot be larger than max_runners")
	}

	paramEntity, err := scaleSet.GetEntity()
	if err != nil {
		return params.ScaleSet{}, fmt.Errorf("error getting entity: %w", err)
	}

	entity, err := r.store.GetForgeEntity(ctx, paramEntity.EntityType, paramEntity.ID)
	if err != nil {
		return params.ScaleSet{}, fmt.Errorf("error getting entity: %w", err)
	}

	ghCli, err := github.Client(ctx, entity)
	if err != nil {
		return params.ScaleSet{}, fmt.Errorf("error creating github client: %w", err)
	}

	scalesetCli, err := scalesets.NewClient(ghCli)
	if err != nil {
		return params.ScaleSet{}, fmt.Errorf("error getting scaleset client: %w", err)
	}

	callback := func(old, newSet params.ScaleSet) error {
		updateParams := params.RunnerScaleSet{}
		hasUpdates := false
		if old.Name != newSet.Name {
			updateParams.Name = newSet.Name
			hasUpdates = true
		}

		if old.GitHubRunnerGroup != newSet.GitHubRunnerGroup {
			runnerGroup, err := scalesetCli.GetRunnerGroupByName(ctx, newSet.GitHubRunnerGroup)
			if err != nil {
				return fmt.Errorf("error fetching runner group from github: %w", err)
			}
			updateParams.RunnerGroupID = runnerGroup.ID
			hasUpdates = true
		}

		if old.DisableUpdate != newSet.DisableUpdate {
			updateParams.RunnerSetting.DisableUpdate = newSet.DisableUpdate
			hasUpdates = true
		}

		if hasUpdates {
			_, err := scalesetCli.UpdateRunnerScaleSet(ctx, newSet.ScaleSetID, updateParams)
			if err != nil {
				return fmt.Errorf("failed to update scaleset in github: %w", err)
			}
		}
		return nil
	}

	newScaleSet, err := r.store.UpdateEntityScaleSet(ctx, entity, scaleSetID, param, callback)
	if err != nil {
		return params.ScaleSet{}, fmt.Errorf("error updating pool: %w", err)
	}
	return newScaleSet, nil
}

func (r *Runner) CreateEntityScaleSet(ctx context.Context, entityType params.ForgeEntityType, entityID string, param params.CreateScaleSetParams) (scaleSetRet params.ScaleSet, err error) {
	if !auth.IsAdmin(ctx) {
		return params.ScaleSet{}, runnerErrors.ErrUnauthorized
	}

	if param.RunnerBootstrapTimeout == 0 {
		param.RunnerBootstrapTimeout = appdefaults.DefaultRunnerBootstrapTimeout
	}

	if param.GitHubRunnerGroup == "" {
		param.GitHubRunnerGroup = "Default"
	}

	entity, err := r.store.GetForgeEntity(ctx, entityType, entityID)
	if err != nil {
		return params.ScaleSet{}, fmt.Errorf("error getting entity: %w", err)
	}

	if entity.Credentials.ForgeType != params.GithubEndpointType {
		return params.ScaleSet{}, runnerErrors.NewBadRequestError("scale sets are only supported for github entities")
	}

	ghCli, err := github.Client(ctx, entity)
	if err != nil {
		return params.ScaleSet{}, fmt.Errorf("error creating github client: %w", err)
	}

	scalesetCli, err := scalesets.NewClient(ghCli)
	if err != nil {
		return params.ScaleSet{}, fmt.Errorf("error getting scaleset client: %w", err)
	}
	var runnerGroupID int64 = 1
	if param.GitHubRunnerGroup != "Default" {
		runnerGroup, err := scalesetCli.GetRunnerGroupByName(ctx, param.GitHubRunnerGroup)
		if err != nil {
			return params.ScaleSet{}, fmt.Errorf("error getting runner group: %w", err)
		}
		runnerGroupID = runnerGroup.ID
	}

	createParam := &params.RunnerScaleSet{
		Name:          param.Name,
		RunnerGroupID: runnerGroupID,
		Labels: []params.Label{
			{
				Name: param.Name,
				Type: "System",
			},
		},
		RunnerSetting: params.RunnerSetting{
			Ephemeral:     true,
			DisableUpdate: param.DisableUpdate,
		},
		Enabled: &param.Enabled,
	}

	runnerScaleSet, err := scalesetCli.CreateRunnerScaleSet(ctx, createParam)
	if err != nil {
		return params.ScaleSet{}, fmt.Errorf("error creating runner scale set: %w", err)
	}

	defer func() {
		if err != nil {
			if innerErr := scalesetCli.DeleteRunnerScaleSet(ctx, runnerScaleSet.ID); innerErr != nil {
				slog.With(slog.Any("error", innerErr)).ErrorContext(ctx, "failed to cleanup scale set")
			}
		}
	}()
	param.ScaleSetID = runnerScaleSet.ID

	scaleSet, err := r.store.CreateEntityScaleSet(ctx, entity, param)
	if err != nil {
		return params.ScaleSet{}, fmt.Errorf("error creating scale set: %w", err)
	}

	return scaleSet, nil
}

func (r *Runner) ListScaleSetInstances(ctx context.Context, scalesetID uint) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	instances, err := r.store.ListScaleSetInstances(ctx, scalesetID)
	if err != nil {
		return []params.Instance{}, fmt.Errorf("error fetching instances: %w", err)
	}
	return instances, nil
}

func (r *Runner) ListEntityScaleSets(ctx context.Context, entityType params.ForgeEntityType, entityID string) ([]params.ScaleSet, error) {
	if !auth.IsAdmin(ctx) {
		return []params.ScaleSet{}, runnerErrors.ErrUnauthorized
	}
	entity := params.ForgeEntity{
		ID:         entityID,
		EntityType: entityType,
	}
	scaleSets, err := r.store.ListEntityScaleSets(ctx, entity)
	if err != nil {
		return nil, fmt.Errorf("error fetching scale sets: %w", err)
	}
	return scaleSets, nil
}
