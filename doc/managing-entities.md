# Managing Repositories, Organizations, and Enterprises

GARM manages runners at three levels: **repositories**, **organizations**, and **enterprises**. Each entity is associated with credentials that determine the GitHub/Gitea endpoint it belongs to.

<!-- TOC -->

- [Managing Repositories, Organizations, and Enterprises](#managing-repositories-organizations-and-enterprises)
    - [Endpoints](#endpoints)
        - [List endpoints](#list-endpoints)
        - [Add a GHES endpoint](#add-a-ghes-endpoint)
        - [Add a Gitea endpoint](#add-a-gitea-endpoint)
        - [Delete an endpoint](#delete-an-endpoint)
    - [Repositories](#repositories)
        - [Add a repository](#add-a-repository)
        - [List repositories](#list-repositories)
        - [Delete a repository](#delete-a-repository)
    - [Organizations](#organizations)
        - [Add an organization](#add-an-organization)
        - [List organizations](#list-organizations)
        - [Delete an organization](#delete-an-organization)
    - [Enterprises](#enterprises)
        - [Add an enterprise](#add-an-enterprise)
        - [List enterprises](#list-enterprises)
        - [Delete an enterprise](#delete-an-enterprise)
    - [Managing webhooks](#managing-webhooks)
        - [Show webhook status](#show-webhook-status)
        - [Install a webhook](#install-a-webhook)
        - [Uninstall a webhook](#uninstall-a-webhook)
        - [Manual webhook setup](#manual-webhook-setup)
    - [Controller settings](#controller-settings)
        - [Viewing controller settings](#viewing-controller-settings)
        - [Controller URLs](#controller-urls)
        - [Other controller settings](#other-controller-settings)
        - [Updating controller settings](#updating-controller-settings)

<!-- /TOC -->

## Endpoints

Endpoints tell GARM where to find the GitHub or Gitea API. A default `github.com` endpoint is created automatically.

### List endpoints

```bash
garm-cli github endpoint list
```

### Add a GHES endpoint

```bash
garm-cli github endpoint create \
  --name my-ghes \
  --description "Internal GHES" \
  --base-url https://ghes.example.com \
  --upload-url https://upload.ghes.example.com \
  --api-base-url https://api.ghes.example.com \
  --ca-cert-path /path/to/ca-cert.pem
```

The `--ca-cert-path` is optional. Use it when your GHES uses certificates signed by an internal CA.

### Add a Gitea endpoint

```bash
garm-cli gitea endpoint create \
  --name my-gitea \
  --description "Internal Gitea" \
  --api-base-url https://gitea.example.com/ \
  --base-url https://gitea.example.com/ \
  --ca-cert-path /path/to/ca-cert.pem
```

The `--ca-cert-path` is optional. Use it when your Gitea server uses certificates signed by an internal CA.

### Delete an endpoint

```bash
garm-cli github endpoint delete my-ghes
```

> [!IMPORTANT]
> You cannot delete an endpoint that has credentials associated with it or is in use by entities. The default `github.com` endpoint can be deleted if nothing references it.

## Repositories

### Add a repository

```bash
garm-cli repo add \
  --owner your-org \
  --name your-repo \
  --credentials my-pat \
  --random-webhook-secret \
  --install-webhook \
  --pool-balancer-type roundrobin
```

| Option | Description |
|--------|-------------|
| `--owner` | GitHub user or organization that owns the repo |
| `--name` | Repository name |
| `--credentials` | Name of credentials to use (must exist) |
| `--random-webhook-secret` | Generate a secure random webhook secret |
| `--install-webhook` | Automatically install the webhook in GitHub |
| `--pool-balancer-type` | `roundrobin` (default) or `pack` |

The `--pool-balancer-type` controls how GARM picks a pool when multiple pools match a job:

- **roundrobin** -- cycles through matching pools
- **pack** -- fills one pool before moving to the next (ordered by priority)

> [!IMPORTANT]
> Pool balancing only applies to pools. For scale sets, GitHub handles scheduling.

### List repositories

```bash
garm-cli repo list
```

### Delete a repository

```bash
garm-cli repo delete <REPO_ID>
```

This removes the entity from GARM and cleans up the webhook if one was installed via the Controller Webhook URL. GARM only removes webhooks that are namespaced to its own Controller ID -- webhooks pointing to the base Webhook URL are left untouched.

> [!IMPORTANT]
> The credentials used when adding an entity determine which endpoint (and therefore which forge) it belongs to. If you later swap credentials on an entity, the new credentials must belong to the same endpoint as the original ones.

## Organizations

### Add an organization

```bash
garm-cli org add \
  --name my-org \
  --credentials my-pat \
  --random-webhook-secret \
  --install-webhook
```

### List organizations

```bash
garm-cli org list
```

### Delete an organization

```bash
garm-cli org delete <ORG_ID>
```

## Enterprises

Enterprise webhook management is **manual** -- GARM does not install or manage webhooks at the enterprise level. The level of API access required for enterprise webhook management is broad, and since most organizations have only one enterprise, the effort-to-risk ratio doesn't justify automating it.

### Add an enterprise

```bash
garm-cli enterprise add \
  --name my-enterprise-slug \
  --credentials my-enterprise-pat \
  --webhook-secret "your-secure-webhook-secret"
```

The `--name` is the enterprise ["slug"](https://docs.github.com/en/enterprise-cloud@latest/admin/managing-your-enterprise-account/creating-an-enterprise-account).

After adding, manually configure the webhook in GitHub Enterprise using the Controller Webhook URL from `garm-cli controller show`.

### List enterprises

```bash
garm-cli enterprise list
```

### Delete an enterprise

```bash
garm-cli enterprise delete <ENTERPRISE_ID>
```

## Managing webhooks

GARM can manage webhooks for repositories and organizations (not enterprises).

### Show webhook status

```bash
garm-cli repo webhook show <REPO_ID>
garm-cli org webhook show <ORG_ID>
```

### Install a webhook

```bash
garm-cli repo webhook install <REPO_ID>
garm-cli org webhook install <ORG_ID>
```

### Uninstall a webhook

```bash
garm-cli repo webhook uninstall <REPO_ID>
garm-cli org webhook uninstall <ORG_ID>
```

### Manual webhook setup

If you prefer to set up webhooks manually in the GitHub UI, see [Webhooks](webhooks.md).

## Controller settings

Controller settings define the URLs that runners and GitHub/Gitea use to communicate with GARM, along with operational parameters like job backoff and agent tools sync. These settings are critical -- if they're wrong, runners can't phone home and webhooks won't arrive.

### Viewing controller settings

```bash
garm-cli controller show
```

```
+---------------------------+----------------------------------------------------------------------------+
| FIELD                     | VALUE                                                                      |
+---------------------------+----------------------------------------------------------------------------+
| Controller ID             | a4dd5f41-8e1e-42a7-af53-c0ba5ff6b0b3                                       |
| Hostname                  | garm                                                                       |
| Metadata URL              | https://garm.example.com/api/v1/metadata                                   |
| Callback URL              | https://garm.example.com/api/v1/callbacks                                  |
| Webhook Base URL          | https://garm.example.com/webhooks                                          |
| Controller Webhook URL    | https://garm.example.com/webhooks/a4dd5f41-8e1e-42a7-af53-c0ba5ff6b0b3     |
| Agent URL                 | https://garm.example.com/agent                                             |
| Minimum Job Age Backoff   | 30                                                                         |
| Version                   | v0.2.0                                                                     |
+---------------------------+----------------------------------------------------------------------------+
```

### Controller URLs

GARM exposes several URLs that serve different purposes. Each URL must be reachable by a specific party (runners or GitHub/Gitea). GARM cannot auto-detect these URLs because it may sit behind a NAT, load balancer, or reverse proxy where the externally reachable address differs from the local one.

When you initialize GARM for the first time, it assumes that all URLs share the same base address you logged in with. This works for most setups. If your network topology is more complex (e.g. separate internal and external addresses, or a reverse proxy with different paths), you'll need to update them.

| URL | Who must reach it | Purpose |
|-----|-------------------|---------|
| **Metadata URL** | Runners | Runners connect here during bootstrap to retrieve setup information (runner token, labels, etc). Injected into runner userdata. |
| **Callback URL** | Runners | Runners connect here to send status updates and system info (OS version, runner agent ID, etc) back to the controller. Injected into runner userdata. Authentication uses a short-lived JWT scoped to the specific instance -- it can only update that instance's status and fetch its own metadata. Token validity equals the pool bootstrap timeout (default 20 min) plus the GARM polling interval (5 min). |
| **Agent URL** | Runners (agent mode) | In agent mode, runners connect here via WebSocket for bidirectional communication instead of using callbacks. Only needed when agent mode is enabled on an entity. |
| **Webhook Base URL** | GitHub / Gitea | The base path where GitHub/Gitea sends `workflow_job` webhook events. Prefer using the Controller Webhook URL instead. |
| **Controller Webhook URL** | GitHub / Gitea | Same as the Webhook Base URL but with the Controller ID appended (e.g. `/webhooks/<controller-id>`). This is the preferred webhook URL because it's unique per GARM installation, allowing multiple controllers on the same repo/org. Automatically derived from the Webhook Base URL. |

The default URL paths match GARM's internal routes:

| Setting | Default path |
|---------|-------------|
| Metadata URL | `/api/v1/metadata` |
| Callback URL | `/api/v1/callbacks` |
| Agent URL | `/agent` |
| Webhook Base URL | `/webhooks` |

### Other controller settings

| Setting | Default | Description |
|---------|---------|-------------|
| **Controller ID** | (auto-generated) | Unique identifier for this GARM installation. Runners are tagged with this ID so multiple controllers can manage the same repos without conflicts. |
| **Minimum Job Age Backoff** | `30` | Seconds to wait after receiving a job webhook before spinning up a runner. Gives existing idle runners time to pick up the job. Set to `0` for immediate reaction (useful for scale-to-zero). |
| **CA Cert Bundle** | (none) | Optional CA certificate bundle injected into runner userdata. Allows runners to trust certificates signed by an internal CA when communicating with the controller. |
| **GARM Agent Tools Sync URL** | (none) | URL for automatic garm-agent binary sync, typically the garm-agent GitHub releases API (e.g. `https://api.github.com/repos/cloudbase/garm-agent/releases`). |
| **Tools Sync Enabled** | `false` | Whether GARM periodically fetches new garm-agent releases from the sync URL. |

### Updating controller settings

```bash
garm-cli controller update \
  --metadata-url https://garm.example.com/api/v1/metadata \
  --callback-url https://garm.example.com/api/v1/callbacks \
  --webhook-url https://garm.example.com/webhooks \
  --agent-url https://garm.example.com/agent
```

Other settings:

```bash
# Change job age backoff
garm-cli controller update --minimum-job-age-backoff 15

# Set a CA certificate bundle
garm-cli controller update --ca-bundle /path/to/ca-bundle.pem

# Clear the CA certificate bundle
garm-cli controller update --clear-ca-bundle

# Enable automatic agent tools sync
garm-cli controller update \
  --garm-tools-url https://api.github.com/repos/cloudbase/garm-agent/releases \
  --enable-tools-sync
```

After updating URLs, verify they are routable to the correct GARM API endpoints **and** accessible by the relevant parties (runners for metadata/callback/agent, GitHub/Gitea for webhooks).
