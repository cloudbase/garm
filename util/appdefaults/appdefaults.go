package appdefaults

import "time"

const (
	// DefaultJWTTTL is the default duration in seconds a JWT token
	// will be valid.
	DefaultJWTTTL time.Duration = 24 * time.Hour

	// DefaultRunnerBootstrapTimeout is the default timeout in minutes a runner is
	// considered to be defunct. If a runner does not join github in the alloted amount
	// of time and no new updates have been made to it's state, it will be removed.
	DefaultRunnerBootstrapTimeout = 20

	// DefaultGithubURL is the default URL where Github or Github Enterprise can be accessed.
	DefaultGithubURL = "https://github.com"

	// DefaultConfigFilePath is the default path on disk to the garm
	// configuration file.
	DefaultConfigFilePath = "/etc/garm/config.toml"

	// DefaultUser is the default username that should exist on the instances.
	DefaultUser = "runner"
	// DefaultUserShell is the shell for the default user.
	DefaultUserShell = "/bin/bash"

	// DefaultPoolQueueSize is the default size for a pool queue.
	DefaultPoolQueueSize = 10

	// GithubDefaultBaseURL is the default URL for the github API.
	GithubDefaultBaseURL = "https://api.github.com/"

	// uploadBaseURL is the default URL for guthub uploads.
	GithubDefaultUploadBaseURL = "https://uploads.github.com/"
)

var (
	// DefaultConfigDir is the default path on disk to the config dir. The config
	// file will probably be in the same folder, but it is not mandatory.
	DefaultConfigDir = "/etc/garm"

	// DefaultUserGroups are the groups the default user will be part of.
	DefaultUserGroups = []string{
		"sudo", "adm", "cdrom", "dialout",
		"dip", "video", "plugdev", "netdev",
		"docker", "lxd",
	}
)
