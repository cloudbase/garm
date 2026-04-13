
<p align="center">
    <img src="doc/images/garm-light.svg#gh-light-mode-only" width="384px" alt="Light mode image" />
    <img src="doc/images/garm-dark.svg#gh-dark-mode-only" width="384px" alt="Dark mode image" />
</p>

# GitHub Actions Runner Manager (GARM)

[![Go Tests](https://github.com/cloudbase/garm/actions/workflows/go-tests.yml/badge.svg)](https://github.com/cloudbase/garm/actions/workflows/go-tests.yml)
[![slack](https://img.shields.io/badge/slack-garm-brightgreen.svg?logo=slack)](https://communityinviter.com/apps/garm-hq/garm)

GARM is an open-source, self-hosted runner manager for [GitHub Actions](https://docs.github.com/en/actions/hosting-your-own-runners/about-self-hosted-runners) and [Gitea Actions](https://github.com/go-gitea/gitea/). It automatically creates, scales, and destroys ephemeral runner instances across multiple clouds and infrastructure providers from a single controller.

## Highlights

- **Multi-cloud from a single controller** -- manage runners across AWS, Azure, GCP, OpenStack, OCI, LXD, Incus, Kubernetes, and more, all from one GARM instance. Mix and match providers freely.
- **GitHub.com, GitHub Enterprise Server, and Gitea** -- first-class support for all three forges.
- **Pools and Scale Sets** -- webhook-driven pools with configurable balancing (round-robin or bin-packing), plus native GitHub Actions Runner Scale Sets.
- **Scale to zero** -- create on-demand pools that only spin up runners when jobs are queued.
- **Pluggable provider architecture** -- providers are standalone executables. Use the [10+ existing providers](#supported-providers) or [write your own](#write-your-own-provider) in any language.
- **Single binary, minimal dependencies** -- no external database server, no message broker. GARM ships as one binary with an embedded SQLite database.
- **Built-in web UI** -- manage runners, pools, credentials, and endpoints from the browser.
- **Kubernetes operator** -- production-grade k8s integration via the [GARM operator](https://github.com/mercedes-benz/garm-operator/) by [@mercedes-benz](https://github.com/mercedes-benz/).

## Architecture

GARM supports two scaling modes:

**Pools** receive `workflow_job` webhooks from GitHub/Gitea, match jobs to pools by label, and create runners on demand. When multiple pools match, a configurable balancer (round-robin or pack) decides which pool handles the job.

**Scale Sets** use GitHub's native message queue. GitHub handles scheduling; GARM handles provisioning.

<details>
<summary>Architecture diagram (pools)</summary>

![GARM architecture diagram](/doc/images/garm-light.diagram.svg?raw=true#gh-light-mode-only)
![GARM architecture diagram](/doc/images/garm-dark.diagram.svg?raw=true#gh-dark-mode-only)

</details>

> [!IMPORTANT]
> The README and documentation in the `main` branch are relevant to the not yet released code that is present in `main`. Following the documentation from the `main` branch for a stable release of GARM, may lead to errors. To view the documentation for the latest stable release, please switch to the appropriate tag. For information about setting up `v0.2.0-beta1`, please refer to the [v0.2.0-beta1 tag](https://github.com/cloudbase/garm/tree/v0.2.0-beta1).

> [!CAUTION]
> The `main` branch holds the latest code and is not guaranteed to be stable. If you are looking for a stable release, please check the releases page. If you plan to use the `main` branch, please do so on a new instance. Do not upgrade from a stable release to `main`.

## Getting started

Pick the quickstart that matches your setup:

- **[Quickstart: Docker](/doc/quickstart-docker.md)** -- deploy GARM as a Docker container (simplest)
- **[Quickstart: Systemd](/doc/quickstart-systemd.md)** -- deploy GARM as a native Linux service
- **[First Steps](/doc/first-steps.md)** -- add credentials, a repository, and your first runner pool

For Kubernetes deployments, see the [GARM operator](https://github.com/mercedes-benz/garm-operator/). To build from source, see [Building from Source](/doc/building-from-source.md).

## Documentation

Full documentation lives in the [doc/](/doc/README.md) directory:

| Section | Guides |
|---------|--------|
| **Setup** | [Credentials](/doc/credentials.md) &middot; [Configuration](/doc/configuration.md) &middot; [Webhooks](/doc/webhooks.md) |
| **Usage** | [Managing Entities](/doc/managing-entities.md) &middot; [Pools and Scaling](/doc/pools-and-scaling.md) &middot; [Scale Sets](/doc/scale-sets.md) |
| **Advanced** | [Templates](/doc/templates.md) &middot; [Providers](/doc/providers.md) &middot; [Gitea](/doc/gitea.md) &middot; [Agent and Object Store](/doc/agent-and-object-store.md) |
| **Operations** | [Monitoring](/doc/monitoring.md) &middot; [Performance](/doc/performance.md) &middot; [FAQ](/doc/faq.md) |

If you find the documentation lacking, please [open an issue](https://github.com/cloudbase/garm/issues/new). Feedback from new users is especially valuable.

## Supported providers

GARM uses external providers to create runners in a particular IaaS. Providers are standalone executables that GARM calls to manage runner instances.

| Provider | Repository |
|----------|------------|
| Akamai/Linode | [flatcar/garm-provider-linode](https://github.com/flatcar/garm-provider-linode) (experimental) |
| Amazon EC2 | [cloudbase/garm-provider-aws](https://github.com/cloudbase/garm-provider-aws) |
| Azure | [cloudbase/garm-provider-azure](https://github.com/cloudbase/garm-provider-azure) |
| CloudStack | [nexthop-ai/garm-provider-cloudstack](https://github.com/nexthop-ai/garm-provider-cloudstack) |
| GCP | [cloudbase/garm-provider-gcp](https://github.com/cloudbase/garm-provider-gcp) |
| Incus | [cloudbase/garm-provider-incus](https://github.com/cloudbase/garm-provider-incus) |
| Kubernetes | [mercedes-benz/garm-provider-k8s](https://github.com/mercedes-benz/garm-provider-k8s) |
| LXD | [cloudbase/garm-provider-lxd](https://github.com/cloudbase/garm-provider-lxd) |
| OpenStack | [cloudbase/garm-provider-openstack](https://github.com/cloudbase/garm-provider-openstack) |
| Oracle OCI | [cloudbase/garm-provider-oci](https://github.com/cloudbase/garm-provider-oci) |

Follow the instructions in each provider's README to install them.

## Write your own provider

Providers are external executables that GARM calls to manage runner lifecycle in a given IaaS. They can be written in any language. See [Writing an external provider](/doc/external_provider.md) for details.

## Community

Whether you're running into issues or just want to drop by and say "hi", feel free to [join us on Slack](https://communityinviter.com/apps/garm-hq/garm).
