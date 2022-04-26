package params

import (
	"runner-manager/config"
	"runner-manager/runner/providers/common"

	"github.com/google/go-github/v43/github"
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

type Instance struct {
	// ID is the database ID of this instance.
	ID string `json:"id"`
	// PeoviderID is the unique ID the provider associated
	// with the compute instance. We use this to identify the
	// instance in the provider.
	ProviderID string `json:"provider_id,omitempty"`
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
	Status       common.InstanceStatus `json:"status"`
	RunnerStatus common.RunnerStatus   `json:"runner_status"`
	PoolID       string                `json:"pool_id"`

	// Do not serialize sensitive info.
	CallbackURL   string `json:"-"`
	CallbackToken string `json:"-"`
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
	// in order to send updated back to the runner-manager via CallbackURL.
	InstanceToken string `json:"instance-token"`
	// SSHKeys are the ssh public keys we may want to inject inside the runners, if the
	// provider supports it.
	SSHKeys []string `json:"ssh-keys"`

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
	ID             string        `json:"id"`
	ProviderName   string        `json:"provider_name"`
	MaxRunners     uint          `json:"max_runners"`
	MinIdleRunners uint          `json:"min_idle_runners"`
	Image          string        `json:"image"`
	Flavor         string        `json:"flavor"`
	OSType         config.OSType `json:"os_type"`
	OSArch         config.OSArch `json:"os_arch"`
	Tags           []Tag         `json:"tags"`
}

type Internal struct {
	OAuth2Token         string `json:"oauth2"`
	ControllerID        string `json:"controller_id"`
	InstanceCallbackURL string `json:"instance_callback_url"`
}

type Repository struct {
	ID    string `json:"id"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
	Pools []Pool `json:"pool,omitempty"`
	// Do not serialize sensitive info.
	WebhookSecret string   `json:"-"`
	Internal      Internal `json:"-"`
}

type Organization struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Pools []Pool `json:"pool,omitempty"`
	// Do not serialize sensitive info.
	WebhookSecret string   `json:"-"`
	Internal      Internal `json:"-"`
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
}

/*
	Name          string `gorm:"uniqueIndex"`
	OSType        config.OSType
	OSArch        config.OSArch
	OSName        string
	OSVersion     string
	Addresses     []Address `gorm:"foreignKey:id"`
	Status        string
	RunnerStatus  string
	CallbackURL   string
	CallbackToken []byte

	Pool Pool `gorm:"foreignKey:id"`
*/

type CreateInstanceParams struct {
	Name          string
	OSType        config.OSType
	OSArch        config.OSArch
	Status        common.InstanceStatus
	RunnerStatus  common.RunnerStatus
	CallbackURL   string
	CallbackToken string

	Pool string
}

type UpdatePoolParams struct{}
