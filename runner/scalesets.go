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
	"encoding/json"
	"fmt"
	"log/slog"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/appdefaults"
	"github.com/cloudbase/garm/util/github"
	"github.com/cloudbase/garm/util/github/scalesets"
	"github.com/pkg/errors"
)

func (r *Runner) ListAllScaleSets(ctx context.Context) ([]params.ScaleSet, error) {
	if !auth.IsAdmin(ctx) {
		return []params.ScaleSet{}, runnerErrors.ErrUnauthorized
	}

	scalesets, err := r.store.ListAllScaleSets(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fetching pools")
	}
	return scalesets, nil
}

func (r *Runner) GetScaleSetByID(ctx context.Context, scaleSet uint) (params.ScaleSet, error) {
	if !auth.IsAdmin(ctx) {
		return params.ScaleSet{}, runnerErrors.ErrUnauthorized
	}

	set, err := r.store.GetScaleSetByID(ctx, scaleSet)
	if err != nil {
		return params.ScaleSet{}, errors.Wrap(err, "fetching scale set")
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
			return errors.Wrap(err, "fetching scale set")
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
		return errors.Wrap(err, "getting entity")
	}

	entity, err := r.store.GetGithubEntity(ctx, paramEntity.EntityType, paramEntity.ID)
	if err != nil {
		return errors.Wrap(err, "getting entity")
	}

	ghCli, err := github.Client(ctx, entity)
	if err != nil {
		return errors.Wrap(err, "creating github client")
	}

	scalesetCli, err := scalesets.NewClient(ghCli)
	if err != nil {
		return errors.Wrap(err, "getting scaleset client")
	}

	slog.DebugContext(ctx, "deleting scale set", "scale_set_id", scaleSet.ScaleSetID)
	if err := scalesetCli.DeleteRunnerScaleSet(ctx, scaleSet.ScaleSetID); err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			slog.InfoContext(ctx, "scale set not found", "scale_set_id", scaleSet.ScaleSetID)
			return nil
		}
		slog.With(slog.Any("error", err)).ErrorContext(ctx, "failed to delete scale set from github")
		return errors.Wrap(err, "deleting scale set from github")
	}
	if err := r.store.DeleteScaleSetByID(ctx, scaleSetID); err != nil {
		return errors.Wrap(err, "deleting scale set")
	}
	return nil
}

func (r *Runner) UpdateScaleSetByID(ctx context.Context, scaleSetID uint, param params.UpdateScaleSetParams) (params.ScaleSet, error) {
	if !auth.IsAdmin(ctx) {
		return params.ScaleSet{}, runnerErrors.ErrUnauthorized
	}

	scaleSet, err := r.store.GetScaleSetByID(ctx, scaleSetID)
	if err != nil {
		return params.ScaleSet{}, errors.Wrap(err, "fetching scale set")
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
		return params.ScaleSet{}, errors.Wrap(err, "getting entity")
	}

	entity, err := r.store.GetGithubEntity(ctx, paramEntity.EntityType, paramEntity.ID)
	if err != nil {
		return params.ScaleSet{}, errors.Wrap(err, "getting entity")
	}

	ghCli, err := github.Client(ctx, entity)
	if err != nil {
		return params.ScaleSet{}, errors.Wrap(err, "creating github client")
	}

	callback := func(old, new params.ScaleSet) error {
		scalesetCli, err := scalesets.NewClient(ghCli)
		if err != nil {
			return errors.Wrap(err, "getting scaleset client")
		}

		updateParams := params.RunnerScaleSet{}
		hasUpdates := false
		if old.Name != new.Name {
			updateParams.Name = new.Name
			hasUpdates = true
		}

		if old.GitHubRunnerGroup != new.GitHubRunnerGroup {
			runnerGroup, err := scalesetCli.GetRunnerGroupByName(ctx, new.GitHubRunnerGroup)
			if err != nil {
				return fmt.Errorf("error fetching runner group from github: %w", err)
			}
			updateParams.RunnerGroupID = int(runnerGroup.ID)
			hasUpdates = true
		}

		if old.DisableUpdate != new.DisableUpdate {
			updateParams.RunnerSetting.DisableUpdate = new.DisableUpdate
			hasUpdates = true
		}

		if hasUpdates {
			result, err := scalesetCli.UpdateRunnerScaleSet(ctx, new.ScaleSetID, updateParams)
			if err != nil {
				return fmt.Errorf("failed to update scaleset in github: %w", err)
			}
			asJs, _ := json.MarshalIndent(result, "", "  ")
			slog.Info("update result", "data", string(asJs))
		}
		return nil
	}

	newScaleSet, err := r.store.UpdateEntityScaleSet(ctx, entity, scaleSetID, param, callback)
	if err != nil {
		return params.ScaleSet{}, errors.Wrap(err, "updating pool")
	}
	return newScaleSet, nil
}

