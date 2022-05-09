package external

import (
	"context"
	"encoding/json"
	"fmt"

	"garm/config"
	garmErrors "garm/errors"
	"garm/params"
	"garm/runner/common"
	"garm/util/exec"

	"github.com/pkg/errors"
)

var _ common.Provider = (*external)(nil)

func NewProvider(ctx context.Context, cfg *config.Provider, controllerID string) (common.Provider, error) {
	if cfg.ProviderType != config.ExternalProvider {
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

func (e *external) configEnvVar() string {
	return fmt.Sprintf("GARM_PROVIDER_CONFIG_FILE=%s", e.cfg.External.ConfigFile)
}

// CreateInstance creates a new compute instance in the provider.
func (e *external) CreateInstance(ctx context.Context, bootstrapParams params.BootstrapInstance) (params.Instance, error) {
	asEnv := bootstrapParamsToEnv(bootstrapParams)
	asEnv = append(asEnv, createInstanceCommand)
	asEnv = append(asEnv, fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID))
	asEnv = append(asEnv, fmt.Sprintf("GARM_POOL_ID=%s", bootstrapParams.PoolID))
	asEnv = append(asEnv, e.configEnvVar())

	asJs, err := json.Marshal(bootstrapParams)
	if err != nil {
		return params.Instance{}, errors.Wrap(err, "serializing bootstrap params")
	}

	out, err := exec.Exec(ctx, e.execPath, asJs, asEnv)
	if err != nil {
		return params.Instance{}, garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}

	var param params.Instance
	if err := json.Unmarshal(out, &param); err != nil {
		return params.Instance{}, garmErrors.NewProviderError("failed to decode response from binary: %s", err)
	}
	return param, nil
}

// Delete instance will delete the instance in a provider.
func (e *external) DeleteInstance(ctx context.Context, instance string) error {
	asEnv := []string{
		deleteInstanceCommand,
		e.configEnvVar(),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
	}

	_, err := exec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		return garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}
	return nil
}

// GetInstance will return details about one instance.
func (e *external) GetInstance(ctx context.Context, instance string) (params.Instance, error) {
	asEnv := []string{
		getInstanceCommand,
		e.configEnvVar(),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
	}

	out, err := exec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		return params.Instance{}, garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}

	var param params.Instance
	if err := json.Unmarshal(out, &param); err != nil {
		return params.Instance{}, garmErrors.NewProviderError("failed to decode response from binary: %s", err)
	}
	return param, nil
}

// ListInstances will list all instances for a provider.
func (e *external) ListInstances(ctx context.Context, poolID string) ([]params.Instance, error) {
	asEnv := []string{
		listInstancesCommand,
		e.configEnvVar(),
		fmt.Sprintf("GARM_POOL_ID=%s", poolID),
	}

	out, err := exec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		return []params.Instance{}, garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}

	var param []params.Instance
	if err := json.Unmarshal(out, &param); err != nil {
		return []params.Instance{}, garmErrors.NewProviderError("failed to decode response from binary: %s", err)
	}
	return param, nil
}

// RemoveAllInstances will remove all instances created by this provider.
func (e *external) RemoveAllInstances(ctx context.Context) error {
	asEnv := []string{
		removeAllInstancesCommand,
		e.configEnvVar(),
		fmt.Sprintf("GARM_CONTROLLER_ID=%s", e.controllerID),
	}
	_, err := exec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		return garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}
	return nil
}

// Stop shuts down the instance.
func (e *external) Stop(ctx context.Context, instance string, force bool) error {
	asEnv := []string{
		stopInstanceCommand,
		e.configEnvVar(),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
	}
	_, err := exec.Exec(ctx, e.execPath, nil, asEnv)
	if err != nil {
		return garmErrors.NewProviderError("provider binary %s returned error: %s", e.execPath, err)
	}
	return nil
}

// Start boots up an instance.
func (e *external) Start(ctx context.Context, instance string) error {
	asEnv := []string{
		startInstanceCommand,
		e.configEnvVar(),
		fmt.Sprintf("GARM_INSTANCE_ID=%s", instance),
	}
	_, err := exec.Exec(ctx, e.execPath, nil, asEnv)
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
