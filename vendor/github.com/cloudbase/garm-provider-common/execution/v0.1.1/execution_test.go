// Copyright 2023 Cloudbase Solutions SRL
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

package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"

	gErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/params"
	"github.com/stretchr/testify/require"
)

type testExternalProvider struct {
	mockErr      error
	mockInstance params.ProviderInstance
}

func (e *testExternalProvider) CreateInstance(ctx context.Context, bootstrapParams params.BootstrapInstance) (params.ProviderInstance, error) {
	if e.mockErr != nil {
		return params.ProviderInstance{}, e.mockErr
	}
	return e.mockInstance, nil
}

func (p *testExternalProvider) DeleteInstance(context.Context, string) error {
	if p.mockErr != nil {
		return p.mockErr
	}
	return nil
}

func (p *testExternalProvider) GetInstance(context.Context, string) (params.ProviderInstance, error) {
	if p.mockErr != nil {
		return params.ProviderInstance{}, p.mockErr
	}
	return p.mockInstance, nil
}

func (p *testExternalProvider) ListInstances(context.Context, string) ([]params.ProviderInstance, error) {
	if p.mockErr != nil {
		return nil, p.mockErr
	}
	return []params.ProviderInstance{p.mockInstance}, nil
}

func (p *testExternalProvider) RemoveAllInstances(context.Context) error {
	if p.mockErr != nil {
		return p.mockErr
	}
	return nil
}

func (p *testExternalProvider) Stop(context.Context, string, bool) error {
	if p.mockErr != nil {
		return p.mockErr
	}
	return nil
}

func (p *testExternalProvider) Start(context.Context, string) error {
	if p.mockErr != nil {
		return p.mockErr
	}
	return nil
}

func TestResolveErrorToExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
	}{
		{
			name: "nil error",
			err:  nil,
			code: 0,
		},
		{
			name: "not found error",
			err:  gErrors.ErrNotFound,
			code: ExitCodeNotFound,
		},
		{
			name: "duplicate entity error",
			err:  gErrors.ErrDuplicateEntity,
			code: ExitCodeDuplicate,
		},
		{
			name: "other error",
			err:  errors.New("other error"),
			code: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code := ResolveErrorToExitCode(tc.err)
			require.Equal(t, tc.code, code)
		})
	}
}

