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
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"log/slog"

	"github.com/pkg/errors"

	"github.com/cloudbase/garm-provider-common/defaults"
	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/params"
)

var githubSystemdUnitTemplate = `[Unit]
Description=GitHub Actions Runner ({{.ServiceName}})
After=network.target

[Service]
ExecStart=/home/{{.RunAsUser}}/actions-runner/runsvc.sh
User={{.RunAsUser}}
WorkingDirectory=/home/{{.RunAsUser}}/actions-runner
KillMode=process
KillSignal=SIGTERM
TimeoutStopSec=5min

[Install]
WantedBy=multi-user.target
`

var giteaSystemdUnitTemplate = `[Unit]
Description=Act Runner ({{.ServiceName}})
After=network.target

[Service]
ExecStart=/home/{{.RunAsUser}}/act-runner/act_runner daemon --once
User={{.RunAsUser}}
WorkingDirectory=/home/{{.RunAsUser}}/act-runner
KillMode=process
KillSignal=SIGTERM
TimeoutStopSec=5min
Restart=always

[Install]
WantedBy=multi-user.target
`

func validateInstanceState(ctx context.Context) (params.Instance, error) {
	status := auth.InstanceRunnerStatus(ctx)
	if status != params.RunnerPending && status != params.RunnerInstalling {
		return params.Instance{}, runnerErrors.ErrUnauthorized
	}

	instance, err := auth.InstanceParams(ctx)
	if err != nil {
		return params.Instance{}, runnerErrors.ErrUnauthorized
	}
	return instance, nil
}

func (r *Runner) getForgeEntityFromInstance(ctx context.Context, instance params.Instance) (params.ForgeEntity, error) {
	var entityGetter params.EntityGetter
	var err error
	switch {
	case instance.PoolID != "":
		entityGetter, err = r.store.GetPoolByID(r.ctx, instance.PoolID)
	case instance.ScaleSetID != 0:
		entityGetter, err = r.store.GetScaleSetByID(r.ctx, instance.ScaleSetID)
	default:
		return params.ForgeEntity{}, errors.New("instance not associated with a pool or scale set")
	}

	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get entity getter",
			"instance", instance.Name)
		return params.ForgeEntity{}, errors.Wrap(err, "fetching entity getter")
	}

	poolEntity, err := entityGetter.GetEntity()
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get entity",
			"instance", instance.Name)
		return params.ForgeEntity{}, errors.Wrap(err, "fetching entity")
	}

	entity, err := r.store.GetForgeEntity(r.ctx, poolEntity.EntityType, poolEntity.ID)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get entity",
			"instance", instance.Name)
		return params.ForgeEntity{}, errors.Wrap(err, "fetching entity")
	}
	return entity, nil
}

func (r *Runner) getServiceNameForEntity(entity params.ForgeEntity) (string, error) {
	switch entity.EntityType {
	case params.ForgeEntityTypeEnterprise:
		return fmt.Sprintf("actions.runner.%s.%s", entity.Owner, entity.Name), nil
	case params.ForgeEntityTypeOrganization:
		return fmt.Sprintf("actions.runner.%s.%s", entity.Owner, entity.Name), nil
	case params.ForgeEntityTypeRepository:
		return fmt.Sprintf("actions.runner.%s-%s.%s", entity.Owner, entity.Name, entity.Name), nil
	default:
		return "", errors.New("unknown entity type")
	}
}

func (r *Runner) GetRunnerServiceName(ctx context.Context) (string, error) {
	instance, err := validateInstanceState(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get instance params")
		return "", runnerErrors.ErrUnauthorized
	}
	entity, err := r.getForgeEntityFromInstance(ctx, instance)
	if err != nil {
		slog.ErrorContext(r.ctx, "failed to get entity", "error", err)
		return "", errors.Wrap(err, "fetching entity")
	}

	serviceName, err := r.getServiceNameForEntity(entity)
	if err != nil {
		slog.ErrorContext(r.ctx, "failed to get service name", "error", err)
		return "", errors.Wrap(err, "fetching service name")
	}
	return serviceName, nil
}

