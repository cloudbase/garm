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
