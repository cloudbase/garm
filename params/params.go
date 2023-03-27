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
	"time"

	"github.com/cloudbase/garm/runner/providers/common"
	"github.com/cloudbase/garm/util/appdefaults"

	"github.com/google/go-github/v48/github"
	uuid "github.com/satori/go.uuid"
)

type PoolType string
type AddressType string
type EventType string
type EventLevel string
type OSType string
type OSArch string
type ProviderType string

const (
	// LXDProvider represents the LXD provider.
	LXDProvider ProviderType = "lxd"
	// ExternalProvider represents an external provider.
	ExternalProvider ProviderType = "external"
)

const (
	RepositoryPool   PoolType = "repository"
	OrganizationPool PoolType = "organization"
	EnterprisePool   PoolType = "enterprise"
)

const (
	PublicAddress  AddressType = "public"
	PrivateAddress AddressType = "private"
)

const (
	StatusEvent     EventType = "status"
	FetchTokenEvent EventType = "fetchToken"
)

const (
	EventInfo    EventLevel = "info"
	EventWarning EventLevel = "warning"
	EventError   EventLevel = "error"
)

const (
	Windows OSType = "windows"
	Linux   OSType = "linux"
	Unknown OSType = "unknown"
)

const (
	Amd64 OSArch = "amd64"
	I386  OSArch = "i386"
	Arm64 OSArch = "arm64"
	Arm   OSArch = "arm"
)

type Address struct {
	Address string      `json:"address"`
	Type    AddressType `json:"type"`
}

type StatusMessage struct {
	CreatedAt  time.Time  `json:"created_at"`
	Message    string     `json:"message"`
	EventType  EventType  `json:"event_type"`
	EventLevel EventLevel `json:"event_level"`
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
	OSType OSType `json:"os_type,omitempty"`

	// OSName is the name of the OS. Eg: ubuntu, centos, etc.
	OSName string `json:"os_name,omitempty"`

	// OSVersion is the version of the operating system.
	OSVersion string `json:"os_version,omitempty"`

	// OSArch is the operating system architecture.
	OSArch OSArch `json:"os_arch,omitempty"`

	// Addresses is a list of IP addresses the provider reports
	// for this instance.
	Addresses []Address `json:"addresses,omitempty"`

	// Status is the status of the instance inside the provider (eg: running, stopped, etc)
	Status common.InstanceStatus `json:"status,omitempty"`

	// RunnerStatus is the github runner status as it appears on GitHub.
	RunnerStatus common.RunnerStatus `json:"runner_status,omitempty"`

	// PoolID is the ID of the garm pool to which a runner belongs.
	PoolID string `json:"pool_id,omitempty"`

	// ProviderFault holds any error messages captured from the IaaS provider that is
	// responsible for managing the lifecycle of the runner.
	ProviderFault []byte `json:"provider_fault,omitempty"`

	// StatusMessages is a list of status messages sent back by the runner as it sets itself
	// up.
	StatusMessages []StatusMessage `json:"status_messages,omitempty"`

	// UpdatedAt is the timestamp of the last update to this runner.
	UpdatedAt time.Time `json:"updated_at"`

	// GithubRunnerGroup is the github runner group to which the runner belongs.
	// The runner group must be created by someone with access to the enterprise.
	GitHubRunnerGroup string `json:"github-runner-group"`

	// Do not serialize sensitive info.
	CallbackURL   string `json:"-"`
	MetadataURL   string `json:"-"`
	CreateAttempt int    `json:"-"`
	TokenFetched  bool   `json:"-"`
}

func (i Instance) GetName() string {
	return i.Name
}

func (i Instance) GetID() string {
	return i.ID
}

