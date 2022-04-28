package params

import (
	"runner-manager/config"
	"runner-manager/errors"
	"runner-manager/runner/providers/common"
)

type InstanceRequest struct {
	Name      string        `json:"name"`
	OSType    config.OSType `json:"os_type"`
	OSVersion string        `json:"os_version"`
}

type CreateRepoParams struct {
	Owner           string `json:"owner"`
	Name            string `json:"name"`
	CredentialsName string `json:"credentials_name"`
	WebhookSecret   string `json:"webhook_secret"`
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
	Tags           []Tag         `json:"tags"`
	Enabled        *bool         `json:"enabled"`
	MaxRunners     *uint         `json:"max_runners"`
	MinIdleRunners *uint         `json:"min_idle_runners"`
	Image          string        `json:"image"`
	Flavor         string        `json:"flavor"`
	OSType         config.OSType `json:"os_type"`
	OSArch         config.OSArch `json:"os_arch"`
}

type CreateInstanceParams struct {
	Name         string
	OSType       config.OSType
	OSArch       config.OSArch
	Status       common.InstanceStatus
	RunnerStatus common.RunnerStatus
	CallbackURL  string

	Pool string
}

type CreatePoolParams struct {
	ProviderName   string        `json:"provider_name"`
	MaxRunners     uint          `json:"max_runners"`
	MinIdleRunners uint          `json:"min_idle_runners"`
	Image          string        `json:"image"`
	Flavor         string        `json:"flavor"`
	OSType         config.OSType `json:"os_type"`
	OSArch         config.OSArch `json:"os_arch"`
	Tags           []string      `json:"tags"`
	Enabled        bool          `json:"enabled"`
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
	Status       common.InstanceStatus `json:"status"`
	RunnerStatus common.RunnerStatus   `json:"runner_status"`
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
