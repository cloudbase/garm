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

package v010

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"

	garmErrors "github.com/cloudbase/garm-provider-common/errors"
	commonExecution "github.com/cloudbase/garm-provider-common/execution/common"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	garmExec "github.com/cloudbase/garm-provider-common/util/exec"
	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/metrics"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	commonExternal "github.com/cloudbase/garm/runner/providers/common"
)

var _ common.Provider = (*external)(nil)

// NewProvider creates a legacy external provider.
func NewProvider(ctx context.Context, cfg *config.Provider, controllerID string) (common.Provider, error) {
	if cfg.ProviderType != params.ExternalProvider {
		return nil, garmErrors.NewBadRequestError("invalid provider config")
	}

	execPath, err := cfg.External.ExecutablePath()
	if err != nil {
		return nil, fmt.Errorf("error fetching executable path: %w", err)
	}

	// Set GARM_INTERFACE_VERSION to the version of the interface that the external
	// provider implements. This is used to ensure compatibility between the external
	// provider and garm

	envVars := cfg.External.GetEnvironmentVariables()
	envVars = append(envVars, fmt.Sprintf("GARM_INTERFACE_VERSION=%s", common.Version010))

	return &external{
		ctx:                  ctx,
		controllerID:         controllerID,
		cfg:                  cfg,
		execPath:             execPath,
		environmentVariables: envVars,
	}, nil
}

type external struct {
	ctx                  context.Context
	controllerID         string
	cfg                  *config.Provider
	execPath             string
	environmentVariables []string
}

// CreateInstance creates a new compute instance in the provider.
func (e *external) CreateInstance(ctx context.Context, bootstrapParams commonParams.BootstrapInstance, _ common.CreateInstanceParams) (commonParams.ProviderInstance, error) {
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", commonExecution.CreateInstanceCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_POOL_ID=%s", bootstrapParams.PoolID),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	asEnv = append(asEnv, e.environmentVariables...)

	asJs, err := json.Marshal(bootstrapParams)
	if err != nil {
		return commonParams.ProviderInstance{}, fmt.Errorf("error serializing bootstrap params: %w", err)
	}

	metrics.InstanceOperationCount.WithLabelValues(
		"CreateInstance", // label: operation
		e.cfg.Name,       // label: provider
	).Inc()

	out, err := garmExec.Exec(ctx, e.execPath, asJs, asEnv)
	if err != nil {
		metrics.InstanceOperationFailedCount.WithLabelValues(
			"CreateInstance", // label: operation
			e.cfg.Name,       // label: provider
		).Inc()
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}

	var param commonParams.ProviderInstance
	if err := json.Unmarshal(out, &param); err != nil {
		metrics.InstanceOperationFailedCount.WithLabelValues(
			"CreateInstance", // label: operation
			e.cfg.Name,       // label: provider
		).Inc()
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to decode response from binary: %s", err)
	}

	if err := commonExternal.ValidateResult(param); err != nil {
		metrics.InstanceOperationFailedCount.WithLabelValues(
			"CreateInstance", // label: operation
			e.cfg.Name,       // label: provider
		).Inc()
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to validate result: %s", err)
	}

	retAsJs, _ := json.MarshalIndent(param, "", "  ")
	slog.DebugContext(
		ctx, "provider returned",
		"output", string(retAsJs))
	return param, nil
}

// Delete instance will delete the instance in a provider.
func (e *external) DeleteInstance(ctx context.Context, instance string, _ common.DeleteInstanceParams) error {
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", commonExecution.DeleteInstanceCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	asEnv = append(asEnv, e.environmentVariables...)

	metrics.InstanceOperationCount.WithLabelValues(
		"DeleteInstance", // label: operation
		e.cfg.Name,       // label: provider
	).Inc()
	_, err := garmExec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != commonExecution.ExitCodeNotFound {
			metrics.InstanceOperationFailedCount.WithLabelValues(
				"DeleteInstance", // label: operation
				e.cfg.Name,       // label: provider
			).Inc()
			return garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
		}
	}
	return nil
}

