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
	"strconv"
	"time"

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

// opLabels carries the identity labels attached to provider operation
// metrics, resolved from the base params callers thread through. Exactly one
// of poolID / scaleSetID is populated. Callers populate the versioned params
// regardless of the provider interface version, so the v0.1.0 wrapper can
// use them for metrics even though the v0.1.0 binary contract does not.
type opLabels struct {
	poolID     string
	scaleSetID string
	entityType string
	entityID   string
}

func labelsFromBase(base common.ProviderBaseParams) opLabels {
	var scaleSetID string
	if base.ScaleSetID != 0 {
		scaleSetID = strconv.FormatUint(uint64(base.ScaleSetID), 10)
	}
	return opLabels{
		poolID:     base.PoolInfo.ID,
		scaleSetID: scaleSetID,
		entityType: string(base.EntityType),
		entityID:   base.EntityID,
	}
}

// execWithTimeout invokes the provider binary, bounding the call by the
// provider's configured exec timeout (if any). On expiry the child process is
// killed and a ProviderError is returned so the operation fails and normal
// cleanup takes over, instead of the instance being stuck in a transient
// state (e.g. creating) for as long as a hung binary sits there.
//
// The operation and the identity carried by base are recorded on the
// provider operation metrics: successful executions observe a duration
// sample, failures count an error partitioned by kind.
func (e *external) execWithTimeout(ctx context.Context, operation string, base common.ProviderBaseParams, stdinData []byte, environ []string) ([]byte, error) {
	timeout := e.cfg.External.ExecTimeout()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	start := time.Now()
	out, err := garmExec.Exec(ctx, e.execPath, stdinData, environ)
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		e.recordOpError(operation, base, metrics.ProviderErrKindTimeout)
		return nil, garmErrors.NewProviderError("provider binary %s timed out (context deadline exceeded)", e.execPath)
	}
	if err != nil {
		e.recordOpError(operation, base, metrics.ProviderErrKindProvider)
		return out, err
	}
	labels := labelsFromBase(base)
	metrics.ProviderOperationDuration.WithLabelValues(
		e.cfg.Name,        // label: provider
		operation,         // label: operation
		labels.poolID,     // label: pool_id
		labels.scaleSetID, // label: scaleset_id
		labels.entityType, // label: entity_type
		labels.entityID,   // label: entity_id
	).Observe(time.Since(start).Seconds())
	return out, err
}

// recordOp counts a provider operation attempt.
func (e *external) recordOp(operation string, base common.ProviderBaseParams) {
	labels := labelsFromBase(base)
	metrics.InstanceOperationCount.WithLabelValues(
		operation,         // label: operation
		e.cfg.Name,        // label: provider
		labels.poolID,     // label: pool_id
		labels.scaleSetID, // label: scaleset_id
		labels.entityType, // label: entity_type
		labels.entityID,   // label: entity_id
	).Inc()
}

// recordOpFailure counts a failed provider operation attempt.
func (e *external) recordOpFailure(operation string, base common.ProviderBaseParams) {
	labels := labelsFromBase(base)
	metrics.InstanceOperationFailedCount.WithLabelValues(
		operation,         // label: operation
		e.cfg.Name,        // label: provider
		labels.poolID,     // label: pool_id
		labels.scaleSetID, // label: scaleset_id
		labels.entityType, // label: entity_type
		labels.entityID,   // label: entity_id
	).Inc()
}

// recordOpError counts a failed provider operation on the provider operation
// metrics, by error kind.
func (e *external) recordOpError(operation string, base common.ProviderBaseParams, errKind string) {
	labels := labelsFromBase(base)
	metrics.ProviderOperationErrorsCount.WithLabelValues(
		e.cfg.Name,        // label: provider
		operation,         // label: operation
		labels.poolID,     // label: pool_id
		labels.scaleSetID, // label: scaleset_id
		labels.entityType, // label: entity_type
		labels.entityID,   // label: entity_id
		errKind,           // label: error_kind
	).Inc()
}

// CreateInstance creates a new compute instance in the provider.
func (e *external) CreateInstance(ctx context.Context, bootstrapParams commonParams.BootstrapInstance, createInstanceParams common.CreateInstanceParams) (commonParams.ProviderInstance, error) {
	base := createInstanceParams.CreateInstanceV011.ProviderBaseParams
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

	e.recordOp("CreateInstance", base)

	out, err := e.execWithTimeout(ctx, "CreateInstance", base, asJs, asEnv)
	if err != nil {
		e.recordOpFailure("CreateInstance", base)
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}

	var param commonParams.ProviderInstance
	if err := json.Unmarshal(out, &param); err != nil {
		e.recordOpFailure("CreateInstance", base)
		e.recordOpError("CreateInstance", base, metrics.ProviderErrKindDecode)
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to decode response from binary: %s", err)
	}

	if err := commonExternal.ValidateResult(param); err != nil {
		e.recordOpFailure("CreateInstance", base)
		e.recordOpError("CreateInstance", base, metrics.ProviderErrKindValidation)
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to validate result: %s", err)
	}

	retAsJs, _ := json.MarshalIndent(param, "", "  ")
	slog.DebugContext(
		ctx, "provider returned",
		"output", string(retAsJs))
	return param, nil
}

