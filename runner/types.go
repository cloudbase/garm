package runner

import "runner-manager/config"

type HookTargetType string

const (
	RepoHook         HookTargetType = "repository"
	OrganizationHook HookTargetType = "organization"
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
