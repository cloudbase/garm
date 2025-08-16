
<p align="center">
    <img src="doc/images/garm-light.svg#gh-light-mode-only" width="384px" alt="Light mode image" />
    <img src="doc/images/garm-dark.svg#gh-dark-mode-only" width="384px" alt="Dark mode image" />
</p>

# GitHub Actions Runner Manager (GARM)

[![Go Tests](https://github.com/cloudbase/garm/actions/workflows/go-tests.yml/badge.svg)](https://github.com/cloudbase/garm/actions/workflows/go-tests.yml)

<!-- TOC -->

- [GitHub Actions Runner Manager GARM](#github-actions-runner-manager-garm)
    - [About GARM](#about-garm)
    - [Join us on slack](#join-us-on-slack)
    - [Installing](#installing)
        - [Quickstart](#quickstart)
        - [Installing on Kubernetes](#installing-on-kubernetes)
    - [Configuring GARM for GHES](#configuring-garm-for-ghes)
    - [Configuring GARM for Gitea](#configuring-garm-for-gitea)
    - [Enabling the web UI](#enabling-the-web-ui)
    - [Using GARM](#using-garm)
    - [Supported providers](#supported-providers)
        - [Installing external providers](#installing-external-providers)
    - [Optimizing your runners](#optimizing-your-runners)
    - [Write your own provider](#write-your-own-provider)

<!-- /TOC -->

## About GARM

Welcome to GARM!

GARM enables you to create and automatically maintain pools of self-hosted runners in both [Github](https://docs.github.com/en/actions/hosting-your-own-runners/about-self-hosted-runners) and [Gitea](https://github.com/go-gitea/gitea/) with auto-scaling that can be used inside your workflow runs.

The goal of ```GARM``` is to be simple to set up, simple to configure and simple to use. The server itself is a single binary that can run on any GNU/Linux machine without any other requirements other than the providers you want to enable in your setup. It is intended to be easy to deploy in any environment and can create runners in virtually any system you can write a provider for (if one does not alreay exist). There is no complicated setup process and no extremely complex concepts to understand. Once set up, it's meant to stay out of your way.

Through the use of providers, `GARM` can create runners in a variety of environments using the same `GARM` instance. Whether you want to create runners in your OpenStack cloud, your Azure cloud or your Kubernetes cluster, that is easily achieved by installing the appropriate providers, configuring them in `GARM` and creating pools that use them. You can create zero-runner pools for instances with high costs (large VMs, GPU enabled instances, etc) and have them spin up on demand, or you can create large pools of eagerly created k8s backed runners that can be used for your CI/CD pipelines at a moment's notice. You can mix them up and create pools in any combination of providers or resource allocations you want.

GARM supports two modes of operation:

* Pools
* Scale sets

Here is a brief architectural diagram of how pools work and how GARM reacts to workflows triggered in GitHub (click the image to see a larger version):

![GARM architecture diagram](/doc/images/garm-light.diagram.svg?raw=true#gh-light-mode-only)
![GARM architecture diagram](/doc/images/garm-dark.diagram.svg?raw=true#gh-dark-mode-only)

**Scale sets** work differently. While pools (as they are defined in GARM) rely on webhooks to know when a job was started and GARM needs to internally make the right decission in terms of which pool should handle that runner, scale sets have a lot of the scheduling and decission making logic done in GitHub itself.

> [!IMPORTANT]
> The README and documentation in the `main` branch are relevant to the not yet released code that is present in `main`. Following the documentation from the `main` branch for a stable release of GARM, may lead to errors. To view the documentation for the latest stable release, please switch to the appropriate tag. For information about setting up `v0.1.6`, please refer to the [v0.1.6 tag](https://github.com/cloudbase/garm/tree/v0.1.6).

> [!CAUTION]
> The `main` branch holds the latest code and is not guaranteed to be stable. If you are looking for a stable release, please check the releases page. If you plan to use the `main` branch, please do so on a new instance. Do not upgrade from a stable release to `main`.

## Join us on slack

Whether you're running into issues or just want to drop by and say "hi", feel free to [join us on slack](https://communityinviter.com/apps/garm-hq/garm).

[![slack](https://img.shields.io/badge/slack-garm-brightgreen.svg?logo=slack)](https://communityinviter.com/apps/garm-hq/garm)

## Installing

### Quickstart

Check out the [quickstart](/doc/quickstart.md) document for instructions on how to install ```GARM```. If you'd like to build from source, check out the [building from source](/doc/building_from_source.md) document.

### Installing on Kubernetes

Thanks to the efforts of the amazing folks at [@mercedes-benz](https://github.com/mercedes-benz/), GARM can now be integrated into k8s via their operator. Check out the [GARM operator](https://github.com/mercedes-benz/garm-operator/) for more details.

## Configuring GARM for GHES

GARM supports creating pools and scale sets in either GitHub itself or in your own deployment of [GitHub Enterprise Server](https://docs.github.com/en/enterprise-server@3.10/admin/overview/about-github-enterprise-server). For instructions on how to use ```GARM``` with GHE, see the [credentials](/doc/github_credentials.md) section of the documentation.

## Configuring GARM for Gitea

GARM now has support for Gitea (>=1.24.0). For information on getting started with Gitea, see the [Gitea quickstart](/doc/gitea.md) document.

## Enabling the web UI

GARM now ships with a single page application. To enable it, add the following to your GARM config:

```toml
[apiserver.webui]
  enable = true
```

Check the [README.md](/webapp/README.md) file for details on the web UI.

## Using GARM

GARM is designed with simplicity in mind. At least we try to keep it as simple as possible. We're aware that adding a new tool in your workflow can be painful, especially when you already have to deal with so many. The cognitive load for OPS has reached a level where it feels overwhelming at times to even wrap your head around a new tool. As such, we believe that tools should be simple, should take no more than a few hours to understand and set up and if you absolutely need to interact with the tool, it should be as intuitive as possible. Although we try our best to make this happen, we're aware that GARM has some rough edges, especially for new users. If you encounter issues or feel like the setup process was too complicated, please let us know. We're always looking to improve the user experience.

We've written a short introduction into some of the commands that GARM has and some of the concepts involved in setting up GARM, managing runners and how GitHub does some of the things it does.

[You can find it here](/doc/using_garm.md).

Please, feel free to [open an issue](https://github.com/cloudbase/garm/issues/new) if you find the documentation lacking and would like more info. Sometimes we forget the challenges that new users face as we're so close to the code and how it works. Any feedback is welcome and we're always looking to improve the documentation.

## Supported providers

GARM uses providers to create runners in a particular IaaS. The providers are external executables that GARM calls into to create runners. Before you can create runners, you'll need to install at least one provider.

### Installing external providers

External providers are binaries that GARM calls into to create runners in a particular IaaS. There are several external providers available:

* [Akamai/Linode](https://github.com/flatcar/garm-provider-linode) - Experimental
* [Amazon EC2](https://github.com/cloudbase/garm-provider-aws)
* [Azure](https://github.com/cloudbase/garm-provider-azure)
* [Equinix Metal](https://github.com/cloudbase/garm-provider-equinix)
* [Google Cloud Platform (GCP)](https://github.com/cloudbase/garm-provider-gcp)
* [Incus](https://github.com/cloudbase/garm-provider-incus)
* [Kubernetes](https://github.com/mercedes-benz/garm-provider-k8s) - Thanks to the amazing folks at @mercedes-benz for sharing their awesome provider!
* [LXD](https://github.com/cloudbase/garm-provider-lxd)
* [OpenStack](https://github.com/cloudbase/garm-provider-openstack)
* [Oracle Cloud Infrastructure (OCI)](https://github.com/cloudbase/garm-provider-oci)

Follow the instructions in the README of each provider to install them. 

## Optimizing your runners

If you would like to optimize the startup time of new instance, take a look at the [performance considerations](/doc/performance_considerations.md) page.

## Write your own provider

The providers are interfaces between ```GARM``` and a particular IaaS in which we spin up GitHub Runners. **External** providers can be written in any language, as they are in the form of an external executable that ```GARM``` calls into. Please see the [Writing an external provider](/doc/external_provider.md) document for details. Also, feel free to inspect the two available sample external providers in this repository.
