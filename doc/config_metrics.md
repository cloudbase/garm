# The metrics section

This is one of the features in GARM that I really love having. For one thing, it's community contributed and for another, it really adds value to the project. It allows us to create some pretty nice visualizations of what is happening with GARM.

## Common metrics

| Metric name              | Type    | Labels                                                                                                                                                                                                                                              | Description                                                                                          |
|--------------------------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------|
| `garm_health`            | Gauge   | `controller_id`=&lt;controller id&gt; <br>`callback_url`=&lt;callback url&gt; <br>`controller_webhook_url`=&lt;controller webhook url&gt; <br>`metadata_url`=&lt;metadata url&gt; <br>`webhook_url`=&lt;webhook url&gt; <br>`name`=&lt;hostname&gt; | This is a gauge that is set to 1 if GARM is healthy and 0 if it is not. This is useful for alerting. |
| `garm_webhooks_received` | Counter | `valid`=&lt;valid request&gt; <br>`reason`=&lt;reason for invalid requests&gt;                                                                                                                                                                      | This is a counter that increments every time GARM receives a webhook from GitHub.                    |

## Enterprise metrics

| Metric name                           | Type  | Labels                                                                                          | Description                                                                                    |
|---------------------------------------|-------|-------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------|
| `garm_enterprise_info`                | Gauge | `id`=&lt;enterprise id&gt; <br>`name`=&lt;enterprise name&gt;                                   | This is a gauge that is set to 1 and expose enterprise information                             |
| `garm_enterprise_pool_manager_status` | Gauge | `id`=&lt;enterprise id&gt; <br>`name`=&lt;enterprise name&gt; <br>`running`=&lt;true\|false&gt; | This is a gauge that is set to 1 if the enterprise pool manager is running and set to 0 if not |

## Organization metrics

| Metric name                             | Type  | Labels                                                                                              | Description                                                                                      |
|-----------------------------------------|-------|-----------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------|
| `garm_organization_info`                | Gauge | `id`=&lt;organization id&gt; <br>`name`=&lt;organization name&gt;                                   | This is a gauge that is set to 1 and expose organization information                             |
| `garm_organization_pool_manager_status` | Gauge | `id`=&lt;organization id&gt; <br>`name`=&lt;organization name&gt; <br>`running`=&lt;true\|false&gt; | This is a gauge that is set to 1 if the organization pool manager is running and set to 0 if not |

## Repository metrics

| Metric name                           | Type  | Labels                                                                                          | Description                                                                                    |
|---------------------------------------|-------|-------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------|
| `garm_repository_info`                | Gauge | `id`=&lt;repository id&gt; <br>`name`=&lt;repository name&gt;                                   | This is a gauge that is set to 1 and expose repository information                             |
| `garm_repository_pool_manager_status` | Gauge | `id`=&lt;repository id&gt; <br>`name`=&lt;repository name&gt; <br>`running`=&lt;true\|false&gt; | This is a gauge that is set to 1 if the repository pool manager is running and set to 0 if not |

## Provider metrics

| Metric name          | Type  | Labels                                                                                                            | Description                                                      |
|----------------------|-------|-------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------|
| `garm_provider_info` | Gauge | `description`=&lt;provider description&gt; <br>`name`=&lt;provider name&gt; <br>`type`=&lt;internal\|external&gt; | This is a gauge that is set to 1 and expose provider information |

## Pool metrics

| Metric name                   | Type  | Labels                                                                                                                                                                                                                                                                                                                                                                               | Description                                                                 |
|-------------------------------|-------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------|
| `garm_pool_info`              | Gauge | `flavor`=&lt;flavor&gt; <br>`id`=&lt;pool id&gt; <br>`image`=&lt;image name&gt; <br>`os_arch`=&lt;defined OS arch&gt; <br>`os_type`=&lt;defined OS name&gt; <br>`pool_owner`=&lt;owner name&gt; <br>`pool_type`=&lt;repository\|organization\|enterprise&gt; <br>`prefix`=&lt;prefix&gt; <br>`provider`=&lt;provider name&gt; <br>`tags`=&lt;concatenated list of pool tags&gt; <br> | This is a gauge that is set to 1 and expose pool information                |
| `garm_pool_status`            | Gauge | `enabled`=&lt;true\|false&gt; <br>`id`=&lt;pool id&gt;                                                                                                                                                                                                                                                                                                                               | This is a gauge that is set to 1 if the pool is enabled and set to 0 if not |
| `garm_pool_bootstrap_timeout` | Gauge | `id`=&lt;pool id&gt;                                                                                                                                                                                                                                                                                                                                                                 | This is a gauge that is set to the pool bootstrap timeout                   |
| `garm_pool_max_runners`       | Gauge | `id`=&lt;pool id&gt;                                                                                                                                                                                                                                                                                                                                                                 | This is a gauge that is set to the pool max runners                         |
| `garm_pool_min_idle_runners`  | Gauge | `id`=&lt;pool id&gt;                                                                                                                                                                                                                                                                                                                                                                 | This is a gauge that is set to the pool min idle runners                    |

## Runner metrics

| Metric name          | Type  | Labels                                                                                                                                                                                                                                                                                                                                                            | Description                                                               |
|----------------------|-------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------|
| `garm_runner_status` | Gauge | `name`=&lt;runner name&gt; <br>`pool_owner`=&lt;owner name&gt; <br>`pool_type`=&lt;repository\|organization\|enterprise&gt; <br>`provider`=&lt;provider name&gt; <br>`runner_status`=&lt;running\|stopped\|error\|pending_delete\|deleting\|pending_create\|creating\|unknown&gt; <br>`status`=&lt;idle\|pending\|terminated\|installing\|failed\|active&gt; <br> | This is a gauge value that gives us details about the runners garm spawns |

More metrics will be added in the future.

## Enabling metrics

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

## Configuring prometheus

The following section assumes that your garm instance is running at `garm.example.com` and has TLS enabled.

First, generate a new JWT token valid only for the metrics endpoint:

```bash
garm-cli metrics-token create
```

Note: The token validity is equal to the TTL you set in the [JWT config section](/doc/config_jwt_auth.md).

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