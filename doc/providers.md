# Provider configuration

GARM was designed to be extensible. Providers can be written as external executables which implement the needed interface to create/delete/list compute systems that are used by ```GARM``` to create runners.

- [Providers](#providers)
    - [Available external providers](#available-external-providers)

## Providers

GARM delegates the functionality needed to create the runners to external executables. These executables can be either binaries or scripts. As long as they adhere to the needed interface, they can be used to create runners in any target IaaS. You might find this behavior familiar if you've ever had to deal with installing `CNIs` in `containerd`. The principle is the same.

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
  # This option will pass all environment variables that start with AWS_ to the provider.
  # To pass in individual variables, you can add the entire name to the list.
  environment_variables = ["AWS_"]
```

The external provider has three options:

* `provider_executable`
* `config_file`
* `environment_variables`

The ```provider_executable``` option is the absolute path to an executable that implements the provider logic. GARM will delegate all provider operations to this executable. This executable can be anything (bash, python, perl, go, etc). See [Writing an external provider](./external_provider.md) for more details.

The ```config_file``` option is a path on disk to an arbitrary file, that is passed to the external executable via the environment variable ```GARM_PROVIDER_CONFIG_FILE```. This file is only relevant to the external provider. GARM itself does not read it. In the case of the sample OpenStack provider, this file contains access information for an OpenStack cloud (what you would typically find in a ```keystonerc``` file) as well as some provider specific options like whether or not to boot from volume and which tenant network to use. You can check out the [sample config file](../contrib/providers.d/openstack/keystonerc) in this repository.

The `environment_variables` option is a list of environment variables that will be passed to the external provider. By default GARM will pass a clean env to providers, consisting only of variables that the [provider interface](./external_provider.md) expects. However, in some situations, provider may need access to certain environment variables set in the env of GARM itself. This might be needed to enable access to IAM roles (ec2) or managed identity (azure). This option takes a list of environment variables or prefixes of environment variables that will be passed to the provider. For example, if you want to pass all environment variables that start with `AWS_` to the provider, you can set this option to `["AWS_"]`. 

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
* [Oracle Cloud Infrastructure (OCI)](https://github.com/cloudbase/garm-provider-oci)

Details on how to install and configure them are available in their respective repositories.

If you wrote a provider and would like to add it to the above list, feel free to open a PR.
