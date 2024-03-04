// Copyright 2022 Cloudbase Solutions SRL
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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func getDefaultExternalConfig(t *testing.T) External {
	dir, err := os.MkdirTemp("", "garm-test")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	// nolint:golangci-lint,gosec
	err = os.WriteFile(filepath.Join(dir, "garm-external-provider"), []byte{}, 0o755)
	if err != nil {
		t.Fatalf("failed to write file: %s", err)
	}

	return External{
		ConfigFile:  "",
		ProviderDir: dir,
	}
}

func TestExternal(t *testing.T) {
	cfg := getDefaultExternalConfig(t)

	tests := []struct {
		name      string
		cfg       External
		errString string
	}{
		{
			name:      "Config is valid",
			cfg:       cfg,
			errString: "",
		},
		{
			name: "Config path cannot be relative path",
			cfg: External{
				ConfigFile:  "../test",
				ProviderDir: cfg.ProviderDir,
			},
			errString: "path to config file must be an absolute path",
		},
		{
			name: "Config must exist if specified",
			cfg: External{
				ConfigFile:  "/there/is/no/config/here",
				ProviderDir: cfg.ProviderDir,
			},
			errString: "failed to access config file /there/is/no/config/here",
		},
		{
			name: "Provider dir path must be absolute",
			cfg: External{
				ConfigFile:  "",
				ProviderDir: "../test",
			},
			errString: "failed to get executable path: executable path must be an absolute path",
		},
		{
			name: "Provider executable path must be absolute",
			cfg: External{
				ConfigFile:         "",
				ProviderExecutable: "../test",
			},
			errString: "failed to get executable path: executable path must be an absolute path",
		},
		{
			name: "Provider executable not found",
			cfg: External{
				ConfigFile:  "",
				ProviderDir: "/tmp",
			},
			errString: "failed to access external provider binary /tmp/garm-external-provider",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.errString == "" {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				require.EqualError(t, err, tc.errString)
			}
		})
	}
}

func TestProviderExecutableIsExecutable(t *testing.T) {
	cfg := getDefaultExternalConfig(t)

	execPath, err := cfg.ExecutablePath()
	require.Nil(t, err)
	err = os.Chmod(execPath, 0o644)
	require.Nil(t, err)

	err = cfg.Validate()
	require.NotNil(t, err)
	require.EqualError(t, err, fmt.Sprintf("external provider binary %s is not executable", execPath))
}

func TestExternalEnvironmentVariables(t *testing.T) {
	cfg := getDefaultExternalConfig(t)

	tests := []struct {
		name                         string
		cfg                          External
		expectedEnvironmentVariables []string
		environmentVariables         map[string]string
	}{
		{
			name:                         "Provider with no additional environment variables",
			cfg:                          cfg,
			expectedEnvironmentVariables: []string{},
			environmentVariables: map[string]string{
				"PATH":                 "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"PROVIDER_LOG_LEVEL":   "debug",
				"PROVIDER_TIMEOUT":     "30",
				"PROVIDER_RETRY_COUNT": "3",
				"INFRA_REGION":         "us-east-1",
			},
		},
		{
			name: "Provider with additional environment variables",
			cfg: External{
				ConfigFile:  "",
				ProviderDir: "../test",
				EnvironmentVariables: []string{
					"PROVIDER_",
					"INFRA_REGION",
				},
			},
			environmentVariables: map[string]string{
				"PATH":                 "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"PROVIDER_LOG_LEVEL":   "debug",
				"PROVIDER_TIMEOUT":     "30",
				"PROVIDER_RETRY_COUNT": "3",
				"INFRA_REGION":         "us-east-1",
				"GARM_POOL_ID":         "f3b21376-e189-43ae-a1bd-7a3ffee57a58",
			},
			expectedEnvironmentVariables: []string{
				"PROVIDER_LOG_LEVEL=debug",
				"PROVIDER_TIMEOUT=30",
				"PROVIDER_RETRY_COUNT=3",
				"INFRA_REGION=us-east-1",
			},
		},
		{
			name: "GARM variables are getting ignored",
			cfg: External{
				ConfigFile:  "",
				ProviderDir: "../test",
				EnvironmentVariables: []string{
					"PROVIDER_",
					"INFRA_REGION",
					"GARM_SERVER",
				},
			},
			environmentVariables: map[string]string{
				"PATH":                 "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"PROVIDER_LOG_LEVEL":   "debug",
				"PROVIDER_TIMEOUT":     "30",
				"PROVIDER_RETRY_COUNT": "3",
				"INFRA_REGION":         "us-east-1",
				"GARM_POOL_ID":         "f3b21376-e189-43ae-a1bd-7a3ffee57a58",
				"GARM_SERVER_SHUTDOWN": "true",
				"GARM_SERVER_INSECURE": "true",
			},
			expectedEnvironmentVariables: []string{
				"PROVIDER_LOG_LEVEL=debug",
				"PROVIDER_TIMEOUT=30",
				"PROVIDER_RETRY_COUNT=3",
				"INFRA_REGION=us-east-1",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// set environment variables
			for k, v := range tc.environmentVariables {
				err := os.Setenv(k, v)
				if err != nil {
					t.Fatalf("failed to set environment variable: %s", err)
				}
			}

			envVars := tc.cfg.GetEnvironmentVariables()

			// sort slices to make them comparable
			slices.Sort(envVars)
			slices.Sort(tc.expectedEnvironmentVariables)

			// compare slices
			require.Equal(t, tc.expectedEnvironmentVariables, envVars)
		})
	}
}
