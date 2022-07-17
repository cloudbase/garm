# Provider configuration

Garm was designed to be extensible. The database layer as well as the providers are defined as interfaces. Currently there are two providers:
  * [LXD](https://linuxcontainers.org/lxd/introduction/)
  * External

LXD is the simplest cloud-like system you can easily set up on any GNU/Linux machine, which enables you to create both containers and Virtual Machines. The ```external``` provider is a special type of provider, which delegates functionality to external executables.

## The LXD provider

Garm leverages the virtual machines feature of LXD to create the runners. Here is a sample config section for an LXD provider:

```toml
# Currently, providers are defined statically in the config. This is due to the fact
# that we have not yet added support for storing secrets in something like Barbican
# or Vault. This will change in the future. However, for now, it's important to remember
# that once you create a pool using one of the providers defined here, the name of that
# provider must not be changes, or the pool will no longer work. Make sure you remove any
# pools before removing or changing a provider.
[[provider]]
  # An arbitrary string describing this provider.
  name = "lxd_local"
  # Provider type. Garm is designed to allow creating providers which are used to spin
  # up compute resources, which in turn will run the github runner software.
  # Currently, LXD is the only supprted provider, but more will be written in the future.
  provider_type = "lxd"
  # A short description of this provider. The name, description and provider types will
  # be included in the information returned by the API when listing available providers.
  description = "Local LXD installation"
  [provider.lxd]
    # the path to the unix socket that LXD is listening on. This works if garm and LXD
    # are on the same system, and this option takes precedence over the "url" option,
    # which connects over the network.
    unix_socket_path = "/var/snap/lxd/common/lxd/unix.socket"
    # When defining a pool for a repository or an organization, you have an option to
    # specify a "flavor". In LXD terms, this translates to "profiles". Profiles allow
    # you to customize your instances (memory, cpu, disks, nics, etc).
    # This option allows you to inject the "default" profile along with the profile selected
    # by the flavor.
    include_default_profile = false
    # enable/disable secure boot. If the image you select for the pool does not have a
    # signed bootloader, set this to false, otherwise your instances won't boot.
    secure_boot = false
    # Project name to use. You can create a separate project in LXD for runners.
    project_name = "default"
    # URL is the address on which LXD listens for connections (ex: https://example.com:8443)
    url = ""
    # garm supports certificate authentication for LXD remote connections. The easiest way
    # to get the needed certificates, is to install the lxc client and add a remote. The
    # client_certificate, client_key and tls_server_certificate can be then fetched from
    # $HOME/snap/lxd/common/config.
    client_certificate = ""
    client_key = ""
    tls_server_certificate = ""
    [provider.lxd.image_remotes]
      # Image remotes are important. These are the default remotes used by lxc. The names
      # of these remotes are important. When specifying an "image" for the pool, that image
      # can be a hash of an existing image on your local LXD installation or it can be a
      # remote image from one of these remotes. You can specify the images as follows:
      # Example:
      #
      #    * ubuntu:20.04
      #    * ubuntu_daily:20.04
      #    * images:centos/8/cloud
      #
      # Ubuntu images come pre-installed with cloud-init which we use to set up the runner
      # automatically and customize the runner. For non Ubuntu images, you need to use the
      # variant that has "/cloud" in the name. Those images come with cloud-init.
      [provider.lxd.image_remotes.ubuntu]
        addr = "https://cloud-images.ubuntu.com/releases"
        public = true
        protocol = "simplestreams"
        skip_verify = false
      [provider.lxd.image_remotes.ubuntu_daily]
        addr = "https://cloud-images.ubuntu.com/daily"
        public = true
        protocol = "simplestreams"
        skip_verify = false
      [provider.lxd.image_remotes.images]
        addr = "https://images.linuxcontainers.org"
        public = true
        protocol = "simplestreams"
        skip_verify = false
```

You can choose to connect to a local LXD server by using the ```unix_socket_path``` option, or you can connect to a remote LXD cluster/server by using the ```url``` option. If both are specified, the unix socket takes precedence. The config file is fairly well commented, but I will add a note about remotes.

### LXD remotes

By default, garm does not load any image remotes. You get to choose which remotes you add (if any). An image remote is a repository of images that LXD uses to create new instances, either virtual machines or containers. In the absence of any remote, garm will attempt to find the image you configure for a pool of runners, on the LXD server we're connecting to. If one is present, it will be used, otherwise it will fail and you will need to configure a remote.

The sample config file in this repository has the usual default ```LXD``` remotes:

  * https://cloud-images.ubuntu.com/releases (ubuntu) - Official Ubuntu images
  * https://cloud-images.ubuntu.com/daily (ubuntu_daily) - Official Ubuntu images, daily build
  * https://images.linuxcontainers.org (images) - Comunity maintained images for various operating systems

When creating a new pool, you'll be able to specify which image you want to use. The images are referenced by ```remote_name:image_tag```. For example, if you want to launch a runner on an Ubuntu 20.04, the image name would be ```ubuntu:20.04```. For a daily image it would be ```ubuntu_daily:20.04```. And for one of the unnoficial images it would be ```images:centos/8-Stream/cloud```. Note, for unofficial images you need to use the tags that have ```/cloud``` in the name. These images come pre-installed with ```cloud-init``` which we need to set up the runners automatically.

You can also create your own image remote, where you can host your own custom images. If you want to build your own images, have a look at [distrobuilder](https://github.com/lxc/distrobuilder).

Image remotes in the ```garm``` config, is a map of strings to remote settins. The name of the remote is the last bit of string in the section header. For example, the following section ```[provider.lxd.image_remotes.ubuntu_daily]```, defines the image remote named **ubuntu_daily**. Use this name to reference images inside that remote.


## The External provider

The external provider is a special kind of provider. It delegates the functionality needed to create the runners to external executables. These executables can be either binaries or scripts. As long as they adhere to the needed interface, they can be used to create runners in any target IaaS. This is identical to what ```containerd``` does with ```CNIs```.

There is currently one external provider for [OpenStack](https://www.openstack.org/) available in the [contrb folder of this repository](../contrib/providers.d/openstack). The provider is written in ```bash``` and it is just a sample. A production ready provider would need more error checking and idempotency, but it serves as an example of what can be done. As it stands, it is functional.

The configuration for an external provider is quite simple:

```toml
# This is an example external provider. External providers are executables that
# implement the needed interface to create/delete/list compute systems that are used
# by garm to create runners.
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

The external provider has three options:

  * ```provider_executable```
  * ```config_file```


The ```provider_executable``` option is the absolute path to an executable that implements the provider logic. Garm will delegate all provider operations to this executable. This executable can be anything (bash, python, perl, go, etc). See [Writing an external provider](./external_provider.md) for more details.

The ```config_file``` option is a path on disk to an arbitrary file, that is passed to the external executable via the environment variable ```GARM_PROVIDER_CONFIG_FILE```. This file is only relevant to the external provider. Garm itself does not read it. In the case of the OpenStack provider, this file contains access information for an OpenStack cloud (what you would tipically find in a ```keystonerc``` file) as well as some provider specific options like whether or not to boot from volume and which tenant network to use. You can check out the [sample config file](../contrib/providers.d/openstack/keystonerc) in this repository.

If you want to implement an external provider, you can use this file for anything you need to pass into the binary when ```garm``` calls it to execute a particular operation.