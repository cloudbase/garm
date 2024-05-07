# Quick start

Okay, I lied. It's not that quick. But it's not that long either. I promise.

In this guide I'm going to take you through the entire process of setting up garm from scratch. This will include editing the config file (which will probably take the longest amount of time), fetching a proper [PAT](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token) (personal access token) from GitHub, setting up the webhooks endpoint, defining your repo/org/enterprise and finally setting up a runner pool.

For the sake of this guide, we'll assume you have access to the following setup:

* A linux machine (ARM64 or AMD64)
* Optionally, docker/podman installed on that machine
* A public IP address or port forwarding set up on your router for port `80` or `443`. You can forward any ports, we just need to remember to use the same ports when we define the webhook in github, and the two URLs in the config file (more on that later). For the sake of this guide, I will assume you have port `80` or `443` forwarded to your machine.
* An `A` record pointing to your public IP address (optional, but recommended). Alternatively, you can use the IP address directly. I will use `garm.example.com` in this guide. If you'll be using an IP address, just replace `garm.example.com` with your IP address throughout this guide.
* All config files and data will be stored in `/etc/garm`.
* A [Personal Access Token (PAT)](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token)

Why the need to expose GARM to the internet? Well, GARM uses webhooks sent by GitHub to automatically scale runners. Whenever a new job starts, a webhook is generated letting GARM know that there is a need for a runner. GARM then spins up a new runner instance and registers it with GitHub. When the job is done, the runner instance is automatically removed. This workflow is enabled by webhooks.

## The GitHub PAT (Personal Access Token)

Let's start by fetching a PAT so we get that out of the way. You can use the [GitHub docs](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token) to create a PAT.

For a `classic` PAT, GARM needs the following permissions to function properly (depending on the hierarchy level you want to manage):

* ```public_repo``` - for access to a repository
* ```repo``` - for access to a private repository
* ```admin:org``` - if you plan on using this with an organization to which you have access
* ```manage_runners:enterprise``` - if you plan to use garm at the enterprise level

This doc will be updated at a future date with the exact permissions needed in case you want to use a fine grained PAT.

## Create the config folder

All of our config files and data will be stored in `/etc/garm`. Let's create that folder:

```bash
sudo mkdir -p /etc/garm
```

