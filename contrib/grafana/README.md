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
