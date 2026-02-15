# Configuration

The ```GARM``` configuration is a simple ```toml```. The sample config file in [the testdata folder](/testdata/config.toml) is fairly well commented and should be enough to get you started. The configuration file is split into several sections, each of which is documented in its own page. The sections are:

<!-- TOC -->

- [Configuration](#configuration)
    - [The default config section](#the-default-config-section)
        - [The callback_url option](#the-callback_url-option)
        - [The metadata_url option](#the-metadata_url-option)
        - [The debug_server option](#the-debug_server-option)
        - [The log_file option](#the-log_file-option)
            - [Rotating log files](#rotating-log-files)
        - [The enable_log_streamer option](#the-enable_log_streamer-option)
    - [The logging section](#the-logging-section)
    - [Database configuration](#database-configuration)
    - [Provider configuration](#provider-configuration)
        - [Providers](#providers)
            - [Available external providers](#available-external-providers)
    - [The metrics section](#the-metrics-section)
        - [Common metrics](#common-metrics)
        - [Enterprise metrics](#enterprise-metrics)
        - [Organization metrics](#organization-metrics)
        - [Repository metrics](#repository-metrics)
        - [Provider metrics](#provider-metrics)
        - [Pool metrics](#pool-metrics)
        - [Runner metrics](#runner-metrics)
        - [Job metrics](#job-metrics)
        - [Github metrics](#github-metrics)
        - [Enabling metrics](#enabling-metrics)
        - [Configuring prometheus](#configuring-prometheus)
    - [The JWT authentication config section](#the-jwt-authentication-config-section)
    - [The API server config section](#the-api-server-config-section)

<!-- /TOC -->

## The default config section

The `default` config section holds configuration options that don't need a category of their own, but are essential to the operation of the service. In this section we will detail each of the options available in the `default` section.

```toml
[default]
# Uncomment this line if you'd like to log to a file instead of standard output.
# log_file = "/tmp/runner-manager.log"

# Enable streaming logs via web sockets. Use garm-cli debug-log.
enable_log_streamer = false

# Enable the golang debug server. See the documentation in the "doc" folder for more information.
debug_server = false
```

### The callback_url option

Your runners will call back home with status updates as they install. Once they are set up, they will also send the GitHub agent ID they were allocated. You will need to configure the ```callback_url``` option in the ```garm``` server config. This URL needs to point to the following API endpoint:

  ```txt
  POST /api/v1/callbacks/status
  ```

Example of a runner sending status updates:

  ```bash
  garm-cli runner show garm-DvxiVAlfHeE7
  +-----------------+------------------------------------------------------------------------------------+
  | FIELD           | VALUE                                                                              |
  +-----------------+------------------------------------------------------------------------------------+
  | ID              | 16b96ba2-d406-45b8-ab66-b70be6237b4e                                               |
  | Provider ID     | garm-DvxiVAlfHeE7                                                                  |
  | Name            | garm-DvxiVAlfHeE7                                                                  |
  | OS Type         | linux                                                                              |
  | OS Architecture | amd64                                                                              |
  | OS Name         | ubuntu                                                                             |
  | OS Version      | jammy                                                                              |
  | Status          | running                                                                            |
  | Runner Status   | idle                                                                               |
  | Pool ID         | 8ec34c1f-b053-4a5d-80d6-40afdfb389f9                                               |
  | Addresses       | 10.198.117.120                                                                     |
  | Status Updates  | 2023-07-08T06:26:46: runner registration token was retrieved                       |
  |                 | 2023-07-08T06:26:46: using cached runner found in /opt/cache/actions-runner/latest |
  |                 | 2023-07-08T06:26:50: configuring runner                                            |
  |                 | 2023-07-08T06:26:56: runner successfully configured after 1 attempt(s)             |
  |                 | 2023-07-08T06:26:56: installing runner service                                     |
  |                 | 2023-07-08T06:26:56: starting service                                              |
  |                 | 2023-07-08T06:26:57: runner successfully installed                                 |
  +-----------------+------------------------------------------------------------------------------------+

  ```

This URL must be set and must be accessible by the instance. If you wish to restrict access to it, a reverse proxy can be configured to accept requests only from networks in which the runners ```garm``` manages will be spun up. This URL doesn't need to be globally accessible, it just needs to be accessible by the instances.

For example, in a scenario where you expose the API endpoint directly, this setting could look like the following:

  ```toml
  callback_url = "https://garm.example.com/api/v1/callbacks"
  ```

Authentication is done using a short-lived JWT token, that gets generated for a particular instance that we are spinning up. That JWT token grants access to the instance to only update its own status and to fetch metadata for itself. No other API endpoints will work with that JWT token. The validity of the token is equal to the pool bootstrap timeout value (default 20 minutes) plus the garm polling interval (5 minutes).

There is a sample ```nginx``` config [in the testdata folder](/testdata/nginx-server.conf). Feel free to customize it in any way you see fit.

### The metadata_url option

The metadata URL is the base URL for any information an instance may need to fetch in order to finish setting itself up. As this URL may be placed behind a reverse proxy, you'll need to configure it in the ```garm``` config file. Ultimately this URL will need to point to the following ```garm``` API endpoint:

  ```bash
  GET /api/v1/metadata
  ```

This URL needs to be accessible only by the instances ```garm``` sets up. This URL will not be used by anyone else. To configure it in ```garm``` add the following line in the ```[default]``` section of your ```garm``` config:

  ```toml
  metadata_url = "https://garm.example.com/api/v1/metadata"
  ```

### The debug_server option

GARM can optionally enable the golang profiling server. This is useful if you suspect garm may be have a bottleneck in any way. To enable the profiling server, add the following section to the garm config:

```toml
[default]

debug_server = true
```

And restart garm. You can then use the following command to start profiling:

```bash
go tool pprof http://127.0.0.1:9997/debug/pprof/profile?seconds=120
```

> **IMPORTANT NOTE on profiling when behind a reverse proxy**: The above command will hang for a fairly long time. Most reverse proxies will timeout after about 60 seconds. To avoid this, you should only profile on localhost by connecting directly to garm.

It's also advisable to exclude the debug server URLs from your reverse proxy and only make them available locally.

Now that the debug server is enabled, here is a blog post on how to profile golang applications: https://blog.golang.org/profiling-go-programs


### The log_file option

By default, GARM logs everything to standard output.

You can optionally log to file by adding the following to your config file:

```toml
[default]
# Use this if you'd like to log to a file instead of standard output.
log_file = "/tmp/runner-manager.log"
```

#### Rotating log files

GARM automatically rotates the log if it reaches 500 MB in size or 28 days, whichever comes first.

However, if you want to manually rotate the log file, you can send a `SIGHUP` signal to the GARM process.

You can add the following to your systemd unit file to enable `reload`:

```ini
[Service]
ExecReload=/bin/kill -HUP $MAINPID
```

Then you can simply:

```bash
systemctl reload garm
```

### The enable_log_streamer option

This option allows you to stream garm logs directly to your terminal. Set this option to true, then you can use the following command to stream logs:

```bash
garm-cli debug-log
```

An important note on enabling this option when behind a reverse proxy. The log streamer uses websockets to stream logs to you. You will need to configure your reverse proxy to allow websocket connections. If you're using nginx, you will need to add the following to your nginx `server` config:

```nginx
location /api/v1/ws {
    proxy_pass http://garm_backend;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "Upgrade";
    proxy_set_header Host $host;
}
```

## The logging section

GARM has switched to the `slog` package for logging, adding structured logging. As such, we added a dedicated `logging` section to the config to tweak the logging settings. We moved the `enable_log_streamer` and the `log_file` options from the `default` section to the `logging` section. They are still available in the `default` section for backwards compatibility, but they are deprecated and will be removed in a future release.

An example of the new `logging` section:

```toml
[logging]
# Uncomment this line if you'd like to log to a file instead of standard output.
# log_file = "/tmp/runner-manager.log"

# enable_log_streamer enables streaming the logs over websockets
enable_log_streamer = true
# log_format is the output format of the logs. GARM uses structured logging and can
# output as "text" or "json"
log_format = "text"
# log_level is the logging level GARM will output. Available log levels are:
#  * debug
#  * info
#  * warn
#  * error
log_level = "debug"
# log_source will output information about the function that generated the log line.
log_source = false
```

By default GARM logs everything to standard output. You can optionally log to file by adding the `log_file` option to the `logging` section. The `enable_log_streamer` option allows you to stream GARM logs directly to your terminal. Set this option to `true`, then you can use the following command to stream logs:

```bash
garm-cli debug-log
```

The `log_format`, `log_level` and `log_source` options allow you to tweak the logging output. The `log_format` option can be set to `text` or `json`. The `log_level` option can be set to `debug`, `info`, `warn` or `error`. The `log_source` option will output information about the function that generated the log line. All these options influence how the structured logging is output.

This will allow you to ingest GARM logs in a central location such as an ELK stack or similar.

## Database configuration

GARM currently supports SQLite3. Support for other stores will be added in the future.

```toml
[database]
  # Turn on/off debugging for database queries.
  debug = false
  # Database backend to use. Currently supported backends are:
  #   * sqlite3
  backend = "sqlite3"
  # the passphrase option is a temporary measure by which we encrypt the webhook
  # secret that gets saved to the database, using AES256. In the future, secrets
  # will be saved to something like Barbican or Vault, eliminating the need for
  # this. This string needs to be 32 characters in size.
  passphrase = "shreotsinWadquidAitNefayctowUrph"
  [database.sqlite3]
    # Path on disk to the sqlite3 database file.
    db_file = "/home/runner/garm.db"
```

## Provider configuration

GARM was designed to be extensible. Providers can be written as external executables which implement the needed interface to create/delete/list compute systems that are used by ```GARM``` to create runners.

### Providers

GARM delegates the functionality needed to create the runners to external executables. These executables can be either binaries or scripts. As long as they adhere to the needed interface, they can be used to create runners in any target IaaS. You might find this behavior familiar if you've ever had to deal with installing `CNIs` in `containerd`. The principle is the same.

The configuration for an external provider is quite simple:

```toml
# This is an example external provider. External providers are executables that
# implement the needed interface to create/delete/list compute systems that are used
# by GARM to create runners.
[[provider]]
name = "openstack_external"
description = "external openstack provider"
provider_type = "external"
  [provider.external]
  # config file passed to the executable via GARM_PROVIDER_CONFIG_FILE environment variable
  config_file = "/etc/garm/providers.d/openstack/keystonerc"
  # Absolute path to an executable that implements the provider logic. This executable can be
  # anything (bash, a binary, python, etc). See documentation in this repo on how to write an
  # external provider.
  provider_executable = "/etc/garm/providers.d/openstack/garm-external-provider"
  # This option will pass all environment variables that start with AWS_ to the provider.
  # To pass in individual variables, you can add the entire name to the list.
  environment_variables = ["AWS_"]
```

The external provider has three options:

* `provider_executable`
* `config_file`
* `environment_variables`

The ```provider_executable``` option is the absolute path to an executable that implements the provider logic. GARM will delegate all provider operations to this executable. This executable can be anything (bash, python, perl, go, etc). See [Writing an external provider](./external_provider.md) for more details.

The ```config_file``` option is a path on disk to an arbitrary file, that is passed to the external executable via the environment variable ```GARM_PROVIDER_CONFIG_FILE```. This file is only relevant to the external provider. GARM itself does not read it. Let's take the [OpenStack provider](https://github.com/cloudbase/garm-provider-openstack) as an example. The [config file](https://github.com/cloudbase/garm-provider-openstack/blob/ac46d4d5a542bca96cd0309c89437d3382c3ea26/testdata/config.toml) contains access information for an OpenStack cloud as well as some provider specific options like whether or not to boot from volume and which tenant network to use.

The `environment_variables` option is a list of environment variables that will be passed to the external provider. By default GARM will pass a clean env to providers, consisting only of variables that the [provider interface](./external_provider.md) expects. However, in some situations, provider may need access to certain environment variables set in the env of GARM itself. This might be needed to enable access to IAM roles (ec2) or managed identity (azure). This option takes a list of environment variables or prefixes of environment variables that will be passed to the provider. For example, if you want to pass all environment variables that start with `AWS_` to the provider, you can set this option to `["AWS_"]`.

If you want to implement an external provider, you can use this file for anything you need to pass into the binary when ```GARM``` calls it to execute a particular operation.

#### Available external providers

For non-testing purposes, these are the external providers currently available:

* [Amazon EC2](https://github.com/cloudbase/garm-provider-aws)
* [Azure](https://github.com/cloudbase/garm-provider-azure)
* [CloudStack](https://github.com/nexthop-ai/garm-provider-cloudstack)
* [Equinix Metal](https://github.com/cloudbase/garm-provider-equinix)
* [Google Cloud Platform (GCP)](https://github.com/cloudbase/garm-provider-gcp)
* [Incus](https://github.com/cloudbase/garm-provider-incus)
* [Kubernetes](https://github.com/mercedes-benz/garm-provider-k8s) - Thanks to the amazing folks at @mercedes-benz for sharing their awesome provider!
* [LXD](https://github.com/cloudbase/garm-provider-lxd)
* [OpenStack](https://github.com/cloudbase/garm-provider-openstack)
* [Oracle Cloud Infrastructure (OCI)](https://github.com/cloudbase/garm-provider-oci)

Details on how to install and configure them are available in their respective repositories.

If you wrote a provider and would like to add it to the above list, feel free to open a PR.


## The metrics section

This is one of the features in GARM that I really love having. For one thing, it's community contributed and for another, it really adds value to the project. It allows us to create some pretty nice visualizations of what is happening with GARM.

### Common metrics

| Metric name              | Type    | Labels                                                                                                                                                                                                                                              | Description                                                                                          |
|--------------------------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------|
| `garm_health`            | Gauge   | `controller_id`=&lt;controller id&gt; <br>`callback_url`=&lt;callback url&gt; <br>`controller_webhook_url`=&lt;controller webhook url&gt; <br>`metadata_url`=&lt;metadata url&gt; <br>`webhook_url`=&lt;webhook url&gt; <br>`name`=&lt;hostname&gt; | This is a gauge that is set to 1 if GARM is healthy and 0 if it is not. This is useful for alerting. |
| `garm_webhooks_received` | Counter | `valid`=&lt;valid request&gt; <br>`reason`=&lt;reason for invalid requests&gt;                                                                                                                                                                      | This is a counter that increments every time GARM receives a webhook from GitHub.                    |

### Enterprise metrics

| Metric name                           | Type  | Labels                                                                                          | Description                                                                                    |
|---------------------------------------|-------|-------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------|
| `garm_enterprise_info`                | Gauge | `id`=&lt;enterprise id&gt; <br>`name`=&lt;enterprise name&gt;                                   | This is a gauge that is set to 1 and expose enterprise information                             |
| `garm_enterprise_pool_manager_status` | Gauge | `id`=&lt;enterprise id&gt; <br>`name`=&lt;enterprise name&gt; <br>`running`=&lt;true\|false&gt; | This is a gauge that is set to 1 if the enterprise pool manager is running and set to 0 if not |

### Organization metrics

| Metric name                             | Type  | Labels                                                                                              | Description                                                                                      |
|-----------------------------------------|-------|-----------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------|
| `garm_organization_info`                | Gauge | `id`=&lt;organization id&gt; <br>`name`=&lt;organization name&gt;                                   | This is a gauge that is set to 1 and expose organization information                             |
| `garm_organization_pool_manager_status` | Gauge | `id`=&lt;organization id&gt; <br>`name`=&lt;organization name&gt; <br>`running`=&lt;true\|false&gt; | This is a gauge that is set to 1 if the organization pool manager is running and set to 0 if not |

### Repository metrics

| Metric name                           | Type  | Labels                                                                                          | Description                                                                                    |
|---------------------------------------|-------|-------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------|
| `garm_repository_info`                | Gauge | `id`=&lt;repository id&gt; <br>`name`=&lt;repository name&gt;                                   | This is a gauge that is set to 1 and expose repository information                             |
| `garm_repository_pool_manager_status` | Gauge | `id`=&lt;repository id&gt; <br>`name`=&lt;repository name&gt; <br>`running`=&lt;true\|false&gt; | This is a gauge that is set to 1 if the repository pool manager is running and set to 0 if not |

### Provider metrics

| Metric name          | Type  | Labels                                                                                                            | Description                                                      |
|----------------------|-------|-------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------|
| `garm_provider_info` | Gauge | `description`=&lt;provider description&gt; <br>`name`=&lt;provider name&gt; <br>`type`=&lt;internal\|external&gt; | This is a gauge that is set to 1 and expose provider information |

### Pool metrics

| Metric name                   | Type  | Labels                                                                                                                                                                                                                                                                                                                                                                               | Description                                                                 |
|-------------------------------|-------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------|
| `garm_pool_info`              | Gauge | `flavor`=&lt;flavor&gt; <br>`id`=&lt;pool id&gt; <br>`image`=&lt;image name&gt; <br>`os_arch`=&lt;defined OS arch&gt; <br>`os_type`=&lt;defined OS name&gt; <br>`pool_owner`=&lt;owner name&gt; <br>`pool_type`=&lt;repository\|organization\|enterprise&gt; <br>`prefix`=&lt;prefix&gt; <br>`provider`=&lt;provider name&gt; <br>`tags`=&lt;concatenated list of pool tags&gt; <br> | This is a gauge that is set to 1 and expose pool information                |
| `garm_pool_status`            | Gauge | `enabled`=&lt;true\|false&gt; <br>`id`=&lt;pool id&gt;                                                                                                                                                                                                                                                                                                                               | This is a gauge that is set to 1 if the pool is enabled and set to 0 if not |
| `garm_pool_bootstrap_timeout` | Gauge | `id`=&lt;pool id&gt;                                                                                                                                                                                                                                                                                                                                                                 | This is a gauge that is set to the pool bootstrap timeout                   |
| `garm_pool_max_runners`       | Gauge | `id`=&lt;pool id&gt;                                                                                                                                                                                                                                                                                                                                                                 | This is a gauge that is set to the pool max runners                         |
| `garm_pool_min_idle_runners`  | Gauge | `id`=&lt;pool id&gt;                                                                                                                                                                                                                                                                                                                                                                 | This is a gauge that is set to the pool min idle runners                    |

### Runner metrics

| Metric name                    | Type    | Labels                                                                                                                                                                                                                                                                                                                                                            | Description                                                                  |
|--------------------------------|---------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------|
| `garm_runner_status`           | Gauge   | `name`=&lt;runner name&gt; <br>`pool_owner`=&lt;owner name&gt; <br>`pool_type`=&lt;repository\|organization\|enterprise&gt; <br>`provider`=&lt;provider name&gt; <br>`runner_status`=&lt;running\|stopped\|error\|pending_delete\|deleting\|pending_create\|creating\|unknown&gt; <br>`status`=&lt;idle\|pending\|terminated\|installing\|failed\|active&gt; <br> | This is a gauge value that gives us details about the runners garm spawns    |
| `garm_runner_operations_total` | Counter | `provider`=&lt;provider name&gt; <br>`operation`=&lt;CreateInstance\|DeleteInstance\|GetInstance\|ListInstances\|RemoveAllInstances\|Start\Stop&gt;                                                                                                                                                                                                               | This is a counter that increments every time a runner operation is performed |
| `garm_runner_errors_total`     | Counter | `provider`=&lt;provider name&gt; <br>`operation`=&lt;CreateInstance\|DeleteInstance\|GetInstance\|ListInstances\|RemoveAllInstances\|Start\Stop&gt;                                                                                                                                                                                                               | This is a counter that increments every time a runner operation errored      |

### Job metrics

| Metric name       | Type  | Labels                                                                                                                                                                                                                                                                                                                                                                                                                    | Description                   |
|-------------------|-------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------|
| `garm_job_status` | Gauge | `job_id`=&lt;job id&gt; <br>`workflow_job_id`=&lt;workflow job id&gt; <br>`scaleset_job_id`=&lt;scaleset job id&gt; <br>`workflow_run_id`=&lt;workflow run id&gt; <br>`name`=&lt;job name&gt; <br>`status`=&lt;job status&gt; <br>`conclusion`=&lt;job conclusion&gt; <br>`runner_name`=&lt;runner name&gt; <br>`owner`=&lt;owner&gt; <br>`repository`=&lt;repository&gt; <br>`requested_labels`=&lt;requested labels&gt; | List of jobs and their status |

### Github metrics

| Metric name                           | Type    | Labels                                                                                                                 | Description                                                                           |
|---------------------------------------|---------|------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------|
| `garm_github_operations_total`        | Counter | `operation`=&lt;ListRunners\|CreateRegistrationToken\|...&gt; <br>`scope`=&lt;Organization\|Repository\|Enterprise&gt; | This is a counter that increments every time a github operation is performed          |
| `garm_github_errors_total`            | Counter | `operation`=&lt;ListRunners\|CreateRegistrationToken\|...&gt; <br>`scope`=&lt;Organization\|Repository\|Enterprise&gt; | This is a counter that increments every time a github operation errored               |
| `garm_github_rate_limit_limit`        | Gauge   | `credential_name`=&lt;credential name&gt; <br>`credential_id`=&lt;credential id&gt; <br>`endpoint`=&lt;endpoint name&gt; | The maximum number of requests allowed per hour for GitHub API                        |
| `garm_github_rate_limit_remaining`    | Gauge   | `credential_name`=&lt;credential name&gt; <br>`credential_id`=&lt;credential id&gt; <br>`endpoint`=&lt;endpoint name&gt; | The number of requests remaining in the current rate limit window                     |
| `garm_github_rate_limit_used`         | Gauge   | `credential_name`=&lt;credential name&gt; <br>`credential_id`=&lt;credential id&gt; <br>`endpoint`=&lt;endpoint name&gt; | The number of requests used in the current rate limit window                          |
| `garm_github_rate_limit_reset_timestamp` | Gauge   | `credential_name`=&lt;credential name&gt; <br>`credential_id`=&lt;credential id&gt; <br>`endpoint`=&lt;endpoint name&gt; | Unix timestamp when the rate limit resets                                             |

### Enabling metrics

Metrics are disabled by default. To enable them, add the following to your config file:

```toml
[metrics]

# Toggle to disable authentication (not recommended) on the metrics endpoint.
# If you do disable authentication, I encourage you to put a reverse proxy in front
# of garm and limit which systems can access that particular endpoint. Ideally, you
# would enable some kind of authentication using the reverse proxy, if the built-in auth
# is not sufficient for your needs.
#
# Default: false
disable_auth = true

# Toggle metrics. If set to false, the API endpoint for metrics collection will
# be disabled.
#
# Default: false
enable = true

# period is the time interval when the /metrics endpoint will update internal metrics about
# controller specific objects (e.g. runners, pools, etc.)
#
# Default: "60s"
period = "30s"
```

You can choose to disable authentication if you wish, however it's not terribly difficult to set up, so I generally advise against disabling it.

### Configuring prometheus

The following section assumes that your garm instance is running at `garm.example.com` and has TLS enabled.

First, generate a new JWT token valid only for the metrics endpoint:

```bash
garm-cli metrics-token create
```

Note: The token validity is equal to the TTL you set in the [JWT config section](#the-jwt-authentication-config-section).

Copy the resulting token, and add it to your prometheus config file. The following is an example of how to add garm as a target in your prometheus config file:

```yaml
scrape_configs:
  - job_name: "garm"
    # Connect over https. If you don't have TLS enabled, change this to http.
    scheme: https
    static_configs:
      - targets: ["garm.example.com"]
    authorization:
      credentials: "superSecretTokenYouGeneratedEarlier"
```

## The JWT authentication config section

This section configures the JWT authentication used by the API server. GARM is currently a single user system and that user has the right to do anything and everything GARM is capable of. As a result, the JWT auth we have does not include a refresh token. The token is valid for the duration of the time to live (TTL) set in the config file. Once the token expires, you will need to log in again.

It is recommended that the secret be a long, randomly generated string. Changing the secret at any time will invalidate all existing tokens.

```toml
[jwt_auth]
# A JWT token secret used to sign tokens. Obviously, this needs to be changed :).
secret = ")9gk_4A6KrXz9D2u`0@MPea*sd6W`%@5MAWpWWJ3P3EqW~qB!!(Vd$FhNc*eU4vG"

# Time to live for tokens. Both the instances and you will use JWT tokens to
# authenticate against the API. However, this TTL is applied only to tokens you
# get when logging into the API. The tokens issued to the instances we manage,
# have a TTL based on the runner bootstrap timeout set on each pool. The minimum
# TTL for this token is 24h.
time_to_live = "8760h"
```

## The API server config section

This section allows you to configure the GARM API server. The API server is responsible for serving all the API endpoints used by the `garm-cli`, the runners that phone home their status and by GitHub when it sends us webhooks.

The config options are fairly straight forward.

```toml
[apiserver]
  # Bind the API to this IP
  bind = "0.0.0.0"
  # Bind the API to this port
  port = 9997
  # Whether or not to set up TLS for the API endpoint. If this is set to true,
  # you must have a valid apiserver.tls section.
  use_tls = false
  # Set a list of allowed origins
  # By default, if this option is omitted or empty, we will check
  # only that the origin is the same as the originating server.
  # A literal of "*" will allow any origin
  cors_origins = ["*"]
  [apiserver.tls]
    # Path on disk to a x509 certificate bundle.
    # NOTE: if your certificate is signed by an intermediary CA, this file
    # must contain the entire certificate bundle needed for clients to validate
    # the certificate. This usually means concatenating the certificate and the
    # CA bundle you received.
    certificate = ""
    # The path on disk to the corresponding private key for the certificate.
    key = ""
  [apiserver.webui]
    enable = true
```

The GARM API server has the option to enable TLS, but I suggest you use a reverse proxy and enable TLS termination in that reverse proxy. There is an `nginx` sample in this repository with TLS termination enabled.

You can of course enable TLS in both garm and the reverse proxy. The choice is yours.
