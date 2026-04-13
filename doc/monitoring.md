# Monitoring and Debugging

GARM provides built-in tools for monitoring, live log streaming, event watching, and an interactive terminal dashboard.

<!-- TOC -->

- [Monitoring and Debugging](#monitoring-and-debugging)
    - [Prometheus metrics](#prometheus-metrics)
        - [Enable metrics](#enable-metrics)
        - [Generate a metrics token](#generate-a-metrics-token)
        - [Prometheus configuration](#prometheus-configuration)
        - [Metrics reference](#metrics-reference)
            - [Health](#health)
            - [Webhooks](#webhooks)
            - [Entities repositories organizations enterprises](#entities-repositories-organizations-enterprises)
            - [Providers](#providers)
            - [Pools](#pools)
            - [Runner instances](#runner-instances)
            - [Jobs](#jobs)
            - [GitHubGitea API](#githubgitea-api)
    - [Live log streaming](#live-log-streaming)
        - [Filtering logs](#filtering-logs)
    - [Database events](#database-events)
        - [Event structure](#event-structure)
        - [Programmatic access](#programmatic-access)
    - [Interactive dashboard](#interactive-dashboard)
    - [Job monitoring](#job-monitoring)
    - [Reverse proxy considerations](#reverse-proxy-considerations)

<!-- /TOC -->

## Prometheus metrics

### Enable metrics

In `config.toml`:

```toml
[metrics]
enable = true
disable_auth = false
```

### Generate a metrics token

```bash
garm-cli metrics-token create
```

The token validity matches the `time_to_live` in `[jwt_auth]`.

### Prometheus configuration

```yaml
scrape_configs:
  - job_name: "garm"
    scheme: https
    static_configs:
      - targets: ["garm.example.com"]
    authorization:
      credentials: "your-metrics-token"
```

### Metrics reference

All metrics use the `garm_` namespace. Metrics fall into two groups:

- **Snapshot metrics** are reset and recomputed on every tick (default every 60s, configured via `period` in `[metrics]`). These reflect the current state: pools, instances, entities, jobs.
- **Cumulative metrics** are counters or gauges updated as GARM operates: webhooks received, provider operations, GitHub API calls, rate limits.

#### Health

| Metric | Type | Labels |
|--------|------|--------|
| `garm_health` | Gauge | `metadata_url`, `callback_url`, `webhook_url`, `controller_webhook_url`, `controller_id` |

Set to 1 if GARM is healthy, 0 otherwise. Useful for alerting.

#### Webhooks

| Metric | Type | Labels |
|--------|------|--------|
| `garm_webhook_received` | Counter | `valid`, `reason` |

Increments on every webhook received from GitHub/Gitea. The `valid` label is `true`/`false`; `reason` explains why invalid webhooks were rejected.

#### Entities (repositories, organizations, enterprises)

| Metric | Type | Labels |
|--------|------|--------|
| `garm_repository_info` | Gauge | `name`, `id` |
| `garm_repository_pool_manager_status` | Gauge | `name`, `id`, `running` |
| `garm_organization_info` | Gauge | `name`, `id` |
| `garm_organization_pool_manager_status` | Gauge | `name`, `id`, `running` |
| `garm_enterprise_info` | Gauge | `name`, `id` |
| `garm_enterprise_pool_manager_status` | Gauge | `name`, `id`, `running` |

The `_info` gauges are always set to 1; the labels are what carry the information. The `pool_manager_status` gauges are 1 when the pool manager for that entity is running.

#### Providers

| Metric | Type | Labels |
|--------|------|--------|
| `garm_provider_info` | Gauge | `name`, `type`, `description` |

#### Pools

| Metric | Type | Labels |
|--------|------|--------|
| `garm_pool_info` | Gauge | `id`, `image`, `flavor`, `prefix`, `os_type`, `os_arch`, `tags`, `provider`, `pool_owner`, `pool_type` |
| `garm_pool_status` | Gauge | `id`, `enabled` |
| `garm_pool_max_runners` | Gauge | `id` |
| `garm_pool_min_idle_runners` | Gauge | `id` |
| `garm_pool_bootstrap_timeout` | Gauge | `id` |

> [!NOTE]
> Pool metrics only cover pools, not scale sets. Scale sets currently have no dedicated metrics (but jobs from scale sets are captured by `garm_job_status` via the `scaleset_job_id` label).

#### Runner instances

| Metric | Type | Labels |
|--------|------|--------|
| `garm_runner_status` | Gauge | `name`, `status`, `runner_status`, `pool_owner`, `pool_type`, `pool_id`, `provider` |
| `garm_runner_operations_total` | Counter | `operation`, `provider` |
| `garm_runner_errors_total` | Counter | `operation`, `provider` |

The `operation` label on `garm_runner_operations_total` / `garm_runner_errors_total` takes one of these values:

| Operation | Description |
|-----------|-------------|
| `CreateInstance` | Create a new compute instance |
| `DeleteInstance` | Delete a compute instance |
| `GetInstance` | Get details about a compute instance |
| `ListInstances` | List all instances for a pool |
| `RemoveAllInstances` | Remove all instances created by a provider |
| `Start` | Boot up an instance |
| `Stop` | Shut down an instance |

#### Jobs

| Metric | Type | Labels |
|--------|------|--------|
| `garm_job_status` | Gauge | `job_id`, `workflow_job_id`, `scaleset_job_id`, `workflow_run_id`, `name`, `status`, `conclusion`, `runner_name`, `owner`, `repository`, `requested_labels` |

#### GitHub/Gitea API

| Metric | Type | Labels |
|--------|------|--------|
| `garm_github_operations_total` | Counter | `operation`, `scope` |
| `garm_github_errors_total` | Counter | `operation`, `scope` |
| `garm_github_rate_limit_limit` | Gauge | `credential_name`, `credential_id`, `endpoint` |
| `garm_github_rate_limit_remaining` | Gauge | `credential_name`, `credential_id`, `endpoint` |
| `garm_github_rate_limit_used` | Gauge | `credential_name`, `credential_id`, `endpoint` |
| `garm_github_rate_limit_reset_timestamp` | Gauge | `credential_name`, `credential_id`, `endpoint` |

The `scope` label is `Repository`, `Organization`, or `Enterprise`. The `operation` label takes one of the values listed below.

**GitHub client operations** (hooks, runners, registration tokens):

| Operation | Description |
|-----------|-------------|
| `ListHooks` | List webhooks on an entity |
| `GetHook` | Get a single webhook |
| `CreateHook` | Create a webhook |
| `DeleteHook` | Delete a webhook |
| `PingHook` | Ping a webhook |
| `ListEntityRunners` | List runners for an entity |
| `ListEntityRunnerApplicationDownloads` | List runner application downloads |
| `RemoveEntityRunner` | Remove a runner from an entity |
| `CreateEntityRegistrationToken` | Create a runner registration token |
| `ListOrganizationRunnerGroups` | List organization runner groups |
| `ListRunnerGroups` | List enterprise runner groups |
| `GetEntityJITConfig` | Generate a JIT runner configuration |
| `GetRateLimit` | Fetch API rate limit information |

**Scale set operations** (scale set management and message queue):

| Operation | Description |
|-----------|-------------|
| `GetRunnerScaleSetByNameAndRunnerGroup` | Look up a scale set by name and runner group |
| `GetRunnerScaleSetByID` | Look up a scale set by ID |
| `ListRunnerScaleSets` | List all scale sets |
| `CreateRunnerScaleSet` | Create a scale set |
| `UpdateRunnerScaleSet` | Update a scale set |
| `DeleteRunnerScaleSet` | Delete a scale set |
| `GetRunnerGroupByName` | Look up a runner group by name |
| `GenerateJitRunnerConfig` | Generate a JIT runner config for a scale set |
| `GetRunner` | Get a runner by ID |
| `ListAllRunners` | List all runners |
| `GetRunnerByName` | Get a runner by name |
| `RemoveRunner` | Remove a scale set runner |
| `AcquireJobs` | Acquire jobs for a scale set |
| `GetAcquirableJobs` | Get acquirable jobs for a scale set |
| `GetActionServiceInfo` | Get actions service admin info |
| `CreateMessageSession` | Create a message queue session |
| `DeleteMessageSession` | Delete a message queue session |
| `RefreshMessageSession` | Refresh a message queue session token |
| `GetMessage` | Get a message from the message queue |
| `DeleteMessage` | Delete a message from the message queue |

## Live log streaming

Stream GARM logs to your terminal in real time:

```bash
garm-cli debug-log
```

This requires `enable_log_streamer = true` in `[logging]`.

### Filtering logs

```bash
# Only ERROR level and above
garm-cli debug-log --log-level ERROR

# Filter by attribute
garm-cli debug-log --filter "pool_id=9daa34aa-..."

# Filter by message content
garm-cli debug-log --filter "msg=creating instance"

# Multiple filters (OR by default)
garm-cli debug-log --filter "pool_id=abc" --filter "pool_id=def"

# Multiple filters with AND
garm-cli debug-log --filter "pool_id=abc" --filter "msg=error" --filter-mode all
```

> [!IMPORTANT]
> The log streaming and events WebSocket endpoints are authenticated, but you should still only expose them within trusted networks. If GARM is behind a reverse proxy, restrict access to the `/api/v1/ws` path from untrusted sources.

## Database events

The `debug-events` command consumes database change events. Whenever an entity is created, updated, or deleted in the database, an event is generated and exported via WebSocket. This endpoint is designed for integration -- external tools can subscribe without polling the API.

Watch real-time entity changes:

```bash
# All events
garm-cli debug-events --filters='{"send-everything": true}'

# Only instance create/delete events
garm-cli debug-events --filters='{"filters": [{"entity-type": "instance", "operations": ["create", "delete"]}]}'
```

Available entity types: `repository`, `organization`, `enterprise`, `pool`, `user`, `instance`, `job`, `controller`, `github_credentials`, `gitea_credentials`, `github_endpoint`, `scaleset`

Operations: `create`, `update`, `delete`

### Event structure

Each event is a JSON object:

```json
{
    "entity-type": "instance",
    "operation": "create",
    "payload": { ... }
}
```

The `payload` contains the same JSON you would get from the corresponding REST API endpoint. Sensitive data (tokens, keys) is stripped. For `delete` operations, some entities return the full object prior to deletion while others return only the `ID`. Assume that future versions will return only the `ID` for all delete operations.

### Programmatic access

The events endpoint is a WebSocket at `/api/v1/ws/events`. Connect with a JWT token and send a filter message to start receiving events. By default, the endpoint returns no events -- all events are filtered until you send a filter message:

```json
// Receive all events
{"send-everything": true}

// Receive only specific entity/operation combinations
{
  "filters": [
    {"entity-type": "instance", "operations": ["create", "delete"]},
    {"entity-type": "pool", "operations": ["update"]}
  ]
}
```

See the [events documentation](https://github.com/cloudbase/garm/blob/main/doc/events.md) for the full filter schema and a Go code example using `garm-provider-common`.

## Interactive dashboard

The `top` command shows a live terminal dashboard:

```bash
garm-cli top
```

This displays entities, pools, scale sets, runner instances, and jobs in an interactive view, refreshing every 5 seconds.

## Job monitoring

View recorded workflow jobs:

```bash
garm-cli job list
```

GARM only records jobs for which it has a matching pool or scale set. Jobs whose labels don't match any configured pool are silently ignored -- there's no point in recording jobs GARM can't act on. If you've set everything up but `garm-cli job list` is empty, verify that your webhook URLs are correct and that GitHub can reach them (see [Controller settings](managing-entities.md#controller-settings)).

## Reverse proxy considerations

If GARM is behind a reverse proxy, the WebSocket endpoints need special configuration. For nginx:

```nginx
location /api/v1/ws {
    proxy_pass http://garm_backend;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "Upgrade";
    proxy_set_header Host $host;
}
```

This is required for `debug-log`, `debug-events`, `top`, and the Web UI. A full sample nginx config with TLS termination is available in the [testdata folder](/testdata/nginx-server.conf).
