# Scale Sets

Scale sets are an alternative to webhook-driven pools. Instead of relying on webhooks to know when jobs are queued, GARM subscribes to a GitHub message queue and receives runner requests directly from GitHub.

Scale sets were introduced by GitHub to improve the reliability and efficiency of runner scheduling. While webhooks work well most of the time, under heavy load they may not fire, or they may fire while the auto scaler is offline, leading to lost messages. Recovering from missed webhooks requires listing all workflow runs across repos -- infeasible at org or enterprise scale. Scale sets solve this by letting GARM subscribe to a message queue via HTTP long poll, receiving messages for that specific scale set. This reduces API calls, improves message delivery reliability, and allows GitHub to handle scheduling directly.

## Scale sets vs pools

| Feature | Pools | Scale Sets |
|---------|-------|------------|
| Job delivery | Webhooks (push) | Message queue (long poll) |
| Webhook setup required | Yes | No |
| Scheduling | GARM picks the pool | GitHub picks the scale set |
| Runner groups | Limited ([group not in webhook payload](https://github.com/orgs/community/discussions/158000)) | Full support |
| Missed events | Possible if GARM is down | Queued until GARM reconnects |
| Identifier | Labels (tags) | Scale set name + optional labels |

### When to use scale sets

- You want to eliminate the webhook endpoint from your security surface
- You need reliable runner group support
- You want guaranteed job delivery even during GARM restarts
- You're on GitHub.com or GHES 3.10+

### When to use pools

- You're using Gitea (scale sets are GitHub-only)
- You need simple label-based routing across multiple pools
- Your setup is already working well with webhooks

## Creating a scale set

```bash
garm-cli scaleset add \
  --repo <REPO_ID> \
  --provider-name incus \
  --image ubuntu:22.04 \
  --name my-scale-set \
  --flavor default \
  --enabled \
  --min-idle-runners=0 \
  --max-runners=20 \
  --labels=ubuntu,generic
```

Scale sets require a `--name` that must be unique within a runner group. Since GitHub handles scheduling for scale sets, the name is what workflows use to target a specific scale set (unlike pools where labels are used). You can also assign custom labels with `--labels`:

```bash
--labels=ubuntu,generic,gpu
```

> **Important:** Labels can only be set at creation time. They **cannot be updated** after the scale set is created. If you need different labels, delete and re-create the scale set.

Scale sets can be attached to `--repo`, `--org`, or `--enterprise`, just like pools.

## Managing scale sets

### List scale sets

```bash
garm-cli scaleset list --repo <REPO_ID>
```

### Show scale set details

```bash
garm-cli scaleset show <SCALESET_ID>
```

### Update a scale set

```bash
garm-cli scaleset update <SCALESET_ID> --max-runners=10
```

### List runners in a scale set

```bash
garm-cli scaleset runner list <SCALESET_ID>
```

### Rotate idle runners

Replace all idle runners with fresh instances:

```bash
garm-cli scaleset runner rotate <SCALESET_ID>
```

### Delete a scale set

```bash
garm-cli scaleset delete <SCALESET_ID>
```

## Targeting a scale set in workflows

Use the scale set name in `runs-on`:

```yaml
jobs:
  build:
    runs-on: my-scale-set
    steps:
      - uses: actions/checkout@v4
      - run: echo "Running on a GARM scale set runner!"
```
