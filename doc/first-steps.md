# First Steps

<!-- TOC -->

- [First Steps](#first-steps)
    - [Resource hierarchy](#resource-hierarchy)
    - [Add an endpoint if not using github.com](#add-an-endpoint-if-not-using-githubcom)
    - [Add credentials](#add-credentials)
    - [Add a repository](#add-a-repository)
    - [Create a runner pool](#create-a-runner-pool)
    - [Watch your runner come up](#watch-your-runner-come-up)
    - [Use the runner in a workflow](#use-the-runner-in-a-workflow)
    - [What's next](#whats-next)

<!-- /TOC -->

You have GARM running (via [Docker](quickstart-docker.md) or [systemd](quickstart-systemd.md)). This guide walks you through the steps to get your first runner spinning:

1. Understand the resource hierarchy
2. (Optional) Add an endpoint
3. Add credentials
4. Add a repository (or organization)
5. Create a runner pool
6. Verify runners appear in GitHub

## Resource hierarchy

GARM resources form a hierarchy:

```
Endpoint (github.com, GHES, Gitea)
  └── Credential (PAT, GitHub App, Gitea token)
        └── Entity (repository, organization, enterprise)
              ├── Pool (webhook-driven runners)
              └── Scale Set (message-queue-driven runners)
```

- **Endpoints** represent a forge: github.com, a GitHub Enterprise Server instance, or a Gitea server.
- **Credentials** are scoped to an endpoint and authenticate GARM's access to that forge.
- **Entities** (repos/orgs/enterprises) are tied to credentials, which determines their endpoint.
- **Pools** and **Scale Sets** are tied to entities and define how runners are created.

## 1. Add an endpoint (if not using github.com)

GARM ships with a default `github.com` endpoint. If you're using github.com, skip to step 2.

For **GitHub Enterprise Server**:

```bash
garm-cli github endpoint create \
  --name my-ghes \
  --description "Internal GHES" \
  --base-url https://ghes.example.com \
  --upload-url https://upload.ghes.example.com \
  --api-base-url https://api.ghes.example.com \
  --ca-cert-path /path/to/ca-cert.pem
```

For **Gitea** (1.24+):

```bash
garm-cli gitea endpoint create \
  --name my-gitea \
  --description "Internal Gitea" \
  --api-base-url https://gitea.example.com/ \
  --base-url https://gitea.example.com/ \
  --ca-cert-path /path/to/ca-cert.pem
```

The `--ca-cert-path` is optional for both GHES and Gitea. Use it when the server uses certificates signed by an internal CA.

List endpoints:

```bash
garm-cli github endpoint list   # GitHub / GHES endpoints
garm-cli gitea endpoint list    # Gitea endpoints
```

See [Managing Entities](managing-entities.md) for more details on endpoints.

## 2. Add credentials

Credentials are scoped to an endpoint. GARM needs them to manage runners and webhooks. See [Credentials](credentials.md) for full details on permissions and credential types.

Add a PAT (for github.com):

```bash
garm-cli github credentials add \
  --name my-pat \
  --description "GitHub PAT for runner management" \
  --auth-type pat \
  --pat-oauth-token gh_yourTokenGoesHere \
  --endpoint github.com
```

Or a GitHub App:

```bash
garm-cli github credentials add \
  --name my-app \
  --description "GitHub App for runner management" \
  --endpoint github.com \
  --auth-type app \
  --app-id 12345 \
  --app-installation-id 67890 \
  --private-key-path /path/to/private-key.pem
```

For GHES or Gitea, replace `--endpoint github.com` with the endpoint name you created in step 1. For Gitea, use `garm-cli gitea credentials add` with the same flags.

Verify:

```bash
garm-cli github credentials list   # for GitHub / GHES credentials
garm-cli gitea credentials list    # for Gitea credentials
```

## 3. Add a repository

```bash
garm-cli repo add \
  --owner your-org \
  --name your-repo \
  --credentials my-pat \
  --random-webhook-secret \
  --install-webhook \
  --pool-balancer-type roundrobin
```

This does three things:

- Registers the repository in GARM
- Generates a random webhook secret
- Installs a webhook in GitHub that sends `workflow_job` events to GARM

> [!IMPORTANT]
> `--install-webhook` requires the PAT or App to have webhook management permissions. If you prefer to set up webhooks manually, see [Webhooks](webhooks.md).

Verify:

```bash
garm-cli repo list
```

Note the repository **ID** from the output -- you'll need it for the next step. You can also use the `owner/name` format (e.g., `my-org/my-repo`) in place of the UUID wherever `--repo` is expected.

## 4. Create a runner pool

Check which providers are available:

```bash
garm-cli provider list
```

Create a pool using the LXD provider from the quickstart:

```bash
garm-cli pool add \
  --repo <REPO_ID> \
  --enabled \
  --provider-name lxd_local \
  --flavor default \
  --image ubuntu:22.04 \
  --max-runners 5 \
  --min-idle-runners 1 \
  --os-arch amd64 \
  --os-type linux \
  --tags ubuntu,generic
```

Key options:

| Option | Description |
|--------|-------------|
| `--min-idle-runners` | Runners kept warm and waiting for jobs. Set to `0` for pure on-demand scaling. |
| `--max-runners` | Upper limit on runners in this pool. |
| `--tags` | Labels applied to runners. Workflows target these with `runs-on:`. |
| `--flavor` | Provider-specific sizing (LXD profile, cloud instance type). |
| `--image` | Provider-specific image (LXD image alias, cloud AMI, etc). |

## 5. Watch your runner come up

```bash
garm-cli runner list --repo <REPO_ID>
```

After a few minutes:

```
+----+-------------------+---------+---------------+--------------------------------------------+
| NR | NAME              | STATUS  | RUNNER STATUS | POOL / SCALE SET                           |
+----+-------------------+---------+---------------+--------------------------------------------+
|  1 | garm-tdtD6zpsXhj1 | running | idle          | Pool: 344e4a72-2035-4a18-a3d5-87bd3874b56c |
+----+-------------------+---------+---------------+--------------------------------------------+
```

Get detailed status:

```bash
garm-cli runner show garm-tdtD6zpsXhj1
```

The runner should now appear in GitHub under **Settings > Actions > Runners**.

## 6. Use the runner in a workflow

Target your runner using the tags you set on the pool:

```yaml
# .github/workflows/test.yml
jobs:
  build:
    runs-on: [ubuntu, generic]
    steps:
      - uses: actions/checkout@v4
      - run: echo "Running on a GARM-managed runner!"
```

## What's next

- [Managing Repositories, Orgs, and Enterprises](managing-entities.md) -- Add organizations and enterprises
- [Pools and Scaling](pools-and-scaling.md) -- Fine-tune pool settings, scaling behavior, and pool balancing
- [Scale Sets](scale-sets.md) -- Use GitHub scale sets instead of webhook-driven pools
- [Templates](templates.md) -- Customize runner bootstrap scripts
- [Credentials](credentials.md) -- PAT scopes, GitHub Apps, and Gitea tokens
- [FAQ](faq.md) -- Common questions and answers
