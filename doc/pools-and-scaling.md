# Pools and Scaling

Pools are the core of GARM's runner management. Each pool defines a set of runners with the same configuration: provider, image, flavor, labels, and scaling parameters.

<!-- TOC -->

- [Pools and Scaling](#pools-and-scaling)
    - [Creating a pool](#creating-a-pool)
        - [Pool options](#pool-options)
    - [Listing pools](#listing-pools)
    - [Showing pool details](#showing-pool-details)
    - [Updating a pool](#updating-a-pool)
    - [Deleting a pool](#deleting-a-pool)
    - [Scaling behavior](#scaling-behavior)
        - [On-demand scaling min-idle-runners = 0](#on-demand-scaling-min-idle-runners--0)
        - [Warm pool min-idle-runners > 0](#warm-pool-min-idle-runners--0)
        - [Job age backoff](#job-age-backoff)
    - [Pool balancing](#pool-balancing)
        - [Pool priority](#pool-priority)
    - [Labels and job matching](#labels-and-job-matching)
        - [Default labels](#default-labels)
    - [Extra specs](#extra-specs)
    - [Runners](#runners)
        - [List runners](#list-runners)
        - [Show runner details](#show-runner-details)
        - [Delete a runner](#delete-a-runner)

<!-- /TOC -->

## Creating a pool

```bash
garm-cli pool add \
  --repo <REPO_ID> \
  --enabled \
  --provider-name lxd_local \
  --flavor default \
  --image ubuntu:22.04 \
  --max-runners 10 \
  --min-idle-runners 2 \
  --os-arch amd64 \
  --os-type linux \
  --tags ubuntu,generic
```

Pools can be attached to a `--repo`, `--org`, or `--enterprise`.

The `--image` and `--flavor` values are provider-specific. For example, on LXD/Incus `--image` is an image alias (e.g. `ubuntu:22.04`) and `--flavor` maps to an LXD profile that defines CPU, RAM, and disk. On a cloud provider, `--image` might be an AMI ID and `--flavor` an instance type. The `--extra-specs` flag passes provider-specific JSON -- consult each provider's README for available options.

### Pool options

| Option | Default | Description |
|--------|---------|-------------|
| `--provider-name` | (required) | Infrastructure provider to use |
| `--image` | (required) | Provider-specific image (LXD alias, cloud AMI, etc) |
| `--flavor` | (required) | Provider-specific sizing (LXD profile, instance type, etc) |
| `--os-type` | `linux` | `linux` or `windows` |
| `--os-arch` | `amd64` | `amd64` or `arm64` |
| `--tags` | | Comma-separated labels applied to runners |
| `--min-idle-runners` | `0` | Runners kept warm waiting for jobs |
| `--max-runners` | `5` | Maximum runners in this pool |
| `--enabled` | `false` | Whether the pool creates runners |
| `--priority` | `0` | Higher priority pools are preferred by the balancer |
| `--runner-prefix` | `garm` | Prefix for runner names |
| `--runner-bootstrap-timeout` | `20` | Minutes before a runner is considered failed. Increase for slow providers (e.g. bare-metal via OpenStack Ironic). |
| `--runner-install-template` | | Custom bootstrap template name |
| `--runner-group` | | GitHub runner group name |
| `--extra-specs` | | Provider-specific JSON configuration |
| `--enable-shell` | `false` | Enable remote shell access (agent mode) |

## Listing pools

```bash
# All pools
garm-cli pool list

# Pools for a specific entity
garm-cli pool list --repo <REPO_ID>
garm-cli pool list --org <ORG_ID>
garm-cli pool list --enterprise <ENTERPRISE_ID>
```

## Showing pool details

```bash
garm-cli pool show <POOL_ID>
```

## Updating a pool

Nearly every pool setting can be changed after creation:

```bash
# Enable a pool
garm-cli pool update <POOL_ID> --enabled=true

# Change scaling
garm-cli pool update <POOL_ID> --min-idle-runners=3 --max-runners=20

# Change tags
garm-cli pool update <POOL_ID> --tags=ubuntu,large,gpu

# Add extra specs
garm-cli pool update <POOL_ID> --extra-specs='{"disable_updates": true}'
```

## Deleting a pool

Pools must be empty (no runners) before deletion:

```bash
# Step 1: Disable the pool to stop new runners
garm-cli pool update <POOL_ID> --enabled=false

# Step 2: Wait for runners to finish or delete them
garm-cli runner list <POOL_ID>
garm-cli runner delete <RUNNER_NAME>

# Step 3: Delete the pool
garm-cli pool delete <POOL_ID>
```

## Scaling behavior

### On-demand scaling (min-idle-runners = 0)

With `--min-idle-runners=0`, GARM creates runners only in response to queued jobs. This minimizes cost but adds startup latency.

### Warm pool (min-idle-runners > 0)

With `--min-idle-runners=2`, GARM keeps 2 idle runners ready at all times. When a runner picks up a job, GARM immediately creates a replacement to maintain the minimum.

### Job age backoff

GARM waits before reacting to new jobs (default: 30 seconds). This gives existing idle runners time to pick up the job before spinning up a new one.

View and change the backoff:

```bash
# View current setting
garm-cli controller show

# Change it
garm-cli controller update --minimum-job-age-backoff 15
```

Set to `0` for immediate reaction (useful for scale-to-zero setups).

## Pool balancing

When multiple pools match a job's labels, the **pool balancer** decides which pool gets the runner.

Set the balancer when adding the entity:

```bash
garm-cli repo add --pool-balancer-type pack ...
```

| Strategy | Behavior |
|----------|----------|
| `roundrobin` (default) | Distributes runners across pools evenly |
| `pack` | Fills higher-priority pools first, spills to next |

### Pool priority

Higher priority values are preferred. Set via:

```bash
garm-cli pool update <POOL_ID> --priority=10
```

With `pack` balancing and multiple providers, this allows cost optimization: use cheap on-prem pools first, overflow to cloud.

> [!IMPORTANT]
> Pool balancing only applies to pools. For scale sets, GitHub handles scheduling.

## Labels and job matching

Pool tags become runner labels. A job's `runs-on` labels are matched against runners that have **all** the requested labels (superset matching).

```yaml
# Workflow targets runners with BOTH "ubuntu" AND "generic" labels
runs-on: [ubuntu, generic]
```

A pool with `--tags=ubuntu,generic,large` would match this job. A pool with only `--tags=ubuntu` would not.

### Default labels

Before runner version 2.305.0, the registration process automatically appended default labels (`self-hosted`, `$OS_TYPE`, `$OS_ARCH`) to every runner. This caused problems in large organizations where workflows targeting the generic `self-hosted` label would match all runners regardless of other labels, potentially routing expensive jobs to low-resource runners or vice versa.

Since runner version 2.305.0 (and JIT runners on GHES 3.10+), GARM registers runners with the `--no-default-labels` flag, so only the tags you explicitly set via `--tags` are applied. If you still need the old default labels, add them manually:

```bash
--tags=self-hosted,linux,x64,ubuntu,generic
```

## Extra specs

The `--extra-specs` option passes provider-specific JSON configuration. Each provider defines its own supported fields -- consult the provider's documentation. You can also use `--extra-specs-file` to load the JSON from a file, which is easier for complex configurations.

Common examples:

```bash
# Disable OS updates during bootstrap (faster startup)
--extra-specs='{"disable_updates": true}'

# Provider-specific options (vary by provider)
--extra-specs='{"boot_disk_size": 50, "enable_boot_debug": true}'

# Load extra specs from a file
--extra-specs-file=/path/to/extra-specs.json
```

## Runners

### List runners

```bash
# All runners
garm-cli runner list

# Runners in a pool
garm-cli runner list <POOL_ID>

# Runners for an entity
garm-cli runner list --repo <REPO_ID>
```

### Show runner details

```bash
garm-cli runner show <RUNNER_NAME>
```

This shows status updates from the runner's bootstrap process, IP addresses, and current state.

### Delete a runner

```bash
garm-cli runner delete <RUNNER_NAME>
```

Only idle runners can be deleted. If a runner is executing a job, wait for it to finish or cancel the job in GitHub.

Force-delete when the provider is in error:

```bash
garm-cli runner delete --force-remove-runner <RUNNER_NAME>
```

Force-delete when GitHub credentials are invalid:

```bash
garm-cli runner delete --bypass-github-unauthorized <RUNNER_NAME>
```

> **Warning:** `--bypass-github-unauthorized` may leave orphaned runners in GitHub. Update your credentials afterward.
