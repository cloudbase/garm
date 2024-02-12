# Using GARM

This document will walk you through the various commands and options available in GARM. It is assumed that you have already installed GARM and have it running. If you haven't, please check out the [quickstart](/doc/quickstart.md) document for instructions on how to install GARM.

While using the GARM cli, you will most likely spend most of your time listing pools and runners, but we will cover most of the available commands and options. Some of them we'll skip (like the `init` or `profile` subcommands), as they've been covered in the [quickstart](/doc/quickstart.md) document.

## Table of contents


## Listing controller info

You can list the controller info by running the following command:

```bash
garm-cli controller-info show
+------------------------+----------------------------------------------------------------------------+
| FIELD                  | VALUE                                                                      |
+------------------------+----------------------------------------------------------------------------+
| Controller ID          | a4dd5f41-8e1e-42a7-af53-c0ba5ff6b0b3                                       |
| Hostname               | garm                                                                       |
| Metadata URL           | https://garm.example.com/api/v1/metadata                                   |
| Callback URL           | https://garm.example.com/api/v1/callbacks                                  |
| Webhook Base URL       | https://garm.example.com/webhooks                                          |
| Controller Webhook URL | https://garm.example.com/webhooks/a4dd5f41-8e1e-42a7-af53-c0ba5ff6b0b3     |
+------------------------+----------------------------------------------------------------------------+
```

There are several things of interest in this output.

* `Controller ID` - This is the unique identifier of the controller. Each GARM installation, on first run will automatically generate a unique controller ID. This is important for several reasons. For one, it allows us to run several GARM controllers on the same repos/orgs/enterprises, without accidentally clasing with each other. Each runner started by a GARM controller, will be tagged with this controller ID in order to easily identify runners that we manage.
* `Hostname` - This is the hostname of the machine where GARM is running. This is purely informative.
* `Metadata URL` - This URL is configured by the user in the GARM config file, and is the URL that is presented to the runners via userdata when they get set up. Runners will connect to this URL and retrieve information they might need to set themselves up. GARM cannot automatically determine this URL, as it is dependent on the user's network setup. GARM may be hidden behind a load balancer or a reverse proxy, in which case, the URL by which the GARM controller can be accessed may be different than the IP addresses that are locally visible to GARM.
* `Callback URL` - This URL is configured by the user in the GARM config file, and is the URL that is presented to the runners via userdata when they get set up. Runners will connect to this URL and send status updates and system information (OS version, OS name, github runner agent ID, etc) to the controller.
* `Webhook Base URL` - This is the base URL for webhooks. It is configured by the user in the GARM config file. This URL can be called into by GitHub itself when hooks get triggered by a workflow. GARM needs to know when a new job is started in order to schedule the createion of a new runner. Job webhooks sent to this URL will be recorded by GARM and acter upon. While you can configure this URL directly in your GitHub repo settings, it is advised to use the `Controller Webhook URL` instead, as it is unique to each controller, and allows you to potentially install multiple GARM controller inside the same repo.
* `Controller Webhook URL` - This is the URL that GitHub will call into when a webhook is triggered. This URL is unique to each GARM controller and is the preferred URL to use in order to receive webhooks from GitHub. It serves the same purpose as the `Webhook Base URL`, but is unique to each controller, allowing you to potentially install multiple GARM controllers inside the same repo.

We will see the `Controller Webhook URL` later when we set up the GitHub repo to send webhooks to GARM.

## Listing configured providers

GARM uses providers to create runners. These providers are external executables that GARM calls into to create runners in a particular IaaS.

Once configured (see [provider configuration](/doc/providers.md)), you can list the configured providers by running the following command:

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

## Listing github credentials

GARM needs access to your GitHub repositories, organizations or enterprise in order to manage runners. This is done via a [GitHub personal access token](/doc/github_credentials.md). You can configure multiple tokens with access to various repositories, organizations or enterprises, either on GitHub or on GitHub Enterprise Server.

The credentials sections allow you to override the API URL, Upload URL and base URLs, unlocking the ability to use GARM with GitHub Enterprise Server.

To list existing credentials, run the following command:

```bash
ubuntu@garm:~$ garm-cli credentials list 
+-------------+------------------------------------+--------------------+-------------------------+-----------------------------+
| NAME        | DESCRIPTION                        | BASE URL           | API URL                 | UPLOAD URL                  |
+-------------+------------------------------------+--------------------+-------------------------+-----------------------------+
| gabriel     | github token or user gabriel       | https://github.com | https://api.github.com/ | https://uploads.github.com/ |
+-------------+------------------------------------+--------------------+-------------------------+-----------------------------+
| gabriel_org | github token with org level access | https://github.com | https://api.github.com/ | https://uploads.github.com/ |
+-------------+------------------------------------+--------------------+-------------------------+-----------------------------+
```

