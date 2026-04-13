# Quickstart: Docker

This guide gets GARM running in Docker with the LXD provider. By the end, you will have a working GARM instance that can create self-hosted GitHub Actions runners on demand.

<!-- TOC -->

- [Quickstart: Docker](#quickstart-docker)
    - [Prerequisites](#prerequisites)
    - [Create the config directory](#create-the-config-directory)
    - [Write the GARM configuration](#write-the-garm-configuration)
    - [Write the LXD provider configuration](#write-the-lxd-provider-configuration)
    - [Fix iptables for LXD + Docker](#fix-iptables-for-lxd--docker)
    - [Start GARM](#start-garm)
    - [Install garm-cli](#install-garm-cli)
    - [Initialize GARM](#initialize-garm)
    - [Next steps](#next-steps)
    - [Using Docker Compose](#using-docker-compose)

<!-- /TOC -->

## Prerequisites

- A Linux host with Docker installed
- LXD installed and initialized (`sudo lxd init --auto` if you haven't already)
- A GitHub PAT, GitHub App, or Gitea token with the [required permissions](credentials.md#github-permissions)

## 1. Create the config directory

```bash
sudo mkdir -p /etc/garm
```

## 2. Write the GARM configuration

Create `/etc/garm/config.toml`:

```bash
sudo tee /etc/garm/config.toml > /dev/null <<'EOF'
[default]
enable_webhook_management = true

[logging]
enable_log_streamer = true
log_format = "text"
log_level = "info"
log_source = false

[metrics]
enable = true
disable_auth = false

[jwt_auth]
# CHANGE THIS to a random string (32+ characters).
secret = ")9gk_4A6KrXz9D2u`0@MPea*sd6W`%@5MAWpWWJ3P3EqW~qB!!(Vd$FhNc*eU4vG"
time_to_live = "8760h"

[apiserver]
  bind = "0.0.0.0"
  port = 80
  use_tls = false
  [apiserver.webui]
    enable = true

[database]
  backend = "sqlite3"
  # CHANGE THIS to a random 32-character string.
  passphrase = "shreotsinWadquidAitNefayctowUrph"
  [database.sqlite3]
    db_file = "/etc/garm/garm.db"

[[provider]]
  name = "lxd_local"
  provider_type = "external"
  description = "Local LXD installation"
  [provider.external]
    provider_executable = "/opt/garm/providers.d/garm-provider-lxd"
    config_file = "/etc/garm/garm-provider-lxd.toml"
EOF
```

## 3. Write the LXD provider configuration

Create `/etc/garm/garm-provider-lxd.toml`:

```bash
sudo tee /etc/garm/garm-provider-lxd.toml > /dev/null <<'EOF'
unix_socket_path = "/var/snap/lxd/common/lxd/unix.socket"
include_default_profile = false
instance_type = "container"
secure_boot = false
project_name = "default"
url = ""
client_certificate = ""
client_key = ""
tls_server_certificate = ""

[image_remotes]
  [image_remotes.ubuntu]
  addr = "https://cloud-images.ubuntu.com/releases"
  public = true
  protocol = "simplestreams"
  skip_verify = false
  [image_remotes.ubuntu_daily]
  addr = "https://cloud-images.ubuntu.com/daily"
  public = true
  protocol = "simplestreams"
  skip_verify = false
  [image_remotes.images]
  addr = "https://images.lxd.canonical.com"
  public = true
  protocol = "simplestreams"
  skip_verify = false
EOF
```

## 4. Fix iptables for LXD + Docker

LXD and Docker can conflict on iptables rules. Without this fix, LXD containers (your runners) will not have internet access:

```bash
sudo iptables -I DOCKER-USER -j ACCEPT
```

> [!IMPORTANT]
> This rule does not persist across reboots. To make it permanent, use `iptables-persistent` or add it to a startup script.

## 5. Start GARM

Replace the image tag with the [latest release version](https://github.com/cloudbase/garm/releases):

```bash
docker run -d \
  --name garm \
  -p 80:80 \
  -v /etc/garm:/etc/garm:rw \
  -v /var/snap/lxd/common/lxd/unix.socket:/var/snap/lxd/common/lxd/unix.socket:rw \
  ghcr.io/cloudbase/garm:v0.2.0-beta1
```

Check the logs:

```bash
docker logs garm
```

You should see lines like:

```
level=INFO msg="Loading provider" provider=lxd_local
level=INFO msg="setting up metric routes"
level=INFO msg="register metrics"
```

## 6. Install garm-cli

Copy the CLI from the container:

```bash
sudo docker cp garm:/bin/garm-cli /usr/local/bin/garm-cli
sudo chmod +x /usr/local/bin/garm-cli
```

Or download the release binary:

```bash
wget -q -O - \
  https://github.com/cloudbase/garm/releases/latest/download/garm-cli-linux-amd64.tgz \
  | sudo tar xzf - -C /usr/local/bin/
```

## 7. Initialize GARM

Replace `garm.example.com` with the hostname or IP where GARM is reachable:

```bash
garm-cli init --name="my_garm" --url http://garm.example.com
```

You will be prompted for a username, email, and password. These are your admin credentials.

The output shows your admin user and controller details:

```
Admin user information:

+----------+--------------------------------------+
| FIELD    | VALUE                                |
+----------+--------------------------------------+
| ID       | 4f38839b-a10e-4732-9bba-4abb235583a9 |
| Username | admin                                |
| Email    | admin@example.com                    |
| Enabled  | true                                 |
+----------+--------------------------------------+

Controller information:

+---------------------------+-----------------------------------------------------------------------------+
| FIELD                     | VALUE                                                                       |
+---------------------------+-----------------------------------------------------------------------------+
| Controller ID             | 9febbf3f-a8ab-4952-9b5b-0416444492b5                                        |
| Metadata URL              | http://garm.example.com/api/v1/metadata                                     |
| Callback URL              | http://garm.example.com/api/v1/callbacks                                    |
| Webhook Base URL          | http://garm.example.com/webhooks                                            |
| Controller Webhook URL    | http://garm.example.com/webhooks/9febbf3f-a8ab-4952-9b5b-0416444492b5       |
| Agent URL                 | http://garm.example.com/agent                                               |
| GARM agent tools sync URL | https://api.github.com/repos/cloudbase/garm-agent/releases                  |
| Tools sync enabled        | false                                                                       |
| Minimum Job Age Backoff   | 30                                                                          |
| Version                   | v0.2.0-beta1                                                                |
+---------------------------+-----------------------------------------------------------------------------+
```

Key URLs to verify:

- **Metadata URL** and **Callback URL** must be reachable by the runner instances.
- **Webhook Base URL** / **Controller Webhook URL** must be reachable by GitHub/Gitea.

By default, GARM derives all URLs from the `--url` you passed to `init`. If your setup has different internal and external addresses (e.g. behind a reverse proxy or NAT), you can override individual URLs at init time:

```bash
garm-cli init --name="my_garm" --url http://garm.example.com \
  --callback-url https://internal.example.com/api/v1/callbacks \
  --metadata-url https://internal.example.com/api/v1/metadata \
  --webhook-url https://external.example.com/webhooks \
  --ca-bundle /path/to/ca-bundle.pem  # optional: for internal CAs
```

You can also change these later with `garm-cli controller update`. See [Controller settings](managing-entities.md#controller-settings) for details.

Each `garm-cli init` creates a CLI **profile** stored locally. To manage multiple GARM instances, add profiles and switch between them:

```bash
garm-cli profile add --name="prod_garm" --url https://garm-prod.example.com
garm-cli profile switch prod_garm
```

## Next steps

Your GARM instance is running. Continue with [First Steps](first-steps.md) to add credentials, a repository, and your first runner pool.

---

## Using Docker Compose

For a more maintainable setup, create a `docker-compose.yaml`:

```yaml
services:
  garm:
    image: ghcr.io/cloudbase/garm:v0.2.0-beta1
    container_name: garm
    restart: always
    volumes:
      - /etc/garm:/etc/garm:rw
      - /var/snap/lxd/common/lxd/unix.socket:/var/snap/lxd/common/lxd/unix.socket:rw
    ports:
      - "80:80"
```

Start with:

```bash
docker compose up -d
```

Then apply the [iptables fix](#4-fix-iptables-for-lxd--docker) and continue from [step 6 (Install garm-cli)](#6-install-garm-cli) above.

> **Tip:** The GARM container image includes provider binaries for LXD, Incus, Azure, AWS, GCP, OpenStack, OCI, and Kubernetes in `/opt/garm/providers.d/`. You can use any of them by adding the corresponding `[[provider]]` section to your config.