func (r *Runner) GenerateSystemdUnitFile(ctx context.Context, runAsUser string) ([]byte, error) {
	instance, err := validateInstanceState(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get instance params")
		return nil, runnerErrors.ErrUnauthorized
	}
	entity, err := r.getForgeEntityFromInstance(ctx, instance)
	if err != nil {
		slog.ErrorContext(r.ctx, "failed to get entity", "error", err)
		return nil, errors.Wrap(err, "fetching entity")
	}

	serviceName, err := r.getServiceNameForEntity(entity)
	if err != nil {
		slog.ErrorContext(r.ctx, "failed to get service name", "error", err)
		return nil, errors.Wrap(err, "fetching service name")
	}

	var unitTemplate *template.Template
	switch entity.Credentials.ForgeType {
	case params.GithubEndpointType:
		unitTemplate, err = template.New("").Parse(githubSystemdUnitTemplate)
	case params.GiteaEndpointType:
		unitTemplate, err = template.New("").Parse(giteaSystemdUnitTemplate)
	default:
		slog.ErrorContext(r.ctx, "unknown forge type", "forge_type", entity.Credentials.ForgeType)
		return nil, errors.New("unknown forge type")
	}
	if err != nil {
		slog.ErrorContext(r.ctx, "failed to parse template", "error", err)
		return nil, errors.Wrap(err, "parsing template")
	}

	if runAsUser == "" {
		runAsUser = defaults.DefaultUser
	}

	data := struct {
		ServiceName string
		RunAsUser   string
	}{
		ServiceName: serviceName,
		RunAsUser:   runAsUser,
	}

	var unitFile bytes.Buffer
	if err := unitTemplate.Execute(&unitFile, data); err != nil {
		slog.ErrorContext(r.ctx, "failed to execute template", "error", err)
		return nil, errors.Wrap(err, "executing template")
	}
	return unitFile.Bytes(), nil
}

func (r *Runner) GetJITConfigFile(ctx context.Context, file string) ([]byte, error) {
	if !auth.InstanceHasJITConfig(ctx) {
		return nil, fmt.Errorf("instance not configured for JIT: %w", runnerErrors.ErrNotFound)
	}

	instance, err := validateInstanceState(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get instance params")
		return nil, runnerErrors.ErrUnauthorized
	}
	jitConfig := instance.JitConfiguration
	contents, ok := jitConfig[file]
	if !ok {
		return nil, errors.Wrap(runnerErrors.ErrNotFound, "retrieving file")
	}

	decoded, err := base64.StdEncoding.DecodeString(contents)
	if err != nil {
		return nil, errors.Wrap(err, "decoding file contents")
	}

	return decoded, nil
}

func (r *Runner) GetInstanceGithubRegistrationToken(ctx context.Context) (string, error) {
	// Check if this instance already fetched a registration token or if it was configured using
	// the new Just In Time runner feature. If we're still using the old way of configuring a runner,
	// we only allow an instance to fetch one token. If the instance fails to bootstrap after a token
	// is fetched, we reset the token fetched field when re-queueing the instance.
	if auth.InstanceTokenFetched(ctx) || auth.InstanceHasJITConfig(ctx) {
		return "", runnerErrors.ErrUnauthorized
	}

	status := auth.InstanceRunnerStatus(ctx)
	if status != params.RunnerPending && status != params.RunnerInstalling {
		return "", runnerErrors.ErrUnauthorized
	}

	instance, err := auth.InstanceParams(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get instance params")
		return "", runnerErrors.ErrUnauthorized
	}

	poolMgr, err := r.getPoolManagerFromInstance(ctx, instance)
	if err != nil {
		return "", errors.Wrap(err, "fetching pool manager for instance")
	}

	token, err := poolMgr.GithubRunnerRegistrationToken()
	if err != nil {
		return "", errors.Wrap(err, "fetching runner token")
	}

	tokenFetched := true
	updateParams := params.UpdateInstanceParams{
		TokenFetched: &tokenFetched,
	}

	if _, err := r.store.UpdateInstance(r.ctx, instance.Name, updateParams); err != nil {
		return "", errors.Wrap(err, "setting token_fetched for instance")
	}

	if err := r.store.AddInstanceEvent(ctx, instance.Name, params.FetchTokenEvent, params.EventInfo, "runner registration token was retrieved"); err != nil {
		return "", errors.Wrap(err, "recording event")
	}

	return token, nil
}

func (r *Runner) GetRootCertificateBundle(ctx context.Context) (params.CertificateBundle, error) {
	instance, err := auth.InstanceParams(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get instance params")
		return params.CertificateBundle{}, runnerErrors.ErrUnauthorized
	}

	poolMgr, err := r.getPoolManagerFromInstance(ctx, instance)
	if err != nil {
		return params.CertificateBundle{}, errors.Wrap(err, "fetching pool manager for instance")
	}

	bundle, err := poolMgr.RootCABundle()
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get root CA bundle",
			"instance", instance.Name,
			"pool_manager", poolMgr.ID())
		// The root CA bundle is invalid. Return an empty bundle to the runner and log the event.
		return params.CertificateBundle{
			RootCertificates: make(map[string][]byte),
		}, nil
	}
	return bundle, nil
}
