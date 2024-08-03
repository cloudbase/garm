# Quick start

<!-- TOC -->

- [Quick start](#quick-start)
    - [Create the config folder](#create-the-config-folder)
    - [The config file](#the-config-file)
    - [The provider section](#the-provider-section)
    - [Starting the service](#starting-the-service)
        - [Using Docker](#using-docker)
        - [Setting up GARM as a system service](#setting-up-garm-as-a-system-service)
    - [Initializing GARM](#initializing-garm)
    - [Setting up the webhook](#setting-up-the-webhook)
    - [Creating a GitHub endpoint Optional](#creating-a-github-endpoint-optional)
    - [Adding credentials](#adding-credentials)
    - [Define a repo](#define-a-repo)
    - [Create a pool](#create-a-pool)

<!-- /TOC -->

## Create the config folder

All of our config files and data will be stored in `/etc/garm`. Let's create that folder:

```bash
sudo mkdir -p /etc/garm
```

Coincidentally, this is also where the docker container [looks for the config](../Dockerfile#L29) when it starts up. You can either use `Docker` or you can set up garm directly on your system. We'll walk you through both options. In both cases, we need to first create the config folder and a proper config file.

## The config file

There is a full config file, with detailed comments for each option, in the [testdata folder](../testdata/config.toml). You can use that as a reference. But for the purposes of this guide, we'll be using a minimal config file and add things on as we proceed.

Open `/etc/garm/config.toml` in your favorite editor and paste the following:

```toml
[default]
enable_webhook_management = true

[logging]
# If using nginx, you'll need to configure connection upgrade headers
# for the /api/v1/ws location. See the sample config in the testdata
# folder.
enable_log_streamer = true
# Set this to "json" if you want to consume these logs in something like
# Loki or ELK.
log_format = "text"
log_level = "info"
log_source = false

[metrics]
enable = true
disable_auth = false

[jwt_auth]
# Obviously, this needs to be changed :).
secret = ")9gk_4A6KrXz9D2u`0@MPea*sd6W`%@5MAWpWWJ3P3EqW~qB!!(Vd$FhNc*eU4vG"
time_to_live = "8760h"

[apiserver]
  bind = "0.0.0.0"
  port = 80
  use_tls = false

[database]
  backend = "sqlite3"
  # This needs to be changed.
  passphrase = "shreotsinWadquidAitNefayctowUrph"
  [database.sqlite3]
    db_file = "/etc/garm/garm.db"
```

This is a minimal config, with no providers defined. In this example we have the [default](./config_default.md), [logging](./config_logging.md), [metrics](./config_metrics.md), [jwt_auth](./config_jwt_auth.md), [apiserver](./config_api_server.md) and [database](./database.md) sections. Each are documented separately. Feel free to read through the available docs if, for example you need to enable TLS without using an nginx reverse proxy or if you want to enable the debug server, the log streamer or a log file.

In this sample config we:

* set up logging prefrences
* enable metrics with authentication
* set a JWT secret which is used to sign JWT tokens
* set a time to live for the JWT tokens
* enable the API server on port `80` and bind it to all interfaces
* set the database backend to `sqlite3` and set a passphrase for sealing secrets (just webhook secrets for now)

At this point, we have a valid config file, but we still need to add the `provider` section.

## The provider section

This is where you have a decision to make. GARM has a number of providers you can leverage. At the time of this writing, we have support for:

* [OpenStack](https://github.com/cloudbase/garm-provider-openstack)
* [Azure](https://github.com/cloudbase/garm-provider-azure)
* [Kubernetes](https://github.com/mercedes-benz/garm-provider-k8s) - Thanks to the amazing folks at @mercedes-benz for sharing their awesome provider!
* [LXD](https://github.com/cloudbase/garm-provider-lxd)
* [Incus](https://github.com/cloudbase/garm-provider-incus)
* [Equinix Metal](https://github.com/cloudbase/garm-provider-equinix)
* [Amazon EC2](https://github.com/cloudbase/garm-provider-aws)
* [Google Cloud Platform (GCP)](https://github.com/cloudbase/garm-provider-gcp)
* [Oracle Cloud Infrastructure (OCI)](https://github.com/cloudbase/garm-provider-oci)

The easiest provider to set up is probably the LXD or Incus provider. Incus is a fork of LXD so the functionality is identical (for now). For the purpose of this document, we'll continue with LXD. You don't need an account on an external cloud. You can just use your machine.

You will need to have LXD installed and configured. There is an excellent [getting started guide](https://documentation.ubuntu.com/lxd/en/latest/getting_started/) for LXD. Follow the instructions there to install and configure LXD, then come back here.

Once you have LXD installed and configured, you can add the provider section to your config file. If you're connecting to the `local` LXD installation, the [config snippet for the LXD provider](https://github.com/cloudbase/garm-provider-lxd/blob/4ee4e6fc579da4a292f40e0f7deca1e396e223d0/testdata/garm-provider-lxd.toml) will work out of the box. We'll be connecting using the unix socket so no further configuration will be needed.

Go ahead and create a new config in a location where GARM can access it and paste that entire snippet. For the purposes of this doc, we'll assume you created a new file called `/etc/garm/garm-provider-lxd.toml`. That config file will be used by the provider itself. Remember, the providers are external executables that are called by GARM. They have their own configs which are relevant only to those executables, not GARM itself.

We now need to define the provider in the GARM config file and tell GARM how it can find both the provider binary and the provider specific config file. To do that, open the GARM config file `/etc/garm/config.toml` in your favorite editor and paste the following config snippet at the end:

```toml
[[provider]]
  name = "lxd_local"
  provider_type = "external"
  description = "Local LXD installation"
  [provider.external]
    provider_executable = "/opt/garm/providers.d/garm-provider-lxd"
    config_file = "/etc/garm/garm-provider-lxd.toml"
```

This config snippet assumes that the LXD provider executable is available, or is going to be available in `/opt/garm/providers.d/garm-provider-lxd`. If you're using the container image, the executable is already there. If you're installing GARM as a systemd service, don't worry, instructions on how to get the LXD provider executable are coming up.

## Starting the service

You can start GARM using docker or directly on your system. I'll show you both ways.

### Using Docker

If you're using docker, you can start the service with:

```bash
docker run -d \
  --name garm \
  -p 80:80 \
  -v /etc/garm:/etc/garm:rw \
  -v /var/snap/lxd/common/lxd/unix.socket:/var/snap/lxd/common/lxd/unix.socket:rw \
  ghcr.io/cloudbase/garm:v0.1.4
```

You will notice we also mounted the LXD unix socket from the host inside the container where the config you pasted expects to find it. If you plan to use an external provider that does not need to connect to LXD over a unix socket, feel free to remove that mount.

Check the logs to make sure everything is working as expected:

```bash
ubuntu@garm:~$ docker logs garm
signal.NotifyContext(context.Background, [interrupt terminated])
2023/07/17 21:55:43 Loading provider lxd_local
2023/07/17 21:55:43 registering prometheus metrics collectors
2023/07/17 21:55:43 setting up metric routes
```

### Setting up GARM as a system service

This process is a bit more involved. We'll need to create a new user for garm and set up permissions for that user to connect to LXD.

First, create the user:

```bash
useradd --shell /usr/bin/false \
      --system \
      --groups lxd \
      --no-create-home garm
```

Adding the `garm` user to the LXD group will allow it to connect to the LXD unix socket. We'll need that considering the config we crafted above. The recommendation is to use TCP connections to connect to a remote LXD installation. The local setup of an LXD provider is just for demonstration purposes/testing.

Next, download the latest release from the [releases page](https://github.com/cloudbase/garm/releases).

```bash
wget -q -O - https://github.com/cloudbase/garm/releases/download/v0.1.5/garm-linux-amd64.tgz |  tar xzf - -C /usr/local/bin/
```

We'll be running under an unprivileged user. If we want to be able to listen on any port under `1024`, we'll have to set some capabilities on the binary:

```bash
setcap cap_net_bind_service=+ep /usr/local/bin/garm
```

Create a folder for the external providers:

```bash
sudo mkdir -p /opt/garm/providers.d
```

Download the LXD provider binary:

```bash
git clone https://github.com/cloudbase/garm-provider-lxd
cd garm-provider-lxd
go build -o /opt/garm/providers.d/garm-provider-lxd
```

Change the permissions on the config dir:

```bash
chown -R garm:garm /etc/garm
```

Copy the sample `systemd` service file:

```bash
wget -O /etc/systemd/system/garm.service \
  https://raw.githubusercontent.com/cloudbase/garm/v0.1.5/contrib/garm.service
```

Reload the `systemd` daemon and start the service:

```bash
systemctl daemon-reload
systemctl start garm
```

Check the logs to make sure everything is working as expected:

```bash
ubuntu@garm:~$ sudo journalctl -u garm
```

Check that you can make a request to the API:

```bash
ubuntu@garm:~$ docker logs garm
signal.NotifyContext(context.Background, [interrupt terminated])
2023/07/17 22:21:33 Loading provider lxd_local
2023/07/17 22:21:33 registering prometheus metrics collectors
2023/07/17 22:21:33 setting up metric routes
```

Excellent! We have a working GARM installation. Now we need to initialize the controller and set up the webhook in GitHub.

## Initializing GARM

Before we can start using GARM, we need initialize it. This will create the `admin` user and generate a unique controller ID that will identify this GARM installation. This process allows us to use multiple GARM installations with the same GitHub account, if we want or need to. GARM will use the controller ID to identify the runners it creates. This way we won't run the risk of accidentally removing runners we don't manage.

To initialize GARM, we'll use the `garm-cli` tool. You can download the latest release from the [releases page](https://github.com/cloudbase/garm/releases):

```bash
wget -q -O - https://github.com/cloudbase/garm/releases/download/v0.1.5/garm-cli-linux-amd64.tgz |  tar xzf - -C /usr/local/bin/
```

Now we can initialize GARM:

```bash
ubuntu@garm:~$ garm-cli init --name="local_garm" --url http://garm.example.com
Username: admin
Email: admin@garm.example.com
✔ Password: ************█
✔ Confirm password: ************█
Congrats! Your controller is now initialized.

Following are the details of the admin user and details about the controller.

Admin user information:

+----------+--------------------------------------+
| FIELD    | VALUE                                |
+----------+--------------------------------------+
| ID       | 6b0d8f67-4306-4702-80b6-eb0e2e4ee695 |
| Username | admin                                |
| Email    | admin@garm.example.com               |
| Enabled  | true                                 |
+----------+--------------------------------------+

Controller information:

+------------------------+-----------------------------------------------------------------------+
| FIELD                  | VALUE                                                                 |
+------------------------+-----------------------------------------------------------------------+
| Controller ID          | 0c54fd66-b78b-450a-b41a-65af2fd0f71b                                  |
| Metadata URL           | http://garm.example.com/api/v1/metadata                               |
| Callback URL           | http://garm.example.com/api/v1/callbacks                              |
| Webhook Base URL       | http://garm.example.com/webhooks                                      |
| Controller Webhook URL | http://garm.example.com/webhooks/0c54fd66-b78b-450a-b41a-65af2fd0f71b |
+------------------------+-----------------------------------------------------------------------+

Make sure that the URLs in the table above are reachable by the relevant parties.

The metadata and callback URLs *must* be accessible by the runners that GARM spins up.
The base webhook and the controller webhook URLs must be accessible by GitHub or GHES. 
```

Every time you init a new GARM instance, a new profile will be created in your local `garm-cli` config. You can also log into an already initialized instance using:

```bash
garm-cli profile add \
  --name="another_garm" \
  --url https://garm2.example.com
```

Then you can switch between profiles using:

```bash
garm-cli profile switch another_garm
```

## Setting up the webhook

There are two options when it comes to setting up the webhook in GitHub. You can manually set up the webhook in the GitHub UI, and then use the resulting secret when creating the entity (repo, org, enterprise), or you can let GARM do it automatically if the app or PAT you're using has the [required privileges](./github_credentials.md).

If you want to manually set up the webhooks, have a look at the [webhooks doc](./webhooks.md) for more information.

In this guide, I'll show you how to do it automatically when adding a new repo, assuming you have the required privileges. Note, you'll still have to manually set up webhooks if you want to use GARM at the enterprise level. Automatic webhook management is only available for repos and orgs.

## Creating a GitHub endpoint (Optional)

This section is only of interest if you're using a GitHub Enterprise Server (GHES) deployment. If you're using [github.com](https://github.com), you can skip this section.

Let's list existing endpoints:

```bash
gabriel@rossak:~$ garm-cli github endpoint list
+------------+--------------------+-------------------------+
| NAME       | BASE URL           | DESCRIPTION             |
+------------+--------------------+-------------------------+
| github.com | https://github.com | The github.com endpoint |
+------------+--------------------+-------------------------+
```

By default, GARM creates a default `github.com` endpoint. This endpoint cannot be updated or deleted. If you want to add a new endpoint, you can do so using the `github endpoint create` command:

```bash
garm-cli github endpoint create \
    --name example \
    --description "Just an example ghes endpoint" \
    --base-url https://ghes.example.com \
    --upload-url https://upload.ghes.example.com \
    --api-base-url https://api.ghes.example.com \
    --ca-cert-path $HOME/ca-cert.pem
```

In this exampe, we add a new github endpoint called `example`. The `ca-cert-path` is optional and is used to verify the server's certificate. If you don't provide a path, GARM will use the system's default CA certificates.

## Adding credentials

Before we can add a new entity, we need github credentials to interact with that entity (manipulate runners, create webhooks, etc). Credentials are tied to a specific github endpoint. In this section we'll be adding credentials that are valid for either [github.com](https://github.com) or your own GHES server (if you added one in the previous section).

When creating a new entity (repo, org, enterprise) using the credentials you define here, GARM will automatically associate that entity with the gitHub endpoint that the credentials use.

If you want to swap the credentials for an entity, the new credentials will need to be associated with the same endpoint as the old credentials.

Let's add some credentials:

```bash
garm-cli github credentials add \
  --name gabriel \
  --description "GitHub PAT for user gabriel" \
  --auth-type pat \
  --pat-oauth-token gh_theRestOfThePAT \
  --endpoint github.com
```

You can also add a GitHub App as credentials. The process is similar, but you'll need to provide the `app_id`, `private_key_path` and `installation_id`:

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

All sensitive info is encrypted at rest. Also, the API will not return sensitive data.

## Define a repo

We now have a working GARM installation, with github credentials and a provider added. It's time to add a repo.

Before we add a repo, let's list credentials. We'll need their names when we'll add a new repo.

```bash
ubuntu@garm:~$ garm-cli github credentials list
+----+-------------+------------------------------------+--------------------+-------------------------+-----------------------------+------+
| ID | NAME        | DESCRIPTION                        | BASE URL           | API URL                 | UPLOAD URL                  | TYPE |
+----+-------------+------------------------------------+--------------------+-------------------------+-----------------------------+------+
|  1 | gabriel     | GitHub PAT for user gabriel        | https://github.com | https://api.github.com/ | https://uploads.github.com/ | pat  |
+----+-------------+------------------------------------+--------------------+-------------------------+-----------------------------+------+
|  2 | gabriel_app | Github App with access to repos    | https://github.com | https://api.github.com/ | https://uploads.github.com/ | app  |
+----+-------------+------------------------------------+--------------------+-------------------------+-----------------------------+------+
```

Now we can add a repo:

```bash
garm-cli repo add \
  --owner gsamfira \
  --name scripts \
  --credentials gabriel \
  --random-webhook-secret \
  --install-webhook \
  --pool-balancer-type roundrobin
```

This will add a new repo called `scripts` under the `gsamfira` org. We also tell GARM to generate a random secret and install a webhook using that random secret. If you want to use a specific secret, you can use the `--webhook-secret` option, but in that case, you'll have to manually set up the webhook in GitHub.

The `--pool-balancer-type` option is used to set the pool balancer type. That dictates how GARM will choose in which pool it should create a new runner when consuming recorded queued jobs. If `roundrobin` (default) is used, GARM will cycle through all pools and create a runner in the first pool that has available resources. If `pack` is used, GARM will try to fill up a pool before moving to the next one. The order of the pools is determined by the pool priority. We'll see more about pools in the next section.

You should see something like this:

```bash
gabriel@rossak:~$ garm-cli repo add \
  --name scripts \
  --credentials gabriel_org \
  --install-webhook \
  --random-webhook-secret \
  --owner gsamfira \
  --pool-balancer-type roundrobin
+----------------------+--------------------------------------+
| FIELD                | VALUE                                |
+----------------------+--------------------------------------+
| ID                   | 0c91d9fd-2417-45d4-883c-05daeeaa8272 |
| Owner                | gsamfira                             |
| Name                 | scripts                              |
| Pool balancer type   | roundrobin                           |
| Credentials          | gabriel_app                          |
| Pool manager running | true                                 |
+----------------------+--------------------------------------+
```

We can now list the repos:

```bash
gabriel@rock:~$ garm-cli repo ls
+--------------------------------------+----------+--------------+------------------+--------------------+------------------+
| ID                                   | OWNER    | NAME         | CREDENTIALS NAME | POOL BALANCER TYPE | POOL MGR RUNNING |
+--------------------------------------+----------+--------------+------------------+--------------------+------------------+
| 0c91d9fd-2417-45d4-883c-05daeeaa8272 | gsamfira | scripts      | gabriel          | roundrobin         | true             |
+--------------------------------------+----------+--------------+------------------+--------------------+------------------+
```

Excellent! Make a note of the ID. We'll need it later when we create a pool.

## Create a pool

This is the last step. You're almost there!

To create a pool we'll need the repo ID from the previous step (which we have) and a provider in which the pool will spin up new runners. We'll use the LXD provider we defined earlier, but we need its name:

```bash
gabriel@rossak:~$ garm-cli provider list
+-----------+------------------------+-----------+
| NAME      | DESCRIPTION            | TYPE      |
+-----------+------------------------+-----------+
| lxd_local | Local LXD installation | external  |
+-----------+------------------------+-----------+
```

Now we can create a pool:

```bash
garm-cli pool add \
  --repo 0c91d9fd-2417-45d4-883c-05daeeaa8272 \
  --enabled true \
  --provider-name lxd_local \
  --flavor default \
  --image ubuntu:22.04 \
  --max-runners 5 \
  --min-idle-runners 0 \
  --os-arch amd64 \
  --os-type linux \
  --tags ubuntu,generic
```

You should see something like this:

```bash
gabriel@rossak:~$ garm-cli pool add \
>   --repo 0c91d9fd-2417-45d4-883c-05daeeaa8272 \
>   --enabled true \
>   --provider-name lxd_local \
>   --flavor default \
>   --image ubuntu:22.04 \
>   --max-runners 5 \
>   --min-idle-runners 0 \
>   --os-arch amd64 \
>   --os-type linux \
>   --tags ubuntu,generic
+--------------------------+--------------------------------------------+
| FIELD                    | VALUE                                      |
+--------------------------+--------------------------------------------+
| ID                       | 344e4a72-2035-4a18-a3d5-87bd3874b56c       |
| Provider Name            | lxd_local                                  |
| Priority                 | 0                                          |
| Image                    | ubuntu:22.04                               |
| Flavor                   | default                                    |
| OS Type                  | linux                                      |
| OS Architecture          | amd64                                      |
| Max Runners              | 5                                          |
| Min Idle Runners         | 0                                          |
| Runner Bootstrap Timeout | 20                                         |
| Tags                     | ubuntu, generic                            |
| Belongs to               | gsamfira/scripts                           |
| Level                    | repo                                       |
| Enabled                  | true                                       |
| Runner Prefix            | garm                                       |
| Extra specs              |                                            |
| GitHub Runner Group      |                                            |
+--------------------------+--------------------------------------------+
```

If we list the pool we should see it:

```bash
gabriel@rock:~$ garm-cli pool ls -a
+--------------------------------------+---------------------------+--------------+-----------------+------------------+-------+---------+---------------+----------+
| ID                                   | IMAGE                     | FLAVOR       | TAGS            | BELONGS TO       | LEVEL | ENABLED | RUNNER PREFIX | PRIORITY |
+--------------------------------------+---------------------------+--------------+-----------------+------------------+-------+---------+---------------+----------+
| 344e4a72-2035-4a18-a3d5-87bd3874b56c | ubuntu:22.04              | default      | ubuntu generic  | gsamfira/scripts | repo  | true    |  garm         |        0 |
+--------------------------------------+---------------------------+--------------+-----------------+------------------+-------+---------+---------------+----------+
```

This pool is enabled, but the `min-idle-runners` option is set to 0. This means that it will not create any idle runners. It will only create runners when a job is started and a webhook is sent to our GARM server. Optionally, you can set `min-idle-runners` to a value greater than 0, but keep in mind that depending on the provider you use, this may incur cost.

For the purposes of this guide, we'll increase it to 1 so we have a runner created.

First, list current runners:

```bash
gabriel@rossak:~$ garm-cli runner ls -a
+----+------+--------+---------------+---------+
| NR | NAME | STATUS | RUNNER STATUS | POOL ID |
+----+------+--------+---------------+---------+
+----+------+--------+---------------+---------+
```

No runners. Now, let's update the pool and set `min-idle-runners` to 1:

```bash
gabriel@rossak:~$ garm-cli pool update 344e4a72-2035-4a18-a3d5-87bd3874b56c --min-idle-runners=1
+--------------------------+--------------------------------------------+
| FIELD                    | VALUE                                      |
+--------------------------+--------------------------------------------+
| ID                       | 344e4a72-2035-4a18-a3d5-87bd3874b56c       |
| Provider Name            | lxd_local                                  |
| Priority                 | 0                                          |
| Image                    | ubuntu:22.04                               |
| Flavor                   | default                                    |
| OS Type                  | linux                                      |
| OS Architecture          | amd64                                      |
| Max Runners              | 5                                          |
| Min Idle Runners         | 1                                          |
| Runner Bootstrap Timeout | 20                                         |
| Tags                     | ubuntu, generic                            |
| Belongs to               | gsamfira/scripts                           |
| Level                    | repo                                       |
| Enabled                  | true                                       |
| Runner Prefix            | garm                                       |
| Extra specs              |                                            |
| GitHub Runner Group      |                                            |
+--------------------------+--------------------------------------------+
```

Now if we list the runners:

```bash
gabriel@rossak:~$ garm-cli runner ls -a
+----+-------------------+----------------+---------------+--------------------------------------+
| NR | NAME              | STATUS         | RUNNER STATUS | POOL ID                              |
+----+-------------------+----------------+---------------+--------------------------------------+
|  1 | garm-tdtD6zpsXhj1 | pending_create | pending       | 344e4a72-2035-4a18-a3d5-87bd3874b56c |
+----+-------------------+----------------+---------------+--------------------------------------+
```

If we check our LXD, we should also see it there as well:

```bash
gabriel@rossak:~$ lxc list
+-------------------+---------+---------------------+------+-----------+-----------+
|       NAME        |  STATE  |        IPV4         | IPV6 |   TYPE    | SNAPSHOTS |
+-------------------+---------+---------------------+------+-----------+-----------+
| garm-tdtD6zpsXhj1 | RUNNING | 10.44.30.155 (eth0) |      | CONTAINER | 0         |
+-------------------+---------+---------------------+------+-----------+-----------+
```

If we wait for a bit and run:

```bash
gabriel@rossak:~$ garm-cli runner show garm-tdtD6zpsXhj1
+-----------------+------------------------------------------------------------------------------------------------------+
| FIELD           | VALUE                                                                                                |
+-----------------+------------------------------------------------------------------------------------------------------+
| ID              | 7ac024c9-1854-4911-9859-d061059244a6                                                                 |
| Provider ID     | garm-tdtD6zpsXhj1                                                                                    |
| Name            | garm-tdtD6zpsXhj1                                                                                    |
| OS Type         | linux                                                                                                |
| OS Architecture | amd64                                                                                                |
| OS Name         | ubuntu                                                                                               |
| OS Version      | jammy                                                                                                |
| Status          | running                                                                                              |
| Runner Status   | idle                                                                                                 |
| Pool ID         | 344e4a72-2035-4a18-a3d5-87bd3874b56c                                                                 |
| Addresses       | 10.44.30.155                                                                                         |
| Status Updates  | 2023-07-18T14:32:26: runner registration token was retrieved                                         |
|                 | 2023-07-18T14:32:26: downloading tools from https://github.com/actions/runner/releases/download/v2.3 |
|                 | 06.0/actions-runner-linux-amd64-2.306.0.tar.gz                                                       |
|                 | 2023-07-18T14:32:30: extracting runner                                                               |
|                 | 2023-07-18T14:32:36: installing dependencies                                                         |
|                 | 2023-07-18T14:33:03: configuring runner                                                              |
|                 | 2023-07-18T14:33:14: runner successfully configured after 1 attempt(s)                               |
|                 | 2023-07-18T14:33:14: installing runner service                                                       |
|                 | 2023-07-18T14:33:15: starting service                                                                |
|                 | 2023-07-18T14:33:16: runner successfully installed                                                   |
+-----------------+------------------------------------------------------------------------------------------------------+
```

We can see the runner getting installed and phoning home with status updates. You should now see it in your GitHub repo under `Settings --> Actions --> Runners`.

You can also target this runner using one or more of its labels. In this case, we can target it using `ubuntu` or `generic`.

You can also view jobs sent to your garm instance using the `garm-cli job ls` command:

```bash
gabriel@rossak:~$ garm-cli job ls
+----+------+--------+------------+-------------+------------+------------------+-----------+
| ID | NAME | STATUS | CONCLUSION | RUNNER NAME | REPOSITORY | REQUESTED LABELS | LOCKED BY |
+----+------+--------+------------+-------------+------------+------------------+-----------+
+----+------+--------+------------+-------------+------------+------------------+-----------+
```

There are no jobs sent yet to my GARM install, but once you start sending jobs, you'll see them here as well.

That's it! You now have a working GARM installation. You can add more repos, orgs or enterprises and create more pools. You can also add more providers for different clouds and credentials with access to different GitHub resources.
