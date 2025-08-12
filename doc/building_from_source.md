# Building GARM from source

The procedure is simple. You will need to gave [go](https://golang.org/) installed as well as `make`.

First, clone the repository:

```bash
git clone https://github.com/cloudbase/garm
cd garm
```

Then build garm:

```bash
make build
```

You should now have both `garm` and `garm-cli` available in the `./bin` folder.

If you have docker/podman installed, you can also build a static binary against `musl`:

```bash
make build-static
```

This command will also build for both AMD64 and ARM64. Resulting binaries will be in the `./bin` folder.

## Hacking

If you're hacking on GARM and want to override the default version GARM injects, you can run the following command:

```bash
VERSION=v1.0.0 make build
```

> [!IMPORTANT]
> This only works for `make build`. The `make build-static` command does not support version overrides.

## The Web UI SPA

GARM now ships with a single page application. The application is written in svelte and tailwind CSS. To rebuild it or hack on it, you will need a number of dependencies installed and placed in your `$PATH`.

### Prerequisites

- **Node.js 24+** and **npm**
- **Go 1.21+** (for building the GARM backend)
- **openapi-generator-cli** in your PATH (for API client generation)

### Installing openapi-generator-cli

**Option 1: NPM Global Install**
```bash
npm install -g @openapitools/openapi-generator-cli
```

**Option 2: Manual Install**
Download from [OpenAPI Generator releases](https://github.com/OpenAPITools/openapi-generator/releases) and add to your PATH.

**Verify Installation:**

```bash
openapi-generator-cli version
```



### Hacking on the Web UI

If you need to change something in the `webapp/src` folder, make sure to rebuild the webapp before rebuilding GARM:

```bash
make build-webui
make build
```

> [!IMPORTANT]
> The Web UI that GARM ships with has `go generate` stanzas that require `@openapitools/openapi-generator-cli` and `tailwindcss` to be installed. You will also have to make sure that if you change API models, the Web UI still works, as adding new fields or changing the json tags of old fields will change accessors in the client code.

### Changing API models

If you need to change the models in the `params/` package, you will also need to regenerate the client both for garm-cli and for the web application we ship with GARM. To do this, you can run:

```bash
make generate
```

You will also need to make sure that the web app still works.
