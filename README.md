# GitHub Actions Runner Manager (GARM)

[![Go Tests](https://github.com/cloudbase/garm/actions/workflows/go-tests.yml/badge.svg)](https://github.com/cloudbase/garm/actions/workflows/go-tests.yml)

Welcome to GARM!

GARM enables you to create and automatically maintain pools of [self-hosted GitHub runners](https://docs.github.com/en/actions/hosting-your-own-runners/about-self-hosted-runners), with auto-scaling that can be used inside your github workflow runs.

The goal of ```GARM``` is to be simple to set up, simple to configure and simple to use. It is a single binary that can run on any GNU/Linux machine without any other requirements other than the providers it creates the runners in. It is intended to be easy to deploy in any environment and can create runners in any system you can write a provider for. There is no complicated setup process and no extremely complex concepts to understand. Once set up, it's meant to stay out of your way.

GARM supports creating pools on either GitHub itself or on your own deployment of [GitHub Enterprise Server](https://docs.github.com/en/enterprise-server@3.5/admin/overview/about-github-enterprise-server). For instructions on how to use ```GARM``` with GHE, see the [credentials](/doc/github_credentials.md) section of the documentation.

Through the use of providers, `GARM` can create runners in a variety of environments using the same `GARM` instance. Want to create pools of runners in your OpenStack cloud, your Azure cloud and your Kubernetes cluster? No problem! Just install the appropriate providers, configure them in `GARM` and you're good to go. Create zero-runner pools for instances with high costs (large VMs, GPU enabled instances, etc) and have them spin up on demand, or create large pools of k8s backed runners that can be used for your CI/CD pipelines at a moment's notice. You can mix them up and create pools in any combination of providers or resource allocations you want.

## Join us on slack

Whether you're running into issues or just want to drop by and say "hi", feel free to [join us on slack](https://communityinviter.com/apps/garm-hq/garm).

[![slack](https://img.shields.io/badge/slack-garm-brightgreen.svg?logo=slack)](https://communityinviter.com/apps/garm-hq/garm)

## Installing

### On virtual or physical machines

Check out the [quickstart](/doc/quickstart.md) document for instructions on how to install ```GARM```. If you'd like to build from source, check out the [building from source](/doc/building_from_source.md) document.

### On Kubernetes

Thanks to the efforts of the amazing folks at @mercedes-benz, GARM can now be integrated into k8s via their operator. Check out the [GARM operator](https://github.com/mercedes-benz/garm-operator/) for more details.

## Supported providers

GARM uses providers to create runners in a particular IaaS. The providers are external executables that GARM calls into to create runners. Before you can create runners, you'll need to install at least one provider.

## Installing external providers

External providers are binaries that GARM calls into to create runners in a particular IaaS. There are several external providers available:

* [OpenStack](https://github.com/cloudbase/garm-provider-openstack)
* [Azure](https://github.com/cloudbase/garm-provider-azure)
* [Kubernetes](https://github.com/mercedes-benz/garm-provider-k8s) - Thanks to the amazing folks at @mercedes-benz for sharing their awesome provider!
* [LXD](https://github.com/cloudbase/garm-provider-lxd)
* [Incus](https://github.com/cloudbase/garm-provider-incus)
* [Equinix Metal](https://github.com/cloudbase/garm-provider-equinix)
* [Amazon EC2](https://github.com/cloudbase/garm-provider-aws)

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

The providers are interfaces between ```GARM``` and a particular IaaS in which we spin up GitHub Runners. **External** providers can be written in any language, as they are in the form of an external executable that ```GARM``` calls into. Please see the [Writing an external provider](/doc/external_provider.md) document for details. Also, feel free to inspect the two available sample external providers in this repository.
