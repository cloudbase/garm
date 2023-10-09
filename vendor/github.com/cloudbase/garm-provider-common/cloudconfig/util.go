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

package cloudconfig

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cloudbase/garm-provider-common/defaults"
	"github.com/cloudbase/garm-provider-common/params"
	"github.com/pkg/errors"
)

// CloudConfigSpec is a struct that holds extra specs that can be used to customize user data.
type CloudConfigSpec struct {
	// RunnerInstallTemplate can be used to override the default runner install template.
	// If used, the caller is responsible for the correctness of the template as well as the
	// suitability of the template for the target OS.
	RunnerInstallTemplate []byte `json:"runner_install_template"`
	// PreInstallScripts is a map of pre-install scripts that will be run before the
	// runner install script. These will run as root and can be used to prep a generic image
	// before we attempt to install the runner. The key of the map is the name of the script
	// as it will be written to disk. The value is a byte array with the contents of the script.
	//
	// These scripts will be added and run in alphabetical order.
	//
	// On Linux, we will set the executable flag. On Windows, the name matters as Windows looks for an
	// extension to determine if the file is an executable or not. In theory this can hold binaries,
	// but in most cases this will most likely hold scripts. We do not currenly validate the payload,
	// so it's up to the user what they upload here.
	// Caution needs to be exercised when using this feature, as the total size of userdata is limited
	// on most providers.
	PreInstallScripts map[string][]byte `json:"pre_install_scripts"`
	// ExtraContext is a map of extra context that will be passed to the runner install template.
	ExtraContext map[string]string `json:"extra_context"`
}

func sortMapKeys(m map[string][]byte) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

// GetSpecs returns the cloud config specific extra specs from the bootstrap params.
func GetSpecs(bootstrapParams params.BootstrapInstance) (CloudConfigSpec, error) {
	var extraSpecs CloudConfigSpec
	if len(bootstrapParams.ExtraSpecs) == 0 {
		return extraSpecs, nil
	}

	if err := json.Unmarshal(bootstrapParams.ExtraSpecs, &extraSpecs); err != nil {
		return CloudConfigSpec{}, errors.Wrap(err, "unmarshaling extra specs")
	}

	if extraSpecs.ExtraContext == nil {
		extraSpecs.ExtraContext = map[string]string{}
	}

	if extraSpecs.PreInstallScripts == nil {
		extraSpecs.PreInstallScripts = map[string][]byte{}
	}

	return extraSpecs, nil
}

// GetRunnerInstallScript returns the runner install script for the given bootstrap params.
// This function will return either the default script for the given OS type or will use the supplied template
// if one is provided.
func GetRunnerInstallScript(bootstrapParams params.BootstrapInstance, tools params.RunnerApplicationDownload, runnerName string) ([]byte, error) {
	if tools.GetFilename() == "" {
		return nil, fmt.Errorf("missing tools filename")
	}

	if tools.GetDownloadURL() == "" {
		return nil, fmt.Errorf("missing tools download URL")
	}

	tempToken := tools.GetTempDownloadToken()
	extraSpecs, err := GetSpecs(bootstrapParams)
	if err != nil {
		return nil, errors.Wrap(err, "getting specs")
	}

	installRunnerParams := InstallRunnerParams{
		FileName:          tools.GetFilename(),
		DownloadURL:       tools.GetDownloadURL(),
		TempDownloadToken: tempToken,
		MetadataURL:       bootstrapParams.MetadataURL,
		RunnerUsername:    defaults.DefaultUser,
		RunnerGroup:       defaults.DefaultUser,
		RepoURL:           bootstrapParams.RepoURL,
		RunnerName:        runnerName,
		RunnerLabels:      strings.Join(bootstrapParams.Labels, ","),
		CallbackURL:       bootstrapParams.CallbackURL,
		CallbackToken:     bootstrapParams.InstanceToken,
		GitHubRunnerGroup: bootstrapParams.GitHubRunnerGroup,
		ExtraContext:      extraSpecs.ExtraContext,
		EnableBootDebug:   bootstrapParams.UserDataOptions.EnableBootDebug,
		UseJITConfig:      bootstrapParams.JitConfigEnabled,
	}

	if bootstrapParams.CACertBundle != nil && len(bootstrapParams.CACertBundle) > 0 {
		installRunnerParams.CABundle = string(bootstrapParams.CACertBundle)
	}

	installScript, err := InstallRunnerScript(installRunnerParams, bootstrapParams.OSType, string(extraSpecs.RunnerInstallTemplate))
	if err != nil {
		return nil, errors.Wrap(err, "generating script")
	}

	return installScript, nil
}

