# Credentials

GARM needs credentials to interact with GitHub or Gitea: creating runners, managing webhooks, and fetching registration tokens. Credentials are always tied to an **endpoint** (github.com, a GHES instance, or a Gitea server).

<!-- TOC -->

- [Credentials](#credentials)
  - [Credential types](#credential-types)
  - [GitHub permissions](#github-permissions)
    - [PAT (classic) scopes](#pat-classic-scopes)
    - [Fine-grained PAT permissions](#fine-grained-pat-permissions)
    - [GitHub App permissions](#github-app-permissions)
  - [Managing credentials](#managing-credentials)
    - [Add a PAT](#add-a-pat)
    - [Add a GitHub App](#add-a-github-app)
    - [List credentials](#list-credentials)
    - [Show credential details](#show-credential-details)
    - [Delete a credential](#delete-a-credential)
  - [Rate limit usage reservation](#rate-limit-usage-reservation)
  - [Gitea credentials](#gitea-credentials)
    - [Create a Gitea token](#create-a-gitea-token)
    - [Add Gitea credentials to GARM](#add-gitea-credentials-to-garm)
  - [Credential and endpoint relationship](#credential-and-endpoint-relationship)
  - [Security](#security)

<!-- /TOC -->

## Credential types

| Type | Supports | Best for |
| ------ | ---------- | ---------- |
| PAT (classic) | Repos, orgs, enterprises | Simple setups, enterprise-level access |
| Fine-grained PAT | Repos, orgs | Scoped access to specific repos |
| GitHub App | Repos, orgs | Production setups, better rate limits |
| Gitea token | Repos, orgs | Gitea instances |

> [!IMPORTANT]
> GitHub Apps are **not** available at the enterprise level. Use a PAT for enterprise runner management.

## GitHub permissions

### PAT (classic) scopes

| Scope | When needed |
| ------- | ------------- |
| `public_repo` | Public repositories |
| `repo` | Private repositories |
| `admin:org` | Organization-level runner management |
| `manage_runners:enterprise` | Enterprise-level runner management |
| `admin:repo_hook` | Automatic webhook management on repos |
| `admin:org_hook` | Automatic webhook management on orgs |

### Fine-grained PAT permissions

**Repository permissions:**

- `Administration: Read & write` -- manage runners, generate JIT config
- `Metadata: Read-only` -- automatically required
- `Webhooks: Read & write` -- automatic webhook management

**Organization permissions:**

- `Self-hosted runners: Read & write` -- manage runners in the org
- `Webhooks: Read & write` -- automatic webhook management on the org

### GitHub App permissions

Same as fine-grained PAT:

- Repository: `Administration: Read & write`, `Metadata: Read-only`, `Webhooks: Read & write`
- Organization: `Self-hosted runners: Read & write`, `Webhooks: Read & write`

## Managing credentials

### Add a PAT

```bash
garm-cli github credentials add \
  --name my-pat \
  --description "PAT for runner management" \
  --auth-type pat \
  --pat-oauth-token gh_yourTokenGoesHere \
  --endpoint github.com
```

### Add a GitHub App

```bash
garm-cli github credentials add \
  --name my-app \
  --description "GitHub App for runners" \
  --endpoint github.com \
  --auth-type app \
  --app-id 12345 \
  --app-installation-id 67890 \
  --private-key-path /path/to/private-key.pem
```

### List credentials

```bash
garm-cli github credentials list
```

```bash
+----+---------+----------------------------+--------------------+------+
| ID | NAME    | DESCRIPTION                | BASE URL           | TYPE |
+----+---------+----------------------------+--------------------+------+
|  1 | my-pat  | PAT for runner management  | https://github.com | pat  |
|  2 | my-app  | GitHub App for runners     | https://github.com | app  |
+----+---------+----------------------------+--------------------+------+
```

### Show credential details

```bash
garm-cli github credentials show 1
```

The detail view shows which repositories, organizations, and enterprises are currently using this credential.

### Delete a credential

> [!IMPORTANT]
> You cannot delete credentials that are in use by a repository, organization, or enterprise. Replace the credentials on the entity first.

```bash
garm-cli github credentials delete 1
```

## Rate limit usage reservation

GARM records the API rate limit values returned with every forge response and can pause work when the quota runs low. On top of that, each GitHub credential can **reserve a slice of its rate limit quota for critical operations** — removing runners that finished their jobs or were marked for deletion. Those operations directly impact IaaS cost: if GARM cannot delete finished runners, you keep paying for idle machines.

```bash
garm-cli github credentials update 1 \
  --reserve-usage-enabled=true \
  --reserve-usage-percentage=10
```

The same flags are available on `garm-cli github credentials add`, and the setting can also be changed from the Web UI credentials forms.

How it behaves:

- While the remaining quota is above the reserved threshold, everything runs normally.
- Once the remaining quota dips into the reserve, **non-critical operations pause**: scaling up, reacting to new jobs, periodic reconciliation and cleanup sweeps. Critical operations (executing already-decided runner deletions) keep running from the reserve.
- When the quota is fully exhausted, everything that needs a forge API call pauses.
- Paused operations **resume automatically** once the quota resets.

The percentage is capped at **50%**. Reserving more than half of the quota would starve normal operations. A value between 5% and 20% is safe on most setups; adjust based on your usage patterns.

> [!NOTE]
> This is a GitHub (and GHES with rate limiting enabled) feature. Gitea does not report rate limits, so the setting has no effect there. Credentials that have never seen a rate limited response are never paused.

The current rate limit values for each credential are visible in `garm-cli github credentials show` and are exported as Prometheus metrics (see [monitoring](monitoring.md)).

## Gitea credentials

Gitea uses personal access tokens. The token needs `write:repository` and `write:organization` scopes.

### Create a Gitea token

```bash
curl -s -X POST http://gitea.example.com/api/v1/users/admin/tokens \
  -u 'admin:password' \
  -H "Content-Type: application/json" \
  -d '{"name": "garm-token", "scopes": ["write:repository", "write:organization"]}'
```

### Add Gitea credentials to GARM

```bash
garm-cli gitea credentials add \
  --endpoint my-gitea \
  --auth-type pat \
  --pat-oauth-token <token-from-above> \
  --name gitea-token \
  --description "Gitea runner management"
```

## Credential and endpoint relationship

Credentials are always tied to an endpoint. When you create a repository/organization/enterprise in GARM, the credentials you use determine which endpoint the entity is associated with.

If you later want to **replace** the credentials on an entity, the new credentials **must** be associated with the **same endpoint** as the original ones.

## Security

All sensitive credential data (tokens, private keys) is encrypted at rest in the GARM database using the `passphrase` configured in `[database]`. The API never returns sensitive information.
