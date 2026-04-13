# GARM Documentation

GARM (GitHub Actions Runner Manager) automatically creates and manages self-hosted GitHub Actions runners across multiple infrastructure providers. It supports GitHub.com, GitHub Enterprise Server, and Gitea.

## Quick start

Get GARM running and create your first runner pool:

1. **[Quickstart: Docker](quickstart-docker.md)** -- Deploy GARM as a Docker container (simplest)
2. **[Quickstart: Systemd](quickstart-systemd.md)** -- Deploy GARM as a native Linux service
3. **[First Steps](first-steps.md)** -- Add credentials, a repository, and your first runner pool

## Guides

| Guide | Description |
|-------|-------------|
| [Credentials](credentials.md) | PATs, GitHub Apps, Gitea tokens, and required permissions |
| [Managing Entities](managing-entities.md) | Repositories, organizations, enterprises, endpoints, and webhooks |
| [Pools and Scaling](pools-and-scaling.md) | Pool configuration, scaling behavior, balancing, labels, and runner management |
| [Scale Sets](scale-sets.md) | GitHub scale sets as an alternative to webhook-driven pools |
| [Templates](templates.md) | Customizing runner bootstrap scripts |
| [Using GARM with Gitea](gitea.md) | Gitea-specific setup and differences from GitHub |
| [Agent Mode and Object Store](agent-and-object-store.md) | WebSocket agent mode, remote shell, file storage |

## Reference

| Page | Description |
|------|-------------|
| [Configuration Reference](configuration.md) | Complete `config.toml` reference |
| [Providers](providers.md) | Supported providers and configuration |
| [Monitoring and Debugging](monitoring.md) | Metrics, log streaming, events, and the `top` dashboard |
| [Performance](performance.md) | Cached runners, image optimization, LXD tuning |
| [Webhooks](webhooks.md) | Automatic and manual webhook setup |
| [Building from Source](building-from-source.md) | Compiling GARM, the Web UI, and regenerating API clients |
| [FAQ](faq.md) | Common questions and answers |

## How it works

```
GitHub/Gitea                         GARM                        Provider (LXD, AWS, etc.)
     |                                |                                |
     |-- webhook: job queued -------->|                                |
     |                                |-- create instance ------------>|
     |                                |                                |-- boots VM/container
     |                                |<-- instance ready -------------|
     |                                |                                |
     |<-- runner registers -----------|                                |
     |                                |                                |
     |-- job runs on runner --------->|                                |
     |                                |                                |
     |-- webhook: job completed ----->|                                |
     |                                |-- delete instance ------------>|
     |                                |                                |
```

1. GitHub sends a `workflow_job` webhook when a job is queued (or GARM polls a scale set message queue)
2. GARM finds a matching pool and asks the provider to create an instance
3. The instance boots, installs the runner, and registers with GitHub
4. The runner picks up the job and executes it
5. When the job completes, GARM deletes the instance

## Key concepts

| Concept | Description |
|---------|-------------|
| **Controller** | A GARM installation, identified by a unique Controller ID |
| **Endpoint** | A GitHub.com, GHES, or Gitea server that GARM connects to |
| **Credential** | A PAT or GitHub App tied to an endpoint, used to manage runners |
| **Entity** | A repository, organization, or enterprise managed by GARM |
| **Provider** | An external executable that creates/destroys infrastructure (LXD, AWS, etc.) |
| **Pool** | A group of runners with the same config (image, flavor, labels, provider) |
| **Scale Set** | An alternative to pools using GitHub's message queue instead of webhooks |
| **Runner** | A self-hosted GitHub/Gitea runner instance |
| **Template** | A script that customizes how runners are bootstrapped |

## Supported providers

| Provider | Repository |
|----------|-----------|
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
