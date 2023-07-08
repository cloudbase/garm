# GitHub Actions Runner Manager (garm)

[![Go Tests](https://github.com/cloudbase/garm/actions/workflows/go-tests.yml/badge.svg)](https://github.com/cloudbase/garm/actions/workflows/go-tests.yml)

Welcome to garm!

Garm enables you to create and automatically maintain pools of [self-hosted GitHub runners](https://docs.github.com/en/actions/hosting-your-own-runners/about-self-hosted-runners), with autoscaling that can be used inside your github workflow runs.

The goal of ```garm``` is to be simple to set up, simple to configure and simple to use. It is a single binary that can run on any GNU/Linux machine without any other requirements other than the providers it creates the runners in. It is intended to be easy to deploy in any environment and can create runners in any system you can write a provider for. There is no complicated setup process and no extremely complex concepts to understand. Once set up, it's meant to stay out of your way.

Garm supports creating pools on either GitHub itself or on your own deployment of [GitHub Enterprise Server](https://docs.github.com/en/enterprise-server@3.5/admin/overview/about-github-enterprise-server). For instructions on how to use ```garm``` with GHE, see the [credentials](/doc/github_credentials.md) section of the documentation.

## Join us on slack

Whether you're running into issues or just want to drop by and say "hi", feel free to [join us on slack](https://communityinviter.com/apps/garm-hq/garm).

[![slack](https://img.shields.io/badge/slack-garm-brightgreen.svg?logo=slack)](https://communityinviter.com/apps/garm-hq/garm)

## Installing

## Build from source

You need to have Go installed, then run:

  ```bash
  go install github.com/cloudbase/garm/cmd/garm@latest
  go install github.com/cloudbase/garm/cmd/garm-cli@latest
  ```

This will install the garm binaries in ```$GOPATH/bin``` folder. Move them somewhere in your ```$PATH``` to make them available system-wide.

If you have docker/podman installed, you can also build statically linked binaries by running:

  ```bash
  git clone https://github.com/cloudbase/garm
  cd garm
  git checkout release/v0.1
  make build-static
  ```

The ```garm``` and ```garm-cli``` binaries will be built and copied to the ```bin/``` folder in your current working directory.

## Install the service

Add a new system user:

  ```bash
  useradd --shell /usr/bin/false \
      --system \
      --groups lxd \
      --no-create-home garm
  ```

The ```lxd``` group is only needed if you have a local LXD install and want to connect to the unix socket to use it. If you're connecting to a remote LXD server over TCP, you can skip adding the ```garm``` user to the ```lxd``` group.

Copy the binary to somewhere in the system ```$PATH```:

  ```bash
  sudo cp $(go env GOPATH)/bin/garm /usr/local/bin/garm
  ```

Or if you built garm using ```make```:

  ```bash
  sudo cp ./bin/garm /usr/local/bin/garm
  ```

Create the config folder:

  ```bash
  sudo mkdir -p /etc/garm
  ```

Copy the config template:

  ```bash
  sudo cp ./testdata/config.toml /etc/garm/
  ```

Copy the systemd service file:

  ```bash
  sudo cp ./contrib/garm.service /etc/systemd/system/
  ```

Change permissions on config folder:

  ```bash
  sudo chown -R garm:garm /etc/garm
  sudo chmod 750 -R /etc/garm
  ```

Enable the service:

  ```bash
  sudo systemctl enable garm
  ```

Customize the config in ```/etc/garm/config.toml```, and start the service:

  ```bash
  sudo systemctl start garm
  ```

## Installing external providers

External providers are binaries that GARM calls into to create runners in a particular IaaS. There are currently two external providers available:

* [OpenStack](https://github.com/cloudbase/garm-provider-openstack)
* [Azure](https://github.com/cloudbase/garm-provider-azure)

Follow the instructions in the README of each provider to install them.

## Configuration

The ```garm``` configuration is a simple ```toml```. The sample config file in [the testdata folder](/testdata/config.toml) is fairly well commented and should be enough to get you started. The configuration file is split into several sections, each of which is documented in its own page. The sections are:

* [The default section](/doc/config_default.md)
* [Metrics](/doc/config_metrics.md)
* [JWT authentication](/doc/config_jwt_auth.md)
* [API server](/doc/config_api_server.md)
* [Github credentials](/doc/github_credentials.md)
* [Providers](/doc/providers.md)
* [Database](/doc/database.md)

Once you've configured your database, providers and github credentials, you'll need to configure your [webhooks and the callback_url](/doc/webhooks_and_callbacks.md).

At this point, you should be done. Have a look at the [running garm document](/doc/running_garm.md) for usage instructions and available features.

If you would like to use ```garm``` with a different IaaS than the ones already available, have a look at the [writing an external provider](/doc/external_provider.md) page.

If you like to optimize the startup time of new instance, take a look at the [performance considerations](/doc/performance_considerations.md) page.

## Write your own provider

The providers are interfaces between ```garm``` and a particular IaaS in which we spin up GitHub Runners. These providers can be either **native** or **external**. The **native** providers are written in ```Go```, and must implement [the interface defined here](https://github.com/cloudbase/garm/blob/main/runner/common/provider.go#L22-L39). **External** providers can be written in any language, as they are in the form of an external executable that ```garm``` calls into.

There is currently one **native** provider for [LXD](https://linuxcontainers.org/lxd/) and two **external** providers for [Openstack and Azure](/contrib/providers.d/).

If you want to write your own provider, you can choose to write a native one, or implement an **external** one. The easiest one to write is probably an **external** provider. Please see the [Writing an external provider](/doc/external_provider.md) document for details. Also, feel free to inspect the two available external providers in this repository.
