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

package params

import (
	"encoding/json"
)

type (
	AddressType    string
	InstanceStatus string
	OSType         string
	OSArch         string
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

const (
	InstanceRunning            InstanceStatus = "running"
	InstanceStopped            InstanceStatus = "stopped"
	InstanceError              InstanceStatus = "error"
	InstancePendingDelete      InstanceStatus = "pending_delete"
	InstancePendingForceDelete InstanceStatus = "pending_force_delete"
	InstanceDeleting           InstanceStatus = "deleting"
	InstanceDeleted            InstanceStatus = "deleted"
	InstancePendingCreate      InstanceStatus = "pending_create"
	InstanceCreating           InstanceStatus = "creating"
	InstanceStatusUnknown      InstanceStatus = "unknown"
)

const (
	PublicAddress  AddressType = "public"
	PrivateAddress AddressType = "private"
)

type UserDataOptions struct {
	DisableUpdatesOnBoot bool     `json:"disable_updates_on_boot"`
	ExtraPackages        []string `json:"extra_packages"`
	EnableBootDebug      bool     `json:"enable_boot_debug"`
}

type BootstrapInstance struct {
	Name  string                      `json:"name"`
	Tools []RunnerApplicationDownload `json:"tools"`
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

	// UserDataOptions are the options for the user data generation.
	UserDataOptions UserDataOptions `json:"user_data_options"`

	// JitConfigEnabled is a flag that indicates if the runner should be configured to use
	// just-in-time configuration. If set to true, providers must attempt to fetch the JIT configuration
	// from the metadata service instead of the runner registration token. The runner registration token
	// is not available if the runner is configured to use JIT.
	JitConfigEnabled bool `json:"jit_config_enabled"`
}

type Address struct {
	Address string      `json:"address"`
	Type    AddressType `json:"type"`
}

type ProviderInstance struct {
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
	Status InstanceStatus `json:"status,omitempty"`

	// ProviderFault holds any error messages captured from the IaaS provider that is
	// responsible for managing the lifecycle of the runner.
	ProviderFault []byte `json:"provider_fault,omitempty"`
}
