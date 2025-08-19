# Deploying GARM with Helm

This document provides instructions on how to deploy GARM on a Kubernetes cluster using the provided Helm chart.

As this chart is not currently published to a public Helm repository, you will need to clone the GARM git repository to deploy it.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.2.0+
- `kubectl` configured to connect to your cluster.

### Admin Credentials

By default, the chart creates a Kubernetes secret to store the admin user\'s credentials (`username`, `password`, `email`), the JWT secret, and the database passphrase.

If you want to use an existing secret, you can specify its name using `garm.admin.secretName`. Ensure the secret contains the necessary keys as defined by `garm.admin.usernameKey`, `garm.admin.passwordKey`, etc.

### Persistence

The chart uses a PersistentVolumeClaim (PVC) to store the GARM database. By default, persistence is enabled. You can disable it by setting `persistence.enabled` to `false`, but this is not recommended for production environments.

### Providers Configuration

You can configure external providers by adding them to the `providers` list in your `values.yaml` file. Each provider entry requires a `name`, `description`, `executable` path, and a `config` block with the provider-specific settings.

Example for a GCP provider:

```yaml
providers:
  - name: "gcp"
    type: "external"
    description: "GCP provider"
    executable: "/opt/garm/providers.d/garm-provider-gcp"
    config:
      project_id: "my-gcp-project"
      zone: "us-central1-a"
```

### Forge Credentials (GitHub/Gitea)

The chart\'s initialization script can automatically configure GARM with your GitHub and/or Gitea credentials. To use this feature, you must first create Kubernetes secrets containing your forge credentials.

Then, configure the `forges.github.credentials` or `forges.gitea.credentials` sections in your `values.yaml`.

**GitHub Example:**

First, create a secret:

```bash
kubectl create secret generic my-github-secret \
  --from-literal=token='ghp_xxxxxxxx'
```

Then, configure `values.yaml`:

```yaml
forges:
  github:
    credentials:
      - name: "my-github"
        secretName: "my-github-secret"
        authType: "pat"
        tokenKey: "token"
```

**Gitea Example:**

First, create a secret:

```bash
kubectl create secret generic garm-gitea-config \
  --from-literal=server-url='https://gitea.example.com' \
  --from-literal=access-token='xxxxxxxx'
```

Then, configure `values.yaml`:

```yaml
forges:
  gitea:
    credentials:
      - name: "my-gitea"
        secretName: "garm-gitea-config"
        urlKey: "server-url"
        tokenKey: "access-token"
```

## Installation Steps

The `helm-chart/values.yaml` file serves as a template and is not intended for direct use, as it contains placeholder values. To properly deploy the chart, follow these steps:

1.  **Create a custom values file:**
    Clone the repository and copy the `values.yaml` to a new file. For example, `my-values.yaml`.

    ```bash
    git clone https://github.com/cloudbase/garm.git
    cd garm
    cp helm-chart/values.yaml my-values.yaml
    ```

2.  **Configure your deployment:**
    Edit `my-values.yaml` and modify the parameters to match your environment. At a minimum, you will likely need to configure:
    - `garm.url` (or the individual `callbackUrl`, `metadataUrl`, `webhookUrl`)
    - `ingress.host` if you are using Ingress.
    - `providers` to set up at least one compute provider.
    - `forges` to configure credentials for GitHub or Gitea.

3.  **Install the chart:**
    Once you have configured your `my-values.yaml`, install the chart using the `-f` flag to specify your custom values file.

    ```bash
    helm install my-garm ./helm-chart -f my-values.yaml
    ```

## Uninstallation

To uninstall the `my-garm` deployment:

```bash
helm uninstall my-garm
```

The command removes all the Kubernetes components associated with the chart and deletes the release.
