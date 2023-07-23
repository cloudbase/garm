package external

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"

	"github.com/cloudbase/garm-provider-common/execution"

	commonParams "github.com/cloudbase/garm-provider-common/params"

	garmErrors "github.com/cloudbase/garm-provider-common/errors"
	garmExec "github.com/cloudbase/garm-provider-common/util/exec"
	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"

	"github.com/pkg/errors"
)

var _ common.Provider = (*external)(nil)

func NewProvider(ctx context.Context, cfg *config.Provider, controllerID string) (common.Provider, error) {
	if cfg.ProviderType != params.ExternalProvider {
		return nil, garmErrors.NewBadRequestError("invalid provider config")
	}

	execPath, err := cfg.External.ExecutablePath()
	if err != nil {
		return nil, errors.Wrap(err, "fetching executable path")
	}
	return &external{
		ctx:          ctx,
		controllerID: controllerID,
		cfg:          cfg,
		execPath:     execPath,
	}, nil
}

type external struct {
	ctx          context.Context
	controllerID string
	cfg          *config.Provider
	execPath     string
}

func (e *external) validateResult(inst commonParams.ProviderInstance) error {
	if inst.ProviderID == "" {
		return garmErrors.NewProviderError("missing provider ID")
	}

	if inst.Name == "" {
		return garmErrors.NewProviderError("missing instance name")
	}

	if inst.OSName == "" || inst.OSArch == "" || inst.OSType == "" {
		// we can still function without this info (I think)
		log.Printf("WARNING: missing OS information")
	}
	if !IsValidProviderStatus(inst.Status) {
		return garmErrors.NewProviderError("invalid status returned (%s)", inst.Status)
	}

	return nil
}

// CreateInstance creates a new compute instance in the provider.
func (e *external) CreateInstance(ctx context.Context, bootstrapParams commonParams.BootstrapInstance) (commonParams.ProviderInstance, error) {
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", execution.CreateInstanceCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_POOL_ID=%s", bootstrapParams.PoolID),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}

	asJs, err := json.Marshal(bootstrapParams)
	if err != nil {
		return commonParams.ProviderInstance{}, errors.Wrap(err, "serializing bootstrap params")
	}

	out, err := garmExec.Exec(ctx, e.execPath, asJs, asEnv)
	if err != nil {
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}

	var param commonParams.ProviderInstance
	if err := json.Unmarshal(out, &param); err != nil {
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to decode response from binary: %s", err)
	}

	if err := e.validateResult(param); err != nil {
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to validate result: %s", err)
	}

	retAsJs, _ := json.MarshalIndent(param, "", "  ")
	log.Printf("provider returned: %s", string(retAsJs))
	return providerInstanceToParamsInstance(param), nil
}

// Delete instance will delete the instance in a provider.
func (e *external) DeleteInstance(ctx context.Context, instance string) error {
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", execution.DeleteInstanceCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}

	_, err := garmExec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != execution.ExitCodeNotFound {
			return garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
		}

	}
	return nil
}

// GetInstance will return details about one instance.
func (e *external) GetInstance(ctx context.Context, instance string) (commonParams.ProviderInstance, error) {
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", execution.GetInstanceCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}

	// TODO(gabriel-samfira): handle error types. Of particular insterest is to
	// know when the error is ErrNotFound.
	out, err := garmExec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}

	var param commonParams.ProviderInstance
	if err := json.Unmarshal(out, &param); err != nil {
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to decode response from binary: %s", err)
	}

	if err := e.validateResult(param); err != nil {
		return commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to validate result: %s", err)
	}

	return providerInstanceToParamsInstance(param), nil
}

// ListInstances will list all instances for a provider.
func (e *external) ListInstances(ctx context.Context, poolID string) ([]commonParams.ProviderInstance, error) {
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", execution.ListInstancesCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_POOL_ID=%s", poolID),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}

	out, err := garmExec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		return []commonParams.ProviderInstance{}, garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}

	var param []commonParams.ProviderInstance
	if err := json.Unmarshal(out, &param); err != nil {
		return []commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to decode response from binary: %s", err)
	}

	ret := make([]commonParams.ProviderInstance, len(param))
	for idx, inst := range param {
		if err := e.validateResult(inst); err != nil {
			return []commonParams.ProviderInstance{}, garmErrors.NewProviderError("failed to validate result: %s", err)
		}
		ret[idx] = providerInstanceToParamsInstance(inst)
	}
	return ret, nil
}

// RemoveAllInstances will remove all instances created by this provider.
func (e *external) RemoveAllInstances(ctx context.Context) error {
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", execution.RemoveAllInstancesCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	_, err := garmExec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		return garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}
	return nil
}

// Stop shuts down the instance.
func (e *external) Stop(ctx context.Context, instance string, force bool) error {
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", execution.StopInstanceCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	_, err := garmExec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		return garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}
	return nil
}

// Start boots up an instance.
func (e *external) Start(ctx context.Context, instance string) error {
	asEnv := []string{
		fmt.Sprintf("GARM_COMMAND=%s", execution.StartInstanceCommand),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
		fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile),
	}
	_, err := garmExec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
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
