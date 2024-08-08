# GitHub Actions Runner Manager (GARM)

[![Go Tests](https://github.com/cloudbase/garm/actions/workflows/go-tests.yml/badge.svg)](https://github.com/cloudbase/garm/actions/workflows/go-tests.yml)

<!-- TOC -->

- [About GARM](#about-garm)
- [Join us on slack](#join-us-on-slack)
- [Installing](#installing)
    - [Quickstart](#quickstart)
    - [Installing on Kubernetes](#installing-on-kubernetes)
- [Using GARM](#using-garm)
- [Supported providers](#supported-providers)
    - [Installing external providers](#installing-external-providers)
- [Optimizing your runners](#optimizing-your-runners)
- [Write your own provider](#write-your-own-provider)

<!-- /TOC -->

## About GARM

Welcome to GARM!

GARM enables you to create and automatically maintain pools of [self-hosted GitHub runners](https://docs.github.com/en/actions/hosting-your-own-runners/about-self-hosted-runners), with auto-scaling that can be used inside your github workflow runs.

The goal of ```GARM``` is to be simple to set up, simple to configure and simple to use. The server itself is a single binary that can run on any GNU/Linux machine without any other requirements other than the providers you want to enable in your setup. It is intended to be easy to deploy in any environment and can create runners in virtually any system you can write a provider for. There is no complicated setup process and no extremely complex concepts to understand. Once set up, it's meant to stay out of your way.

GARM supports creating pools in either GitHub itself or in your own deployment of [GitHub Enterprise Server](https://docs.github.com/en/enterprise-server@3.10/admin/overview/about-github-enterprise-server). For instructions on how to use ```GARM``` with GHE, see the [credentials](/doc/github_credentials.md) section of the documentation.

Through the use of providers, `GARM` can create runners in a variety of environments using the same `GARM` instance. Whether you want to create pools of runners in your OpenStack cloud, your Azure cloud or your Kubernetes cluster, that is easily achieved by just installing the appropriate providers, configuring them in `GARM` and creating pools that use them. You can create zero-runner pools for instances with high costs (large VMs, GPU enabled instances, etc) and have them spin up on demand, or you can create large pools of eagerly created k8s backed runners that can be used for your CI/CD pipelines at a moment's notice. You can mix them up and create pools in any combination of providers or resource allocations you want.

Here is a brief architectural diagram of how GARM reacts to workflows triggered in GitHub (click the image to see a larger version):

![GARM architecture diagram](/doc/images/garm-light.drawio.svg?raw=true#gh-light-mode-only)
![GARM architecture diagram](/doc/images/garm-dark.drawio.svg?raw=true#gh-dark-mode-only)

:warning: **Important note**: The README and documentation in the `main` branch are relevant to the not yet released code that is present in `main`. Following the documentation from the `main` branch for a stable release of GARM, may lead to errors. To view the documentation for the latest stable release, please switch to the appropriate tag. For information about setting up `v0.1.5`, please refer to the [v0.1.5 tag](https://github.com/cloudbase/garm/tree/v0.1.5).

## Join us on slack

Whether you're running into issues or just want to drop by and say "hi", feel free to [join us on slack](https://communityinviter.com/apps/garm-hq/garm).

[![slack](https://img.shields.io/badge/slack-garm-brightgreen.svg?logo=slack)](https://communityinviter.com/apps/garm-hq/garm)

## Installing

### Quickstart

Check out the [quickstart](/doc/quickstart.md) document for instructions on how to install ```GARM```. If you'd like to build from source, check out the [building from source](/doc/building_from_source.md) document.

### Installing on Kubernetes

Thanks to the efforts of the amazing folks at [@mercedes-benz](https://github.com/mercedes-benz/), GARM can now be integrated into k8s via their operator. Check out the [GARM operator](https://github.com/mercedes-benz/garm-operator/) for more details.

## Using GARM

GARM is designed with simplicity in mind. At least we try to keep it as simple as possible. We're aware that adding a new tool in your workflow can be painful, especially when you already have to deal with so many. The cognitive load for OPS has reached a level where it feels overwhelming at times to even wrap your head around a new tool. As such, we believe that tools should be simple, should take no more than a few hours to understand and set up and if you absolutely need to interact with the tool, it should be as intuitive as possible. Although we try our best to make this happen, we're aware that GARM has some rough edges, especially for new users. If you encounter issues or feel like the setup process was too complicated, please let us know. We're always looking to improve the user experience.

We've written a short introduction into some of the commands that GARM has and some of the concepts involved in setting up GARM, managing runners and how GitHub does some of the things it does.

[You can find it here](/doc/using_garm.md).

Please, feel free to [open an issue](https://github.com/cloudbase/garm/issues/new) if you find the documentation lacking and would like more info. Sometimes we forget the challenges that new users face as we're so close to the code and how it works. Any feedback is welcome and we're always looking to improve the documentation.

## Supported providers

GARM uses providers to create runners in a particular IaaS. The providers are external executables that GARM calls into to create runners. Before you can create runners, you'll need to install at least one provider.

### Installing external providers

External providers are binaries that GARM calls into to create runners in a particular IaaS. There are several external providers available:

* [OpenStack](https://github.com/cloudbase/garm-provider-openstack)
* [Azure](https://github.com/cloudbase/garm-provider-azure)
* [Kubernetes](https://github.com/mercedes-benz/garm-provider-k8s) - Thanks to the amazing folks at @mercedes-benz for sharing their awesome provider!
* [LXD](https://github.com/cloudbase/garm-provider-lxd)
* [Incus](https://github.com/cloudbase/garm-provider-incus)
* [Equinix Metal](https://github.com/cloudbase/garm-provider-equinix)
* [Amazon EC2](https://github.com/cloudbase/garm-provider-aws)
* [Google Cloud Platform (GCP)](https://github.com/cloudbase/garm-provider-gcp)
* [Oracle Cloud Infrastructure (OCI)](https://github.com/cloudbase/garm-provider-oci)

Follow the instructions in the README of each provider to install them. 

## Optimizing your runners

If you would like to optimize the startup time of new instance, take a look at the [performance considerations](/doc/performance_considerations.md) page.

## Write your own provider

The providers are interfaces between ```GARM``` and a particular IaaS in which we spin up GitHub Runners. **External** providers can be written in any language, as they are in the form of an external executable that ```GARM``` calls into. Please see the [Writing an external provider](/doc/external_provider.md) document for details. Also, feel free to inspect the two available sample external providers in this repository.
