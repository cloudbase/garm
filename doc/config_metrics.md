# The metrics section

This is one of the features in GARM that I really love having. For one thing, it's community contributed and for another, it really adds value to the project. It allows us to create some pretty nice visualizations of what is happening with GARM.

At the moment there are only three meaningful metrics being collected, besides the default ones that the prometheus golang package enables by default. These are:

* `garm_health` - This is a gauge that is set to 1 if GARM is healthy and 0 if it is not. This is useful for alerting.
* `garm_runner_status` - This is a gauge value that gives us details about the runners garm spawns
* `garm_webhooks_received` - This is a counter that increments every time GARM receives a webhook from GitHub.

More metrics will be added in the future.

## Enabling metrics

Metrics are disabled by default. To enable them, add the following to your config file:

```toml
[metrics]
# Toggle metrics. If set to false, the API endpoint for metrics collection will
# be disabled.
enable = true
# Toggle to disable authentication (not recommended) on the metrics endpoint.
# If you do disable authentication, I encourage you to put a reverse proxy in front
# of garm and limit which systems can access that particular endpoint. Ideally, you
# would enable some kind of authentication using the reverse proxy, if the built-in auth
# is not sufficient for your needs.
disable_auth = false
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