package config

import (
	"fmt"
	"os"
	"path/filepath"

	"garm/util/exec"

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
}

func (e *External) ExecutablePath() (string, error) {
	execPath := filepath.Join(e.ProviderDir, "garm-external-provider")
	if !filepath.IsAbs(execPath) {
		return "", fmt.Errorf("executable path must be an absolut epath")
	}
	return filepath.Join(e.ProviderDir, "garm-external-provider"), nil
}

func (e *External) Validate() error {
	if e.ConfigFile != "" {
		if _, err := os.Stat(e.ConfigFile); err != nil {
			return fmt.Errorf("failed to access cofig file %s", e.ConfigFile)
		}
		if !filepath.IsAbs(e.ConfigFile) {
			return fmt.Errorf("path to config file must be an absolute path")
		}
	}

	if e.ProviderDir == "" {
		return fmt.Errorf("missing provider dir")
	}

	if !filepath.IsAbs(e.ProviderDir) {
		return fmt.Errorf("path to provider dir must be absolute")
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