These credentials are configured in the GARM config file. You can add, remove or modify them as needed. When using GitHub, you don't need to explicitly set the API URL, Upload URL or base URL, as they are automatically set to the GitHub defaults. When using GitHub Enterprise Server, you will need to set these URLs explicitly. See the [github credentials](/doc/github_credentials.md) section for more details.

## Adding a new repository

To add a new repository we need to use credentials that has access to the repository. We've listed credentials above, so let's add our first repository:

```bash
ubuntu@garm:~$ garm-cli repository add \
    --name garm \
    --owner gabriel-samfira \
    --credentials gabriel \
    --install-webhook \
    --random-webhook-secret
+----------------------+--------------------------------------+
| FIELD                | VALUE                                |
+----------------------+--------------------------------------+
| ID                   | be3a0673-56af-4395-9ebf-4521fea67567 |
| Owner                | gabriel-samfira                      |
| Name                 | garm                                 |
| Credentials          | gabriel                              |
| Pool manager running | true                                 |
+----------------------+--------------------------------------+
```

Lets break down the command a bit and explain what happened above. We added a new repository to GARM, that belogs to the user `gabriel-samfira` and is called `garm`. When using GitHub, this translates to `https://github.com/gabriel-samfira/garm`.

As part of the above command, we used the credentials called `gabriel` to authenticate to GitHub. If those credentials didn't have access to the repository, we would have received an error when adding the repo.

The other interesting bit about the above command is that we automatically added the `webhook` to the repository and generated a secure random secret to authenticate the webhooks that come in from GitHub for this new repo. Any webhook claiming to be for the `gabriel-samfira/garm` repo, will be validated against the secret that was generated.

### Managing repository webhooks

The `webhook` URL that was used, will correspond to the `Controller Webhook URL` that we saw earlier when we listed the controller info. Let's list it and see what it looks like:

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

The `--install-webhook` and `--random-webhook-secret` options are convenience options that allow you to quickly add a new repository to GARM and have it ready to receive webhooks from GitHub. If you don't want to install the webhook, you can add the repository without it, and then install it later using the `garm-cli repository webhook install` command (which we'll show in a second) or manually add it in the GitHub UI.

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
| URL          | https://webhooks.samfira.com/webhooks/a4dd5f41-8e1e-42a7-af53-c0ba5ff6b0b3 |
| Events       | [workflow_job]                                                             |
| Active       | true                                                                       |
| Insecure SSL | false                                                                      |
+--------------+----------------------------------------------------------------------------+
```

To allow GARM to manage webhooks, the PAT you're using must have the `admin:repo_hook` and `admin:org_hook` scopes. Webhook management is not available for enterprises. For enterprises you will have to add the webhook manually.

To manually add a webhook, see the [webhooks](/doc/webhooks.md) section.

## Listing repositories

To list existing repositories, run the following command:

```bash
ubuntu@garm:~$ garm-cli repository list
+--------------------------------------+-----------------+---------+------------------+------------------+
| ID                                   | OWNER           | NAME    | CREDENTIALS NAME | POOL MGR RUNNING |
+--------------------------------------+-----------------+---------+------------------+------------------+
| be3a0673-56af-4395-9ebf-4521fea67567 | gabriel-samfira | garm    | gabriel          | true             |
+--------------------------------------+-----------------+---------+------------------+------------------+
```

This will list all the repositories that GARM is currently managing.

## Removing a repository

To remove a repository, you can use the following command:

```bash
garm-cli repository delete be3a0673-56af-4395-9ebf-4521fea67567
```

This will remove the repository from GARM, and if a webhook was installed, will also clean up the webhook from the repository.

Note: GARM will not remove a webhook that points to the `Base Webhook URL`. It will only remove webhooks that are namespaced to the running controller.

## Adding a new organization

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

Managing webhooks for organizations is similar to managing webhooks for repositories. You can list, show, install and uninstall webhooks for organizations using the `garm-cli organization webhook` subcommand. We won't go into details here, as it's similar to managing webhooks for repositories.

All the other operations that exist on repositories, like listing, removing, etc, also exist for organizations and enterprises. Have a look at the help for the `garm-cli organization` subcommand for more details.

## Adding an enterprise

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

## Creating a runner pool

Now that we have a repository, organization or enterprise added to GARM, we can create a runner pool for it. A runner pool is a collection of runners of the same type, that are managed by GARM and are used to run workflows for the repository, organization or enterprise.

You can create multiple pools of runners for the same entity (repository, organization or enterprise), and you can create pools of runners of different types. For example, you can have a pool of runners that are created on AWS, and another pool of runners that are created on Azure, k8s, LXD, etc. For repositories or organizations with complex needs, you can set up a number of pools that cover a wide range of needs, based on cost, capability (GPUs, FPGAs, etc) or sheer raw computing power. You don't have to pick just one and managing all of them is done using the exact same commands, as we'll show below.

Before we create a pool, we have to decide on which provider we want to use. We've listed the providers above, so let's pick one and create a pool of runners for our repository. For the purpose of this example, we'll use the `incus` provider. We'll show you how to create a pool using this provider, but keep in mind that adding another pool using a different provider is done using the exact same commands. The only difference will be in the `--image`, `--flavor` and `--extra-specs` options that you'll use when creating the pool.

Out of those three options, only the `--image` and `--flavor` are mandatory. The `--extra-specs` option is optional and is used to pass additional information to the provider when creating the pool. The `--extra-specs` option is provider specific, and you'll have to consult the provider documentation to see what options are available.

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
| Tags                     | self-hosted, x64, Linux, ubuntu, incus |
| Belongs to               | gabriel-samfira/garm                   |
| Level                    | repo                                   |
| Enabled                  | false                                  |
| Runner Prefix            | garm                                   |
| Extra specs              |                                        |
| GitHub Runner Group      |                                        |
+--------------------------+----------------------------------------+
```

