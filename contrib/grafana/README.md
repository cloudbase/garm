# GARM Grafana dashboards

Ready-made Grafana dashboards for a GARM deployment, built on the metrics
documented in [doc/monitoring.md](../../doc/monitoring.md). They work with
Grafana 10+ and any Prometheus-compatible datasource (Prometheus, Mimir,
VictoriaMetrics, Thanos).

| Dashboard | File | What it answers |
| ----------- | ------ | ----------------- |
| GARM / Fleet Overview | `dashboards/garm-fleet-overview.json` | Is the fleet healthy, how much capacity is in use, why are runners being removed |
| GARM / Jobs & SLOs | `dashboards/garm-jobs.json` | How long do jobs wait for runners, throughput and success rates per owner/repo |
| GARM / Pools & Scale Sets | `dashboards/garm-pools-scalesets.json` | Per-pool utilization and provider health, scale set demand and listener freshness |
| GARM / Control Plane | `dashboards/garm-control-plane.json` | Watcher pipeline health, forge API usage and errors, provider latency |

All dashboards share a `datasource` variable, are tagged `garm` and
cross-link through the dashboard link dropdown.

## Importing

**Via the UI:** Dashboards → New → Import → upload the JSON file, then pick
your Prometheus datasource.

**Via provisioning:** drop the JSON files into your provisioning path and add
a provider:

```yaml
# /etc/grafana/provisioning/dashboards/garm.yaml
apiVersion: 1
providers:
  - name: garm
    folder: GARM
    type: file
    options:
      path: /var/lib/grafana/dashboards/garm
```

## Scraping GARM

Enabling metrics, generating a scrape token and configuring Prometheus are
covered in [doc/monitoring.md](../../doc/monitoring.md). One
dashboard-specific note: a scrape interval of 30s or lower is recommended —
the queue-time heatmap and listener freshness panels benefit from it.

## Alerting

Starter Prometheus alert rules covering the failure modes these dashboards
surface are in [`alerts/garm-alerts.yaml`](alerts/garm-alerts.yaml). Adjust
the queue-time SLO threshold to your own target before deploying.

## Recording rules

The per-runner and per-job snapshot gauges (`garm_runner_status`,
`garm_job_status`, `garm_job_scaleset_status`) carry instance identity in
their labels, so series come and go as runners cycle. On deployments with
high runner turnover, querying them directly from dashboards and alerts
gets expensive.

[`rules/garm-recording-rules.yaml`](rules/garm-recording-rules.yaml)
pre-aggregates them into low-cardinality series whose label values are
bounded by the number of entities, pools, scale sets and providers, not by
runner or job identity:

| Recorded series | What it counts |
| ----------------- | ---------------- |
| `garm:runner:count` | Runners by status, owning entity and provider |
| `garm:pool_runner:count` | Runners per pool, by status |
| `garm:scaleset_runner:count` | Runners per scale set, by status |
| `garm:scaleset_runner:count:named` | Same, with the scale set name joined in |
| `garm:job:count` | Webhook jobs by status, owner and repository |
| `garm:pool_job:count` | Webhook jobs per pool, by status. The pool is known once one of its runners picks the job up |
| `garm:scaleset_job:count` | Scale set jobs per scale set, by status. `status="queued"` is the queued demand to watch when sizing `max_runners` |
| `garm:scaleset_job:count:named` | Same, with the scale set name joined in |

Load them by adding the file to `rule_files` in your Prometheus
configuration:

```yaml
rule_files:
  - /etc/prometheus/rules/garm-recording-rules.yaml
```
