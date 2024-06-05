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
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/url"

	"github.com/pkg/errors"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
)

const (
	DefaultRunnerPrefix string = "garm"
	httpsScheme         string = "https"
	httpScheme          string = "http"
)

type InstanceRequest struct {
	Name      string              `json:"name"`
	OSType    commonParams.OSType `json:"os_type"`
	OSVersion string              `json:"os_version"`
}

type CreateRepoParams struct {
	Owner            string           `json:"owner"`
	Name             string           `json:"name"`
	CredentialsName  string           `json:"credentials_name"`
	WebhookSecret    string           `json:"webhook_secret"`
	PoolBalancerType PoolBalancerType `json:"pool_balancer_type"`
}

func (c *CreateRepoParams) Validate() error {
	if c.Owner == "" {
		return runnerErrors.NewBadRequestError("missing owner")
	}

	if c.Name == "" {
		return runnerErrors.NewBadRequestError("missing repo name")
	}

	if c.CredentialsName == "" {
		return runnerErrors.NewBadRequestError("missing credentials name")
	}
	if c.WebhookSecret == "" {
		return runnerErrors.NewMissingSecretError("missing secret")
	}

	switch c.PoolBalancerType {
	case PoolBalancerTypeRoundRobin, PoolBalancerTypePack, PoolBalancerTypeNone:
	default:
		return runnerErrors.NewBadRequestError("invalid pool balancer type")
	}

	return nil
}

type CreateOrgParams struct {
	Name             string           `json:"name"`
	CredentialsName  string           `json:"credentials_name"`
	WebhookSecret    string           `json:"webhook_secret"`
	PoolBalancerType PoolBalancerType `json:"pool_balancer_type"`
}

func (c *CreateOrgParams) Validate() error {
	if c.Name == "" {
		return runnerErrors.NewBadRequestError("missing org name")
	}

	if c.CredentialsName == "" {
		return runnerErrors.NewBadRequestError("missing credentials name")
	}
	if c.WebhookSecret == "" {
		return runnerErrors.NewMissingSecretError("missing secret")
	}

	switch c.PoolBalancerType {
	case PoolBalancerTypeRoundRobin, PoolBalancerTypePack, PoolBalancerTypeNone:
	default:
		return runnerErrors.NewBadRequestError("invalid pool balancer type")
	}
	return nil
}

type CreateEnterpriseParams struct {
	Name             string           `json:"name"`
	CredentialsName  string           `json:"credentials_name"`
	WebhookSecret    string           `json:"webhook_secret"`
	PoolBalancerType PoolBalancerType `json:"pool_balancer_type"`
}

func (c *CreateEnterpriseParams) Validate() error {
	if c.Name == "" {
		return runnerErrors.NewBadRequestError("missing enterprise name")
	}
	if c.CredentialsName == "" {
		return runnerErrors.NewBadRequestError("missing credentials name")
	}
	if c.WebhookSecret == "" {
		return runnerErrors.NewMissingSecretError("missing secret")
	}

	switch c.PoolBalancerType {
	case PoolBalancerTypeRoundRobin, PoolBalancerTypePack, PoolBalancerTypeNone:
	default:
		return runnerErrors.NewBadRequestError("invalid pool balancer type")
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

	Tags                   []string            `json:"tags,omitempty"`
	Enabled                *bool               `json:"enabled,omitempty"`
	MaxRunners             *uint               `json:"max_runners,omitempty"`
	MinIdleRunners         *uint               `json:"min_idle_runners,omitempty"`
	RunnerBootstrapTimeout *uint               `json:"runner_bootstrap_timeout,omitempty"`
	Image                  string              `json:"image"`
	Flavor                 string              `json:"flavor"`
	OSType                 commonParams.OSType `json:"os_type"`
	OSArch                 commonParams.OSArch `json:"os_arch"`
	ExtraSpecs             json.RawMessage     `json:"extra_specs,omitempty"`
	// GithubRunnerGroup is the github runner group in which the runners of this
	// pool will be added to.
	// The runner group must be created by someone with access to the enterprise.
	GitHubRunnerGroup *string `json:"github-runner-group,omitempty"`
	Priority          *uint   `json:"priority,omitempty"`
}

type CreateInstanceParams struct {
	Name         string
	OSType       commonParams.OSType
	OSArch       commonParams.OSArch
	Status       commonParams.InstanceStatus
	RunnerStatus RunnerStatus
	CallbackURL  string
	MetadataURL  string
	// GithubRunnerGroup is the github runner group to which the runner belongs.
	// The runner group must be created by someone with access to the enterprise.
	GitHubRunnerGroup string
	CreateAttempt     int   `json:"-"`
	AgentID           int64 `json:"-"`
	AditionalLabels   []string
	JitConfiguration  map[string]string
}

type CreatePoolParams struct {
	RunnerPrefix

	ProviderName           string              `json:"provider_name"`
	MaxRunners             uint                `json:"max_runners"`
	MinIdleRunners         uint                `json:"min_idle_runners"`
	Image                  string              `json:"image"`
	Flavor                 string              `json:"flavor"`
	OSType                 commonParams.OSType `json:"os_type"`
	OSArch                 commonParams.OSArch `json:"os_arch"`
	Tags                   []string            `json:"tags"`
	Enabled                bool                `json:"enabled"`
	RunnerBootstrapTimeout uint                `json:"runner_bootstrap_timeout"`
	ExtraSpecs             json.RawMessage     `json:"extra_specs,omitempty"`
	// GithubRunnerGroup is the github runner group in which the runners of this
	// pool will be added to.
	// The runner group must be created by someone with access to the enterprise.
	GitHubRunnerGroup string `json:"github-runner-group"`
	Priority          uint   `json:"priority"`
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
	Addresses []commonParams.Address `json:"addresses,omitempty"`
	// Status is the status of the instance inside the provider (eg: running, stopped, etc)
	Status           commonParams.InstanceStatus `json:"status,omitempty"`
	RunnerStatus     RunnerStatus                `json:"runner_status,omitempty"`
	ProviderFault    []byte                      `json:"provider_fault,omitempty"`
	AgentID          int64                       `json:"-"`
	CreateAttempt    int                         `json:"-"`
	TokenFetched     *bool                       `json:"-"`
	JitConfiguration map[string]string           `json:"-"`
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
		return runnerErrors.ErrUnauthorized
	}
	return nil
}

