# Configuring github endpoints and credentials

Starting with version `v0.1.5`, GARM saves github endpoints and github credentials in the database.

<!-- TOC -->

- [Configuring github endpoints and credentials](#configuring-github-endpoints-and-credentials)
    - [Create GitHub endpoint](#create-github-endpoint)
    - [Listing GitHub endpoints](#listing-github-endpoints)
    - [Adding GitHub credentials](#adding-github-credentials)
    - [Listing GitHub credentials](#listing-github-credentials)
    - [Deleting GitHub credentials](#deleting-github-credentials)

<!-- /TOC -->

## Create GitHub endpoint

To create a new GitHub endpoint, you can use the following command:

```bash
garm-cli github endpoint create \
    --name example \
    --description "Just an example ghes endpoint" \
    --base-url https://ghes.example.com \
    --upload-url https://upload.ghes.example.com \
    --api-base-url https://api.ghes.example.com \
    --ca-cert-path $HOME/ca-cert.pem
```

## Listing GitHub endpoints

To list the available GitHub endpoints, you can use the following command:

```bash
ubuntu@garm:~/garm$ garm-cli github endpoint list
+------------+--------------------------+-------------------------------+
| NAME       | BASE URL                 | DESCRIPTION                   |
+------------+--------------------------+-------------------------------+
| github.com | https://github.com       | The github.com endpoint       |
+------------+--------------------------+-------------------------------+
| example    | https://ghes.example.com | Just an example ghes endpoint |
+------------+--------------------------+-------------------------------+
```

## Adding GitHub credentials

GARM has the option to use both [Personal Access Tokens (PAT)](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token) or a [GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app).


If you'll use a PAT, you'll have to grant access for the following scopes:

* ```public_repo``` - for access to a repository
* ```repo``` - for access to a private repository
* ```admin:org``` - if you plan on using this with an organization to which you have access
* ```manage_runners:enterprise``` - if you plan to use garm at the enterprise level
* ```admin:repo_hook``` - if you want to allow GARM to install webhooks on repositories (optional)
* ```admin:org_hook``` - if you want to allow GARM to install webhooks on organizations (optional)

If you plan to use github apps, you'll need to select the following permissions:

* **Repository permissions**:
  * ```Administration: Read & write```
  * ```Metadata: Read-only```
  * ```Webhooks: Read & write```
* **Organization permissions**:
  * ```Self-hosted runners: Read & write```
  * ```Webhooks: Read & write```

**Note** :warning:: Github Apps are not available at the enterprise level.

To add a new GitHub credential, you can use the following command:

```bash
garm-cli github credentials add \
  --name gabriel \
  --description "GitHub PAT for user gabriel" \
  --auth-type pat \
  --pat-oauth-token gh_theRestOfThePAT \
  --endpoint github.com
```

To add a new GitHub App credential, you can use the following command:

```bash
garm-cli github credentials add \
  --name gabriel_app \
  --description "Github App with access to repos" \
  --endpoint github.com \
  --auth-type app \
  --app-id 1 \
  --app-installation-id 99 \
  --private-key-path $HOME/yourAppName.2024-03-01.private-key.pem
```

All sensitive data is encrypted at rest. The API will not return any sensitive info.

## Listing GitHub credentials

To list the available GitHub credentials, you can use the following command:

```bash
garm-cli github credentials list
```

## Deleting GitHub credentials

To delete a GitHub credential, you can use the following command:

```bash
garm-cli github credentials delete <CREDENTIAL_ID>
```