func TestValidateEnvironment(t *testing.T) {
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "provider-config")
	if err != nil {
		log.Fatal(err)
	}
	// clean up the temporary file
	t.Cleanup(func() { os.RemoveAll(tmpfile.Name()) })

	tests := []struct {
		name      string
		env       Environment
		errString string
	}{
		{
			name: "valid environment",
			env: Environment{
				Command:            CreateInstanceCommand,
				ControllerID:       "controller-id",
				PoolID:             "pool-id",
				ProviderConfigFile: tmpfile.Name(),
				InstanceID:         "instance-id",
				BootstrapParams: params.BootstrapInstance{
					Name: "instance-name",
				},
			},
			errString: "",
		},
		{
			name: "invalid command",
			env: Environment{
				Command: "",
			},
			errString: "missing GARM_COMMAND",
		},
		{
			name: "invalid provider config file",
			env: Environment{
				Command:            CreateInstanceCommand,
				ProviderConfigFile: "",
			},
			errString: "missing GARM_PROVIDER_CONFIG_FILE",
		},
		{
			name: "error accessing config file",
			env: Environment{
				Command:            CreateInstanceCommand,
				ProviderConfigFile: "invalid-file",
			},
			errString: "error accessing config file",
		},
		{
			name: "invalid controller ID",
			env: Environment{
				Command:            CreateInstanceCommand,
				ProviderConfigFile: tmpfile.Name(),
			},
			errString: "missing GARM_CONTROLLER_ID",
		},

		{
			name: "invalid instance ID",
			env: Environment{
				Command:            DeleteInstanceCommand,
				ProviderConfigFile: tmpfile.Name(),
				ControllerID:       "controller-id",
				InstanceID:         "",
			},
			errString: "missing instance ID",
		},
		{
			name: "invalid pool ID",
			env: Environment{
				Command:            ListInstancesCommand,
				ProviderConfigFile: tmpfile.Name(),
				ControllerID:       "controller-id",
				PoolID:             "",
			},
			errString: "missing pool ID",
		},
		{
			name: "invalid bootstrap params",
			env: Environment{
				Command:            CreateInstanceCommand,
				ProviderConfigFile: tmpfile.Name(),
				ControllerID:       "controller-id",
				PoolID:             "pool-id",
				BootstrapParams:    params.BootstrapInstance{},
			},
			errString: "missing bootstrap params",
		},
		{
			name: "missing pool ID",
			env: Environment{
				Command:            CreateInstanceCommand,
				ProviderConfigFile: tmpfile.Name(),
				ControllerID:       "controller-id",
				PoolID:             "",
				BootstrapParams: params.BootstrapInstance{
					Name: "instance-name",
				},
			},
			errString: "missing pool ID",
		},
		{
			name: "unknown command",
			env: Environment{
				Command:            "unknown-command",
				ProviderConfigFile: tmpfile.Name(),
				ControllerID:       "controller-id",
				PoolID:             "pool-id",
				BootstrapParams: params.BootstrapInstance{
					Name: "instance-name",
				},
			},
			errString: "unknown GARM_COMMAND",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.env.Validate()
			if tc.errString == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Regexp(t, tc.errString, err.Error())
			}
		})
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name             string
		providerEnv      Environment
		providerInstance params.ProviderInstance
		providerErr      error
		expectedErrMsg   string
	}{
		{
			name: "Valid environment",
			providerEnv: Environment{
				Command: CreateInstanceCommand,
			},
			providerInstance: params.ProviderInstance{
				Name:   "test-instance",
				OSType: params.Linux,
			},
			providerErr:    nil,
			expectedErrMsg: "",
		},
		{
			name: "Failed to create instance",
			providerEnv: Environment{
				Command: CreateInstanceCommand,
			},
			providerInstance: params.ProviderInstance{
				Name:   "test-instance",
				OSType: params.Linux,
			},
			providerErr:    fmt.Errorf("error creating test-instance"),
			expectedErrMsg: "failed to create instance in provider: error creating test-instance",
		},
		{
			name: "Failed to get instance",
			providerEnv: Environment{
				Command: GetInstanceCommand,
			},
			providerInstance: params.ProviderInstance{
				Name:   "test-instance",
				OSType: params.Linux,
			},
			providerErr:    fmt.Errorf("error getting test-instance"),
			expectedErrMsg: "failed to get instance from provider: error getting test-instance",
		},
		{
			name: "Failed to list instances",
			providerEnv: Environment{
				Command: ListInstancesCommand,
			},
			providerInstance: params.ProviderInstance{
				Name:   "test-instance",
				OSType: params.Linux,
			},
			providerErr:    fmt.Errorf("error listing instances"),
			expectedErrMsg: "failed to list instances from provider: error listing instances",
		},
		{
			name: "Failed to delete instance",
			providerEnv: Environment{
				Command: DeleteInstanceCommand,
			},
			providerInstance: params.ProviderInstance{
				Name:   "test-instance",
				OSType: params.Linux,
			},
			providerErr:    fmt.Errorf("error deleting test-instance"),
			expectedErrMsg: "failed to delete instance from provider: error deleting test-instance",
		},
		{
			name: "Failed to remove all instances",
			providerEnv: Environment{
				Command: RemoveAllInstancesCommand,
			},
			providerInstance: params.ProviderInstance{
				Name:   "test-instance",
				OSType: params.Linux,
			},
			providerErr:    fmt.Errorf("error removing all instances"),
			expectedErrMsg: "failed to destroy environment: error removing all instances",
		},
		{
			name: "Failed to start instance",
			providerEnv: Environment{
				Command: StartInstanceCommand,
			},
			providerInstance: params.ProviderInstance{
				Name:   "test-instance",
				OSType: params.Linux,
			},
			providerErr:    fmt.Errorf("error starting test-instance"),
			expectedErrMsg: "failed to start instance: error starting test-instance",
		},
		{
			name: "Failed to stop instance",
			providerEnv: Environment{
				Command: StopInstanceCommand,
			},
			providerInstance: params.ProviderInstance{
				Name:   "test-instance",
				OSType: params.Linux,
			},
			providerErr:    fmt.Errorf("error stopping test-instance"),
			expectedErrMsg: "failed to stop instance: error stopping test-instance",
		},
		{
			name: "Invalid command",
			providerEnv: Environment{
				Command: "invalid-command",
			},
			providerInstance: params.ProviderInstance{
				Name:   "test-instance",
				OSType: params.Linux,
			},
			providerErr:    nil,
			expectedErrMsg: "invalid command: invalid-command",
		},
	}

	for _, tc := range tests {
		testExternalProvider := testExternalProvider{
			mockErr:      tc.providerErr,
			mockInstance: tc.providerInstance,
		}

		out, err := Run(context.Background(), &testExternalProvider, tc.providerEnv)

		if tc.expectedErrMsg == "" {
			require.NoError(t, err)
			expectedJs, marshalErr := json.Marshal(tc.providerInstance)
			require.NoError(t, marshalErr)
			require.Equal(t, string(expectedJs), out)
		} else {
			require.Equal(t, err.Error(), tc.expectedErrMsg)
			require.Equal(t, "", out)
		}
	}
}

