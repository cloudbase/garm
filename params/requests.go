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

package params

import (
	"encoding/json"
	"fmt"

	"github.com/cloudbase/garm/errors"
	"github.com/cloudbase/garm/runner/providers/common"
)

const DefaultRunnerPrefix = "garm"

type InstanceRequest struct {
	Name      string `json:"name"`
	OSType    OSType `json:"os_type"`
	OSVersion string `json:"os_version"`
}

type CreateRepoParams struct {
	Owner           string `json:"owner"`
	Name            string `json:"name"`
	CredentialsName string `json:"credentials_name"`
	WebhookSecret   string `json:"webhook_secret"`
}

func (c *CreateRepoParams) Validate() error {
	if c.Owner == "" {
		return errors.NewBadRequestError("missing owner")
	}

	if c.Name == "" {
		return errors.NewBadRequestError("missing repo name")
	}

	if c.CredentialsName == "" {
		return errors.NewBadRequestError("missing credentials name")
	}
	if c.WebhookSecret == "" {
		return errors.NewMissingSecretError("missing secret")
	}
	return nil
}

type CreateOrgParams struct {
	Name            string `json:"name"`
	CredentialsName string `json:"credentials_name"`
	WebhookSecret   string `json:"webhook_secret"`
}

func (c *CreateOrgParams) Validate() error {
	if c.Name == "" {
		return errors.NewBadRequestError("missing org name")
	}

	if c.CredentialsName == "" {
		return errors.NewBadRequestError("missing credentials name")
	}
	if c.WebhookSecret == "" {
		return errors.NewMissingSecretError("missing secret")
	}
	return nil
}

type CreateEnterpriseParams struct {
	Name            string `json:"name"`
	CredentialsName string `json:"credentials_name"`
	WebhookSecret   string `json:"webhook_secret"`
}

func (c *CreateEnterpriseParams) Validate() error {
	if c.Name == "" {
		return errors.NewBadRequestError("missing enterprise name")
	}
	if c.CredentialsName == "" {
		return errors.NewBadRequestError("missing credentials name")
	}
	if c.WebhookSecret == "" {
		return errors.NewMissingSecretError("missing secret")
	}
	return nil
}

// NewUserParams holds the needed information to create
// a new user
type NewUserParams struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	Password string `json:"password"`
	IsAdmin  bool   `json:"-"`
	Enabled  bool   `json:"-"`
}

type UpdatePoolParams struct {
	RunnerPrefix

	Tags                   []string        `json:"tags,omitempty"`
	Enabled                *bool           `json:"enabled,omitempty"`
	MaxRunners             *uint           `json:"max_runners,omitempty"`
	MinIdleRunners         *uint           `json:"min_idle_runners,omitempty"`
	RunnerBootstrapTimeout *uint           `json:"runner_bootstrap_timeout,omitempty"`
	Image                  string          `json:"image"`
	Flavor                 string          `json:"flavor"`
	OSType                 OSType          `json:"os_type"`
	OSArch                 OSArch          `json:"os_arch"`
	ExtraSpecs             json.RawMessage `json:"extra_specs,omitempty"`
	// GithubRunnerGroup is the github runner group in which the runners of this
	// pool will be added to.
	// The runner group must be created by someone with access to the enterprise.
	GitHubRunnerGroup *string `json:"github-runner-group,omitempty"`
}

type CreateInstanceParams struct {
	Name         string
	OSType       OSType
	OSArch       OSArch
	Status       common.InstanceStatus
	RunnerStatus common.RunnerStatus
	CallbackURL  string
	MetadataURL  string
	// GithubRunnerGroup is the github runner group to which the runner belongs.
	// The runner group must be created by someone with access to the enterprise.
	GitHubRunnerGroup string
	CreateAttempt     int `json:"-"`
}

type CreatePoolParams struct {
	RunnerPrefix

	ProviderName           string          `json:"provider_name"`
	MaxRunners             uint            `json:"max_runners"`
	MinIdleRunners         uint            `json:"min_idle_runners"`
	Image                  string          `json:"image"`
	Flavor                 string          `json:"flavor"`
	OSType                 OSType          `json:"os_type"`
	OSArch                 OSArch          `json:"os_arch"`
	Tags                   []string        `json:"tags"`
	Enabled                bool            `json:"enabled"`
	RunnerBootstrapTimeout uint            `json:"runner_bootstrap_timeout"`
	ExtraSpecs             json.RawMessage `json:"extra_specs,omitempty"`
	// GithubRunnerGroup is the github runner group in which the runners of this
	// pool will be added to.
	// The runner group must be created by someone with access to the enterprise.
	GitHubRunnerGroup string `json:"github-runner-group"`
}

func (p *CreatePoolParams) Validate() error {
	if p.ProviderName == "" {
		return fmt.Errorf("missing provider")
	}

	if p.MinIdleRunners > p.MaxRunners {
		return fmt.Errorf("min_idle_runners cannot be larger than max_runners")
	}

	if p.MaxRunners == 0 {
		return fmt.Errorf("max_runners cannot be 0")
	}

	if len(p.Tags) == 0 {
		return fmt.Errorf("missing tags")
	}

	if p.Flavor == "" {
		return fmt.Errorf("missing flavor")
	}

	if p.Image == "" {
		return fmt.Errorf("missing image")
	}

	return nil
}

type UpdateInstanceParams struct {
	ProviderID string `json:"provider_id,omitempty"`
	// OSName is the name of the OS. Eg: ubuntu, centos, etc.
	OSName string `json:"os_name,omitempty"`
	// OSVersion is the version of the operating system.
	OSVersion string `json:"os_version,omitempty"`
	// Addresses is a list of IP addresses the provider reports
	// for this instance.
	Addresses []Address `json:"addresses,omitempty"`
	// Status is the status of the instance inside the provider (eg: running, stopped, etc)
	Status        common.InstanceStatus `json:"status,omitempty"`
	RunnerStatus  common.RunnerStatus   `json:"runner_status,omitempty"`
	ProviderFault []byte                `json:"provider_fault,omitempty"`
	AgentID       int64                 `json:"-"`
	CreateAttempt int                   `json:"-"`
	TokenFetched  *bool                 `json:"-"`
}

type UpdateUserParams struct {
	FullName string `json:"full_name"`
	Password string `json:"password"`
	Enabled  *bool  `json:"enabled"`
}

// PasswordLoginParams holds information used during
// password authentication, that will be passed to a
// password login function
type PasswordLoginParams struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Validate checks if the username and password are set
func (p PasswordLoginParams) Validate() error {
	if p.Username == "" || p.Password == "" {
		return errors.ErrUnauthorized
	}
	return nil
}

type UpdateRepositoryParams struct {
	CredentialsName string `json:"credentials_name"`
	WebhookSecret   string `json:"webhook_secret"`
}

type InstanceUpdateMessage struct {
	Status  common.RunnerStatus `json:"status"`
	Message string              `json:"message"`
	AgentID *int64              `json:"agent_id"`
}