Let's unpack the command a bit and explain what happened above. We added a new pool of runners to GARM, that belongs to the `gabriel-samfira/garm` repository. We used the `incus` provider to create the pool, and we specified the `--image` and `--flavor` options to tell the provider what kind of runners we want to create. On Incus and LXD, the flavor maps to a `profile` and the image maps to an incus or LXD image, as you would normally use when spinning up a new container or VM using the `incus launch` command.

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

## Listing pools

To list pools created for a repository you can run:

```bash
ubuntu@garm:~$ garm-cli pool list --repo=be3a0673-56af-4395-9ebf-4521fea67567
+--------------------------------------+---------------------------+---------+------------------------------------+------------+-------+---------+---------------+
| ID                                   | IMAGE                     | FLAVOR  | TAGS                               | BELONGS TO | LEVEL | ENABLED | RUNNER PREFIX |
+--------------------------------------+---------------------------+---------+------------------------------------+------------+-------+---------+---------------+
| 9daa34aa-a08a-4f29-a782-f54950d8521a | images:ubuntu/22.04/cloud | default | self-hosted x64 Linux ubuntu incus |            |       | false   | garm          |
+--------------------------------------+---------------------------+---------+------------------------------------+------------+-------+---------+---------------+
```

If you want to list pools for an organization or enterprise, you can use the `--org` or `--enterprise` options respectively.

## Showing pool info

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
| Tags                     | self-hosted, x64, Linux, ubuntu, incus |
| Belongs to               | gabriel-samfira/garm                   |
| Level                    | repo                                   |
| Enabled                  | false                                  |
| Runner Prefix            | garm                                   |
| Extra specs              |                                        |
| GitHub Runner Group      |                                        |
+--------------------------+----------------------------------------+
```

## Deleting a pool

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
| Tags                     | self-hosted, x64, Linux, ubuntu, incus |
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

## Update a pool

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
| Tags                     | self-hosted, x64, Linux, ubuntu, incus |
| Belongs to               | gabriel-samfira/garm                   |
| Level                    | repo                                   |
| Enabled                  | true                                   |
| Runner Prefix            | garm                                   |
| Extra specs              |                                        |
| GitHub Runner Group      |                                        |
+--------------------------+----------------------------------------+
```

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

Awesome! This runner will be able to pick up bobs that match the labels we've set on the pool.

## Listing runners

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

## Showing runner info

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

## Deleting a runner

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

## Listing recorded jobs

GARM will record any job that comes in and for which we have a pool configured. If we don't have a pool for a particular job, then that job is ignored. There is no point in recording jobs that we can't do anything about. It would just bloat the database for no reason.

To view existing jobs, run the following command:

```bash
garm-cli job list
```

If you've just set up GARM and have not yet created a pool or triggered a job, this will be empty. If you've configured everything and still don't receive jobs, you'll need to make sure that your URLs (discussed at the begining of this article), are correct. GitHub needs to be able to reach the webhook URL that our GARM instance listens on.