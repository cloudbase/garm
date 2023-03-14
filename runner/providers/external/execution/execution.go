package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/cloudbase/garm/params"

	"github.com/mattn/go-isatty"
)

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
