package params

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