type UpdateEntityParams struct {
	CredentialsName  string           `json:"credentials_name"`
	WebhookSecret    string           `json:"webhook_secret"`
	PoolBalancerType PoolBalancerType `json:"pool_balancer_type"`
}

type InstanceUpdateMessage struct {
	Status  RunnerStatus `json:"status"`
	Message string       `json:"message"`
	AgentID *int64       `json:"agent_id,omitempty"`
}

type CreateGithubEndpointParams struct {
	Name          string `json:"name,omitempty"`
	Description   string `json:"description,omitempty"`
	APIBaseURL    string `json:"api_base_url,omitempty"`
	UploadBaseURL string `json:"upload_base_url,omitempty"`
	BaseURL       string `json:"base_url,omitempty"`
	CACertBundle  []byte `json:"ca_cert_bundle,omitempty"`
}

func (c CreateGithubEndpointParams) Validate() error {
	if c.APIBaseURL == "" {
		return runnerErrors.NewBadRequestError("missing api_base_url")
	}

	url, err := url.Parse(c.APIBaseURL)
	if err != nil || url.Scheme == "" || url.Host == "" {
		return runnerErrors.NewBadRequestError("invalid api_base_url")
	}
	switch url.Scheme {
	case httpsScheme, httpScheme:
	default:
		return runnerErrors.NewBadRequestError("invalid api_base_url")
	}

	if c.UploadBaseURL == "" {
		return runnerErrors.NewBadRequestError("missing upload_base_url")
	}

	url, err = url.Parse(c.UploadBaseURL)
	if err != nil || url.Scheme == "" || url.Host == "" {
		return runnerErrors.NewBadRequestError("invalid upload_base_url")
	}

	switch url.Scheme {
	case httpsScheme, httpScheme:
	default:
		return runnerErrors.NewBadRequestError("invalid api_base_url")
	}

	if c.BaseURL == "" {
		return runnerErrors.NewBadRequestError("missing base_url")
	}

	url, err = url.Parse(c.BaseURL)
	if err != nil || url.Scheme == "" || url.Host == "" {
		return runnerErrors.NewBadRequestError("invalid base_url")
	}

	switch url.Scheme {
	case httpsScheme, httpScheme:
	default:
		return runnerErrors.NewBadRequestError("invalid api_base_url")
	}

	if c.CACertBundle != nil {
		block, _ := pem.Decode(c.CACertBundle)
		if block == nil {
			return runnerErrors.NewBadRequestError("invalid ca_cert_bundle")
		}
		if _, err := x509.ParseCertificates(block.Bytes); err != nil {
			return runnerErrors.NewBadRequestError("invalid ca_cert_bundle")
		}
	}

	return nil
}

type UpdateGithubEndpointParams struct {
	Description   *string `json:"description,omitempty"`
	APIBaseURL    *string `json:"api_base_url,omitempty"`
	UploadBaseURL *string `json:"upload_base_url,omitempty"`
	BaseURL       *string `json:"base_url,omitempty"`
	CACertBundle  []byte  `json:"ca_cert_bundle,omitempty"`
}