Coincidentally, this is also where the docker container [looks for the config](../Dockerfile#L29) when it starts up. You can either use `Docker` or you can set up garm directly on your system. I'll show you both ways. In both cases, we need to first create the config folder and a proper config file.

## The config file

There is a full config file, with detailed comments for each option, in the [testdata folder](../testdata/config.toml). You can use that as a reference. But for the purposes of this guide, we'll be using a minimal config file and add things on as we proceed.

Open `/etc/garm/config.toml` in your favorite editor and paste the following:

```toml
[default]
callback_url = "https://garm.example.com/api/v1/callbacks"
metadata_url = "https://garm.example.com/api/v1/metadata"
webhook_url = "https://garm.example.com/webhooks"
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

This is a minimal config, with no providers or credentials defined. In this example we have the [default](./config_default.md), [logging](./config_logging.md), [metrics](./config_metrics.md), [jwt_auth](./config_jwt_auth.md), [apiserver](./config_api_server.md) and [database](./database.md) sections. Each are documented separately. Feel free to read through the available docs if, for example you need to enable TLS without using an nginx reverse proxy or if you want to enable the debug server, the log streamer or a log file.

In this sample config we:

* define the callback, webhooks and the metadata URLs
* set up logging prefrences
* enable metrics with authentication
* set a JWT secret which is used to sign JWT tokens
* set a time to live for the JWT tokens
* enable the API server on port `80` and bind it to all interfaces
* set the database backend to `sqlite3` and set a passphrase for sealing secrets (just webhook secrets for now)

The callback URLs are really important and need to point back to garm. You will notice that the domain name used in these options, is the same one we defined at the beginning of this guide. If you won't use a domain name, replace `garm.example.com` with your IP address and port number.

We need to tell garm by which addresses it can be reached. There are many ways by which GARMs API endpoints can be exposed, and there is no sane way in which GARM itself can determine if it's behind a reverse proxy or not. The metadata URL may be served by a reverse proxy with a completely different domain name than the callback URL. Both domains pointing to the same installation of GARM in the end.

The information in these two options is used by the instances we spin up to phone home their status and to fetch the needed metadata to finish setting themselves up. For now, the metadata URL is only used to fetch the runner registration token.

The webhook URL is used by GARM itself to know how to set up the webhooks in GitHub. Each controller will have a unique ID and GARM will use the value in `webhook_url` as a base. It will append the controller ID to it and set up the webhook in GitHub. This way we won't overlap with other controllers that may use the same base URL.

We won't go too much into detail about each of the options here. Have a look at the different config sections and their respective docs for more information.

At this point, we have a valid config file, but we still need to add `provider` and `credentials` sections.

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

All currently available providers are `external`.

The easiest provider to set up is probably the LXD or Incus provider. Incus is a fork of LXD so the functionality is identical (for now). For the purpose of this document, we'll continue with LXD. You don't need an account on an external cloud. You can just use your machine.

You will need to have LXD installed and configured. There is an excellent [getting started guide](https://documentation.ubuntu.com/lxd/en/latest/getting_started/) for LXD. Follow the instructions there to install and configure LXD, then come back here.

Once you have LXD installed and configured, you can add the provider section to your config file. If you're connecting to the `local` LXD installation, the [config snippet for the LXD provider](https://github.com/cloudbase/garm-provider-lxd/blob/main/testdata/garm-provider-lxd.toml) will work out of the box. We'll be connecting using the unix socket so no further configuration will be needed.

Go ahead and create a new config somwhere where GARM can access it and paste that entire snippet. For the purposes of this doc, we'll assume you created a new file called `/etc/garm/garm-provider-lxd.toml`. Now we need to define the external provider config in `/etc/garm/config.toml`:

```toml
[[provider]]
  name = "lxd_local"
  provider_type = "external"
  description = "Local LXD installation"
  [provider.external]
    provider_executable = "/opt/garm/providers.d/garm-provider-lxd"
    config_file = "/etc/garm/garm-provider-lxd.toml"
```

## The credentials section

The credentials section is where we define out GitHub credentials. GARM is capable of using either GitHub proper or [GitHub Enterprise Server](https://docs.github.com/en/enterprise-server@3.6/get-started/onboarding/getting-started-with-github-enterprise-server). The credentials section allows you to override the default GitHub API endpoint and point it to your own deployment of GHES.

The credentials section is [documented in a separate doc](./github_credentials.md), but we will include a small snippet here for clarity.
wget -q -O - https://github.com/cloudbase/garm/releases/download/v0.1.4/garm-linux-amd64.tgz |  tar xzf - -C /usr/local/bin/
```toml
# This is a list of credentials that you can define as part of the repository
# or organization definitions. They are not saved inside the database, as there
# is no Vault integration (yet). This will change in the future.
# Credentials defined here can be listed using the API. Obviously, only the name
# and descriptions are returned.
[[github]]
  name = "gabriel"
  description = "github token for user gabriel"
  # This is a personal token with access to the repositories and organizations
  # you plan on adding to garm. The "workflow" option needs to be selected in order
  # to work with repositories, and the admin:org needs to be set if you plan on
  # adding an organization.
  oauth2_token = "super secret token"
```

The `oauth2_token` option will hold the PAT we created earlier. You can add multiple credentials to the config file. Each will be referenced by name when we define the repo/org/enterprise.

Alright, we're almost there. We have a config file with a provider and a credentials section. We now have to start the service and create a webhook in GitHub pointing at our `webhook` endpoint.

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
wget -q -O - https://github.com/cloudbase/garm/releases/download/v0.1.4/garm-linux-amd64.tgz |  tar xzf - -C /usr/local/bin/
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
  https://raw.githubusercontent.com/cloudbase/garm/v0.1.4/contrib/garm.service
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
ubuntu@garm:~$ curl http://garm.example.com/webhooks
ubuntu@garm:~$ docker logs garm
signal.NotifyContext(context.Background, [interrupt terminated])
2023/07/17 22:21:33 Loading provider lxd_local
2023/07/17 22:21:33 registering prometheus metrics collectors
2023/07/17 22:21:33 setting up metric routes
2023/07/17 22:21:35 ignoring unknown event
172.17.0.1 - - [17/Jul/2023:22:21:35 +0000] "GET /webhooks HTTP/1.1" 200 0 "" "curl/7.81.0"
```

Excellent! We have a working GARM installation. Now we need to set up the webhook in GitHub.

## Setting up the webhook

Before we create a pool, we need to set up the webhook in GitHub. This is a fairly simple process.

Head over to the [webhooks doc](./webhooks.md) and follow the instructions there. Come back here when you're done.

After you've finished setting up the webhook, there are just a few more things to do:

* Initialize GARM
* Add a repo/org/enterprise
* Create a pool

## Initializing GARM

Before we can start using GARM, we need initialize it. This will create the `admin` user and generate a unique controller ID that will identify this GARM installation. This process allows us to use multiple GARM installations with the same GitHub account. GARM will use the controller ID to identify the runners it creates. This way we won't run the risk of accidentally removing runners we don't manage.

To initialize GARM, we'll use the `garm-cli` tool. You can download the latest release from the [releases page](https://github.com/cloudbase/garm/releases):

```bash
wget -q -O - https://github.com/cloudbase/garm/releases/download/v0.1.3/garm-cli-linux-amd64.tgz |  tar xzf - -C /usr/local/bin/
```

Now we can initialize GARM:

```bash
ubuntu@garm:~$ garm-cli init --name="local_garm" --url https://garm.example.com
Username: admin
Email: root@localhost
✔ Password: *************
+----------+--------------------------------------+
| FIELD    | VALUE                                |
+----------+--------------------------------------+
| ID       | ef4ab6fd-1252-4d5a-ba5a-8e8bd01610ae |
| Username | admin                                |
| Email    | root@localhost                       |
| Enabled  | true                                 |
+----------+--------------------------------------+
```

The init command also created a local CLI profile for your new GARM server:

```bash
ubuntu@garm:~$ garm-cli profile list
+----------------------+--------------------------+
| NAME                 | BASE URL                 |
+----------------------+--------------------------+
| local_garm (current) | https://garm.example.com |
+----------------------+--------------------------+
```

Every time you init a new GARM instance, a new profile will be created in your local `garm-cli` config. You can also log into an already initialized instance using:

```bash
garm-cli profile add --name="another_garm" --url https://garm2.example.com
```

Then you can switch between profiles using:

```bash
garm-cli profile switch another_garm
```

## Define a repo

We now have a working GARM installation, with github credentials and a provider added. It's time to add a repo.

Before we add a repo, let's list credentials. We'll need their names when we'll add a new repo.

```bash
gabriel@rossak:~$ garm-cli credentials list
+---------+-------------------------------+--------------------+-------------------------+-----------------------------+
| NAME    | DESCRIPTION                   | BASE URL           | API URL                 | UPLOAD URL                  |
+---------+-------------------------------+--------------------+-------------------------+-----------------------------+
| gabriel | github token for user gabriel | https://github.com | https://api.github.com/ | https://uploads.github.com/ |
+---------+-------------------------------+--------------------+-------------------------+-----------------------------+
```

Even though you didn't explicitly set the URLs, GARM will default to the GitHub ones. You can override them if you want to use a GHES deployment.

Now we can add a repo:

```bash
garm-cli repo add \
  --credentials gabriel \
  --owner gsamfira \
  --name scripts \
  --webhook-secret $SECRET
```

In this case, `$SECRET` holds the webhook secret you set previously when you defined the webhook in GitHub. This secret is mandatory as GARM will always validate the webhook payloads it receives.

You should see something like this:

```bash
gabriel@rossak:~$ garm-cli repo add \
>   --credentials gabriel \
>   --owner gsamfira \
>   --name scripts \
>   --webhook-secret $SECRET
+----------------------+--------------------------------------+
| FIELD                | VALUE                                |
+----------------------+--------------------------------------+
| ID                   | f4900c7c-2ec0-41bd-9eab-d70fe9bd850d |
| Owner                | gsamfira                             |
| Name                 | scripts                              |
| Credentials          | gabriel                              |
| Pool manager running | false                                |
| Failure reason       |                                      |
+----------------------+--------------------------------------+
```

We can now list the repos:

```bash
gabriel@rock:~$ garm-cli repo ls
+--------------------------------------+----------+---------+------------------+------------------+
| ID                                   | OWNER    | NAME    | CREDENTIALS NAME | POOL MGR RUNNING |
+--------------------------------------+----------+---------+------------------+------------------+
| f4900c7c-2ec0-41bd-9eab-d70fe9bd850d | gsamfira | scripts | gabriel          | true             |
+--------------------------------------+----------+---------+------------------+------------------+
```

Excellent! Make a note of the ID. We'll need it later when we create a pool.

## Create a pool

This is the last step. You're almost there!

To create a pool we'll need the repo ID from the previous step (which we have) and a provider in which the pool will spin up new runners. We'll use the LXD provider we defined earlier, but we need its name:

```bash
gabriel@rossak:~$ garm-cli provider list
+-----------+------------------------+------+
| NAME      | DESCRIPTION            | TYPE |
+-----------+------------------------+------+
| lxd_local | Local LXD installation | lxd  |
+-----------+------------------------+------+
```

Now we can create a pool:

```bash
garm-cli pool add \
  --repo f4900c7c-2ec0-41bd-9eab-d70fe9bd850d \
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
>   --repo f4900c7c-2ec0-41bd-9eab-d70fe9bd850d \
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
| Image                    | ubuntu:22.04                               |
| Flavor                   | default                                    |
| OS Type                  | linux                                      |
| OS Architecture          | amd64                                      |
| Max Runners              | 5                                          |
| Min Idle Runners         | 0                                          |
| Runner Bootstrap Timeout | 20                                         |
| Tags                     | self-hosted, amd64, Linux, ubuntu, generic |
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
+--------------------------------------+--------------+---------+----------------------------------------+------------------+-------+---------+---------------+
| ID                                   | IMAGE        | FLAVOR  | TAGS                                   | BELONGS TO       | LEVEL | ENABLED | RUNNER PREFIX |
+--------------------------------------+--------------+---------+----------------------------------------+------------------+-------+---------+---------------+
| 344e4a72-2035-4a18-a3d5-87bd3874b56c | ubuntu:22.04 | default | self-hosted amd64 Linux ubuntu generic | gsamfira/scripts | repo  | true    | garm          |
+--------------------------------------+--------------+---------+----------------------------------------+------------------+-------+---------+---------------+
```

This pool is enabled, but the `min-idle-runners` option is set to 0. This means that it will not create any lingering runners. It will only create runners when a job is started. If your provider is slow to boot up new instances, you may want to set this to a value higher than 0.

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
| Image                    | ubuntu:22.04                               |
| Flavor                   | default                                    |
| OS Type                  | linux                                      |
| OS Architecture          | amd64                                      |
| Max Runners              | 5                                          |
| Min Idle Runners         | 1                                          |
| Runner Bootstrap Timeout | 20                                         |
| Tags                     | self-hosted, amd64, Linux, ubuntu, generic |
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
gabriel@rossak:~$ garm-cli  runner show garm-tdtD6zpsXhj1
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
