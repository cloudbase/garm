# GARM SPA (SvelteKit)

This is a Single Page Application (SPA) implementation of the GARM web interface using SvelteKit.

## Features

- **Lightweight**: Uses SvelteKit for minimal bundle size and fast performance
- **Modern**: TypeScript-first development with full type safety
- **Responsive**: Mobile-first design using Tailwind CSS
- **Real-time**: WebSocket integration for live updates
- **API-driven**: Uses the existing GARM REST API endpoints

### Quick Start

1. **Clone the repository** (if not already done)

```bash
git clone https://github.com/cloudbase/garm.git
cd garm
```

2. **Build and test GARM with embedded webapp**

```bash
# You can skip this command if you made no changes to the webapp.
make build-webui
# builds the binary, with the web UI embedded.
make build
```

Make sure you enable the webui in the config:

```toml
[apiserver.webui]
  enable=true
```

3. **Access the webapp**
   - Navigate to `http://localhost:9997/ui/` (or your configured fqdn and port)

### Development Workflow

See the [DEV_SETUP.md](DEV_SETUP.md) file.

### Git Workflow

**DO NOT commit** the following directories:
- `webapp/node_modules/` - Dependencies (managed by package-lock.json)  
- `webapp/.svelte-kit/` - Build cache and generated files
- `webapp/build/` - Production build output

These are already included in `.gitignore`. Only commit source files in `webapp/src/` and configuration files.

### API Client Generation

The webapp uses auto-generated TypeScript clients from the GARM OpenAPI spec using `go generate`. To regenerate the clients, mocks and everything else, run:

```bash
go generate ./...
```

In the root folder of the project.

>[!NOTE]
> See [DEV_SETUP.md](DEV_SETUP.md) for prerequisites, before you try to generate the files.

### Asset Serving

The webapp is embedded using Go's `embed` package in `webapp/assets/assets.go`:

```go
//go:embed all:*
var EmbeddedSPA embed.FS
```

This allows GARM to serve the entire webapp with zero external dependencies. The webapp assets are compiled into the Go binary at build time.

## Running GARM behind a reverse proxy

In production, GARM will serve the web UI and assets from the embedded files inside the binary. The web UI also relies on the [events](/doc/events.md) API for real-time updates.

To have a fully working experience, you will need to configure your reverse proxy to allow websocket upgrades. For an `nginx` example, see [the sample config in the testdata folder](/testdata/nginx-server.conf).

Additionally, in production you can also override the default web UI that is embedded in GARM, without updating the garm binary. To do that, build the webapp, place it in the document root of `nginx` and create a new `location /ui` config in nginx. Something like the following should work:

```
    # Place this before the proxy_pass location
    location ~ ^/ui(/.*)?$ {
        root /var/www/html/garm-webui/;
    }

    location / {
        proxy_set_header X-Forwarded-For $remote_addr;
        proxy_set_header X-Forwarded-Host $http_host;

        proxy_pass http://garm_backend;
        proxy_set_header        Host    $Host;
        proxy_redirect off;
    }
```

This should allow you to override the default web UI embedded in GARM without updating the GARM binary.