func TestGetEnvironment(t *testing.T) {
	tests := []struct {
		name      string
		stdinData string
		envData   map[string]string
		errString string
	}{
		{
			name:      "The environment is valid",
			stdinData: `{"name": "test"}`,
			errString: "",
		},
		{
			name:      "Data is missing from stdin",
			stdinData: ``,
			errString: "CreateInstance requires data passed into stdin",
		},
		{
			name:      "Invalid JSON",
			stdinData: `bogus`,
			errString: "failed to decode instance params: invalid character 'b' looking for beginning of value",
		},
	}

	for _, tc := range tests {
		// Create a temporary file
		tmpfile, err := os.CreateTemp("", "test-get-env")
		if err != nil {
			log.Fatal(err)
		}

		// clean up the temporary file
		t.Cleanup(func() { os.RemoveAll(tmpfile.Name()) })

		// Write some test data to the temporary file
		if _, err := tmpfile.Write([]byte(tc.stdinData)); err != nil {
			log.Fatal(err)
		}
		// Rewind the temporary file to the beginning
		if _, err := tmpfile.Seek(0, 0); err != nil {
			log.Fatal(err)
		}

		// Clean up the temporary file
		t.Cleanup(func() { os.RemoveAll(tmpfile.Name()) })

		oldStdin := os.Stdin
		defer func() { os.Stdin = oldStdin }() // Restore original Stdin

		os.Stdin = tmpfile // mock os.Stdin

		for key, value := range tc.envData {
			os.Setenv(key, value)
		}

		// Define the environment variables
		os.Setenv("GARM_COMMAND", "CreateInstance")
		os.Setenv("GARM_CONTROLLER_ID", "test-controller-id")
		os.Setenv("GARM_POOL_ID", "test-pool-id")
		os.Setenv("GARM_PROVIDER_CONFIG_FILE", tmpfile.Name())

		// Clean up the environment variables
		t.Cleanup(func() {
			for key := range tc.envData {
				os.Unsetenv(key)
			}
		})

		env, err := GetEnvironment()
		if tc.errString == "" {
			require.NoError(t, err)
			require.Equal(t, CreateInstanceCommand, env.Command)
		} else {
			require.Equal(t, tc.errString, err.Error())
		}
	}
}

func TestGetEnvValidateFailed(t *testing.T) {
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "test-get-env")
	if err != nil {
		log.Fatal(err)
	}
	// clean up the temporary file
	t.Cleanup(func() { os.RemoveAll(tmpfile.Name()) })

	os.Setenv("GARM_COMMAND", "unknown-command")
	os.Setenv("GARM_CONTROLLER_ID", "test-controller-id")
	os.Setenv("GARM_POOL_ID", "test-pool-id")
	os.Setenv("GARM_PROVIDER_CONFIG_FILE", tmpfile.Name())

	// Clean up the environment variables
	t.Cleanup(func() {
		os.Unsetenv("GARM_COMMAND")
		os.Unsetenv("GARM_CONTROLLER_ID")
		os.Unsetenv("GARM_POOL_ID")
		os.Unsetenv("GARM_PROVIDER_CONFIG_FILE")
	})

	_, err = GetEnvironment()
	require.Error(t, err)
	require.Equal(t, "failed to validate execution environment: unknown GARM_COMMAND: unknown-command", err.Error())
}
