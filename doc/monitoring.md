# Monitoring and Debugging

GARM provides built-in tools for monitoring, live log streaming, event watching, and an interactive terminal dashboard.

- [Monitoring and Debugging](#monitoring-and-debugging)
  - [Prometheus metrics](#prometheus-metrics)
    - [Enable metrics](#enable-metrics)
    - [Generate a metrics token](#generate-a-metrics-token)
    - [Prometheus configuration](#prometheus-configuration)
    - [Metrics reference](#metrics-reference)
      - [Health](#health)
      - [Webhooks](#webhooks)
      - [Entities (repositories, organizations, enterprises)](#entities-repositories-organizations-enterprises)
      - [Providers](#providers)
      - [Pools](#pools)
      - [Scale sets](#scale-sets)
      - [Runner instances](#runner-instances)
      - [Runner lifecycle](#runner-lifecycle)
      - [Jobs](#jobs)
      - [GitHub/Gitea API](#githubgitea-api)
      - [Database watcher](#database-watcher)
  - [Live log streaming](#live-log-streaming)
    - [Filtering logs](#filtering-logs)
  - [Database events](#database-events)
    - [Event structure](#event-structure)
    - [Programmatic access](#programmatic-access)
  - [Interactive dashboard](#interactive-dashboard)
  - [Job monitoring](#job-monitoring)
  - [Reverse proxy considerations](#reverse-proxy-considerations)

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

Ready-made Grafana dashboards (fleet overview, jobs & SLOs, pools & scale sets, control plane) and starter Prometheus alert rules are available in [contrib/grafana](/contrib/grafana/README.md).

### Metrics reference

All metrics use the `garm_` namespace. Metrics fall into three groups:

- **Snapshot metrics** are reset and recomputed on every tick (default every 60s, configured via `period` in `[metrics]`). These reflect the current state: pools, instances, entities, jobs.
- **Cumulative metrics** are counters or gauges updated as GARM operates: webhooks received, provider operations, GitHub API calls, rate limits.
- **Event-driven metrics** are counters and histograms updated the moment something happens: runner lifecycle transitions, job queue/execution timings, scale set messages, watcher health. Unlike snapshot metrics, these capture transitions that happen between ticks. They are derived from database change events and from instrumentation at the relevant call sites, cost nothing to collect (no extra database queries or network calls) and are enabled together with the rest of the metrics.

#### Health

| Metric | Type | Labels |
| -------- | ------ | -------- |
| `garm_health` | Gauge | `metadata_url`, `callback_url`, `webhook_url`, `controller_webhook_url`, `controller_id` |
| `garm_build_info` | Gauge | `version`, `go_version` |

`garm_health` is set to 1 if GARM is healthy, 0 otherwise. Useful for alerting. `garm_build_info` is always 1; the labels carry the controller version — useful to track which controllers run which release across a fleet.

#### Webhooks

| Metric | Type | Labels |
| -------- | ------ | -------- |
| `garm_webhook_received` | Counter | `valid`, `reason` |

Increments on every webhook received from GitHub/Gitea. The `valid` label is `true`/`false`; `reason` explains why invalid webhooks were rejected.

#### Entities (repositories, organizations, enterprises)

| Metric | Type | Labels |
| -------- | ------ | -------- |
| `garm_repository_info` | Gauge | `name`, `id` |
| `garm_repository_pool_manager_status` | Gauge | `name`, `id`, `running` |
| `garm_organization_info` | Gauge | `name`, `id` |
| `garm_organization_pool_manager_status` | Gauge | `name`, `id`, `running` |
| `garm_enterprise_info` | Gauge | `name`, `id` |
| `garm_enterprise_pool_manager_status` | Gauge | `name`, `id`, `running` |

The `_info` gauges are always set to 1; the labels are what carry the information. The `pool_manager_status` gauges are 1 when the pool manager for that entity is running.

#### Providers

| Metric | Type | Labels |
| -------- | ------ | -------- |
| `garm_provider_info` | Gauge | `name`, `type`, `description` |
| `garm_provider_operation_duration_seconds` | Histogram | `provider`, `operation`, `pool_id`, `scaleset_id`, `entity_type`, `entity_id` |
| `garm_provider_operation_errors_total` | Counter | `provider`, `operation`, `pool_id`, `scaleset_id`, `entity_type`, `entity_id`, `error_kind` |

`garm_provider_operation_duration_seconds` observes the execution time of **successful** provider binary invocations. Failed invocations (including timeouts) are excluded so they don't skew latency percentiles; they are counted in `garm_provider_operation_errors_total` instead. The `operation` and identity labels take the same values as on `garm_runner_operations_total` (see [Runner instances](#runner-instances)).

The `error_kind` label takes one of:

| Error kind | Description |
| ------------ | ------------- |
| `timeout` | The provider binary exceeded its execution deadline and was killed |
| `provider_error` | The provider binary exited with an error |
| `decode_error` | The provider binary returned output that could not be decoded |
| `validation_error` | The provider binary returned output that failed validation |

> [!IMPORTANT]
> The identity labels come from the operation parameters GARM threads through internally, not from the provider interface itself, so they are populated for all operations on **both** provider interface versions. They are only empty on `RemoveAllInstances` (an administrative operation with no owning pool or scale set).

#### Pools

| Metric | Type | Labels |
| -------- | ------ | -------- |
| `garm_pool_info` | Gauge | `id`, `image`, `flavor`, `prefix`, `os_type`, `os_arch`, `tags`, `provider`, `pool_owner`, `pool_type` |
| `garm_pool_status` | Gauge | `id`, `enabled` |
| `garm_pool_max_runners` | Gauge | `id` |
| `garm_pool_min_idle_runners` | Gauge | `id` |
| `garm_pool_bootstrap_timeout` | Gauge | `id` |

#### Scale sets

| Metric | Type | Labels |
| -------- | ------ | -------- |
| `garm_scaleset_info` | Gauge | `id`, `scaleset_id`, `name`, `image`, `flavor`, `prefix`, `os_type`, `os_arch`, `tags`, `provider`, `runner_group`, `scaleset_owner`, `scaleset_type` |
| `garm_scaleset_status` | Gauge | `id`, `enabled`, `state` |
| `garm_scaleset_max_runners` | Gauge | `id` |
| `garm_scaleset_min_idle_runners` | Gauge | `id` |
| `garm_scaleset_desired_runner_count` | Gauge | `id` |
| `garm_scaleset_bootstrap_timeout` | Gauge | `id` |

The `id` label is GARM's internal scale set ID; `scaleset_id` is the numeric ID assigned by GitHub. `garm_scaleset_desired_runner_count` reflects the runner count GitHub has requested for the scale set (unique to scale sets, since GitHub drives scheduling).

Scale sets consume a long-poll message queue hosted by GitHub. The listener that services this queue exposes its own health metrics:

| Metric | Type | Labels |
| -------- | ------ | -------- |
| `garm_scaleset_messages_total` | Counter | `id`, `message_type` |
| `garm_scaleset_listener_last_success_timestamp` | Gauge | `id` |
| `garm_scaleset_listener_restarts_total` | Counter | `id` |

`garm_scaleset_messages_total` counts job messages received from GitHub, with `message_type` one of `JobAvailable`, `JobAssigned`, `JobStarted`, `JobCompleted`. This is the demand-signal feed: a scale set receiving `JobAvailable` but never `JobAssigned` means job acquisition is failing.

`garm_scaleset_listener_last_success_timestamp` is updated after every successful long-poll cycle, including empty ones (the broker holds the poll for ~50 seconds before returning nothing). A stale timestamp means the scale set has silently stopped receiving scaling signals — alert on `time() - garm_scaleset_listener_last_success_timestamp > 300`. The series is removed when the scale set worker stops, so deleted or disabled scale sets don't trigger false staleness alerts.

`garm_scaleset_listener_restarts_total` counts unexpected listener exits. Frequent restarts point at credential or network trouble with the message session.

#### Runner instances

| Metric | Type | Labels |
| -------- | ------ | -------- |
| `garm_runner_status` | Gauge | `name`, `status`, `runner_status`, `pool_owner`, `pool_type`, `pool_id`, `scaleset_id`, `provider` |
| `garm_runner_operations_total` | Counter | `operation`, `provider`, `pool_id`, `scaleset_id`, `entity_type`, `entity_id` |
| `garm_runner_errors_total` | Counter | `operation`, `provider`, `pool_id`, `scaleset_id`, `entity_type`, `entity_id` |

`garm_runner_status` covers both pool-owned and scale-set-owned runners. For any given series, exactly one of `pool_id` / `scaleset_id` is populated. `pool_owner` and `pool_type` describe the owning entity (repo/org/enterprise) and apply to both.

The identity labels on the operation counters attribute provider operations to the pool or scale set they were performed for (exactly one of `pool_id` / `scaleset_id` is populated, same as on `garm_runner_status`) and to the owning forge entity (`entity_type`, `entity_id`). This makes operations distinguishable even when multiple pools share a provider, or multiple controllers name their providers identically. The identity is threaded through the provider call parameters by both runner stacks and applies to both provider interface versions. Queries that aggregate (e.g. `sum by (operation, provider)`) are unaffected by the added labels.

The `operation` label on `garm_runner_operations_total` / `garm_runner_errors_total` takes one of these values:

| Operation | Description |
| ----------- | ------------- |
| `CreateInstance` | Create a new compute instance |
| `DeleteInstance` | Delete a compute instance |
| `GetInstance` | Get details about a compute instance |
| `ListInstances` | List all instances for a pool |
| `RemoveAllInstances` | Remove all instances created by a provider |
| `Start` | Boot up an instance |
| `Stop` | Shut down an instance |

#### Runner lifecycle

These metrics are event-driven: they observe runner state transitions as they happen, for both pool-owned and scale-set-owned runners.

The `pool_owner` label holds the owning entity's identifier: `owner/repo` for repository-level pools (a bare repo name would be ambiguous across owners), the organization or enterprise name otherwise. On `garm_runner_lifecycle_events_total`, exactly one of `pool_id` / `scaleset_id` is populated, same as on `garm_runner_status`.

| Metric | Type | Labels |
| -------- | ------ | -------- |
| `garm_runner_provision_duration_seconds` | Histogram | `provider`, `pool_owner`, `pool_type` |
| `garm_runner_ready_duration_seconds` | Histogram | `provider`, `pool_owner`, `pool_type` |
| `garm_runner_deletion_duration_seconds` | Histogram | `provider` |
| `garm_runner_lifecycle_events_total` | Counter | `outcome`, `provider`, `pool_owner`, `pool_type`, `pool_id`, `scaleset_id` |

`garm_runner_provision_duration_seconds` measures the time from runner creation until the provider reports the instance as running — essentially how long the cloud takes to boot an instance. `garm_runner_ready_duration_seconds` measures the time from runner creation until the runner first reports as idle or active on the forge — the end-to-end time until a runner can actually pick up jobs. The gap between the two is the cost of userdata setup and runner registration. `garm_runner_deletion_duration_seconds` measures the time from a runner being marked for deletion until it is fully removed; long tails point at stuck or failing provider deletions (leaked compute).

Durations are only observed for transitions the controller witnessed. A runner that was already mid-flight when the controller started is not observed, so a restart cannot produce bogus samples.

`garm_runner_lifecycle_events_total` counts runner **removal decisions** by reason. The `outcome` label takes one of:

| Outcome | Description |
| --------- | ------------- |
| `job_completed` | The runner finished its job and was scheduled for removal |
| `idle_scaledown` | An idle runner was removed to scale the pool down |
| `bootstrap_timeout` | The runner never became functional within the bootstrap timeout and was reaped |
| `provider_error` | The provider failed to create the instance |
| `orphaned` | Reconciliation found the runner missing from the forge or the provider |
| `manual_delete` | The runner was deleted explicitly through the API |
| `startup_recovery` | The runner was cleaned up during controller startup recovery |

A spike in `bootstrap_timeout` typically means a broken image, network or userdata; `provider_error` points at the IaaS. This is usually the first metric to alert on for pool health.

#### Jobs

| Metric | Type | Labels |
| -------- | ------ | -------- |
| `garm_job_status` | Gauge | `job_id`, `workflow_job_id`, `scaleset_job_id`, `workflow_run_id`, `name`, `status`, `conclusion`, `runner_name`, `owner`, `repository`, `requested_labels` |
| `garm_job_queue_duration_seconds` | Histogram | `owner` |
| `garm_job_execution_duration_seconds` | Histogram | `owner` |
| `garm_job_completed_total` | Counter | `owner`, `repository`, `conclusion` |

`garm_job_queue_duration_seconds` measures how long jobs wait between being queued and starting to run — the primary end-user SLO for a CI platform. `garm_job_execution_duration_seconds` measures how long jobs run once started, which is what capacity planning is based on. Both are labeled by `owner` only, to keep histogram cardinality bounded on controllers managing many repositories.

`garm_job_completed_total` counts finished jobs by `conclusion` (`success`, `failure`, `cancelled`, `timed_out`, ...). Unlike the `garm_job_status` snapshot gauge — which forgets jobs once they are pruned from the database — this counter is a reliable long-term throughput measure ("how many jobs did this controller run last month").

#### GitHub/Gitea API

| Metric | Type | Labels |
| -------- | ------ | -------- |
| `garm_github_operations_total` | Counter | `operation`, `scope` |
| `garm_github_errors_total` | Counter | `operation`, `scope` |
| `garm_github_rate_limit_limit` | Gauge | `credential_name`, `credential_id`, `endpoint` |
| `garm_github_rate_limit_remaining` | Gauge | `credential_name`, `credential_id`, `endpoint` |
| `garm_github_rate_limit_used` | Gauge | `credential_name`, `credential_id`, `endpoint` |
| `garm_github_rate_limit_reset_timestamp` | Gauge | `credential_name`, `credential_id`, `endpoint` |

The `scope` label is `Repository`, `Organization`, or `Enterprise`. The `operation` label takes one of the values listed below.

**GitHub client operations** (hooks, runners, registration tokens):

| Operation | Description |
| ----------- | ------------- |
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
| ----------- | ------------- |
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

#### Database watcher

GARM's internal workers (caches, entity workers, scale set workers, the provider worker) are driven by database change events published through the watcher. These metrics expose the health of that pipeline.

| Metric | Type | Labels |
| -------- | ------ | -------- |
| `garm_watcher_events_total` | Counter | `entity_type`, `operation` |
| `garm_watcher_notify_timeouts_total` | Counter | (none) |
| `garm_watcher_consumer_queue_depth` | Gauge | `consumer` |

`garm_watcher_events_total` counts every database change event published to the watcher — the change-feed heartbeat of the controller. The `entity_type` and `operation` labels match the values documented under [Database events](#database-events).

`garm_watcher_notify_timeouts_total` counts change notifications that were **dropped** because the watcher could not accept them within the notify timeout. The database write itself succeeded, but downstream consumers never saw the event, so in-memory state may have diverged from the database. Any nonzero value is worth alerting on.

`garm_watcher_consumer_queue_depth` reports, at scrape time, the number of undelivered events queued per consumer kind. Events are never dropped at the consumer — a slow consumer only grows its own queue — so sustained growth here is the early warning that a worker is wedged or overloaded. Consumer IDs that embed entity or user identifiers are normalized to a bounded set of kinds (`entity-worker`, `scaleset-worker`, `scaleset-controller`, `pool-manager`, `ws-event-watcher`, `agent-worker`, plus the static IDs `cache`, `provider-worker`, `entity-controller`, `garm-tools-sync`, `metrics`).

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
