// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
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

	// DefaultPoolQueueSize is the default size for a pool queue.
	DefaultPoolQueueSize = 10

	// GithubDefaultBaseURL is the default URL for the github API.
	GithubDefaultBaseURL = "https://api.github.com/"

	// uploadBaseURL is the default URL for guthub uploads.
	GithubDefaultUploadBaseURL = "https://uploads.github.com/"

	// metrics data update interval
	DefaultMetricsUpdateInterval = 60 * time.Second

	// GiteaRunnerReleasesURL is the public API URL that returns a json of all Gitea runner releases.
	// By default it returns the last 10 releases, which is enough for our needs.
	GiteaRunnerReleasesURL = "https://gitea.com/api/v1/repos/gitea/act_runner/releases"
	// GiteaRunnerMinimumVersion is the minimum version we need in order to support ephemeral runners.
	GiteaRunnerMinimumVersion = "v0.2.12"

	// GARM agent releases URL
	GARMAgentDefaultReleasesURL = "https://api.github.com/repos/cloudbase/garm-agent/releases"
)

var Version string

func GetVersion() string {
	if Version == "" {
		Version = "v0.0.0-unknown"
	}
	return Version
}
