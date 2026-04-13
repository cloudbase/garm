# FAQ

<!-- TOC -->

- [FAQ](#faq)
    - [General](#general)
        - [What is GARM?](#what-is-garm)
        - [Which forges does GARM support?](#which-forges-does-garm-support)
        - [What infrastructure providers are available?](#what-infrastructure-providers-are-available)
        - [What database does GARM use?](#what-database-does-garm-use)
    - [Scaling](#scaling)
        - [Can GARM scale to zero?](#can-garm-scale-to-zero)
        - [What's the difference between min-idle-runners and max-runners?](#whats-the-difference-between-min-idle-runners-and-max-runners)
        - [How does GARM handle burst workloads?](#how-does-garm-handle-burst-workloads)
        - [Can I share pools across multiple repos?](#can-i-share-pools-across-multiple-repos)
        - [How do I update a pool's image without downtime?](#how-do-i-update-a-pools-image-without-downtime)
    - [Pools](#pools)
        - [How do I delete a pool that has runners?](#how-do-i-delete-a-pool-that-has-runners)
        - [What happens if a runner gets stuck?](#what-happens-if-a-runner-gets-stuck)
        - [How does pool balancing work?](#how-does-pool-balancing-work)
    - [Webhooks](#webhooks)
        - [Does GARM need a public webhook endpoint?](#does-garm-need-a-public-webhook-endpoint)
        - [My webhook isn't working. What should I check?](#my-webhook-isnt-working-what-should-i-check)
        - [Can I use GARM with multiple GitHub accounts?](#can-i-use-garm-with-multiple-github-accounts)
    - [GitHub Enterprise Server GHES](#github-enterprise-server-ghes)
        - [I get 404 errors when adding a GHES entity](#i-get-404-errors-when-adding-a-ghes-entity)
        - [Do I need to configure TLS certificates for GHES?](#do-i-need-to-configure-tls-certificates-for-ghes)
    - [Gitea](#gitea)
        - [How do I run Gitea jobs in Docker containers?](#how-do-i-run-gitea-jobs-in-docker-containers)
    - [Performance](#performance)
        - [How can I make runners start faster?](#how-can-i-make-runners-start-faster)
        - [What should I set the bootstrap timeout to?](#what-should-i-set-the-bootstrap-timeout-to)
    - [Security](#security)
        - [Is the GARM API secure?](#is-the-garm-api-secure)
        - [Should I use the Webhook Base URL or Controller Webhook URL?](#should-i-use-the-webhook-base-url-or-controller-webhook-url)
    - [Troubleshooting](#troubleshooting)
        - [GARM creates runners but they don't appear in GitHub](#garm-creates-runners-but-they-dont-appear-in-github)
        - [Jobs are queued but no runners are created](#jobs-are-queued-but-no-runners-are-created)

<!-- /TOC -->



## General

### What is GARM?

GARM (GitHub Actions Runner Manager) is an open-source tool that manages GitHub Actions self-hosted runners. It automatically creates and destroys runner instances in response to workflow jobs, supporting multiple infrastructure providers (LXD, AWS, Azure, GCP, OpenStack, Kubernetes, and more).

### Which forges does GARM support?

- GitHub.com
- GitHub Enterprise Server (GHES)
- Gitea (1.24+)

### What infrastructure providers are available?

Amazon EC2, Azure, CloudStack, GCP, Incus, Kubernetes, LXD, OpenStack, and Oracle OCI. You can also [build your own provider](https://github.com/cloudbase/garm/blob/main/doc/external_provider.md).

### What database does GARM use?

SQLite3 only. The database is a single file on disk, requiring no external database server.

## Scaling

### Can GARM scale to zero?

Yes. Set `--min-idle-runners=0` on your pool. GARM will only create runners in response to queued jobs. Note that this adds startup latency (typically 1-3 minutes depending on your provider and image).

### What's the difference between min-idle-runners and max-runners?

- `min-idle-runners` -- GARM keeps this many idle runners ready at all times. When one picks up a job, a replacement is created immediately.
- `max-runners` -- The upper limit on total runners in the pool/scale set (idle + active). Once reached, no new runners are created until existing ones finish.

### How does GARM handle burst workloads?

When a burst of jobs arrives, GARM creates runners for each queued job (up to `max-runners`). The job age backoff (default 30 seconds) prevents over-provisioning by giving existing idle runners time to pick up jobs first.

### Can I share pools across multiple repos?

Not directly at the pool level. However, you can create an **organization-level** pool, which serves all repositories in that org. This effectively shares runners across repos.

### How do I update a pool's image without downtime?

Update the image while the pool is running:

```bash
garm-cli pool update <POOL_ID> --image=new-image:latest
```

Existing runners continue using the old image until they're replaced. New runners use the updated image.

Optionally, you can recreate outdated idle runners by using the command:

```bash
garm-cli [pool|scaleset] runner rotate --outdated <[pool|scaleset]-id>
```

## Pools

### How do I delete a pool that has runners?

1. Disable the pool: `garm-cli pool update <POOL_ID> --enabled=false`
2. Wait for active runners to finish, or delete idle ones: `garm-cli runner delete <NAME>`
3. Delete the pool: `garm-cli pool delete <POOL_ID>`

### What happens if a runner gets stuck?

GARM has a bootstrap timeout (default 20 minutes). If a runner doesn't register with GitHub within this time, GARM marks it as failed and removes it. You can also force-delete:

```bash
garm-cli runner delete --force-remove-runner <RUNNER_NAME>
```

### How does pool balancing work?

When a job's labels match multiple pools, the **pool balancer** decides which pool to use:

- **roundrobin** (default) -- distributes evenly across matching pools
- **pack** -- fills the highest-priority pool first, overflows to others

Set via `--pool-balancer-type` when adding the entity (repo/org/enterprise).

## Webhooks

### Does GARM need a public webhook endpoint?

Only if using pools (not scale sets). The webhook URL must be reachable by GitHub.com (or your GHES/Gitea server). For scale sets, no webhook is needed -- GARM subscribes to a GitHub message queue instead.

### My webhook isn't working. What should I check?

1. Is the webhook URL reachable from GitHub? Check the webhook delivery status in GitHub settings.
2. Does the webhook secret match? GARM validates every webhook payload.
3. Is "Workflow jobs" selected as the event type?
4. Check GARM logs: `garm-cli debug-log`

### Can I use GARM with multiple GitHub accounts?

Yes. Each GARM controller has a unique Controller ID. Multiple GARM instances can manage runners in the same repos/orgs without conflicts, as each tags runners with its Controller ID.

## GitHub Enterprise Server (GHES)

### I get 404 errors when adding a GHES entity

Check your endpoint URLs. GHES uses different URL patterns than github.com. Ensure `--base-url`, `--api-base-url`, and `--upload-url` are correct for your GHES instance.

### Do I need to configure TLS certificates for GHES?

If your GHES uses certificates signed by an internal CA, provide the CA certificate when creating the endpoint:

```bash
garm-cli github endpoint create \
  --ca-cert-path /path/to/ca-cert.pem \
  ...
```

You can also set a controller-wide CA bundle:

```bash
garm-cli controller update --ca-bundle /path/to/ca-bundle.pem
```

## Gitea

### How do I run Gitea jobs in Docker containers?

Don't use Gitea's extended label syntax (`label:docker://image`) in pool tags — GARM treats tags as opaque strings and won't match them against the short label that Gitea sends in webhooks.

Instead, use the `container` workflow syntax, which works identically on both GitHub and Gitea:

```yaml
jobs:
  build:
    runs-on: my-runner-label
    container:
      image: docker.gitea.com/runner-images:ubuntu-latest
    steps:
      - run: echo "Running in a container"
```

This lets you use a single runner image (with Docker and Node.js installed) and run workflows on any container image. To ensure the required packages are available, set `extra_specs` on the pool:

```bash
garm-cli pool add \
  --tags my-runner-label \
  --extra-specs '{"extra_packages":["docker.io","nodejs"]}' \
  ...
```

See the [GitHub docs on running jobs in containers](https://docs.github.com/en/actions/how-tos/write-workflows/choose-where-workflows-run/run-jobs-in-a-container) for more options (volumes, ports, environment variables). This syntax is supported by GitHub, Gitea, and Forgejo.

For more context, see [#697](https://github.com/cloudbase/garm/issues/697).

## Performance

### How can I make runners start faster?

1. **Cache the runner binary** in your image -- this is the biggest win. See [Performance](performance.md).
2. **Disable OS updates** during bootstrap: `--extra-specs='{"disable_updates": true}'`
3. **Use optimized storage** -- for LXD, choose a storage driver with optimized instance creation.
4. **Enable shiftfs** -- for LXD unprivileged containers.

### What should I set the bootstrap timeout to?

The default is 20 minutes. Increase it if your provider or image takes longer to boot (e.g., bare metal with Ironic, large Windows images). Decrease it if you want faster detection of failed runners.

## Security

### Is the GARM API secure?

GARM uses JWT authentication. The API should be placed behind a reverse proxy with TLS termination for production use. All sensitive data (credentials, tokens) is encrypted at rest.

### Should I use the Webhook Base URL or Controller Webhook URL?

Always prefer the **Controller Webhook URL**. It's unique to your GARM instance and allows multiple GARM controllers to coexist in the same repo/org. The Controller Webhook URL includes the Controller ID in the path.

## Troubleshooting

### GARM creates runners but they don't appear in GitHub

- Check that the **Callback URL** and **Metadata URL** are reachable from runner instances
- Look at runner status updates: `garm-cli runner show <RUNNER_NAME>`
- Check GARM logs for registration errors: `garm-cli debug-log`

### Jobs are queued but no runners are created

- Verify pool tags match the workflow's `runs-on` labels
- Check that the pool is enabled: `garm-cli pool show <POOL_ID>`
- Check the job age backoff isn't too high: `garm-cli controller show`
- Verify max-runners hasn't been reached
- Check `garm-cli job list` to see if GARM received the job
