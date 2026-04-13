# Building GARM from Source

## Prerequisites

- Go 1.22+
- `make`

## Building GARM

```bash
git clone https://github.com/cloudbase/garm
cd garm
make build
```

The `garm` and `garm-cli` binaries will be in `./bin/`.

### Static builds

If you have Docker or Podman installed, you can build static binaries against musl for both AMD64 and ARM64:

```bash
make build-static
```

### Overriding the version

```bash
VERSION=v1.0.0 make build
```

> [!IMPORTANT]
> Version overrides only work with `make build`, not `make build-static`.

## Building the Web UI

GARM ships with a Svelte-based web UI. To rebuild it you need:

- Node.js 24+ and npm
- `openapi-generator-cli` in your PATH

### Installing openapi-generator-cli

```bash
npm install -g @openapitools/openapi-generator-cli
```

Verify:

```bash
openapi-generator-cli version
```

### Rebuilding the UI

If you modify anything in `webapp/src/`, rebuild the UI before rebuilding GARM:

```bash
make build-webui
make build
```

> [!IMPORTANT]
> The Web UI build uses `go generate` stanzas that require `@openapitools/openapi-generator-cli` and `tailwindcss`. If you change API models, ensure the Web UI still works -- changing JSON tags or adding fields will change the generated client code.

## Regenerating API clients

If you change models in the `params/` package, regenerate both the CLI and Web UI clients:

```bash
make generate
```

Then verify the web app still compiles and works.