func (r *Runner) CreateEntityScaleSet(ctx context.Context, entityType params.GithubEntityType, entityID string, param params.CreateScaleSetParams) (scaleSetRet params.ScaleSet, err error) {
	if !auth.IsAdmin(ctx) {
		return params.ScaleSet{}, runnerErrors.ErrUnauthorized
	}

	if param.RunnerBootstrapTimeout == 0 {
		param.RunnerBootstrapTimeout = appdefaults.DefaultRunnerBootstrapTimeout
	}

	if param.GitHubRunnerGroup == "" {
		param.GitHubRunnerGroup = "Default"
	}

	entity, err := r.store.GetGithubEntity(ctx, entityType, entityID)
	if err != nil {
		return params.ScaleSet{}, errors.Wrap(err, "getting entity")
	}

	ghCli, err := github.Client(ctx, entity)
	if err != nil {
		return params.ScaleSet{}, errors.Wrap(err, "creating github client")
	}

	scalesetCli, err := scalesets.NewClient(ghCli)
	if err != nil {
		return params.ScaleSet{}, errors.Wrap(err, "getting scaleset client")
	}
	var runnerGroupID int = 1
	if param.GitHubRunnerGroup != "Default" {
		runnerGroup, err := scalesetCli.GetRunnerGroupByName(ctx, param.GitHubRunnerGroup)
		if err != nil {
			return params.ScaleSet{}, errors.Wrap(err, "getting runner group")
		}
		runnerGroupID = int(runnerGroup.ID)
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
		return params.ScaleSet{}, errors.Wrap(err, "creating runner scale set")
	}

	asJs, _ := json.MarshalIndent(runnerScaleSet, "", "  ")
	slog.InfoContext(ctx, "scale set", "data", string(asJs))

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
		return params.ScaleSet{}, errors.Wrap(err, "creating scale set")
	}

	return scaleSet, nil
}

func (r *Runner) ListScaleSetInstances(ctx context.Context, scalesetID uint) ([]params.Instance, error) {
	if !auth.IsAdmin(ctx) {
		return nil, runnerErrors.ErrUnauthorized
	}

	instances, err := r.store.ListScaleSetInstances(ctx, scalesetID)
	if err != nil {
		return []params.Instance{}, errors.Wrap(err, "fetching instances")
	}
	return instances, nil
}

func (r *Runner) ListEntityScaleSets(ctx context.Context, entityType params.GithubEntityType, entityID string) ([]params.ScaleSet, error) {
	if !auth.IsAdmin(ctx) {
		return []params.ScaleSet{}, runnerErrors.ErrUnauthorized
	}
	entity := params.GithubEntity{
		ID:         entityID,
		EntityType: entityType,
	}
	scaleSets, err := r.store.ListEntityScaleSets(ctx, entity)
	if err != nil {
		return nil, errors.Wrap(err, "fetching scale sets")
	}
	return scaleSets, nil
}