// GetInstance will return details about one instance.
func (e *external) GetInstance(ctx context.Context, instance string, _ common.GetInstanceParams) (commonParams.ProviderInstance, error) {
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", commonExecution.GetInstanceCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	asEnv = append(asEnv, e.environmentVariables...)

	// nolint:golangci-lint,godox
	// TODO(gabriel-samfira): handle error types. Of particular interest is to
	// know when the error is ErrNotFound.
	metrics.InstanceOperationCount.WithLabelValues(
		"GetInstance", // label: operation
		e.cfg.Name,    // label: provider
	).Inc()
	out, err := garmExec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		metrics.InstanceOperationFailedCount.WithLabelValues(
			"GetInstance", // label: operation
			e.cfg.Name,    // label: provider
		).Inc()
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}

	var param commonParams.ProviderInstance
	if err := json.Unmarshal(out, &param); err != nil {
		metrics.InstanceOperationFailedCount.WithLabelValues(
			"GetInstance", // label: operation
			e.cfg.Name,    // label: provider
		).Inc()
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to decode response from binary: %s", err)
	}

	if err := commonExternal.ValidateResult(param); err != nil {
		metrics.InstanceOperationFailedCount.WithLabelValues(
			"GetInstance", // label: operation
			e.cfg.Name,    // label: provider
		).Inc()
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to validate result: %s", err)
	}

	return param, nil
}

// ListInstances will list all instances for a provider.
func (e *external) ListInstances(ctx context.Context, poolID string, _ common.ListInstancesParams) ([]commonParams.ProviderInstance, error) {
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", commonExecution.ListInstancesCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_POOL_ID=%s", poolID),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	asEnv = append(asEnv, e.environmentVariables...)

	metrics.InstanceOperationCount.WithLabelValues(
		"ListInstances", // label: operation
		e.cfg.Name,      // label: provider
	).Inc()

	out, err := garmExec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		metrics.InstanceOperationFailedCount.WithLabelValues(
			"ListInstances", // label: operation
			e.cfg.Name,      // label: provider
		).Inc()
		return []commonParams.ProviderInstance{}, garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}

	var param []commonParams.ProviderInstance
	if err := json.Unmarshal(out, &param); err != nil {
		metrics.InstanceOperationFailedCount.WithLabelValues(
			"ListInstances", // label: operation
			e.cfg.Name,      // label: provider
		).Inc()
		return []commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to decode response from binary: %s", err)
	}

	ret := make([]commonParams.ProviderInstance, len(param))
	for idx, inst := range param {
		if err := commonExternal.ValidateResult(inst); err != nil {
			metrics.InstanceOperationFailedCount.WithLabelValues(
				"ListInstances", // label: operation
				e.cfg.Name,      // label: provider
			).Inc()
			return []commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to validate result: %s", err)
		}
		ret[idx] = inst
	}
	return ret, nil
}

// RemoveAllInstances will remove all instances created by this provider.
func (e *external) RemoveAllInstances(ctx context.Context, _ common.RemoveAllInstancesParams) error {
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", commonExecution.RemoveAllInstancesCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	asEnv = append(asEnv, e.environmentVariables...)

	metrics.InstanceOperationCount.WithLabelValues(
		"RemoveAllInstances", // label: operation
		e.cfg.Name,           // label: provider
	).Inc()

	_, err := garmExec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		metrics.InstanceOperationFailedCount.WithLabelValues(
			"RemoveAllInstances", // label: operation
			e.cfg.Name,           // label: provider
		).Inc()
		return garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}
	return nil
}

// Stop shuts down the instance.
func (e *external) Stop(ctx context.Context, instance string, _ common.StopParams) error {
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", commonExecution.StopInstanceCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	asEnv = append(asEnv, e.environmentVariables...)

	metrics.InstanceOperationCount.WithLabelValues(
		"Stop",     // label: operation
		e.cfg.Name, // label: provider
	).Inc()
	_, err := garmExec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		metrics.InstanceOperationFailedCount.WithLabelValues(
			"Stop",     // label: operation
			e.cfg.Name, // label: provider
		).Inc()
		return garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}
	return nil
}

// Start boots up an instance.
func (e *external) Start(ctx context.Context, instance string, _ common.StartParams) error {
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", commonExecution.StartInstanceCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	asEnv = append(asEnv, e.environmentVariables...)

	metrics.InstanceOperationCount.WithLabelValues(
		"Start",    // label: operation
		e.cfg.Name, // label: provider
	).Inc()

	_, err := garmExec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		metrics.InstanceOperationFailedCount.WithLabelValues(
			"Start",    // label: operation
			e.cfg.Name, // label: provider
		).Inc()
		return garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}
	return nil
}

func (e *external) AsParams() params.Provider {
	return params.Provider{
		Name:         e.cfg.Name,
		Description:  e.cfg.Description,
		ProviderType: e.cfg.ProviderType,
	}
}

// DisableJITConfig tells us if the provider explicitly disables JIT configuration and
// forces runner registration tokens to be used. This may happen if a provider has not yet
// been updated to support JIT configuration.
func (e *external) DisableJITConfig() bool {
	if e.cfg == nil {
		return false
	}
	return e.cfg.DisableJITConfig
}
