# Development Setup

The web app can be started with the `npm run dev` command, which will start a development server with hot reloading. To properly work, there are a number of prerequisites you need to have and some GARM settings to tweak.

## Prerequisites

To have a full development setup, you will need the following prerequisites:

- **Node.js 24+** and **npm**
- **Go 1.24+** (for building the GARM backend)
- **openapi-generator-cli** in your PATH (for API client generation)

The `openapi-generator-cli` will also need java to be installed. If you're running on Ubuntu, running:

```bash
sudo apt-get install default-jre
```

should be enough. Different distros should have an equivalent package available.

>[!NOTE]
>If you don't need to change the web app, you don't need to rebuild it. There is already a pre-built version in the repo.

## Necessary GARM settings

GARM has strict origin checks for websockets and API calls. To allow your local development server to communicate with the GARM backend, you need to configure the following settings:

```toml
[apiserver]
cors_origins = ["https://garm.example.com", "http://127.0.0.1:5173"]
```

>[!IMPORTANT]
> You must include the port.

>[!IMPORTANT]
> Omitting the `cors_origins` option will automatically check same host origin. 

## Development Server

Your GARM server can be started and hosted anywhere. As long as you set the proper `cors_origins` URLs, your web-ui development server can be separate from your GARM server. To point the web app to the GARM server, you will need to create an `.env.development` file in the `webapp/` directory:

```bash
cd /home/ubuntu/garm/webapp
echo "VITE_GARM_API_URL=http://localhost:9997" > .env
echo "NODE_ENV=development" >> .env
npm run dev
```

## Asset Management

During development:
- SVG icons are served from `static/assets/`
- Favicons are served from `static/`
- All static assets are copied from `assets/assets/` to `static/assets/`

## Building for Production

For production deployments, the web app is embedded into the GARM binary. You don't need to serve it separately. To build the web app and embed it into the binary, run the following 2 commands:

```bash
# Build the static webapp
make build-webui
# Build the garm binary with the webapp embedded
make build
```

This creates the production build with:
- Base path set to `/ui`
- All assets embedded for Go to serve
- Optimized bundles

>[!IMPORTANT]
>The web UI is an optional feature in GARM. For the `/ui` URL to be available, you will need to enable it in the garm config file under:
>```toml
>[apiserver.webui]
>  enable=true
>```
>See the sample config file in the `testdata/config.toml` file.