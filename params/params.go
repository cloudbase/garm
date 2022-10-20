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
	"garm/config"
	"garm/runner/providers/common"
	"time"

	"github.com/google/go-github/v48/github"
	uuid "github.com/satori/go.uuid"
)

type AddressType string

const (
	PublicAddress  AddressType = "public"
	PrivateAddress AddressType = "private"
)

type Address struct {
	Address string      `json:"address"`
	Type    AddressType `json:"type"`
}

type StatusMessage struct {
	CreatedAt time.Time `json:"created_at"`
	Message   string    `json:"message"`
}

type Instance struct {
	// ID is the database ID of this instance.
	ID string `json:"id,omitempty"`
	// PeoviderID is the unique ID the provider associated
	// with the compute instance. We use this to identify the
	// instance in the provider.
	ProviderID string `json:"provider_id,omitempty"`
	// AgentID is the github runner agent ID.
	AgentID int64 `json:"agent_id"`
	// Name is the name associated with an instance. Depending on
	// the provider, this may or may not be useful in the context of
	// the provider, but we can use it internally to identify the
	// instance.
	Name string `json:"name,omitempty"`
	// OSType is the operating system type. For now, only Linux and
	// Windows are supported.
	OSType config.OSType `json:"os_type,omitempty"`
	// OSName is the name of the OS. Eg: ubuntu, centos, etc.
	OSName string `json:"os_name,omitempty"`
	// OSVersion is the version of the operating system.
	OSVersion string `json:"os_version,omitempty"`
	// OSArch is the operating system architecture.
	OSArch config.OSArch `json:"os_arch,omitempty"`
	// Addresses is a list of IP addresses the provider reports
	// for this instance.
	Addresses []Address `json:"addresses,omitempty"`
	// Status is the status of the instance inside the provider (eg: running, stopped, etc)
	Status        common.InstanceStatus `json:"status,omitempty"`
	RunnerStatus  common.RunnerStatus   `json:"runner_status,omitempty"`
	PoolID        string                `json:"pool_id,omitempty"`
	ProviderFault []byte                `json:"provider_fault,omitempty"`

	StatusMessages []StatusMessage `json:"status_messages,omitempty"`

	// Do not serialize sensitive info.
	CallbackURL   string    `json:"-"`
	CreateAttempt int       `json:"-"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type BootstrapInstance struct {
	Name  string                              `json:"name"`
	Tools []*github.RunnerApplicationDownload `json:"tools"`
	// RepoURL is the URL the github runner agent needs to configure itself.
	RepoURL string `json:"repo_url"`
	// GithubRunnerAccessToken is the token we fetch from github to allow the runner to
	// register itself.
	GithubRunnerAccessToken string `json:"github_runner_access_token"`
	// CallbackUrl is the URL where the instance can send a post, signaling
	// progress or status.
	CallbackURL string `json:"callback-url"`
	// InstanceToken is the token that needs to be set by the instance in the headers
	// in order to send updated back to the garm via CallbackURL.
	InstanceToken string `json:"instance-token"`
	// SSHKeys are the ssh public keys we may want to inject inside the runners, if the
	// provider supports it.
	SSHKeys []string `json:"ssh-keys"`

	CACertBundle []byte `json:"ca-cert-bundle"`

	OSArch config.OSArch `json:"arch"`
	Flavor string        `json:"flavor"`
	Image  string        `json:"image"`
	Labels []string      `json:"labels"`
	PoolID string        `json:"pool_id"`
}

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Pool struct {
	ID                     string        `json:"id"`
	ProviderName           string        `json:"provider_name"`
	MaxRunners             uint          `json:"max_runners"`
	MinIdleRunners         uint          `json:"min_idle_runners"`
	Image                  string        `json:"image"`
	Flavor                 string        `json:"flavor"`
	OSType                 config.OSType `json:"os_type"`
	OSArch                 config.OSArch `json:"os_arch"`
	Tags                   []Tag         `json:"tags"`
	Enabled                bool          `json:"enabled"`
	Instances              []Instance    `json:"instances"`
	RepoID                 string        `json:"repo_id,omitempty"`
	RepoName               string        `json:"repo_name,omitempty"`
	OrgID                  string        `json:"org_id,omitempty"`
	OrgName                string        `json:"org_name,omitempty"`
	EnterpriseID           string        `json:"enterprise_id,omitempty"`
	EnterpriseName         string        `json:"enterprise_name,omitempty"`
	RunnerBootstrapTimeout uint          `json:"runner_bootstrap_timeout"`
}

func (p *Pool) RunnerTimeout() uint {
	if p.RunnerBootstrapTimeout == 0 {
		return config.DefaultRunnerBootstrapTimeout
	}
	return p.RunnerBootstrapTimeout
}

type Internal struct {
	OAuth2Token         string `json:"oauth2"`
	ControllerID        string `json:"controller_id"`
	InstanceCallbackURL string `json:"instance_callback_url"`
	JWTSecret           string `json:"jwt_secret"`
	// GithubCredentialsDetails contains all info about the credentials, except the
	// token, which is added above.
	GithubCredentialsDetails GithubCredentials `json:"gh_creds_details"`
}

type Repository struct {
	ID              string `json:"id"`
	Owner           string `json:"owner"`
	Name            string `json:"name"`
	Pools           []Pool `json:"pool,omitempty"`
	CredentialsName string `json:"credentials_name"`
	// Do not serialize sensitive info.
	WebhookSecret string `json:"-"`
}

type Organization struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Pools           []Pool `json:"pool,omitempty"`
	CredentialsName string `json:"credentials_name"`
	// Do not serialize sensitive info.
	WebhookSecret string `json:"-"`
}

type Enterprise struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Pools           []Pool `json:"pool,omitempty"`
	CredentialsName string `json:"credentials_name"`
	// Do not serialize sensitive info.
	WebhookSecret string `json:"-"`
}

// Users holds information about a particular user
type User struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	Password  string    `json:"-"`
	Enabled   bool      `json:"enabled"`
	IsAdmin   bool      `json:"is_admin"`
}

// JWTResponse holds the JWT token returned as a result of a
// successful auth
type JWTResponse struct {
	Token string `json:"token"`
}

type ControllerInfo struct {
	ControllerID uuid.UUID `json:"controller_id"`
}

type GithubCredentials struct {
	Name          string `json:"name,omitempty"`
	Description   string `json:"description,omitempty"`
	BaseURL       string `json:"base_url"`
	APIBaseURL    string `json:"api_base_url"`
	UploadBaseURL string `json:"upload_base_url"`
	CABundle      []byte `json:"ca_bundle,omitempty"`
}

type Provider struct {
	Name         string              `json:"name"`
	ProviderType config.ProviderType `json:"type"`
	Description  string              `json:"description"`
}

type UpdatePoolStateParams struct {
	WebhookSecret string
}