// Delete instance will delete the instance in a provider.
func (e *external) DeleteInstance(ctx context.Context, instance string, deleteInstanceParams common.DeleteInstanceParams) error {
	base := deleteInstanceParams.DeleteInstanceV011.ProviderBaseParams
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", commonExecution.DeleteInstanceCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	asEnv = append(asEnv, e.environmentVariables...)

	e.recordOp("DeleteInstance", base)
	_, err := e.execWithTimeout(ctx, "DeleteInstance", base, nil, asEnv)
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != commonExecution.ExitCodeNotFound {
			e.recordOpFailure("DeleteInstance", base)
			return garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
		}
	}
	return nil
}

// GetInstance will return details about one instance.
func (e *external) GetInstance(ctx context.Context, instance string, getInstanceParams common.GetInstanceParams) (commonParams.ProviderInstance, error) {
	base := getInstanceParams.GetInstanceV011.ProviderBaseParams
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
	e.recordOp("GetInstance", base)
	out, err := e.execWithTimeout(ctx, "GetInstance", base, nil, asEnv)
	if err != nil {
		e.recordOpFailure("GetInstance", base)
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}

	var param commonParams.ProviderInstance
	if err := json.Unmarshal(out, &param); err != nil {
		e.recordOpFailure("GetInstance", base)
		e.recordOpError("GetInstance", base, metrics.ProviderErrKindDecode)
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to decode response from binary: %s", err)
	}

	if err := commonExternal.ValidateResult(param); err != nil {
		e.recordOpFailure("GetInstance", base)
		e.recordOpError("GetInstance", base, metrics.ProviderErrKindValidation)
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to validate result: %s", err)
	}

	return param, nil
}

// ListInstances will list all instances for a provider.
func (e *external) ListInstances(ctx context.Context, poolID string, listInstancesParams common.ListInstancesParams) ([]commonParams.ProviderInstance, error) {
	base := listInstancesParams.ListInstancesV011.ProviderBaseParams
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", commonExecution.ListInstancesCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_POOL_ID=%s", poolID),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	asEnv = append(asEnv, e.environmentVariables...)

	e.recordOp("ListInstances", base)

	out, err := e.execWithTimeout(ctx, "ListInstances", base, nil, asEnv)
	if err != nil {
		e.recordOpFailure("ListInstances", base)
		return []commonParams.ProviderInstance{}, garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}

	var param []commonParams.ProviderInstance
	if err := json.Unmarshal(out, &param); err != nil {
		e.recordOpFailure("ListInstances", base)
		e.recordOpError("ListInstances", base, metrics.ProviderErrKindDecode)
		return []commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to decode response from binary: %s", err)
	}

	ret := make([]commonParams.ProviderInstance, len(param))
	for idx, inst := range param {
		if err := commonExternal.ValidateResult(inst); err != nil {
			e.recordOpFailure("ListInstances", base)
			e.recordOpError("ListInstances", base, metrics.ProviderErrKindValidation)
			return []commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to validate result: %s", err)
		}
		ret[idx] = inst
	}
	return ret, nil
}

// RemoveAllInstances will remove all instances created by this provider.
func (e *external) RemoveAllInstances(ctx context.Context, removeAllInstances common.RemoveAllInstancesParams) error {
	base := removeAllInstances.RemoveAllInstancesV011.ProviderBaseParams
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", commonExecution.RemoveAllInstancesCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	asEnv = append(asEnv, e.environmentVariables...)

	e.recordOp("RemoveAllInstances", base)

	_, err := e.execWithTimeout(ctx, "RemoveAllInstances", base, nil, asEnv)
	if err != nil {
		e.recordOpFailure("RemoveAllInstances", base)
		return garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}
	return nil
}

// Stop shuts down the instance.
func (e *external) Stop(ctx context.Context, instance string, stopParams common.StopParams) error {
	base := stopParams.StopV011.ProviderBaseParams
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", commonExecution.StopInstanceCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	asEnv = append(asEnv, e.environmentVariables...)

	e.recordOp("Stop", base)
	_, err := e.execWithTimeout(ctx, "Stop", base, nil, asEnv)
	if err != nil {
		e.recordOpFailure("Stop", base)
		return garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}
	return nil
}

// Start boots up an instance.
func (e *external) Start(ctx context.Context, instance string, startParams common.StartParams) error {
	base := startParams.StartV011.ProviderBaseParams
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", commonExecution.StartInstanceCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	asEnv = append(asEnv, e.environmentVariables...)

	e.recordOp("Start", base)

	_, err := e.execWithTimeout(ctx, "Start", base, nil, asEnv)
	if err != nil {
		e.recordOpFailure("Start", base)
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