// GetCloudInitConfig returns the cloud-init specific userdata config. This config can be used on most clouds
// for most Linux machines. The install runner script must be generated separately either by GetRunnerInstallScript()
// or some other means.
func GetCloudInitConfig(bootstrapParams params.BootstrapInstance, installScript []byte) (string, error) {
	extraSpecs, err := GetSpecs(bootstrapParams)
	if err != nil {
		return "", errors.Wrap(err, "getting specs")
	}

	cloudCfg := NewDefaultCloudInitConfig()

	if bootstrapParams.UserDataOptions.DisableUpdatesOnBoot {
		cloudCfg.PackageUpgrade = false
		cloudCfg.Packages = []string{}
	}
	for _, pkg := range bootstrapParams.UserDataOptions.ExtraPackages {
		cloudCfg.AddPackage(pkg)
	}

	if len(extraSpecs.PreInstallScripts) > 0 {
		names := sortMapKeys(extraSpecs.PreInstallScripts)
		for _, name := range names {
			script := extraSpecs.PreInstallScripts[name]
			cloudCfg.AddFile(script, fmt.Sprintf("/garm-pre-install/%s", name), "root:root", "755")
			cloudCfg.AddRunCmd(fmt.Sprintf("/garm-pre-install/%s", name))
		}
	}
	cloudCfg.AddRunCmd("rm -rf /garm-pre-install")

	cloudCfg.AddSSHKey(bootstrapParams.SSHKeys...)
	cloudCfg.AddFile(installScript, "/install_runner.sh", "root:root", "755")
	cloudCfg.AddRunCmd(fmt.Sprintf("su -l -c /install_runner.sh %s", defaults.DefaultUser))
	cloudCfg.AddRunCmd("rm -f /install_runner.sh")
	if bootstrapParams.CACertBundle != nil && len(bootstrapParams.CACertBundle) > 0 {
		if err := cloudCfg.AddCACert(bootstrapParams.CACertBundle); err != nil {
			return "", errors.Wrap(err, "adding CA cert bundle")
		}
	}

	asStr, err := cloudCfg.Serialize()
	if err != nil {
		return "", errors.Wrap(err, "creating cloud config")
	}

	return asStr, nil
}

// GetCloudConfig is a helper function that generates a cloud-init config for Linux and a powershell script for Windows.
// In most cases this function should do, but in situations where a more custom approach is needed, you may need to call
// GetCloudInitConfig() or GetRunnerInstallScript() directly and compose the final userdata in a different way.
// The extra specs PreInstallScripts is only supported on Linux via cloud-init by this function. On some providers, like Azure
// Windows initialization scripts are run by creating a separate CustomScriptExtension resource for each individual script.
// On other clouds it may be different. This function aims to be generic, which is why it only supports the PreInstallScripts
// via cloud-init.
func GetCloudConfig(bootstrapParams params.BootstrapInstance, tools params.RunnerApplicationDownload, runnerName string) (string, error) {
	installScript, err := GetRunnerInstallScript(bootstrapParams, tools, runnerName)
	if err != nil {
		return "", errors.Wrap(err, "generating script")
	}

	var asStr string
	switch bootstrapParams.OSType {
	case params.Linux:
		cloudCfg, err := GetCloudInitConfig(bootstrapParams, installScript)
		if err != nil {
			return "", errors.Wrap(err, "getting cloud init config")
		}
		return cloudCfg, nil
	case params.Windows:
		asStr = string(installScript)
	default:
		return "", fmt.Errorf("unknown os type: %s", bootstrapParams.OSType)
	}

	return asStr, nil
}
