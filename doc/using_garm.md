# Using GARM

This document will walk you through the various commands and options available in GARM. It is assumed that you have already installed GARM and have it running. If you haven't, please check out the [quickstart](/doc/quickstart.md) document for instructions on how to install GARM.

While using the GARM cli, you will most likely spend most of your time listing pools and runners, but we will cover most of the available commands and options. Some of them we'll skip (like the `init` or `profile` subcommands), as they've been covered in the [quickstart](/doc/quickstart.md) document.
<!-- TOC -->

- [Using GARM](#using-garm)
    - [Controller operations](#controller-operations)
        - [Listing controller info](#listing-controller-info)
        - [Updating controller settings](#updating-controller-settings)
    - [Providers](#providers)
        - [Listing configured providers](#listing-configured-providers)
    - [Github Endpoints](#github-endpoints)
        - [Creating a GitHub Endpoint](#creating-a-github-endpoint)
        - [Listing GitHub Endpoints](#listing-github-endpoints)
        - [Getting information about an endpoint](#getting-information-about-an-endpoint)
        - [Deleting a GitHub Endpoint](#deleting-a-github-endpoint)
    - [GitHub credentials](#github-credentials)
        - [Adding GitHub credentials](#adding-github-credentials)
        - [Listing GitHub credentials](#listing-github-credentials)
        - [Getting detailed information about credentials](#getting-detailed-information-about-credentials)
        - [Deleting GitHub credentials](#deleting-github-credentials)
    - [Repositories](#repositories)
        - [Adding a new repository](#adding-a-new-repository)
        - [Listing repositories](#listing-repositories)
        - [Removing a repository](#removing-a-repository)
    - [Organizations](#organizations)
        - [Adding a new organization](#adding-a-new-organization)
    - [Enterprises](#enterprises)
        - [Adding an enterprise](#adding-an-enterprise)
    - [Managing webhooks](#managing-webhooks)
    - [Pools](#pools)
        - [Creating a runner pool](#creating-a-runner-pool)
        - [Listing pools](#listing-pools)
        - [Showing pool info](#showing-pool-info)
        - [Deleting a pool](#deleting-a-pool)
        - [Update a pool](#update-a-pool)
    - [Runners](#runners)
        - [Listing runners](#listing-runners)
        - [Showing runner info](#showing-runner-info)
        - [Deleting a runner](#deleting-a-runner)
    - [The debug-log command](#the-debug-log-command)
    - [The debug-events command](#the-debug-events-command)
    - [Listing recorded jobs](#listing-recorded-jobs)

<!-- /TOC -->

## Controller operations

The `controller` is essentially GARM itself. Every deployment of GARM will have its own controller ID which will be used to tag runners in github. The controller is responsible for managing runners, webhooks, repositories, organizations and enterprises. There are a few settings at the controller level which you can tweak, which we will cover below.

### Listing controller info

You can list the controller info by running the following command:

```bash
garm-cli controller show
+-------------------------+----------------------------------------------------------------------------+
| FIELD                   | VALUE                                                                      |
+-------------------------+----------------------------------------------------------------------------+
| Controller ID           | a4dd5f41-8e1e-42a7-af53-c0ba5ff6b0b3                                       |
| Hostname                | garm                                                                       |
| Metadata URL            | https://garm.example.com/api/v1/metadata                                   |
| Callback URL            | https://garm.example.com/api/v1/callbacks                                  |
| Webhook Base URL        | https://garm.example.com/webhooks                                          |
| Controller Webhook URL  | https://garm.example.com/webhooks/a4dd5f41-8e1e-42a7-af53-c0ba5ff6b0b3     |
| Minimum Job Age Backoff | 30                                                                         |
| Version                 | v0.1.5                                                                     |
+-------------------------+----------------------------------------------------------------------------+
```

There are several things of interest in this output.

* `Controller ID` - This is the unique identifier of the controller. Each GARM installation, on first run will automatically generate a unique controller ID. This is important for several reasons. For one, it allows us to run several GARM controllers on the same repos/orgs/enterprises, without accidentally clashing with each other. Each runner started by a GARM controller, will be tagged with this controller ID in order to easily identify runners that we manage.
* `Hostname` - This is the hostname of the machine where GARM is running. This is purely informative.
* `Metadata URL` - This URL is configured by the user, and is the URL that is presented to the runners via userdata when they get set up. Runners will connect to this URL and retrieve information they might need to set themselves up. GARM cannot automatically determine this URL, as it is dependent on the user's network setup. GARM may be hidden behind a load balancer or a reverse proxy, in which case, the URL by which the GARM controller can be accessed may be different than the IP addresses that are locally visible to GARM. Runners must be able to connect to this URL.
* `Callback URL` - This URL is configured by the user, and is the URL that is presented to the runners via userdata when they get set up. Runners will connect to this URL and send status updates and system information (OS version, OS name, github runner agent ID, etc) to the controller. Runners must be able to connect to this URL.
* `Webhook Base URL` - This is the base URL for webhooks. It is configured by the user in the GARM config file. This URL can be called into by GitHub itself when hooks get triggered by a workflow. GARM needs to know when a new job is started in order to schedule the creation of a new runner. Job webhooks sent to this URL will be recorded by GARM and acted upon. While you can configure this URL directly in your GitHub repo settings, it is advised to use the `Controller Webhook URL` instead, as it is unique to each controller, and allows you to potentially install multiple GARM controller inside the same repo. Github must be able to connect to this URL.
* `Controller Webhook URL` - This is the URL that GitHub will call into when a webhook is triggered. This URL is unique to each GARM controller and is the preferred URL to use in order to receive webhooks from GitHub. It serves the same purpose as the `Webhook Base URL`, but is unique to each controller, allowing you to potentially install multiple GARM controllers inside the same repo. Github must be able to connect to this URL.
* `Minimum Job Age Backoff` - This is the job age in seconds, after which GARM will consider spinning up a new runner to handle it. By default GARM waits for 30 seconds after receiving a new job, before it spins up a runner. This delay is there to allow any existing idle runners (managed by GARM or not) to pick up the job, before reacting to it. This way we avoid being too eager and spin up a runner for a job that would have been picked up by an existing runner anyway. You can set this to 0 if you want GARM to react immediately.
* `Version` - This is the version of GARM that is running.

We will see the `Controller Webhook URL` later when we set up the GitHub repo to send webhooks to GARM.

### Updating controller settings

As we've mentioned before, there are 3 URLs that are very important for normal operations:

* `metadata_url` - Must be reachable by runners
* `callback_url` - Must be reachable by runners
* `webhook_url` - Must be reachable by GitHub

These URLs depend heavily on how GARM was set up and what the network topology of the user is set up. GARM may be behind a NAT or reverse proxy. There may be different hostnames/URL paths set up for each of the above, etc. The short of it is that we cannot determine these URLs reliably and we must ask the user to tell GARM what they are.

We can assume that the URL that the user logs in at to manage garm is the same URL that the rest of the URLs are present at, but that is just an assumption. By default, when you initialize GARM for the first time, we make this assumption to make things easy. It's also safe to assume that most users will do this anyway, but in case you don't, you will need to update the URLs in the controller and tell GARM what they are.

In the previous section we saw that most URLs were set to `https://garm.example.com`. The URL path was the same as the routes that GARM sets up. For example, the `metadata_url` has `/api/v1/metadata`. The `callback_url` has `/api/v1/callbacks` and the `webhook_url` has `/webhooks`. This is the default setup and is what most users will use.

If you need to update these URLs, you can use the following command:

```bash
garm-cli controller update \
    --metadata-url https://garm.example.com/api/v1/metadata \
    --callback-url https://garm.example.com/api/v1/callbacks \
    --webhook-url https://garm.example.com/webhooks
```

The `Controller Webhook URL` you saw in the previous section is automatically calculated by GARM and is essentially the `webhook_url` with the controller ID appended to it. This URL is unique to each controller and is the preferred URL to use in order to receive webhooks from GitHub.

After updating the URLs, make sure that they are properly routed to the appropriate API endpoint in GARM **and** that they are accessible by the interested parties (runners or github).

## Providers

GARM uses providers to create runners. These providers are external executables that GARM calls into to create runners in a particular IaaS.

### Listing configured providers

Once configured (see [provider configuration](/doc/config.md#providers)), you can list the configured providers by running the following command:

```bash
ubuntu@garm:~$ garm-cli provider list
+--------------+---------------------------------+----------+
| NAME         | DESCRIPTION                     | TYPE     |
+--------------+---------------------------------+----------+
| incus        | Incus external provider         | external |
+--------------+---------------------------------+----------+
| lxd          | LXD external provider           | external |
+--------------+---------------------------------+----------+
| openstack    | OpenStack external provider     | external |
+--------------+---------------------------------+----------+
| azure        | Azure provider                  | external |
+--------------+---------------------------------+----------+
| k8s_external | k8s external provider           | external |
+--------------+---------------------------------+----------+
| Amazon EC2   | Amazon EC2 provider             | external |
+--------------+---------------------------------+----------+
| equinix      | Equinix Metal                   | external |
+--------------+---------------------------------+----------+
```

Each of these providers can be used to set up a runner pool for a repository, organization or enterprise.

## Github Endpoints

GARM can be used to manage runners for repos, orgs and enterprises hosted on `github.com` or on a GitHub Enterprise Server.

Endpoints are the way that GARM identifies where the credentials and entities you create are located and where the API endpoints for the GitHub API can be reached, along with a possible CA certificate that validates the connection. There is a default endpoint for `github.com`, so you don't need to add it, unless you're using GHES.

### Creating a GitHub Endpoint

To create a GitHub endpoint, you can run the following command:

```bash
garm-cli github endpoint create \
    --base-url https://ghes.example.com \
    --api-base-url https://api.ghes.example.com \
    --upload-url https://upload.ghes.example.com \
    --ca-cert-path $HOME/ca-cert.pem \
    --name example \
    --description "Just an example ghes endpoint"
+----------------+------------------------------------------------------------------+
| FIELD          | VALUE                                                            |
+----------------+------------------------------------------------------------------+
| Name           | example                                                          |
| Base URL       | https://ghes.example.com                                         |
| Upload URL     | https://upload.ghes.example.com                                  |
| API Base URL   | https://api.ghes.example.com                                     |
| CA Cert Bundle | -----BEGIN CERTIFICATE-----                                      |
|                | MIICBzCCAY6gAwIBAgIQX7fEm3dxkTeSc+E1uTFuczAKBggqhkjOPQQDAzA2MRkw |
|                | FwYDVQQKExBHQVJNIGludGVybmFsIENBMRkwFwYDVQQDExBHQVJNIGludGVybmFs |
|                | IENBMB4XDTIzMDIyNTE4MzE0NloXDTMzMDIyMjE4MzE0NlowNjEZMBcGA1UEChMQ |
|                | R0FSTSBpbnRlcm5hbCBDQTEZMBcGA1UEAxMQR0FSTSBpbnRlcm5hbCBDQTB2MBAG |
|                | ByqGSM49AgEGBSuBBAAiA2IABKat241Jzvkl+ksDuPq5jFf9wb5/l54NbGYYfcrs |
|                | 4d9/sNXtPP1y8pM61hs+hCltN9UEwtxqr48q5G7Oc3IjH/dddzJTDC2bLcpwysrC |
|                | NYLGtSfNj+o/8AQMwwclAY7t4KNhMF8wDgYDVR0PAQH/BAQDAgIEMB0GA1UdJQQW |
|                | MBQGCCsGAQUFBwMCBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQW |
|                | BBSY+cSG07sIU2UC+fOniODKUGqiUTAKBggqhkjOPQQDAwNnADBkAjBcFz3cZ7vO |
|                | IFVzqn9eqXMmZDGp58HGneHhFhJsJtQE4BkxGQmgZJ2OgTGXDqjXG3wCMGMQRALt |
|                | JxwlI1PJJj7M0g48viS4NjT4kq2t/UFIbTy78aarFynUfykpL9FD9NOmiQ==     |
|                | -----END CERTIFICATE-----                                        |
|                |                                                                  |
+----------------+------------------------------------------------------------------+
```

The name of the endpoint needs to be unique within GARM.

### Listing GitHub Endpoints

To list existing GitHub endpoints, run the following command:

```bash
garm-cli github endpoint list 
+------------+--------------------------+-------------------------------+
| NAME       | BASE URL                 | DESCRIPTION                   |
+------------+--------------------------+-------------------------------+
| github.com | https://github.com       | The github.com endpoint       |
+------------+--------------------------+-------------------------------+
| example    | https://ghes.example.com | Just an example ghes endpoint |
+------------+--------------------------+-------------------------------+
```

### Getting information about an endpoint

To get information about a specific endpoint, you can run the following command:

```bash
garm-cli github endpoint show github.com
+--------------+-----------------------------+
| FIELD        | VALUE                       |
+--------------+-----------------------------+
| Name         | github.com                  |
| Base URL     | https://github.com          |
| Upload URL   | https://uploads.github.com/ |
| API Base URL | https://api.github.com/     |
+--------------+-----------------------------+
```

### Deleting a GitHub Endpoint

You can delete an endpoint unless any of the following conditions are met:

* The endpoint is the default endpoint for `github.com`
* The endpoint is in use by a repository, organization or enterprise
* There are credentials defined against the endpoint you are trying to remove

To delete an endpoint, you can run the following command:

```bash
garm-cli github endpoint delete example
```

## GitHub credentials

GARM needs access to your GitHub repositories, organizations or enterprise in order to manage runners. This is done via a [GitHub personal access token or via a GitHub App](/doc/github_credentials.md). You can configure multiple tokens or apps with access to various repositories, organizations or enterprises, either on GitHub or on GitHub Enterprise Server.

### Adding GitHub credentials

There are two types of credentials:

* PAT - Personal Access Token
* App - GitHub App

To add each of these types of credentials, slightly different command line arguments (obviously) are required. I'm going to give you an example of both.

To add a PAT, you can run the following command:

```bash
garm-cli github credentials add \
    --name deleteme \
    --description "just a test" \
    --auth-type pat \
    --pat-oauth-token gh_yourTokenGoesHere \
    --endpoint github.com
```

To add a GitHub App (only available for repos and orgs), you can run the following command:

```bash
garm-cli github credentials add \
    --name deleteme-app \
    --description "just a test" \
    --endpoint github.com \
    --auth-type app \
    --app-id 1 \
    --app-installation-id 99 \
    --private-key-path /etc/garm/yiourGarmAppKey.2024-12-12.private-key.pem
```

Notice that in both cases we specified the github endpoint for which these credentials are valid. 

### Listing GitHub credentials

To list existing credentials, run the following command:

```bash
ubuntu@garm:~$ garm-cli github credentials ls
+----+-------------+------------------------------------+--------------------+-------------------------+-----------------------------+------+
| ID | NAME        | DESCRIPTION                        | BASE URL           | API URL                 | UPLOAD URL                  | TYPE |
+----+-------------+------------------------------------+--------------------+-------------------------+-----------------------------+------+
|  1 | gabriel     | github token or user gabriel       | https://github.com | https://api.github.com/ | https://uploads.github.com/ | pat  |
+----+-------------+------------------------------------+--------------------+-------------------------+-----------------------------+------+
|  2 | gabriel_org | github token with org level access | https://github.com | https://api.github.com/ | https://uploads.github.com/ | app  |
+----+-------------+------------------------------------+--------------------+-------------------------+-----------------------------+------+
```

For more information about credentials, see the [github credentials](/doc/github_credentials.md) section for more details.

### Getting detailed information about credentials

To get detailed information about one specific credential, you can run the following command:

```bash
garm-cli github credentials show 2
+---------------+------------------------------------+
| FIELD         | VALUE                              |
+---------------+------------------------------------+
| ID            | 2                                  |
| Name          | gabriel_org                        |
| Description   | github token with org level access |
| Base URL      | https://github.com                 |
| API URL       | https://api.github.com/            |
| Upload URL    | https://uploads.github.com/        |
| Type          | app                                |
| Endpoint      | github.com                         |
|               |                                    |
| Repositories  | gsamfira/garm-testing              |
|               |                                    |
| Organizations | gsamfira                           |
+---------------+------------------------------------+
```

### Deleting GitHub credentials

To delete a credential, you can run the following command:

```bash
garm-cli github credentials delete 2
```

> **NOTE**: You may not delete credentials that are currently associated with a repository, organization or enterprise. You will need to first replace the credentials on the entity, and then you can delete the credentials.

## Repositories

### Adding a new repository

To add a new repository we need to use credentials that has access to the repository. We've listed credentials above, so let's add our first repository:

```bash
ubuntu@garm:~$ garm-cli repository add \
    --name garm \
    --owner gabriel-samfira \
    --credentials gabriel \
    --install-webhook \
    --pool-balancer-type roundrobin \
    --random-webhook-secret
+----------------------+--------------------------------------+
| FIELD                | VALUE                                |
+----------------------+--------------------------------------+
| ID                   | 0c91d9fd-2417-45d4-883c-05daeeaa8272 |
| Owner                | gabriel-samfira                      |
| Name                 | garm                                 |
| Pool balancer type   | roundrobin                           |
| Credentials          | gabriel                              |
| Pool manager running | true                                 |
+----------------------+--------------------------------------+
```

Lets break down the command a bit and explain what happened above. We added a new repository to GARM, that belogs to the user `gabriel-samfira` and is called `garm`. When using GitHub, this translates to `https://github.com/gabriel-samfira/garm`.

As part of the above command, we used the credentials called `gabriel` to authenticate to GitHub. If those credentials didn't have access to the repository, we would have received an error when adding the repo.

The other interesting bit about the above command is that we automatically added the `webhook` to the repository and generated a secure random secret to authenticate the webhooks that come in from GitHub for this new repo. Any webhook claiming to be for the `gabriel-samfira/garm` repo, will be validated against the secret that was generated.

Another important aspect to remember is that once the entity (in this case a repository) is created, the credentials associated with the repo at creation time, dictates the GitHub endpoint in which this repository exists.

When updating credentials for this entity, the new credentials **must** be associated with the same endpoint as the old ones. An error is returned if the repo is associated with `github.com` but the new credentials you're trying to set are associated with a GHES endpoint.

### Listing repositories

To list existing repositories, run the following command:

```bash
ubuntu@garm:~$ garm-cli repository list
+--------------------------------------+-----------------+--------------+------------------+--------------------+------------------+
| ID                                   | OWNER           | NAME         | CREDENTIALS NAME | POOL BALANCER TYPE | POOL MGR RUNNING |
+--------------------------------------+-----------------+--------------+------------------+--------------------+------------------+
| be3a0673-56af-4395-9ebf-4521fea67567 | gabriel-samfira | garm         | gabriel          | roundrobin         | true             |
+--------------------------------------+-----------------+--------------+------------------+--------------------+------------------+
```

This will list all the repositories that GARM is currently managing.

### Removing a repository

To remove a repository, you can use the following command:

```bash
garm-cli repository delete be3a0673-56af-4395-9ebf-4521fea67567
```

This will remove the repository from GARM, and if a webhook was installed, will also clean up the webhook from the repository.

> **NOTE**: GARM will not remove a webhook that points to the `Base Webhook URL`. It will only remove webhooks that are namespaced to the running controller.

## Organizations

### Adding a new organization

Adding a new organization is similar to adding a new repository. You need to use credentials that have access to the organization, and you can add the organization to GARM using the following command:

```bash
ubuntu@garm:~$ garm-cli organization add \
    --credentials gabriel_org \
    --name gsamfira \
    --install-webhook \
    --random-webhook-secret
+----------------------+--------------------------------------+
| FIELD                | VALUE                                |
+----------------------+--------------------------------------+
| ID                   | b50f648d-708f-48ed-8a14-cf58887af9cf |
| Name                 | gsamfira                             |
| Credentials          | gabriel_org                          |
| Pool manager running | true                                 |
+----------------------+--------------------------------------+
```

This will add the organization `gsamfira` to GARM, and install a webhook for it. The webhook will be validated against the secret that was generated. The only difference between adding an organization and adding a repository is that you use the `organization` subcommand instead of the `repository` subcommand, and the `--name` option represents the `name` of the organization.

Managing webhooks for organizations is similar to managing webhooks for repositories. You can *list*, *show*, *install* and *uninstall* webhooks for organizations using the `garm-cli organization webhook` subcommand. We won't go into details here, as it's similar to managing webhooks for repositories.

All the other operations that exist on repositories, like listing, removing, etc, also exist for organizations and enterprises. Check out the help for the `garm-cli organization` subcommand for more details.

## Enterprises

### Adding an enterprise

Enterprises are a bit special. Currently we don't support managing webhooks for enterprises, mainly because the level of access that would be required to do so seems a bit too much to enable in GARM itself. And considering that you'll probably ever only have one enterprise with multiple organizations and repositories, the effort/risk to benefit ratio makes this feature not worth implementing at the moment.

To add an enterprise to GARM, you can use the following command:

```bash
garm-cli enterprise add \
    --credentials gabriel_enterprise \
    --name samfira \
    --webhook-secret SuperSecretWebhookTokenPleaseReplaceMe
```

The `name` of the enterprise is the ["slug" of the enterprise](https://docs.github.com/en/enterprise-cloud@latest/admin/managing-your-enterprise-account/creating-an-enterprise-account). 

You will then have to manually add the `Controller Webhook URL` to the enterprise in the GitHub UI.

All the other operations that exist on repositories, like listing, removing, etc, also exist for organizations and enterprises. Have a look at the help for the `garm-cli enterprise` subcommand for more details.

At that point the enterprise will be added to GARM and you can start managing runners for it.

## Managing webhooks

Webhook management is available for repositories and organizations. I'm going to show you how to manage webhooks for a repository, but the same commands apply for organizations. See `--help` for more details.

When we added the repository in the previous section, we specified the `--install-webhook` and the `--random-webhook-secret` options. These two options automatically added a webhook to the repository and generated a random secret for it. The `webhook` URL that was used, will correspond to the `Controller Webhook URL` that we saw earlier when we listed the controller info. Let's list it and see what it looks like:

```bash
ubuntu@garm:~$ garm-cli repository webhook show be3a0673-56af-4395-9ebf-4521fea67567
+--------------+----------------------------------------------------------------------------+
| FIELD        | VALUE                                                                      |
+--------------+----------------------------------------------------------------------------+
| ID           | 460257636                                                                  |
| URL          | https://garm.example.com/webhooks/a4dd5f41-8e1e-42a7-af53-c0ba5ff6b0b3     |
| Events       | [workflow_job]                                                             |
| Active       | true                                                                       |
| Insecure SSL | false                                                                      |
+--------------+----------------------------------------------------------------------------+
```

We can see that it's active, and the events to which it subscribed.

The `--install-webhook` and `--random-webhook-secret` options are convenience options that allow you to quickly add a new repository to GARM and have it ready to receive webhooks from GitHub. As long as you configured the URLs correctly (see previous sections for details), you should see a green checkmark in the GitHub settings page, under `Webhooks`.

If you don't want to install the webhook, you can add the repository without it, and then install it later using the `garm-cli repository webhook install` command (which we'll show in a second) or manually add it in the GitHub UI.

To uninstall a webhook from a repository, you can use the following command:

```bash
garm-cli repository webhook uninstall be3a0673-56af-4395-9ebf-4521fea67567
```

After which listing the webhook will show that it's inactive:

```bash
ubuntu@garm:~$ garm-cli repository webhook show be3a0673-56af-4395-9ebf-4521fea67567
Error: [GET /repositories/{repoID}/webhook][404] GetRepoWebhookInfo default  {Error:Not Found Details:hook not found}
```

You can always add it back using:

```bash
ubuntu@garm:~$ garm-cli repository webhook install be3a0673-56af-4395-9ebf-4521fea67567
+--------------+----------------------------------------------------------------------------+
| FIELD        | VALUE                                                                      |
+--------------+----------------------------------------------------------------------------+
| ID           | 460258767                                                                  |
| URL          | https://garm.example.com/webhooks/a4dd5f41-8e1e-42a7-af53-c0ba5ff6b0b3     |
| Events       | [workflow_job]                                                             |
| Active       | true                                                                       |
| Insecure SSL | false                                                                      |
+--------------+----------------------------------------------------------------------------+
```

To allow GARM to manage webhooks, the PAT or app you're using must have the `admin:repo_hook` and `admin:org_hook` scopes (or equivalent). Webhook management is not available for enterprises. For enterprises you will have to add the webhook manually.

To manually add a webhook, see the [webhooks](/doc/webhooks.md) section.

## Pools

### Creating a runner pool

Now that we have a repository, organization or enterprise added to GARM, we can create a runner pool for it. A runner pool is a collection of runners of the same type, that are managed by GARM and are used to run workflows for the repository, organization or enterprise.

You can create multiple pools of runners for the same entity (repository, organization or enterprise), and you can create multiple pools of runners, each pool defining different runner types. For example, you can have a pool of runners that are created on AWS, and another pool of runners that are created on Azure, k8s, LXD, etc. For repositories or organizations with complex needs, you can set up a number of pools that cover a wide range of needs, based on cost, capability (GPUs, FPGAs, etc) or sheer raw computing power. You don't have to pick just one, especially since managing all of them is done using the exact same commands, as we'll show below.

Before we create a pool, we have to decide which provider we want to use. We've listed the providers above, so let's pick one and create a pool of runners for our repository. For the purpose of this example, we'll use the `incus` provider. We'll show you how to create a pool using this provider, but keep in mind that adding another pool using a different provider is done using the exact same commands. The only difference will be in the `--image`, `--flavor` and `--extra-specs` options that you'll use when creating the pool.

Out of those three options, only the `--image` and `--flavor` are mandatory. The `--extra-specs` flag is optional and is used to pass additional information to the provider when creating the pool. The `--extra-specs` option is provider specific, and you'll have to consult the provider documentation to see what options are available.

But I digress. Let's create a pool of runners using the `incus` provider, for the `gabriel-samfira/garm` repository we created above:

```bash
garm-cli pool add \
    --enabled=false \
    --repo be3a0673-56af-4395-9ebf-4521fea67567 \
    --image "images:ubuntu/22.04/cloud" \
    --flavor default \
    --provider-name incus \
    --min-idle-runners 1 \
    --tags ubuntu,incus
+--------------------------+----------------------------------------+
| FIELD                    | VALUE                                  |
+--------------------------+----------------------------------------+
| ID                       | 9daa34aa-a08a-4f29-a782-f54950d8521a   |
| Provider Name            | incus                                  |
| Image                    | images:ubuntu/22.04/cloud              |
| Flavor                   | default                                |
| OS Type                  | linux                                  |
| OS Architecture          | amd64                                  |
| Max Runners              | 5                                      |
| Min Idle Runners         | 1                                      |
| Runner Bootstrap Timeout | 20                                     |
| Tags                     | ubuntu, incus                          |
| Belongs to               | gabriel-samfira/garm                   |
| Level                    | repo                                   |
| Enabled                  | false                                  |
| Runner Prefix            | garm                                   |
| Extra specs              |                                        |
| GitHub Runner Group      |                                        |
+--------------------------+----------------------------------------+
```

Let's unpack the command and explain what happened above. We added a new pool of runners to GARM, that belongs to the `gabriel-samfira/garm` repository. We used the `incus` provider to create the pool, and we specified the `--image` and `--flavor` options to tell the provider what kind of runners we want to create. On Incus and LXD, the flavor maps to a `profile`. The profile can specify the resources allocated to a container or VM (RAM, CPUs, disk space, etc). The image maps to an incus or LXD image, as you would normally use when spinning up a new container or VM using the `incus launch` command.

We also specified the `--min-idle-runners` option to tell GARM to always keep at least 1 runner idle in the pool. This is useful for repositories that have a lot of workflows that run often, and we want to make sure that we always have a runner ready to pick up a job.

If we review the output of the command, we can see that the pool was created with a maximum number of 5 runners. This is just a default we can tweak when creating the pool, or later using the `garm-cli pool update` command. We can also see that the pool was created with a runner botstrap timeout of 20 minutes. This timeout is important on provider where the instance may take a long time to spin up. For example, on Equinix Metal, some operating systems can take a few minutes to install and reboot. This timeout can be tweaked to a higher value to account for this.

The pool was created with the `--enabled` flag set to `false`, so the pool won't create any runners yet:

```bash
ubuntu@garm:~$ garm-cli runner list 9daa34aa-a08a-4f29-a782-f54950d8521a
+----+------+--------+---------------+---------+
| NR | NAME | STATUS | RUNNER STATUS | POOL ID |
+----+------+--------+---------------+---------+
+----+------+--------+---------------+---------+
```

### Listing pools

To list pools created for a repository you can run:

```bash
ubuntu@garm:~$ garm-cli pool list --repo=be3a0673-56af-4395-9ebf-4521fea67567
+--------------------------------------+---------------------------+---------+--------------+------------+-------+---------+---------------+
| ID                                   | IMAGE                     | FLAVOR  | TAGS         | BELONGS TO | LEVEL | ENABLED | RUNNER PREFIX |
+--------------------------------------+---------------------------+---------+--------------+------------+-------+---------+---------------+
| 9daa34aa-a08a-4f29-a782-f54950d8521a | images:ubuntu/22.04/cloud | default | ubuntu incus |            |       | false   | garm          |
+--------------------------------------+---------------------------+---------+--------------+------------+-------+---------+---------------+
```

If you want to list pools for an organization or enterprise, you can use the `--org` or `--enterprise` options respectively.

You can also list **all** pools from all configureg github entities by using the `--all` option.

```bash
ubuntu@garm:~/garm$ garm-cli pool list --all
+--------------------------------------+---------------------------+--------------+-----------------------------------------+------------------+-------+---------+---------------+----------+
| ID                                   | IMAGE                     | FLAVOR       | TAGS                                    | BELONGS TO       | LEVEL | ENABLED | RUNNER PREFIX | PRIORITY |
+--------------------------------------+---------------------------+--------------+-----------------------------------------+------------------+-------+---------+---------------+----------+
| 8935f6a6-f20f-4220-8fa9-9075e7bd7741 | windows_2022              | c3.small.x86 | self-hosted x64 Windows windows equinix | gsamfira/scripts | repo  | false   | garm          |        0 |
+--------------------------------------+---------------------------+--------------+-----------------------------------------+------------------+-------+---------+---------------+----------+
| 9233b3f5-2ccf-4689-8f86-a8a0d656dbeb | runner-upstream:latest    | small        | self-hosted x64 Linux k8s org           | gsamfira         | org   | false   | garm          |        0 |
+--------------------------------------+---------------------------+--------------+-----------------------------------------+------------------+-------+---------+---------------+----------+
```

### Showing pool info

You can get detailed information about a pool by running the following command:

```bash
ubuntu@garm:~$ garm-cli pool show 9daa34aa-a08a-4f29-a782-f54950d8521a
+--------------------------+----------------------------------------+
| FIELD                    | VALUE                                  |
+--------------------------+----------------------------------------+
| ID                       | 9daa34aa-a08a-4f29-a782-f54950d8521a   |
| Provider Name            | incus                                  |
| Image                    | images:ubuntu/22.04/cloud              |
| Flavor                   | default                                |
| OS Type                  | linux                                  |
| OS Architecture          | amd64                                  |
| Max Runners              | 5                                      |
| Min Idle Runners         | 1                                      |
| Runner Bootstrap Timeout | 20                                     |
| Tags                     | ubuntu, incus                          |
| Belongs to               | gabriel-samfira/garm                   |
| Level                    | repo                                   |
| Enabled                  | false                                  |
| Runner Prefix            | garm                                   |
| Extra specs              |                                        |
| GitHub Runner Group      |                                        |
+--------------------------+----------------------------------------+
```

### Deleting a pool

In order to delete a pool, you must first make sure there are no runners in the pool. To ensure this, we can first disable the pool, to make sure no new runners are created, remove the runners or allow them to be user, then we can delete the pool.

To disable a pool, you can use the following command:

```bash
ubuntu@garm:~$ garm-cli pool update 9daa34aa-a08a-4f29-a782-f54950d8521a --enabled=false
+--------------------------+----------------------------------------+
| FIELD                    | VALUE                                  |
+--------------------------+----------------------------------------+
| ID                       | 9daa34aa-a08a-4f29-a782-f54950d8521a   |
| Provider Name            | incus                                  |
| Image                    | images:ubuntu/22.04/cloud              |
| Flavor                   | default                                |
| OS Type                  | linux                                  |
| OS Architecture          | amd64                                  |
| Max Runners              | 5                                      |
| Min Idle Runners         | 1                                      |
| Runner Bootstrap Timeout | 20                                     |
| Tags                     | ubuntu, incus                          |
| Belongs to               | gabriel-samfira/garm                   |
| Level                    | repo                                   |
| Enabled                  | false                                  |
| Runner Prefix            | garm                                   |
| Extra specs              |                                        |
| GitHub Runner Group      |                                        |
+--------------------------+----------------------------------------+
```

If there are no runners in the pool, you can then remove it:

```bash
ubuntu@garm:~$ garm-cli pool delete 9daa34aa-a08a-4f29-a782-f54950d8521a
```

### Update a pool

You can update a pool by using the `garm-cli pool update` command. Nearly every aspect of a pool can be updated after it has been created. To demonstrate the command, we can enable the pool we created earlier:

```bash
ubuntu@garm:~$ garm-cli pool update 9daa34aa-a08a-4f29-a782-f54950d8521a --enabled=true
+--------------------------+----------------------------------------+
| FIELD                    | VALUE                                  |
+--------------------------+----------------------------------------+
| ID                       | 9daa34aa-a08a-4f29-a782-f54950d8521a   |
| Provider Name            | incus                                  |
| Image                    | images:ubuntu/22.04/cloud              |
| Flavor                   | default                                |
| OS Type                  | linux                                  |
| OS Architecture          | amd64                                  |
| Max Runners              | 5                                      |
| Min Idle Runners         | 1                                      |
| Runner Bootstrap Timeout | 20                                     |
| Tags                     | ubuntu, incus                          |
| Belongs to               | gabriel-samfira/garm                   |
| Level                    | repo                                   |
| Enabled                  | true                                   |
| Runner Prefix            | garm                                   |
| Extra specs              |                                        |
| GitHub Runner Group      |                                        |
+--------------------------+----------------------------------------+
```

See `garm-cli pool update --help` for a list of settings that can be changed.

Now that the pool is enabled, GARM will start creating runners for it. We can list the runners in the pool to see if any have been created:

```bash
ubuntu@garm:~$ garm-cli runner list 9daa34aa-a08a-4f29-a782-f54950d8521a
+----+-------------------+---------+---------------+--------------------------------------+
| NR | NAME              | STATUS  | RUNNER STATUS | POOL ID                              |
+----+-------------------+---------+---------------+--------------------------------------+
|  1 | garm-BFrp51VoVBCO | running | installing    | 9daa34aa-a08a-4f29-a782-f54950d8521a |
+----+-------------------+---------+---------------+--------------------------------------+
```

We can see that a runner has been created and is currently being installed. If we check incus, we should also see it there as well:

```bash
root@incus:~# incus list
+-------------------+---------+----------------------+-----------------------------------------------+-----------+-----------+
|       NAME        |  STATE  |         IPV4         |                     IPV6                      |   TYPE    | SNAPSHOTS |
+-------------------+---------+----------------------+-----------------------------------------------+-----------+-----------+
| garm-BFrp51VoVBCO | RUNNING | 10.23.120.217 (eth0) | fd42:e6ea:8b6c:6cb9:216:3eff:feaa:fabf (eth0) | CONTAINER | 0         |
+-------------------+---------+----------------------+-----------------------------------------------+-----------+-----------+
```

Awesome! This runner will be able to pick up jobs that match the labels we've set on the pool.

## Runners

### Listing runners

You can list runners for a pool, for a repository, organization or enterprise, or for all of them. To list all runners, you can run:

```bash
ubuntu@garm:~$ garm-cli runner list --all
+----+---------------------+---------+---------------+--------------------------------------+
| NR | NAME                | STATUS  | RUNNER STATUS | POOL ID                              |
+----+---------------------+---------+---------------+--------------------------------------+
|  1 | garm-jZWtnxYHR6sG   | running | idle          | 8ec34c1f-b053-4a5d-80d6-40afdfb389f9 |
+----+---------------------+---------+---------------+--------------------------------------+
|  2 | garm-2vtBBaT2dgIvFg | running | idle          | c03c8101-3ae0-49d7-98b7-298a3689d24c |
+----+---------------------+---------+---------------+--------------------------------------+
|  3 | garm-Ew7SzN6LVlEC   | running | idle          | 577627f4-1add-4a45-9c62-3a7cbdec8403 |
+----+---------------------+---------+---------------+--------------------------------------+
|  4 | garm-BFrp51VoVBCO   | running | idle          | 9daa34aa-a08a-4f29-a782-f54950d8521a |
+----+---------------------+---------+---------------+--------------------------------------+
```

Have a look at the help command for the flags available to the `list` subcommand.

### Showing runner info

You can get detailed information about a runner by running the following command:

```bash
ubuntu@garm:~$ garm-cli runner show garm-BFrp51VoVBCO
+-----------------+------------------------------------------------------------------------------------------------------+
| FIELD           | VALUE                                                                                                |
+-----------------+------------------------------------------------------------------------------------------------------+
| ID              | b332a811-0ebf-474c-9997-780124e22382                                                                 |
| Provider ID     | garm-BFrp51VoVBCO                                                                                    |
| Name            | garm-BFrp51VoVBCO                                                                                    |
| OS Type         | linux                                                                                                |
| OS Architecture | amd64                                                                                                |
| OS Name         | Ubuntu                                                                                               |
| OS Version      | 22.04                                                                                                |
| Status          | running                                                                                              |
| Runner Status   | idle                                                                                                 |
| Pool ID         | 9daa34aa-a08a-4f29-a782-f54950d8521a                                                                 |
| Addresses       | 10.23.120.217                                                                                        |
|                 | fd42:e6ea:8b6c:6cb9:216:3eff:feaa:fabf                                                               |
| Status Updates  | 2024-02-11T23:39:54: downloading tools from https://github.com/actions/runner/releases/download/v2.3 |
|                 | 12.0/actions-runner-linux-x64-2.312.0.tar.gz                                                         |
|                 | 2024-02-11T23:40:04: extracting runner                                                               |
|                 | 2024-02-11T23:40:07: installing dependencies                                                         |
|                 | 2024-02-11T23:40:13: configuring runner                                                              |
|                 | 2024-02-11T23:40:13: runner registration token was retrieved                                         |
|                 | 2024-02-11T23:40:19: runner successfully configured after 1 attempt(s)                               |
|                 | 2024-02-11T23:40:20: installing runner service                                                       |
|                 | 2024-02-11T23:40:20: starting service                                                                |
|                 | 2024-02-11T23:40:21: runner successfully installed                                                   |
+-----------------+------------------------------------------------------------------------------------------------------+
```

### Deleting a runner

You can delete a runner by running the following command:

```bash
garm-cli runner rm garm-BFrp51VoVBCO
```

Only idle runners can be removed. If a runner is executing a job, it cannot be removed. However, a runner that is currently running a job, will be removed anyway when that job finishes. You can wait for the job to finish or you can cancel the job from the github workflow page.

In some cases, providers may error out when creating or deleting a runner. This can happen if the provider is misconfigured. To avoid situations in which GARM gets deadlocked trying to remove a runner from a provider that is in err, we can forcefully remove a runner. The `--force` flag will make GARM ignore any error returned by the provider when attempting to delete an instance:

```bash
garm-cli runner remove --force garm-BFrp51VoVBCO
```

Awesome! We've covered all the major parts of using GARM. This is all you need to have your workflows run on your self-hosted runners. Of course, each provider may have its own particularities, config options, extra specs and caveats (all of which should be documented in the provider README), but once added to the GARM config, creating a pool should be the same.

## The debug-log command

GARM outputs logs to standard out, log files and optionally to a websocket for easy debugging. This is just a convenience feature that allows you to stream logs to your terminal without having to log into the server. It's disabled by default, but if you enable it, you'll be able to run:

```bash
ubuntu@garm:~$ garm-cli debug-log 
time=2024-02-12T08:36:18.584Z level=INFO msg=access_log method=GET uri=/api/v1/ws user_agent=Go-http-client/1.1 ip=127.0.0.1:47260 code=200 bytes=0 request_time=447.445µs
time=2024-02-12T08:36:31.251Z level=INFO msg=access_log method=GET uri=/api/v1/instances user_agent=Go-http-client/1.1 ip=127.0.0.1:58460 code=200 bytes=1410 request_time=656.184µs
```

This will bring a real-time log to your terminal. While this feature should be fairly secure, I encourage you to only expose it within networks you know are secure. This can be done by configuring a reverse proxy in front of GARM that only allows connections to the websocket endpoint from certain locations.

## The debug-events command

Starting with GARM v0.1.5 a new command has been added to the CLI that consumes database events recorded by GARM. Whenever something is updated in the database, a new event is generated. These events are generated by the database watcher and are also exported via a websocket endpoint. This websocket endpoint is meant to be consumed by applications that wish to integrate GARM and want to avoid having to poll the API.

This command is not meant to be used to integrate GARM events, it is mearly a debug tool that allows you to see what events are being generated by GARM. To use it, you can run:

```bash
garm-cli debug-events --filters='{"send-everything": true}'
```

This command will send all events to your CLI as they happen. You can also filter by entity or operation like so:

```bash
garm-cli debug-events --filters='{"filters": [{"entity-type": "instance", "operations": ["create", "delete"]}, {"entity-type": "pool"}, {"entity-type": "controller"}]}'
```

The payloads that get sent to your terminal are described in the [events](/doc/events.md) section, but the short description is that you get the operation type (create, update, delete), the entity type (instance, pool, repo, etc) and the json payload as you normaly would when you fetch them through the API. Sensitive info like tokens or passwords are never returned.

## Listing recorded jobs

GARM will record any job that comes in and for which we have a pool configured. If we don't have a pool for a particular job, then that job is ignored. There is no point in recording jobs that we can't do anything about. It would just bloat the database for no reason.

To view existing jobs, run the following command:

```bash
garm-cli job list
```

If you've just set up GARM and have not yet created a pool or triggered a job, this will be empty. If you've configured everything and still don't receive jobs, you'll need to make sure that your URLs (discussed at the begining of this article), are correct. GitHub needs to be able to reach the webhook URL that our GARM instance listens on.