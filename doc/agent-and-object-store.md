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
