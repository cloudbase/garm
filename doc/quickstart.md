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

* All of our config files and data will be stored in `/etc/garm`. Let's create that folder:

  ```bash
  sudo mkdir -p /etc/garm
  ```

Coincidentally, this is also where the docker container [looks for the config](../Dockerfile#L29) when it starts up. You can either use `Docker` or you can set up garm directly on your system. I'll show you both ways. In both cases, we need to first create the config folder and a proper config file.

## The config file

There is a full config file, with detailed comments for each option, in the [testdata folder](../testdata/config.toml). You can use that as a reference. But for the purposes of this guide, we'll be using a minimal config file and add things on as we proceed.

* Open `/etc/garm/config.toml` in your favorite editor and paste the following:

  ```toml
  [default]
  callback_url = "https://garm.example.com/api/v1/callbacks/status"
  metadata_url = "https://garm.example.com/api/v1/metadata"

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

This is a minimal config, with no providers or credentials defined. In this example we have the [default](./config_default.md), [metrics](./config_metrics.md), [jwt_auth](./config_jwt_auth.md), [apiserver](./config_api_server.md) and [database](./config_database.md) sections. Each are documented separately. Feel free to read through the available docs if, for example you need to enable TLS without using an nginx reverse proxy or if you want to enable the debug server, the log streamer or a log file.

In this sample config we:

* define the callback and the metadata URLs
* enable metrics with authentication
* set a JWT secret which is used to sign JWT tokens
* set a time to live for the JWT tokens
* enable the API server on port `80` and bind it to all interfaces
* set the database backend to `sqlite3` and set a passphrase for the database

The callback URLs are really important and need to point back to garm. You will notice that the domain name used in these options, is the same one we defined at the beginning of this guide. If you won't use a domain name, replace `garm.example.com` with your IP address and port number.

We need to tell garm by which addresses it can be reached. There are many ways by which GARMs API endpoints can be exposed, and there is no sane way in which GARM itself can determine if it's behind a reverse proxy or not. The metadata URL may be served by a reverse proxy with a completely different domain name than the callback URL. Both domains pointing to the same installation of GARM in the end.

The information in these two options is used by the instances we spin up to phone home their status and to fetch the needed metadata to finish setting themselves up. For now, the metadata URL is only used to fetch the runner registration token.

We won't go too much into detail about each of the options here. Have a look at the different config sections and their respective docs for more information.

At this point, we have a valid config file, but we still need to add `provider` and `credentials` sections.

## The provider section

This is where you have a decision to make. GARM has a number of providers you can leverage. At the time of this writing, we have support for:

* LXD
* Azure
* OpenStack

The LXD provider is built into GARM itself and has no external requirements. The [Azure](https://github.com/cloudbase/garm-provider-azure) and [OpenStack](https://github.com/cloudbase/garm-provider-openstack) ones are `external` providers in the form of an executable that GARM calls into.

Both the LXD and the external provider configs are [documented in a separate doc](./providers.md).

The easiest provider to set up is probably the LXD provider. You don't need an account on an external cloud. You can just use your machine.

You will need to have LXD installed and configured. There is an excellent [getting started guide](https://documentation.ubuntu.com/lxd/en/latest/getting_started/) for LXD. Follow the instructions there to install and configure LXD, then come back here.

Once you have LXD installed and configured, you can add the provider section to your config file. If you're connecting to the `local` LXD installation, the [config snippet for the LXD provider](./providers.md#lxd-provider) will work out of the box. We'll be connecting using the unix socket so no further configuration will be needed.

Go ahead and copy and paste that entire snippet in your GARM config file (`/etc/garm/config.toml`).

You can also use an external provider instead of LXD. You will need to define the provider section in your config file and point it to the executable and the provider config file. The [config snippet for the external provider](./providers.md#external-provider) gives you an example of how that can be done. Configuring the external provider is outside the scope of this guide. You will need to consult the documentation for the external provider you want to use.

## The credentials section

The credentials section is where we define out GitHub credentials. GARM is capable of using either GitHub proper or [GitHub Enterprise Server](https://docs.github.com/en/enterprise-server@3.6/get-started/onboarding/getting-started-with-github-enterprise-server). The credentials section allows you to override the default GitHub API endpoint and point it to your own deployment of GHES.

* The credentials section is [documented in a separate doc](./github_credentials.md), but we will include a small snippet here for clarity.

  ```toml
  # This is a list of credentials that you can define as part of the repository
  # or organization definitions. They are not saved inside the database, as there
  # is no Vault integration (yet). This will change in the future.
  # Credentials defined here can be listed using the API. Obviously, only the name
  # and descriptions are returned.
  [[github]]
    name = "gabriel"
    description = "github token or user gabriel"
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

* If you're using docker, you can start the service with:

  ```bash
  docker run -d \
    --name garm \
    -p 80:80 \
    -v /etc/garm:/etc/garm:rw \
    -v /var/snap/lxd/common/lxd/unix.socket:/var/snap/lxd/common/lxd/unix.socket:rw \
    ghcr.io/cloudbase/garm:v0.1.2
  ```

You will notice we also mounted the LXD unix socket from the host inside the container where the config you pasted expects to find it. If you plan to use an external provider that does not need to connect to LXD over a unix socket, feel free to remove that mount.

* Check the logs to make sure everything is working as expected:

  ```bash
  ubuntu@garm:~$ docker logs garm
  signal.NotifyContext(context.Background, [interrupt terminated])
  2023/07/17 21:55:43 Loading provider lxd_local
  2023/07/17 21:55:43 registering prometheus metrics collectors
  2023/07/17 21:55:43 setting up metric routes
  ```

### Setting up GARM as a system service

This process is a bit more involved. We'll need to create a new user for garm and set up permissions for that user to connect to LXD.

* First, create the user:

  ```bash
  useradd --shell /usr/bin/false \
        --system \
        --groups lxd \
        --no-create-home garm
  ```

Adding the `garm` user to the LXD group will allow it to connect to the LXD unix socket. We'll need that considering the config we crafted above.

* Next, download the latest release from the [releases page](https://github.com/cloudbase/garm/releases).

  ```bash
  wget -q -O - https://github.com/cloudbase/garm/releases/download/v0.1.2/garm-linux-amd64.tgz |  tar xzf - -C /usr/local/bin/
  ```

* We'll be running under an unprivileged user. If we want to be able to listen on any port under `1024`, we'll have to set some capabilities on the binary:

  ```bash
  setcap cap_net_bind_service=+ep /usr/local/bin/garm
  ```

* Change the permissions on the config dir:

  ```bash
  chown -R garm:garm /etc/garm
  ```

* Copy the sample `systemd` service file:

  ```bash
  wget -O /etc/systemd/system/garm.service \
    https://raw.githubusercontent.com/cloudbase/garm/v0.1.2/contrib/garm.service
  ```

* Reload the `systemd` daemon and start the service:

  ```bash
  systemctl daemon-reload
  systemctl start garm
  ```

* Check the logs to make sure everything is working as expected:

  ```bash
  ubuntu@garm:~$ sudo journalctl -u garm
  ```

* Check that you can make a request to the API:

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

* To initialize GARM, we'll use the `garm-cli` tool. You can download the latest release from the [releases page](https://github.com/cloudbase/garm/releases):

  ```bash
  wget -q -O - https://github.com/cloudbase/garm/releases/download/v0.1.2/garm-cli-linux-amd64.tgz |  tar xzf - -C /usr/local/bin/
  ```

* Now we can initialize GARM:

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

* The init command also created a local CLI profile for your new GARM server:

  ```bash
  ubuntu@garm:~# garm-cli profile list
  +----------------------+--------------------------+
  | NAME                 | BASE URL                 |
  +----------------------+--------------------------+
  | local_garm (current) | https://garm.example.com |
  +----------------------+--------------------------+
  ```

## Define a repo

## Create a pool

This is the last step.
