package defaults

const (
	// DefaultUser is the default username that should exist on the instances.
	DefaultUser = "runner"
	// DefaultUserShell is the shell for the default user.
	DefaultUserShell = "/bin/bash"
)

var (
	// DefaultUserGroups are the groups the default user will be part of.
	DefaultUserGroups = []string{
		"sudo", "adm", "cdrom", "dialout",
		"dip", "video", "plugdev", "netdev",
		"docker", "lxd",
	}
)
