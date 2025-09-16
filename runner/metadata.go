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
	"io"
	"log/slog"
	"net/url"
	"strings"

	"github.com/cloudbase/garm-provider-common/cloudconfig"
	"github.com/cloudbase/garm-provider-common/defaults"
	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
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
		return fmt.Sprintf("actions.runner.%s", entity.Owner), nil
	case params.ForgeEntityTypeOrganization:
		return fmt.Sprintf("actions.runner.%s", entity.Owner), nil
	case params.ForgeEntityTypeRepository:
		return fmt.Sprintf("actions.runner.%s.%s", entity.Owner, entity.Name), nil
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

func (r *Runner) GetInstanceMetadata(ctx context.Context) (params.InstanceMetadata, error) {
	instance, err := validateInstanceState(ctx)
	if err != nil {
		return params.InstanceMetadata{}, runnerErrors.ErrUnauthorized
	}

	var entityGetter params.EntityGetter
	var extraSpecs json.RawMessage
	var enableShell bool
	switch {
	case instance.PoolID != "":
		pool, err := r.store.GetPoolByID(r.ctx, instance.PoolID)
		if err != nil {
			return params.InstanceMetadata{}, fmt.Errorf("failed to get pool: %w", err)
		}
		entityGetter = pool
		extraSpecs = pool.ExtraSpecs
		enableShell = pool.EnableShell
	case instance.ScaleSetID != 0:
		scaleSet, err := r.store.GetScaleSetByID(r.ctx, instance.ScaleSetID)
		if err != nil {
			return params.InstanceMetadata{}, fmt.Errorf("failed to get scale set: %w", err)
		}
		entityGetter = scaleSet
		extraSpecs = scaleSet.ExtraSpecs
		enableShell = scaleSet.EnableShell
	default:
		// This is not actually an unauthorized scenario. This case means that an
		// instance was created but it does not belong to any pool or scale set.
		// This is an internal error state, but it's not something we should expose
		// to a potential runner that is trying to start.
		slog.ErrorContext(ctx, "runner is authentic but does not belong to a pool or scale set", "instance_name", instance.Name, "instance_id", instance.ID)
		return params.InstanceMetadata{}, runnerErrors.ErrUnauthorized
	}

	entity, err := entityGetter.GetEntity()
	if err != nil {
		return params.InstanceMetadata{}, fmt.Errorf("failed to get entity: %w", err)
	}

	dbEntity, err := r.store.GetForgeEntity(ctx, entity.EntityType, entity.ID)
	if err != nil {
		return params.InstanceMetadata{}, fmt.Errorf("failed to get entity: %w", err)
	}

	ret := params.InstanceMetadata{
		RunnerName:            instance.Name,
		RunnerLabels:          getLabelsForInstance(instance),
		RunnerRegistrationURL: dbEntity.ForgeURL(),
		MetadataAccess: params.MetadataServiceAccessDetails{
			CallbackURL: instance.CallbackURL,
			MetadataURL: instance.MetadataURL,
			AgentURL:    cache.ControllerInfo().AgentURL,
		},
		ForgeType:         dbEntity.Credentials.ForgeType,
		JITEnabled:        len(instance.JitConfiguration) > 0,
		AgentMode:         dbEntity.AgentMode,
		AgentShellEnabled: enableShell,
	}

	if dbEntity.AgentMode {
		agentTools, err := r.GetGARMTools(ctx, 0, 25)
		if err != nil {
			return params.InstanceMetadata{}, fmt.Errorf("failed to find garm agent tools: %w", err)
		}
		if agentTools.TotalCount == 0 {
			return params.InstanceMetadata{}, runnerErrors.NewConflictError("agent mode is enabled, but agent tools not available")
		}
		ret.AgentTools = &agentTools.Results[0]
		agentToken, err := r.GetAgentJWTToken(r.ctx, instance.Name)
		if err != nil {
			return params.InstanceMetadata{}, fmt.Errorf("failed to get agent token: %w", err)
		}
		ret.AgentToken = agentToken
	}

	if len(dbEntity.Credentials.Endpoint.CACertBundle) > 0 {
		// We can add other CA bundles here as needed.
		ret.CABundle["forge_ca"] = dbEntity.Credentials.Endpoint.CACertBundle
	}

	if len(extraSpecs) > 0 {
		var specs map[string]any
		if err := json.Unmarshal(extraSpecs, &specs); err != nil {
			return params.InstanceMetadata{}, fmt.Errorf("failed to decode extra specs: %w", err)
		}
		ret.ExtraSpecs = specs
	}

	tools, err := cache.GetGithubToolsCache(dbEntity.ID)
	if err != nil {
		return params.InstanceMetadata{}, fmt.Errorf("failed to find tools: %w", err)
	}

	filtered, err := util.GetTools(instance.OSType, instance.OSArch, tools)
	if err != nil {
		return params.InstanceMetadata{}, fmt.Errorf("no tools available: %w", err)
	}
	ret.RunnerTools = filtered

	switch dbEntity.Credentials.ForgeType {
	case params.GiteaEndpointType:
	case params.GithubEndpointType:
	default:
		return params.InstanceMetadata{}, fmt.Errorf("invalid forge type: %s", dbEntity.Credentials.ForgeType)
	}
	return ret, nil
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

	if specs.ExtraContext == nil {
		specs.ExtraContext = map[string]string{}
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
	// the Just In Time runner feature. If we're still using the old way of configuring a runner,
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

func fileObjectToGARMTool(obj params.FileObject, downloadURL string) (params.GARMAgentTool, error) {
	var version string
	var osType string
	var osArch string
	for _, val := range obj.Tags {
		if strings.HasPrefix(val, "version=") {
			version = val[8:]
		}
		if strings.HasPrefix(val, "os_arch=") {
			osArch = val[8:]
		}
		if strings.HasPrefix(val, "os_type=") {
			osType = val[8:]
		}
	}
	switch {
	case version == "":
		return params.GARMAgentTool{}, runnerErrors.NewConflictError("missing version for tools %d", obj.ID)
	case osType == "":
		return params.GARMAgentTool{}, runnerErrors.NewConflictError("missing os_type for tools %d", obj.ID)
	case osArch == "":
		return params.GARMAgentTool{}, runnerErrors.NewConflictError("missing os_arch for tools %d", obj.ID)
	}
	res := params.GARMAgentTool{
		ID:          obj.ID,
		Name:        obj.Name,
		Size:        obj.Size,
		SHA256SUM:   obj.SHA256,
		Description: obj.Description,
		CreatedAt:   obj.CreatedAt,
		UpdatedAt:   obj.UpdatedAt,
		FileType:    obj.FileType,
		OSType:      commonParams.OSType(osType),
		OSArch:      commonParams.OSArch(osArch),
		DownloadURL: downloadURL,
		Version:     version,
	}
	return res, nil
}

func (r *Runner) GetGARMTools(ctx context.Context, page, pageSize uint64) (params.GARMAgentToolsPaginatedResponse, error) {
	tags := []string{
		garmAgentFileTag,
	}
	instance, err := validateInstanceState(ctx)
	if err != nil {
		if !auth.IsAdmin(ctx) {
			return params.GARMAgentToolsPaginatedResponse{}, runnerErrors.ErrUnauthorized
		}
	} else {
		tags = append(tags, "os_type="+string(instance.OSType))
		tags = append(tags, "os_arch="+string(instance.OSArch))
	}

	files, err := r.store.SearchFileObjectByTags(r.ctx, tags, page, pageSize)
	if err != nil {
		return params.GARMAgentToolsPaginatedResponse{}, fmt.Errorf("failed to list files: %w", err)
	}

	var tools []params.GARMAgentTool
	for _, val := range files.Results {
		objectIDAsString := fmt.Sprintf("%d", val.ID)
		downloadURL, err := url.JoinPath(instance.MetadataURL, "tools/garm-agent", objectIDAsString, "download")
		if err != nil {
			return params.GARMAgentToolsPaginatedResponse{}, fmt.Errorf("failed to construct agent tools download URL: %w", err)
		}
		res, err := fileObjectToGARMTool(val, downloadURL)
		if err != nil {
			return params.GARMAgentToolsPaginatedResponse{}, fmt.Errorf("failed parse tools object: %w", err)
		}
		tools = append(tools, res)
	}
	return params.GARMAgentToolsPaginatedResponse{
		TotalCount:   files.TotalCount,
		Pages:        files.Pages,
		CurrentPage:  files.CurrentPage,
		NextPage:     files.NextPage,
		PreviousPage: files.PreviousPage,
		Results:      tools,
	}, nil
}

func (r *Runner) ShowGARMTools(ctx context.Context, toolsID uint) (params.GARMAgentTool, error) {
	instance, err := validateInstanceState(ctx)
	if err != nil {
		if !auth.IsAdmin(ctx) {
			return params.GARMAgentTool{}, runnerErrors.ErrUnauthorized
		}
	}

	tools, err := r.store.GetFileObject(r.ctx, toolsID)
	if err != nil {
		return params.GARMAgentTool{}, fmt.Errorf("failed to list files: %w", err)
	}

	var version string
	var osType string
	var osArch string
	var category string
	for _, val := range tools.Tags {
		if strings.HasPrefix(val, "version=") {
			version = val[8:]
		}
		if strings.HasPrefix(val, "os_arch=") {
			osArch = val[8:]
		}
		if strings.HasPrefix(val, "os_type=") {
			osType = val[8:]
		}
		if strings.HasPrefix(val, "category=") {
			category = val[9:]
		}
	}
	if category != "garm-agent" {
		slog.InfoContext(ctx, "selected object is not marked as garm-agent", "object_id", toolsID, "instance", instance.Name)
		return params.GARMAgentTool{}, runnerErrors.ErrUnauthorized
	}
	if osType != string(instance.OSType) {
		return params.GARMAgentTool{}, runnerErrors.NewBadRequestError("requested tools OS type (%s) does not match instance OS type (%s)", osType, instance.OSType)
	}
	if osArch != string(instance.OSArch) {
		return params.GARMAgentTool{}, runnerErrors.NewBadRequestError("requested tools OS arch (%s) does not match instance OS arch (%s)", osArch, instance.OSArch)
	}
	agentIDAsString := fmt.Sprintf("%d", tools.ID)
	downloadURL, err := url.JoinPath(instance.MetadataURL, "tools/garm-agent", agentIDAsString, "download")
	if err != nil {
		return params.GARMAgentTool{}, fmt.Errorf("failed to construct agent tools download URL: %w", err)
	}
	res := params.GARMAgentTool{
		ID:          tools.ID,
		Name:        tools.Name,
		Size:        tools.Size,
		SHA256SUM:   tools.SHA256,
		Description: tools.Description,
		CreatedAt:   tools.CreatedAt,
		UpdatedAt:   tools.UpdatedAt,
		FileType:    tools.FileType,
		OSType:      commonParams.OSType(osType),
		OSArch:      commonParams.OSArch(osArch),
		DownloadURL: downloadURL,
	}
	if version != "" {
		res.Version = version
	}
	return res, nil
}

func (r *Runner) GetGARMToolsReadHandler(ctx context.Context, toolsID uint) (io.ReadCloser, error) {
	toolsDetails, err := r.ShowGARMTools(ctx, toolsID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate tools request: %w", err)
	}

	readCloser, err := r.store.OpenFileObjectContent(ctx, toolsDetails.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to open file object: %w", err)
	}
	return readCloser, nil
}