func (u UpdateGithubEndpointParams) Validate() error {
	if u.APIBaseURL != nil {
		url, err := url.Parse(*u.APIBaseURL)
		if err != nil || url.Scheme == "" || url.Host == "" {
			return runnerErrors.NewBadRequestError("invalid api_base_url")
		}
		switch url.Scheme {
		case httpsScheme, httpScheme:
		default:
			return runnerErrors.NewBadRequestError("invalid api_base_url")
		}
	}

	if u.UploadBaseURL != nil {
		url, err := url.Parse(*u.UploadBaseURL)
		if err != nil || url.Scheme == "" || url.Host == "" {
			return runnerErrors.NewBadRequestError("invalid upload_base_url")
		}
		switch url.Scheme {
		case httpsScheme, httpScheme:
		default:
			return runnerErrors.NewBadRequestError("invalid api_base_url")
		}
	}

	if u.BaseURL != nil {
		url, err := url.Parse(*u.BaseURL)
		if err != nil || url.Scheme == "" || url.Host == "" {
			return runnerErrors.NewBadRequestError("invalid base_url")
		}
		switch url.Scheme {
		case httpsScheme, httpScheme:
		default:
			return runnerErrors.NewBadRequestError("invalid api_base_url")
		}
	}

	if u.CACertBundle != nil {
		block, _ := pem.Decode(u.CACertBundle)
		if block == nil {
			return runnerErrors.NewBadRequestError("invalid ca_cert_bundle")
		}
		if _, err := x509.ParseCertificates(block.Bytes); err != nil {
			return runnerErrors.NewBadRequestError("invalid ca_cert_bundle")
		}
	}

	return nil
}

type GithubPAT struct {
	OAuth2Token string `json:"oauth2_token"`
}

type GithubApp struct {
	AppID           int64  `json:"app_id"`
	InstallationID  int64  `json:"installation_id"`
	PrivateKeyBytes []byte `json:"private_key_bytes"`
}

func (g GithubApp) Validate() error {
	if g.AppID == 0 {
		return runnerErrors.NewBadRequestError("missing app_id")
	}

	if g.InstallationID == 0 {
		return runnerErrors.NewBadRequestError("missing installation_id")
	}

	if len(g.PrivateKeyBytes) == 0 {
		return runnerErrors.NewBadRequestError("missing private_key_bytes")
	}

	block, _ := pem.Decode(g.PrivateKeyBytes)
	if block == nil {
		return runnerErrors.NewBadRequestError("invalid private_key_bytes")
	}
	// Parse the private key as PCKS1
	_, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parsing private_key_path: %w", err)
	}

	return nil
}

type CreateGithubCredentialsParams struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Endpoint    string         `json:"endpoint"`
	AuthType    GithubAuthType `json:"auth_type"`
	PAT         GithubPAT      `json:"pat,omitempty"`
	App         GithubApp      `json:"app,omitempty"`
}

func (c CreateGithubCredentialsParams) Validate() error {
	if c.Name == "" {
		return runnerErrors.NewBadRequestError("missing name")
	}

	if c.Endpoint == "" {
		return runnerErrors.NewBadRequestError("missing endpoint")
	}

	switch c.AuthType {
	case GithubAuthTypePAT, GithubAuthTypeApp:
	default:
		return runnerErrors.NewBadRequestError("invalid auth_type")
	}

	if c.AuthType == GithubAuthTypePAT {
		if c.PAT.OAuth2Token == "" {
			return runnerErrors.NewBadRequestError("missing oauth2_token")
		}
	}

	if c.AuthType == GithubAuthTypeApp {
		if err := c.App.Validate(); err != nil {
			return errors.Wrap(err, "invalid app")
		}
	}

	return nil
}

type UpdateGithubCredentialsParams struct {
	Name        *string    `json:"name,omitempty"`
	Description *string    `json:"description,omitempty"`
	PAT         *GithubPAT `json:"pat,omitempty"`
	App         *GithubApp `json:"app,omitempty"`
}

func (u UpdateGithubCredentialsParams) Validate() error {
	if u.PAT != nil && u.App != nil {
		return runnerErrors.NewBadRequestError("cannot update both PAT and App")
	}

	if u.PAT != nil {
		if u.PAT.OAuth2Token == "" {
			return runnerErrors.NewBadRequestError("missing oauth2_token")
		}
	}

	if u.App != nil {
		if err := u.App.Validate(); err != nil {
			return errors.Wrap(err, "invalid app")
		}
	}

	return nil
}

type UpdateControllerParams struct {
	MetadataURL *string `json:"metadata_url,omitempty"`
	CallbackURL *string `json:"callback_url,omitempty"`
	WebhookURL  *string `json:"webhook_url,omitempty"`
}

func (u UpdateControllerParams) Validate() error {
	if u.MetadataURL != nil {
		u, err := url.Parse(*u.MetadataURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return runnerErrors.NewBadRequestError("invalid metadata_url")
		}
	}

	if u.CallbackURL != nil {
		u, err := url.Parse(*u.CallbackURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return runnerErrors.NewBadRequestError("invalid callback_url")
		}
	}

	if u.WebhookURL != nil {
		u, err := url.Parse(*u.WebhookURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return runnerErrors.NewBadRequestError("invalid webhook_url")
		}
	}

	return nil
}
