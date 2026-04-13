# Using GARM with Gitea

Starting with Gitea 1.24 and GARM v0.2.0, GARM supports Gitea as a forge alongside GitHub and GHES. You can manage runners for both GitHub and Gitea from the same GARM instance.

<!-- TOC -->

- [Using GARM with Gitea](#using-garm-with-gitea)
    - [Quickstart: Gitea + GARM with Docker Compose](#quickstart-gitea--garm-with-docker-compose)
        - [Prerequisites](#prerequisites)
        - [Create directories](#create-directories)
        - [Create docker-compose.yaml](#create-docker-composeyaml)
        - [Create GARM and LXD provider configs](#create-garm-and-lxd-provider-configs)
        - [Start services](#start-services)
        - [Create a Gitea user and repo](#create-a-gitea-user-and-repo)
        - [Initialize GARM](#initialize-garm)
    - [Adding a Gitea endpoint](#adding-a-gitea-endpoint)
    - [Adding Gitea credentials](#adding-gitea-credentials)
    - [Adding a repository and pool](#adding-a-repository-and-pool)
    - [Webhook configuration](#webhook-configuration)
    - [Differences from GitHub](#differences-from-github)

<!-- /TOC -->

## Quickstart: Gitea + GARM with Docker Compose

This quickstart deploys both Gitea and GARM using Docker Compose with LXD as the runner provider.

### Prerequisites

- Ubuntu host with Docker and LXD installed (`sudo lxd init --auto`)

> [!IMPORTANT]
> Docker and LXD can conflict on iptables. If LXD containers lose internet, run:
> ```bash
> sudo iptables -I DOCKER-USER -j ACCEPT
> ```

### 1. Create directories

```bash
sudo mkdir -p /etc/gitea /etc/garm
sudo chown 1000:1000 /etc/gitea /etc/garm
```

### 2. Create docker-compose.yaml

```yaml
networks:
  default:
    external: false

services:
  gitea:
    image: docker.gitea.com/gitea:1.25.5
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
    restart: always
    volumes:
      - /etc/gitea/gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "80:80"
      - "22:22"

  garm:
    image: ghcr.io/cloudbase/garm:nightly
    container_name: garm
    restart: always
    volumes:
      - /etc/garm:/etc/garm
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
      - /var/snap/lxd/common/lxd/unix.socket:/var/snap/lxd/common/lxd/unix.socket
    ports:
      - "9997:9997"
```

### 3. Create GARM and LXD provider configs

Follow the same config files as the [Docker Quickstart](quickstart-docker.md), but set `port = 9997` in `[apiserver]`.

### 4. Start services

```bash
docker compose up -d
```

### 5. Create a Gitea user and repo

```bash
docker exec -u=1000 gitea \
  gitea admin user create \
  --username testing \
  --password superSecretPassword \
  --email admin@example.com \
  --admin \
  --must-change-password=false
```

Log into Gitea and create an organization (e.g., `testorg`) and a repository (e.g., `testrepo`).

### 6. Initialize GARM

```bash
sudo docker cp garm:/bin/garm-cli /usr/local/bin/garm-cli
sudo chmod +x /usr/local/bin/garm-cli

garm-cli init --name="my_garm" --url http://<YOUR_IP>:9997
```

## Adding a Gitea endpoint

```bash
garm-cli gitea endpoint create \
  --api-base-url http://<YOUR_IP>/ \
  --base-url http://<YOUR_IP>/ \
  --description "My Gitea server" \
  --name local-gitea
```

## Adding Gitea credentials

Create a Gitea PAT with `write:repository` and `write:organization` scopes:

```bash
LOGIN=$(curl -s -X POST http://localhost/api/v1/users/testing/tokens \
  -u 'testing:superSecretPassword' \
  -H "Content-Type: application/json" \
  -d '{"name": "garm-token", "scopes": ["write:repository", "write:organization"]}')

TOKEN=$(echo $LOGIN | jq -r '.sha1')

garm-cli gitea credentials add \
  --endpoint local-gitea \
  --auth-type pat \
  --pat-oauth-token $TOKEN \
  --name gitea-token \
  --description "Gitea runner management token"
```

## Adding a repository and pool

```bash
garm-cli repo add \
  --credentials gitea-token \
  --name testrepo \
  --owner testorg \
  --random-webhook-secret \
  --install-webhook

garm-cli pool add \
  --repo <REPO_ID> \
  --provider-name lxd_local \
  --image ubuntu:24.04 \
  --tags ubuntu-latest \
  --flavor default \
  --os-arch amd64 \
  --os-type linux \
  --enabled=true \
  --min-idle-runners=1 \
  --max-runners=5
```

Check the runner:

```bash
garm-cli runner list
garm-cli runner show <RUNNER_NAME>
```

## Webhook configuration

If GARM is on a private or internal IP address, Gitea will block webhook deliveries by default. You need to add GARM's address to Gitea's allowed host list in `app.ini`:

```ini
[webhook]
ALLOWED_HOST_LIST = private
```

Setting `ALLOWED_HOST_LIST = private` allows webhooks to private network addresses. See the [Gitea documentation](https://docs.gitea.com/administration/config-cheat-sheet#webhook-webhook) for more options.

After changing `app.ini`, restart Gitea for the setting to take effect.

## Differences from GitHub

- **Runner binary:** Gitea uses `act_runner` instead of the GitHub Actions runner. GARM handles the differences transparently.
- **Enterprise level:** Not available for Gitea
- **Scale sets:** Not available for Gitea (GitHub-only feature)
- **GitHub Apps:** Gitea uses PATs only
