package params

import "github.com/google/go-github/v43/github"

type OSType string

const (
	Linux   OSType = "linux"
	Windows OSType = "windows"
)

type Instance struct {
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
	OSType OSType `json:"os_type,omitempty"`
	// OSVersion is the version of the operating system.
	OSVersion string `json:"os_version,omitempty"`
	// OSArch is the operating system architecture.
	OSArch string `json:"os_arch,omitempty"`
	// Addresses is a list of IP addresses the provider reports
	// for this instance.
	Addresses []string `json:"ip_addresses,omitempty"`
}

type BootstrapInstance struct {
	Tools []*github.RunnerApplicationDownload `json:"tools"`
	// RepoURL is the URL the github runner agent needs to configure itself.
	RepoURL string `json:"repo_url"`
	// GithubRunnerAccessToken is the token we fetch from github to allow the runner to
	// register itself.
	GithubRunnerAccessToken string `json:"github_runner_access_token"`
	// RunnerType is the name of the defined runner type in a particular pool. The provider
	// needs this to determine which flavor/image/settings it needs to use to create the
	// instance. This is provider/runner specific. The config for the runner type is defined
	// in the configuration file, as part of the pool definition.
	RunnerType string `json:"runner_type"`
	// CallbackUrl is the URL where the instance can send a post, signaling
	// progress or status.
	CallbackURL string `json:"callback_url"`
	// InstanceToken is the token that needs to be set by the instance in the headers
	// in order to send updated back to the runner-manager via CallbackURL.
	InstanceToken string `json:"instance_token"`
	// SSHKeys are the ssh public keys we may want to inject inside the runners, if the
	// provider supports it.
	SSHKeys []string `json:"ssh_keys"`
}
