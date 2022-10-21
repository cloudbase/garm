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

package runner

import "garm/config"

type HookTargetType string

const (
	RepoHook         HookTargetType = "repository"
	OrganizationHook HookTargetType = "organization"
	EnterpriseHook   HookTargetType = "business"
)

var (
	// Linux only for now. Will add Windows soon. (famous last words?)
	supportedOSType map[config.OSType]struct{} = map[config.OSType]struct{}{
		config.Linux: {},
	}

	// These are the architectures that Github supports.
	supportedOSArch map[config.OSArch]struct{} = map[config.OSArch]struct{}{
		config.Amd64: {},
		config.Arm:   {},
		config.Arm64: {},
	}
)

func IsSupportedOSType(osType config.OSType) bool {
	_, ok := supportedOSType[osType]
	return ok
}

func IsSupportedArch(arch config.OSArch) bool {
	_, ok := supportedOSArch[arch]
	return ok
}
