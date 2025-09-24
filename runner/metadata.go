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
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"strings"

	"github.com/cloudbase/garm-provider-common/cloudconfig"
	"github.com/cloudbase/garm-provider-common/defaults"
	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/auth"
	"github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/internal/templates"
	"github.com/cloudbase/garm/params"
	garmUtil "github.com/cloudbase/garm/util"
)

var (
	poolIDLabelprefix     = "runner-pool-id"
	controllerLabelPrefix = "runner-controller-id"
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
	entity, err := auth.InstanceEntity(ctx)
	if err != nil {
		slog.ErrorContext(r.ctx, "failed to get entity", "error", err)
		return "", runnerErrors.ErrUnauthorized
	}

	serviceName, err := r.getServiceNameForEntity(entity)
	if err != nil {
		slog.ErrorContext(r.ctx, "failed to get service name", "error", err)
		return "", fmt.Errorf("error fetching service name: %w", err)
	}
	return serviceName, nil
}

func getLabelsForInstance(instance params.Instance) []string {
	jitEnabled := len(instance.JitConfiguration) > 0
	if jitEnabled {
		return []string{}
	}

	if instance.ScaleSetID > 0 {
		return []string{}
	}

	pool, ok := cache.GetPoolByID(instance.PoolID)
	if !ok {
		return []string{}
	}
	var labels []string
	for _, val := range pool.Tags {
		labels = append(labels, val.Name)
	}

	labels = append(labels, fmt.Sprintf("%s=%s", controllerLabelPrefix, cache.ControllerInfo().ControllerID.String()))
	labels = append(labels, fmt.Sprintf("%s=%s", poolIDLabelprefix, instance.PoolID))
	return labels
}

func (r *Runner) getRunnerInstallTemplateContext(instance params.Instance, entity params.ForgeEntity, token string, extraContext map[string]string) (cloudconfig.InstallRunnerParams, error) {
	tools, err := cache.GetGithubToolsCache(entity.ID)
	if err != nil {
		return cloudconfig.InstallRunnerParams{}, fmt.Errorf("failed to get tools: %w", err)
	}

	foundTools, err := util.GetTools(instance.OSType, instance.OSArch, tools)
	if err != nil {
		return cloudconfig.InstallRunnerParams{}, fmt.Errorf("failed to find tools: %w", err)
	}

	installRunnerParams := cloudconfig.InstallRunnerParams{
		FileName:          foundTools.GetFilename(),
		DownloadURL:       foundTools.GetDownloadURL(),
		RunnerUsername:    defaults.DefaultUser,
		RunnerGroup:       defaults.DefaultUser,
		RepoURL:           entity.ForgeURL(),
		MetadataURL:       instance.MetadataURL,
		RunnerName:        instance.Name,
		RunnerLabels:      strings.Join(getLabelsForInstance(instance), ","),
		CallbackURL:       instance.CallbackURL,
		CallbackToken:     token,
		TempDownloadToken: foundTools.GetTempDownloadToken(),
		GitHubRunnerGroup: instance.GitHubRunnerGroup,
		ExtraContext:      extraContext,
		CABundle:          string(entity.Credentials.CABundle),
		UseJITConfig:      len(instance.JitConfiguration) > 0,
	}
	return installRunnerParams, nil
}

