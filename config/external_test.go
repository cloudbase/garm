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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func getDefaultExternalConfig(t *testing.T) External {
	dir, err := ioutil.TempDir("", "garm-test")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	err = ioutil.WriteFile(filepath.Join(dir, "garm-external-provider"), []byte{}, 0755)
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
			errString: "fetching executable path: executable path must be an absolute path",
		},
		{
			name: "Provider executable path must be absolute",
			cfg: External{
				ConfigFile:         "",
				ProviderExecutable: "../test",
			},
			errString: "fetching executable path: executable path must be an absolute path",
		},
		{
			name: "Provider executable not found",
			cfg: External{
				ConfigFile:  "",
				ProviderDir: "/tmp",
			},
			errString: "checking provider executable: stat /tmp/garm-external-provider: no such file or directory",
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
