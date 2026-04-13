# Configuration Reference

GARM is configured via a single TOML file, typically at `/etc/garm/config.toml`. This reference covers every section.
<!-- TOC -->

- [Configuration Reference](#configuration-reference)
    - [Minimal configuration](#minimal-configuration)
    - [[default]](#default)
    - [[logging]](#logging)
    - [[database]](#database)
        - [[database.sqlite3]](#databasesqlite3)
    - [[[provider]]](#provider)
    - [[metrics]](#metrics)
    - [[jwt_auth]](#jwt_auth)
    - [[apiserver]](#apiserver)
        - [[apiserver.tls]](#apiservertls)
        - [[apiserver.webui]](#apiserverwebui)
    - [Available providers](#available-providers)

<!-- /TOC -->

## Minimal configuration

```toml
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
secret = "<random-string-32-chars-or-more>"
time_to_live = "8760h"

[apiserver]
  bind = "0.0.0.0"
  port = 9997
  use_tls = false
  [apiserver.webui]
    enable = true

[database]
  backend = "sqlite3"
  passphrase = "<random-32-char-string>"
  [database.sqlite3]
    db_file = "/etc/garm/garm.db"
```

## [default]

| Option | Default | Description |
|--------|---------|-------------|
| `enable_webhook_management` | `false` | Allow GARM to install/manage webhooks automatically |
| `debug_server` | `false` | Enable the Go pprof profiling server on `127.0.0.1:9997` |

When `debug_server` is enabled, you can profile GARM using:

```bash
go tool pprof http://127.0.0.1:9997/debug/pprof/profile?seconds=120
```

> [!IMPORTANT]
> The profiling command will block for the duration of the profile (e.g. 120 seconds). Most reverse proxies timeout after ~60 seconds, so always profile by connecting directly to GARM on localhost. It's advisable to exclude the debug server URLs from your reverse proxy entirely.

> **Deprecated:** `callback_url`, `metadata_url`, `log_file`, `enable_log_streamer` -- these are now managed via `garm-cli controller update` or the `[logging]` section.

## [logging]

| Option | Default | Description |
|--------|---------|-------------|
| `log_file` | (stdout) | Path to log file. Omit to log to stdout. |
| `enable_log_streamer` | `false` | Enable live log streaming via WebSocket (`garm-cli debug-log`) |
| `log_format` | `"text"` | `"text"` or `"json"`. Use JSON for log aggregation (Loki, ELK). |
| `log_level` | `"info"` | `"debug"`, `"info"`, `"warn"`, or `"error"` |
| `log_source` | `false` | Include source file/line in log output |

**Log rotation:** GARM auto-rotates at 500 MB or 28 days, whichever comes first. Send `SIGHUP` to manually rotate. If running under systemd, add the following to your unit file:

```ini
[Service]
ExecReload=/bin/kill -HUP $MAINPID
```

Then rotate with `systemctl reload garm`.

## [database]

| Option | Default | Description |
|--------|---------|-------------|
| `debug` | `false` | Log all database queries |
| `backend` | `"sqlite3"` | Database backend (only `sqlite3` is supported) |
| `passphrase` | (required) | 32-character string used to encrypt secrets at rest (AES-256). Protects webhook secrets, tokens, and private keys stored in the database. |

### [database.sqlite3]

| Option | Default | Description |
|--------|---------|-------------|
| `db_file` | (required) | Path to the SQLite database file |

## [[provider]]

Providers are external executables that GARM calls to create and manage runner instances. You can define multiple providers.

```toml
[[provider]]
  name = "lxd_local"
  provider_type = "external"
  description = "Local LXD installation"
  [provider.external]
    provider_executable = "/opt/garm/providers.d/garm-provider-lxd"
    config_file = "/etc/garm/garm-provider-lxd.toml"
    environment_variables = ["AWS_"]
```

| Option | Description |
|--------|-------------|
| `name` | Unique name for this provider |
| `provider_type` | Always `"external"` |
| `description` | Human-readable description |
| `provider_executable` | Absolute path to the provider binary |
| `config_file` | Path to the provider's own config file |
| `environment_variables` | List of env var names or prefixes to pass to the provider (e.g., `["AWS_"]` passes all `AWS_*` vars) |

## [metrics]

| Option | Default | Description |
|--------|---------|-------------|
| `enable` | `false` | Enable the `/metrics` Prometheus endpoint |
| `disable_auth` | `false` | Disable JWT authentication on the metrics endpoint |
| `period` | `"60s"` | How often internal metrics are recalculated |

To generate a metrics token for Prometheus:

```bash
garm-cli metrics-token create
```

See [Monitoring and Debugging](monitoring.md) for Prometheus configuration.

## [jwt_auth]

| Option | Default | Description |
|--------|---------|-------------|
| `secret` | (required) | Secret used to sign JWT tokens. Use a long, random string. |
| `time_to_live` | `"24h"` | How long admin tokens are valid. Minimum `24h`. |

This TTL applies only to tokens you get when logging into the API (and metrics tokens). Tokens issued to runner instances have a TTL based on the pool's bootstrap timeout plus the GARM polling interval -- they are not affected by this setting.

Changing the secret invalidates all existing tokens.

## [apiserver]

| Option | Default | Description |
|--------|---------|-------------|
| `bind` | `"0.0.0.0"` | IP address to bind to |
| `port` | `9997` | Port to listen on |
| `use_tls` | `false` | Enable TLS on the API server |
| `cors_origins` | `[]` | Allowed CORS origins. `["*"]` allows all. |

### [apiserver.tls]

| Option | Description |
|--------|-------------|
| `certificate` | Path to x509 certificate (or full chain bundle) |
| `key` | Path to the private key |

> [!IMPORTANT]
> If your certificate is signed by an intermediary CA, the `certificate` file must contain the entire chain (your certificate concatenated with the CA bundle). Without the full chain, clients may fail to validate the connection.

While GARM supports TLS natively, using a reverse proxy (e.g. nginx) for TLS termination is recommended for most production setups.

### [apiserver.webui]

| Option | Default | Description |
|--------|---------|-------------|
| `enable` | `true` | Enable the Web UI at `/ui/` |

## Available providers

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

Each provider has its own configuration file and documentation. Refer to the provider's repository for setup instructions and available extra_specs options.
