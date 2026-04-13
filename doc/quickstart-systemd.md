# Quickstart: Systemd

This guide installs GARM as a native Linux service managed by systemd, using the LXD provider. By the end, you will have a working GARM instance that can create self-hosted GitHub Actions runners on demand.

<!-- TOC -->

- [Quickstart: Systemd](#quickstart-systemd)
    - [Prerequisites](#prerequisites)
    - [Create config directory and system user](#create-config-directory-and-system-user)
    - [Download GARM and garm-cli](#download-garm-and-garm-cli)
    - [Install the LXD provider](#install-the-lxd-provider)
    - [Write the GARM configuration](#write-the-garm-configuration)
    - [Write the LXD provider configuration](#write-the-lxd-provider-configuration)
    - [Set permissions and install the service](#set-permissions-and-install-the-service)
    - [Initialize GARM](#initialize-garm)
    - [Next steps](#next-steps)
    - [Log rotation](#log-rotation)

<!-- /TOC -->

## Prerequisites

- A Linux host
- LXD installed and initialized (`sudo lxd init --auto` if you haven't already)
- A GitHub PAT, GitHub App, or Gitea token with the [required permissions](credentials.md#github-permissions)
- Go 1.22+ (only if building providers from source)

## 1. Create config directory and system user

```bash
sudo mkdir -p /etc/garm
sudo mkdir -p /opt/garm/providers.d

sudo useradd --shell /usr/bin/false \
  --system \
  --groups lxd \
  --no-create-home garm
```

Adding the `garm` user to the `lxd` group allows it to connect to the LXD unix socket.

## 2. Download GARM and garm-cli

```bash
wget -q -O - \
  https://github.com/cloudbase/garm/releases/latest/download/garm-linux-amd64.tgz \
  | sudo tar xzf - -C /usr/local/bin/

wget -q -O - \
  https://github.com/cloudbase/garm/releases/latest/download/garm-cli-linux-amd64.tgz \
  | sudo tar xzf - -C /usr/local/bin/
```

To listen on ports below 1024 (like port 80) without running as root:

```bash
sudo setcap cap_net_bind_service=+ep /usr/local/bin/garm
```

## 3. Install the LXD provider

Download the pre-built release binary:

```bash
wget -q -O - \
  https://github.com/cloudbase/garm-provider-lxd/releases/latest/download/garm-provider-lxd-linux-amd64.tgz \
  | sudo tar xzf - -C /opt/garm/providers.d/
```

Or build from source (requires Go 1.22+):

```bash
git clone https://github.com/cloudbase/garm-provider-lxd
cd garm-provider-lxd
go build -o /opt/garm/providers.d/garm-provider-lxd .
cd ..
```

## 4. Write the GARM configuration

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

## 5. Write the LXD provider configuration

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

## 6. Set permissions and install the service

```bash
sudo chown -R garm:garm /etc/garm

sudo wget -O /etc/systemd/system/garm.service \
  https://raw.githubusercontent.com/cloudbase/garm/main/contrib/garm.service

sudo systemctl daemon-reload
sudo systemctl enable --now garm
```

Check the logs:

```bash
sudo journalctl -u garm -f
```

You should see lines like:

```
level=INFO msg="Loading provider" provider=lxd_local
level=INFO msg="setting up metric routes"
level=INFO msg="register metrics"
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

## Log rotation

GARM auto-rotates logs when they reach 500 MB or 28 days. To manually rotate, send SIGHUP:

```bash
sudo systemctl reload garm
```

The default systemd unit file already includes the `ExecReload` directive needed for this to work.
