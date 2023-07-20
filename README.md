# GitHub Actions Runner Manager (GARM)

[![Go Tests](https://github.com/cloudbase/garm/actions/workflows/go-tests.yml/badge.svg)](https://github.com/cloudbase/garm/actions/workflows/go-tests.yml)

Welcome to GARM!

Garm enables you to create and automatically maintain pools of [self-hosted GitHub runners](https://docs.github.com/en/actions/hosting-your-own-runners/about-self-hosted-runners), with autoscaling that can be used inside your github workflow runs.

The goal of ```GARM``` is to be simple to set up, simple to configure and simple to use. It is a single binary that can run on any GNU/Linux machine without any other requirements other than the providers it creates the runners in. It is intended to be easy to deploy in any environment and can create runners in any system you can write a provider for. There is no complicated setup process and no extremely complex concepts to understand. Once set up, it's meant to stay out of your way.

Garm supports creating pools on either GitHub itself or on your own deployment of [GitHub Enterprise Server](https://docs.github.com/en/enterprise-server@3.5/admin/overview/about-github-enterprise-server). For instructions on how to use ```GARM``` with GHE, see the [credentials](/doc/github_credentials.md) section of the documentation.

## Join us on slack

Whether you're running into issues or just want to drop by and say "hi", feel free to [join us on slack](https://communityinviter.com/apps/garm-hq/garm).

[![slack](https://img.shields.io/badge/slack-garm-brightgreen.svg?logo=slack)](https://communityinviter.com/apps/garm-hq/garm)

## Installing

Check out the [quickstart](/doc/quickstart.md) document for instructions on how to install ```GARM```. If you'd like to build from source, check out the [building from source](/doc/building_from_source.md) document.

## Installing external providers

External providers are binaries that GARM calls into to create runners in a particular IaaS. There are currently two external providers available:

* [OpenStack](https://github.com/cloudbase/garm-provider-openstack)
* [Azure](https://github.com/cloudbase/garm-provider-azure)

Follow the instructions in the README of each provider to install them. 

## Configuration

The ```GARM``` configuration is a simple ```toml```. The sample config file in [the testdata folder](/testdata/config.toml) is fairly well commented and should be enough to get you started. The configuration file is split into several sections, each of which is documented in its own page. The sections are:

* [The default section](/doc/config_default.md)
* [Database](/doc/database.md)
* [Github credentials](/doc/github_credentials.md)
* [Providers](/doc/providers.md)
* [Metrics](/doc/config_metrics.md)
* [JWT authentication](/doc/config_jwt_auth.md)
* [API server](/doc/config_api_server.md)

## Optimizing your runners

If you would like to optimize the startup time of new instance, take a look at the [performance considerations](/doc/performance_considerations.md) page.

## Write your own provider

The providers are interfaces between ```GARM``` and a particular IaaS in which we spin up GitHub Runners. These providers can be either **native** or **external**. The **native** providers are written in ```Go```, and must implement [the interface defined here](https://github.com/cloudbase/garm/blob/main/runner/common/provider.go#L22-L39). **External** providers can be written in any language, as they are in the form of an external executable that ```GARM``` calls into.

There is currently one **native** provider for [LXD](https://linuxcontainers.org/lxd/) and two **external** providers for [Openstack and Azure](/contrib/providers.d/).

If you want to write your own provider, you can choose to write a native one, or implement an **external** one. The easiest one to write is probably an **external** provider. Please see the [Writing an external provider](/doc/external_provider.md) document for details. Also, feel free to inspect the two available external providers in this repository.
