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

	"github.com/cloudbase/garm/util/exec"

	"github.com/pkg/errors"
)

// External represents the config for an external provider.
// The external provider is a provider that delegates all operations
// to an external binary. This way, you can write your own logic in
// whatever programming language you wish, while still remaining compatible
// with garm.
type External struct {
	// ConfigFile is the path on disk to a file which will be passed to
	// the external binary as an environment variable: GARM_PROVIDER_CONFIG
	// You can use this file for any configuration you need to do for the
	// cloud your calling into, to create the compute resources.
	ConfigFile string `toml:"config_file" json:"config-file"`
	// ProviderDir is the path on disk to a folder containing an executable
	// called "garm-external-provider".
	ProviderDir string `toml:"provider_dir" json:"provider-dir"`
	// ProviderExecutable is the full path to the executable that implements
	// the provider. If specified, it will take precedence over the "garm-external-provider"
	// executable in the ProviderDir.
	ProviderExecutable string `toml:"provider_executable" json:"provider-executable"`
}

func (e *External) ExecutablePath() (string, error) {
	execPath := e.ProviderExecutable
	if execPath == "" {
		execPath = filepath.Join(e.ProviderDir, "garm-external-provider")
	}

	if !filepath.IsAbs(execPath) {
		return "", fmt.Errorf("executable path must be an absolute path")
	}
	return execPath, nil
}

func (e *External) Validate() error {
	if e.ConfigFile != "" {
		if !filepath.IsAbs(e.ConfigFile) {
			return fmt.Errorf("path to config file must be an absolute path")
		}
		if _, err := os.Stat(e.ConfigFile); err != nil {
			return fmt.Errorf("failed to access config file %s", e.ConfigFile)
		}
	}

	execPath, err := e.ExecutablePath()
	if err != nil {
		return errors.Wrap(err, "fetching executable path")
	}
	if _, err := os.Stat(execPath); err != nil {
		return errors.Wrap(err, "checking provider executable")
	}
	if !exec.IsExecutable(execPath) {
		return fmt.Errorf("external provider binary %s is not executable", execPath)
	}

	return nil
}