func (r *Runner) GetRunnerInstallScript(ctx context.Context) ([]byte, error) {
	instance, err := validateInstanceState(ctx)
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get instance params")
		return nil, runnerErrors.ErrUnauthorized
	}

	entity, err := auth.InstanceEntity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance entity: %w", err)
	}

	token, err := auth.InstanceAuthToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance token: %w", err)
	}

	var templateID uint
	var specs cloudconfig.CloudConfigSpec
	var extraSpecs json.RawMessage
	switch {
	case instance.PoolID != "":
		pool, err := r.store.GetPoolByID(r.ctx, instance.PoolID)
		if err != nil {
			return nil, fmt.Errorf("failed to get pool for instance: %w", err)
		}
		specs, err = garmUtil.GetCloudConfigSpecFromExtraSpecs(pool.ExtraSpecs)
		if err != nil {
			return nil, fmt.Errorf("failed to extract extra specs from pool: %w", err)
		}
		extraSpecs = pool.ExtraSpecs
		templateID = pool.TemplateID
	case instance.ScaleSetID > 0:
		scaleSet, err := r.store.GetScaleSetByID(r.ctx, instance.ScaleSetID)
		if err != nil {
			return nil, fmt.Errorf("failed to get scale set for instance: %w", err)
		}
		specs, err = garmUtil.GetCloudConfigSpecFromExtraSpecs(scaleSet.ExtraSpecs)
		if err != nil {
			return nil, fmt.Errorf("failed to extract extra specs from scale set: %w", err)
		}
		extraSpecs = scaleSet.ExtraSpecs
		templateID = scaleSet.TemplateID
	default:
		return nil, fmt.Errorf("instance is not part of a pool or scale set")
	}

	if templateID == 0 && len(specs.RunnerInstallTemplate) == 0 {
		return nil, runnerErrors.NewConflictError("pool or scale set has no template associated and no template is defined in extra_specs")
	}

	installCtx, err := r.getRunnerInstallTemplateContext(instance, entity, token, specs.ExtraContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get runner install context: %w", err)
	}

	var extraSpecsMap map[string]any
	if extraSpecs != nil {
		if err := json.Unmarshal(extraSpecs, &extraSpecsMap); err == nil {
			if debug, ok := extraSpecsMap["enable_boot_debug"]; ok {
				installCtx.EnableBootDebug = debug.(bool)
			}
		}
	}

	var tplBytes []byte
	if len(specs.RunnerInstallTemplate) > 0 {
		installCtx.ExtraContext = specs.ExtraContext
		tplBytes = specs.RunnerInstallTemplate
	} else {
		template, err := r.store.GetTemplate(r.ctx, templateID)
		if err != nil {
			return nil, fmt.Errorf("failed to get template: %w", err)
		}
		tplBytes = template.Data
	}

	installScript, err := templates.RenderRunnerInstallScript(string(tplBytes), installCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get runner install script: %w", err)
	}
	return installScript, nil
}

func (r *Runner) GenerateSystemdUnitFile(ctx context.Context, runAsUser string) ([]byte, error) {
	entity, err := auth.InstanceEntity(ctx)
	if err != nil {
		slog.ErrorContext(r.ctx, "failed to get entity", "error", err)
		return nil, runnerErrors.ErrUnauthorized
	}

	serviceName, err := r.getServiceNameForEntity(entity)
	if err != nil {
		slog.ErrorContext(r.ctx, "failed to get service name", "error", err)
		return nil, fmt.Errorf("error fetching service name: %w", err)
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
		return nil, fmt.Errorf("error parsing template: %w", err)
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
		return nil, fmt.Errorf("error executing template: %w", err)
	}
	return unitFile.Bytes(), nil
}

func (r *Runner) GetJITConfigFile(ctx context.Context, file string) ([]byte, error) {
	if !auth.InstanceHasJITConfig(ctx) {
		return nil, runnerErrors.NewNotFoundError("instance not configured for JIT")
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
		return nil, runnerErrors.NewNotFoundError("could not find file %q", file)
	}

	decoded, err := base64.StdEncoding.DecodeString(contents)
	if err != nil {
		return nil, fmt.Errorf("error decoding file contents: %w", err)
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
		return "", fmt.Errorf("error fetching pool manager for instance %s (%s): %w", instance.Name, instance.PoolID, err)
	}

	token, err := poolMgr.GithubRunnerRegistrationToken()
	if err != nil {
		return "", fmt.Errorf("error fetching runner token: %w", err)
	}

	tokenFetched := true
	updateParams := params.UpdateInstanceParams{
		TokenFetched: &tokenFetched,
	}

	if _, err := r.store.UpdateInstance(r.ctx, instance.Name, updateParams); err != nil {
		return "", fmt.Errorf("error setting token_fetched for instance: %w", err)
	}

	if err := r.store.AddInstanceEvent(ctx, instance.Name, params.FetchTokenEvent, params.EventInfo, "runner registration token was retrieved"); err != nil {
		return "", fmt.Errorf("error recording event: %w", err)
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

	entity, err := auth.InstanceEntity(ctx)
	if err != nil {
		slog.ErrorContext(r.ctx, "failed to get entity", "error", err)
		return params.CertificateBundle{}, runnerErrors.ErrUnauthorized
	}

	bundle, err := entity.Credentials.RootCertificateBundle()
	if err != nil {
		slog.With(slog.Any("error", err)).ErrorContext(
			ctx, "failed to get root CA bundle",
			"instance", instance.Name)
		// The root CA bundle is invalid. Return an empty bundle to the runner and log the event.
		return params.CertificateBundle{
			RootCertificates: make(map[string][]byte),
		}, nil
	}
	return bundle, nil
}
