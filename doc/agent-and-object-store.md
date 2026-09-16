# Agent Mode and Object Store

## Agent mode

Agent mode is an alternative to the traditional callback mechanism. Instead of runners calling back to GARM via HTTP, they establish a persistent WebSocket connection through `garm-agent`. This enables:

- Bidirectional communication between GARM and runners
- Remote shell access to runner instances (when enabled)
- More reliable status reporting over unstable networks

### Enabling agent tools sync

GARM can automatically sync `garm-agent` binaries from GitHub releases:

```bash
garm-cli controller update \
  --garm-tools-url https://api.github.com/repos/cloudbase/garm-agent/releases \
  --enable-tools-sync
```

Verify sync status:

```bash
garm-cli controller show
```

### Pinning the garm-agent version

By default, GARM tracks the **latest** garm-agent release from the configured tools URL. If a new agent release introduces a requirement your environment is not ready for, you can pin the controller to a known good version:

```bash
garm-cli controller update --garm-agent-version=v0.1.0
```

Setting it back to `latest` resumes tracking the newest release:

```bash
garm-cli controller update --garm-agent-version=latest
```

To see what is available, list the synced tools and the releases known from the cached release index (the pinned version and what `latest` resolves to are marked):

```bash
garm-cli controller tools list
garm-cli controller tools list --online
```

Release notes for a specific version can be viewed directly from the CLI:

```bash
garm-cli controller tools show-release v0.1.1
```

### Allowing insecure agent connections

Agents connect back to GARM over TLS. For local development, testing, or as a temporary stopgap while an environment transitions to TLS, deployed agents can be configured to connect over plain http/ws:

```bash
garm-cli controller update --allow-insecure-agent=true
```

> [!CAUTION]
> With this enabled, the agent token is sent in plain text. Do not use it in production. This option requires garm-agent v0.1.1 or newer.

### Enabling shell access on a pool

```bash
garm-cli pool update <POOL_ID> --enable-shell=true
```

### Agent URL

The agent URL is initialized when installing the controller. Only change it if you want fine grained controll or if you plan to place a reverse proxy in front that differs from the rest of the URLs.

The agent URL must be reachable by runner instances:

```bash
garm-cli controller update --agent-url https://garm.example.com/agent
```

## Object store

GARM includes a simple database-backed object storage system for storing files like provider binaries, agent binaries, and runner tools.

### Upload a file

```bash
garm-cli object create \
  --name garm-agent-linux-amd64 \
  --description "Linux AMD64 garm-agent binary" \
  --path /path/to/garm-agent \
  --tags "binary,os_type=linux,arch=amd64"
```

### List objects

```bash
# All objects
garm-cli object list

# Filter by tags
garm-cli object list --tags "binary,os_type=linux"
```

### Download an object

```bash
garm-cli object download <OBJECT_ID>
```

### Show object details

```bash
garm-cli object show <OBJECT_ID>
```

### Update object metadata

```bash
garm-cli object update <OBJECT_ID> --name new-name --tags "new,tags"
```

### Delete an object

```bash
garm-cli object remove <OBJECT_ID>
```
