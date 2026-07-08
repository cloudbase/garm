# Providers

Providers are external executables that GARM calls to create and manage runner instances in a target infrastructure. GARM delegates all instance lifecycle operations (create, delete, start, stop, list) to the provider binary.

## Supported providers

| Provider | Repository | Notes |
| ---------- | ----------- | ------- |
| **Akamai/Linode** | [flatcar/garm-provider-linode](https://github.com/flatcar/garm-provider-linode) | Experimental |
| **Amazon EC2** | [cloudbase/garm-provider-aws](https://github.com/cloudbase/garm-provider-aws) | |
| **Azure** | [cloudbase/garm-provider-azure](https://github.com/cloudbase/garm-provider-azure) | |
| **CloudStack** | [nexthop-ai/garm-provider-cloudstack](https://github.com/nexthop-ai/garm-provider-cloudstack) | |
| **GCP** | [cloudbase/garm-provider-gcp](https://github.com/cloudbase/garm-provider-gcp) | |
| **Incus** | [cloudbase/garm-provider-incus](https://github.com/cloudbase/garm-provider-incus) | Fork of LXD |
| **Kubernetes** | [mercedes-benz/garm-provider-k8s](https://github.com/mercedes-benz/garm-provider-k8s) | By Mercedes-Benz |
| **LXD** | [cloudbase/garm-provider-lxd](https://github.com/cloudbase/garm-provider-lxd) | Easiest to get started |
| **OpenStack** | [cloudbase/garm-provider-openstack](https://github.com/cloudbase/garm-provider-openstack) | |
| **Oracle OCI** | [cloudbase/garm-provider-oci](https://github.com/cloudbase/garm-provider-oci) | |

The GARM Docker image includes pre-built binaries for all providers in `/opt/garm/providers.d/`.

## Configuring a provider

Add a `[[provider]]` section to `config.toml`:

```toml
[[provider]]
  name = "lxd_local"
  provider_type = "external"
  description = "Local LXD installation"
  [provider.external]
    provider_executable = "/opt/garm/providers.d/garm-provider-lxd"
    config_file = "/etc/garm/garm-provider-lxd.toml"
```

You can configure **multiple providers** to offer different infrastructure options. Each pool is tied to one provider.

### Environment variable passthrough

By default, GARM passes a clean environment to providers, consisting only of the variables defined by the [provider interface](https://github.com/cloudbase/garm/blob/main/doc/external_provider.md). This is intentional -- providers should be self-contained and not depend on the host environment.

However, some providers need access to host environment variables for authentication. For example, the AWS provider may need `AWS_*` variables for IAM role-based authentication (EC2 instance profiles), and the Azure provider may need variables for managed identity. Use `environment_variables` to pass these through:

```toml
[provider.external]
  provider_executable = "/opt/garm/providers.d/garm-provider-aws"
  config_file = "/etc/garm/garm-provider-aws.toml"
  environment_variables = ["AWS_"]
```

This passes all environment variables starting with `AWS_` to the provider. You can also specify exact variable names (e.g., `["AZURE_CLIENT_ID", "AZURE_TENANT_ID"]`).

## Bounding provider execution time

Instance creation is always bounded by the pool or scale set's `runner_bootstrap_timeout`. If the provider binary does not return within that window, it is killed, the instance is marked as `error` and it is cleaned up through the normal lifecycle. Past that point the runner would be considered failed anyway, so there is no reason to keep waiting on a hung create.

All other provider operations (delete, list, etc.) run without a deadline by default. You can bound every invocation of a provider binary with `exec_timeout_seconds`:

```toml
[provider.external]
  provider_executable = "/opt/garm/providers.d/garm-provider-aws"
  config_file = "/etc/garm/garm-provider-aws.toml"
  exec_timeout_seconds = 900
```

When the timeout is exceeded, the binary is killed and the operation fails.

> [!IMPORTANT]
> For instance creation, the effective deadline is the smaller of `exec_timeout_seconds` and the pool or scale set's `runner_bootstrap_timeout`. If you set `exec_timeout_seconds`, keep it below your bootstrap timeouts — a create that is allowed to outlive the bootstrap timeout only produces a runner that is already considered failed. A value of `0` (the default) leaves non-create operations unbounded.

## Listing configured providers

```bash
garm-cli provider list
```

## Using multiple providers

You can create pools on different providers for the same repository. This is useful for:

- **Cost optimization:** Use cheap on-prem LXD for most jobs, overflow to cloud
- **Capability matching:** GPU workloads on specific providers, regular jobs on others
- **Multi-cloud resilience:** Spread across providers for availability

```bash
# On-prem pool with high priority
garm-cli pool add --repo <ID> --provider-name lxd_local --priority=10 --tags ubuntu ...

# Cloud overflow pool with lower priority
garm-cli pool add --repo <ID> --provider-name aws_ec2 --priority=1 --tags ubuntu ...
```

With `pack` balancing, GARM fills the high-priority LXD pool first and only creates cloud runners when LXD is full.

## Building a custom provider

Providers are executables that respond to GARM commands passed via the `GARM_COMMAND` environment variable. For details on building your own provider, see the [External Provider Interface](https://github.com/cloudbase/garm/blob/main/doc/external_provider.md) documentation.
