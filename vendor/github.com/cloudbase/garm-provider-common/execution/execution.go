package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/params"

	"github.com/mattn/go-isatty"
)

const (
	// ExitCodeNotFound is an exit code that indicates a Not Found error
	ExitCodeNotFound int = 30
	// ExitCodeDuplicate is an exit code that indicates a duplicate error
	ExitCodeDuplicate int = 31
)

func ResolveErrorToExitCode(err error) int {
	if err != nil {
		if errors.Is(err, gErrors.ErrNotFound) {
			return ExitCodeNotFound
		} else if errors.Is(err, gErrors.ErrDuplicateEntity) {
			return ExitCodeDuplicate
		}
		return 1
	}
	return 0
}

func GetEnvironment() (Environment, error) {
	env := Environment{
		Command:            ExecutionCommand(os.Getenv("GARM_COMMAND")),
		ControllerID:       os.Getenv("GARM_CONTROLLER_ID"),
		PoolID:             os.Getenv("GARM_POOL_ID"),
		ProviderConfigFile: os.Getenv("GARM_PROVIDER_CONFIG_FILE"),
		InstanceID:         os.Getenv("GARM_INSTANCE_ID"),
	}

	// If this is a CreateInstance command, we need to get the bootstrap params
	// from stdin
	if env.Command == CreateInstanceCommand {
		if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
			return Environment{}, fmt.Errorf("%s requires data passed into stdin", CreateInstanceCommand)
		}

		var data bytes.Buffer
		if _, err := io.Copy(&data, os.Stdin); err != nil {
			return Environment{}, fmt.Errorf("failed to copy bootstrap params")
		}

		if data.Len() == 0 {
			return Environment{}, fmt.Errorf("%s requires data passed into stdin", CreateInstanceCommand)
		}

		var bootstrapParams params.BootstrapInstance
		if err := json.Unmarshal(data.Bytes(), &bootstrapParams); err != nil {
			return Environment{}, fmt.Errorf("failed to decode instance params: %w", err)
		}
		env.BootstrapParams = bootstrapParams
	}

	if err := env.Validate(); err != nil {
		return Environment{}, fmt.Errorf("failed to validate execution environment: %w", err)
	}

	return env, nil
}

type Environment struct {
	Command            ExecutionCommand
	ControllerID       string
	PoolID             string
	ProviderConfigFile string
	InstanceID         string
	BootstrapParams    params.BootstrapInstance
}

func (e Environment) Validate() error {
	if e.Command == "" {
		return fmt.Errorf("missing GARM_COMMAND")
	}

	if e.ProviderConfigFile == "" {
		return fmt.Errorf("missing GARM_PROVIDER_CONFIG_FILE")
	}

	if _, err := os.Lstat(e.ProviderConfigFile); err != nil {
		return fmt.Errorf("error accessing config file: %w", err)
	}

	if e.ControllerID == "" {
		return fmt.Errorf("missing GARM_CONTROLLER_ID")
	}

	switch e.Command {
	case CreateInstanceCommand:
		if e.BootstrapParams.Name == "" {
			return fmt.Errorf("missing bootstrap params")
		}
		if e.ControllerID == "" {
			return fmt.Errorf("missing controller ID")
		}
		if e.PoolID == "" {
			return fmt.Errorf("missing pool ID")
		}
	case DeleteInstanceCommand, GetInstanceCommand,
		StartInstanceCommand, StopInstanceCommand:
		if e.InstanceID == "" {
			return fmt.Errorf("missing instance ID")
		}
	case ListInstancesCommand:
		if e.PoolID == "" {
			return fmt.Errorf("missing pool ID")
		}
	case RemoveAllInstancesCommand:
		if e.ControllerID == "" {
			return fmt.Errorf("missing controller ID")
		}
	default:
		return fmt.Errorf("unknown GARM_COMMAND: %s", e.Command)
	}
	return nil
}

func Run(ctx context.Context, provider ExternalProvider, env Environment) (string, error) {
	var ret string
	switch env.Command {
	case CreateInstanceCommand:
		instance, err := provider.CreateInstance(ctx, env.BootstrapParams)
		if err != nil {
			return "", fmt.Errorf("failed to create instance in provider: %w", err)
		}

		asJs, err := json.Marshal(instance)
		if err != nil {
			return "", fmt.Errorf("failed to marshal response: %w", err)
		}
		ret = string(asJs)
	case GetInstanceCommand:
		instance, err := provider.GetInstance(ctx, env.InstanceID)
		if err != nil {
			return "", fmt.Errorf("failed to get instance from provider: %w", err)
		}
		asJs, err := json.Marshal(instance)
		if err != nil {
			return "", fmt.Errorf("failed to marshal response: %w", err)
		}
		ret = string(asJs)
	case ListInstancesCommand:
		instances, err := provider.ListInstances(ctx, env.PoolID)
		if err != nil {
			return "", fmt.Errorf("failed to list instances from provider: %w", err)
		}
		asJs, err := json.Marshal(instances)
		if err != nil {
			return "", fmt.Errorf("failed to marshal response: %w", err)
		}
		ret = string(asJs)
	case DeleteInstanceCommand:
		if err := provider.DeleteInstance(ctx, env.InstanceID); err != nil {
			return "", fmt.Errorf("failed to delete instance from provider: %w", err)
		}
	case RemoveAllInstancesCommand:
		if err := provider.RemoveAllInstances(ctx); err != nil {
			return "", fmt.Errorf("failed to destroy environment: %w", err)
		}
	case StartInstanceCommand:
		if err := provider.Start(ctx, env.InstanceID); err != nil {
			return "", fmt.Errorf("failed to start instance: %w", err)
		}
	case StopInstanceCommand:
		if err := provider.Stop(ctx, env.InstanceID, true); err != nil {
			return "", fmt.Errorf("failed to stop instance: %w", err)
		}
	default:
		return "", fmt.Errorf("invalid command: %s", env.Command)
	}
	return ret, nil
}
