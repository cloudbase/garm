# Provider configuration

GARM was designed to be extensible. Providers can be written as external executables. External providers are executables that implement the needed interface to create/delete/list compute systems that are used by ```GARM``` to create runners.

- [External provider](#external-provider)
    - [Available external providers](#available-external-providers)

## External provider

The external provider is a special kind of provider. It delegates the functionality needed to create the runners to external executables. These executables can be either binaries or scripts. As long as they adhere to the needed interface, they can be used to create runners in any target IaaS. This is identical to what ```containerd``` does with ```CNIs```.

There are currently two sample external providers available in the [contrib folder of this repository](../contrib/providers.d/). The providers are written in ```bash``` and are meant as examples of how a provider could be written in ```bash```. Production ready providers would need more error checking and idempotency, but they serve as an example of what can be done. As it stands, they are functional.

The configuration for an external provider is quite simple:

```toml
# This is an example external provider. External providers are executables that
# implement the needed interface to create/delete/list compute systems that are used
# by GARM to create runners.
[[provider]]
name = "openstack_external"
description = "external openstack provider"
provider_type = "external"
  [provider.external]
  # config file passed to the executable via GARM_PROVIDER_CONFIG_FILE environment variable
  config_file = "/etc/garm/providers.d/openstack/keystonerc"
  # Absolute path to an executable that implements the provider logic. This executable can be
  # anything (bash, a binary, python, etc). See documentation in this repo on how to write an
  # external provider.
  provider_executable = "/etc/garm/providers.d/openstack/garm-external-provider"
```

The external provider has two options:

* ```provider_executable```
* ```config_file```

The ```provider_executable``` option is the absolute path to an executable that implements the provider logic. GARM will delegate all provider operations to this executable. This executable can be anything (bash, python, perl, go, etc). See [Writing an external provider](./external_provider.md) for more details.

The ```config_file``` option is a path on disk to an arbitrary file, that is passed to the external executable via the environment variable ```GARM_PROVIDER_CONFIG_FILE```. This file is only relevant to the external provider. GARM itself does not read it. In the case of the sample OpenStack provider, this file contains access information for an OpenStack cloud (what you would typically find in a ```keystonerc``` file) as well as some provider specific options like whether or not to boot from volume and which tenant network to use. You can check out the [sample config file](../contrib/providers.d/openstack/keystonerc) in this repository.

If you want to implement an external provider, you can use this file for anything you need to pass into the binary when ```GARM``` calls it to execute a particular operation.

### Available external providers

For non testing purposes, there are two external providers currently available:

* [OpenStack](https://github.com/cloudbase/garm-provider-openstack)
* [Azure](https://github.com/cloudbase/garm-provider-azure)
* [Kubernetes](https://github.com/mercedes-benz/garm-provider-k8s) - Thanks to the amazing folks at @mercedes-benz for sharing their awesome provider!
* [LXD](https://github.com/cloudbase/garm-provider-lxd)
* [Incus](https://github.com/cloudbase/garm-provider-incus)
* [Equinix Metal](https://github.com/cloudbase/garm-provider-equinix)
* [Amazon EC2](https://github.com/cloudbase/garm-provider-aws)
* [Google Cloud Platform (GCP)](https://github.com/cloudbase/garm-provider-gcp)

Details on how to install and configure them are available in their respective repositories.

If you wrote a provider and would like to add it to the above list, feel free to open a PR.
