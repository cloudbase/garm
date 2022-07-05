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

	"github.com/stretchr/testify/assert"
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
			name: "Missing provider dir",
			cfg: External{
				ConfigFile:  "",
				ProviderDir: "",
			},
			errString: "missing provider dir",
		},
		{
			name: "Provider dir must not be relative",
			cfg: External{
				ConfigFile:  "",
				ProviderDir: "../test",
			},
			errString: "path to provider dir must be absolute",
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
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
				assert.EqualError(t, err, tc.errString)
			}
		})
	}
}

func TestProviderExecutableIsExecutable(t *testing.T) {
	cfg := getDefaultExternalConfig(t)

	execPath, err := cfg.ExecutablePath()
	assert.Nil(t, err)
	err = os.Chmod(execPath, 0o644)
	assert.Nil(t, err)

	err = cfg.Validate()
	assert.NotNil(t, err)
	assert.EqualError(t, err, fmt.Sprintf("external provider binary %s is not executable", execPath))
}
