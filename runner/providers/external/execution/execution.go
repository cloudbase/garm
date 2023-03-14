package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/cloudbase/garm/params"
)

func GetEnvironment() (Environment, error) {
	env := Environment{
		Command:            ExecutionCommand(os.Getenv("GARM_COMMAND")),
		ControllerID:       os.Getenv("GARM_CONTROLLER_ID"),
		PoolID:             os.Getenv("GARM_POOL_ID"),
		ProviderConfigFile: os.Getenv("GARM_PROVIDER_CONFIG_FILE"),
		InstanceID:         os.Getenv("GARM_INSTANCE_ID"),
	}

	if env.Command == CreateInstanceCommand {
		// We need to get the bootstrap params from stdin
		info, err := os.Stdin.Stat()
		if err != nil {
			return Environment{}, fmt.Errorf("failed to get stdin: %w", err)
		}
		if info.Size() == 0 {
			return Environment{}, fmt.Errorf("no data found on stdin")
		}

		var data bytes.Buffer
		if _, err := io.Copy(&data, os.Stdin); err != nil {
			return Environment{}, fmt.Errorf("failed to copy bootstrap params")
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