type BootstrapInstance struct {
	Name  string                              `json:"name"`
	Tools []*github.RunnerApplicationDownload `json:"tools"`
	// RepoURL is the URL the github runner agent needs to configure itself.
	RepoURL string `json:"repo_url"`
	// CallbackUrl is the URL where the instance can send a post, signaling
	// progress or status.
	CallbackURL string `json:"callback-url"`
	// MetadataURL is the URL where instances can fetch information needed to set themselves up.
	MetadataURL string `json:"metadata-url"`
	// InstanceToken is the token that needs to be set by the instance in the headers
	// in order to send updated back to the garm via CallbackURL.
	InstanceToken string `json:"instance-token"`
	// SSHKeys are the ssh public keys we may want to inject inside the runners, if the
	// provider supports it.
	SSHKeys []string `json:"ssh-keys"`
	// ExtraSpecs is an opaque raw json that gets sent to the provider
	// as part of the bootstrap params for instances. It can contain
	// any kind of data needed by providers. The contents of this field means
	// nothing to garm itself. We don't act on the information in this field at
	// all. We only validate that it's a proper json.
	ExtraSpecs json.RawMessage `json:"extra_specs,omitempty"`

	// GitHubRunnerGroup is the github runner group in which the newly installed runner
	// should be added to. The runner group must be created by someone with access to the
	// enterprise.
	GitHubRunnerGroup string `json:"github-runner-group"`

	// CACertBundle is a CA certificate bundle which will be sent to instances and which
	// will tipically be installed as a system wide trusted root CA. by either cloud-init
	// or whatever mechanism the provider will use to set up the runner.
	CACertBundle []byte `json:"ca-cert-bundle"`

	// OSArch is the target OS CPU architecture of the runner.
	OSArch OSArch `json:"arch"`

	// OSType is the target OS platform of the runner (windows, linux).
	OSType OSType `json:"os_type"`

	// Flavor is the platform specific abstraction that defines what resources will be allocated
	// to the runner (CPU, RAM, disk space, etc). This field is meaningful to the provider which
	// handles the actual creation.
	Flavor string `json:"flavor"`

	// Image is the platform specific identifier of the operating system template that will be used
	// to spin up a new machine.
	Image string `json:"image"`

	// Labels are a list of github runner labels that will be added to the runner.
	Labels []string `json:"labels"`

	// PoolID is the ID of the garm pool to which this runner belongs.
	PoolID string `json:"pool_id"`
}

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Pool struct {
	RunnerPrefix

	ID                     string     `json:"id"`
	ProviderName           string     `json:"provider_name"`
	MaxRunners             uint       `json:"max_runners"`
	MinIdleRunners         uint       `json:"min_idle_runners"`
	Image                  string     `json:"image"`
	Flavor                 string     `json:"flavor"`
	OSType                 OSType     `json:"os_type"`
	OSArch                 OSArch     `json:"os_arch"`
	Tags                   []Tag      `json:"tags"`
	Enabled                bool       `json:"enabled"`
	Instances              []Instance `json:"instances"`
	RepoID                 string     `json:"repo_id,omitempty"`
	RepoName               string     `json:"repo_name,omitempty"`
	OrgID                  string     `json:"org_id,omitempty"`
	OrgName                string     `json:"org_name,omitempty"`
	EnterpriseID           string     `json:"enterprise_id,omitempty"`
	EnterpriseName         string     `json:"enterprise_name,omitempty"`
	RunnerBootstrapTimeout uint       `json:"runner_bootstrap_timeout"`
	// ExtraSpecs is an opaque raw json that gets sent to the provider
	// as part of the bootstrap params for instances. It can contain
	// any kind of data needed by providers. The contents of this field means
	// nothing to garm itself. We don't act on the information in this field at
	// all. We only validate that it's a proper json.
	ExtraSpecs json.RawMessage `json:"extra_specs,omitempty"`
	// GithubRunnerGroup is the github runner group in which the runners will be added.
	// The runner group must be created by someone with access to the enterprise.
	GitHubRunnerGroup string `json:"github-runner-group"`
}

func (p Pool) GetID() string {
	return p.ID
}

func (p *Pool) RunnerTimeout() uint {
	if p.RunnerBootstrapTimeout == 0 {
		return appdefaults.DefaultRunnerBootstrapTimeout
	}
	return p.RunnerBootstrapTimeout
}

func (p *Pool) PoolType() PoolType {
	if p.RepoID != "" {
		return RepositoryPool
	} else if p.OrgID != "" {
		return OrganizationPool
	} else if p.EnterpriseID != "" {
		return EnterprisePool
	}
	return ""
}

type Internal struct {
	OAuth2Token         string `json:"oauth2"`
	ControllerID        string `json:"controller_id"`
	InstanceCallbackURL string `json:"instance_callback_url"`
	InstanceMetadataURL string `json:"instance_metadata_url"`
	JWTSecret           string `json:"jwt_secret"`
	// GithubCredentialsDetails contains all info about the credentials, except the
	// token, which is added above.
	GithubCredentialsDetails GithubCredentials `json:"gh_creds_details"`
}

type Repository struct {
	ID                string            `json:"id"`
	Owner             string            `json:"owner"`
	Name              string            `json:"name"`
	Pools             []Pool            `json:"pool,omitempty"`
	CredentialsName   string            `json:"credentials_name"`
	PoolManagerStatus PoolManagerStatus `json:"pool_manager_status,omitempty"`
	// Do not serialize sensitive info.
	WebhookSecret string `json:"-"`
}

func (r Repository) GetName() string {
	return r.Name
}

func (r Repository) GetID() string {
	return r.ID
}

type Organization struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Pools             []Pool            `json:"pool,omitempty"`
	CredentialsName   string            `json:"credentials_name"`
	PoolManagerStatus PoolManagerStatus `json:"pool_manager_status,omitempty"`
	// Do not serialize sensitive info.
	WebhookSecret string `json:"-"`
}

func (o Organization) GetName() string {
	return o.Name
}

func (o Organization) GetID() string {
	return o.ID
}

type Enterprise struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Pools             []Pool            `json:"pool,omitempty"`
	CredentialsName   string            `json:"credentials_name"`
	PoolManagerStatus PoolManagerStatus `json:"pool_manager_status,omitempty"`
	// Do not serialize sensitive info.
	WebhookSecret string `json:"-"`
}

func (e Enterprise) GetName() string {
	return e.Name
}

func (e Enterprise) GetID() string {
	return e.ID
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
	Hostname     string    `json:"hostname"`
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
	Name         string       `json:"name"`
	ProviderType ProviderType `json:"type"`
	Description  string       `json:"description"`
}

type UpdatePoolStateParams struct {
	WebhookSecret string
}

type PoolManagerStatus struct {
	IsRunning     bool   `json:"running"`
	FailureReason string `json:"failure_reason,omitempty"`
}

type RunnerInfo struct {
	Name   string
	Labels []string
}

type RunnerPrefix struct {
	Prefix string `json:"runner_prefix"`
}

func (p RunnerPrefix) GetRunnerPrefix() string {
	if p.Prefix == "" {
		return DefaultRunnerPrefix
	}
	return p.Prefix
}
