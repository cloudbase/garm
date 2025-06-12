# Using GARM with Gitea

Starting with Gitea 1.24 and the latest version of GARM (upcomming v0.2.0 - currently `main`), GARM supports Gitea as a forge, side by side with GitHub/GHES. A new endpoint type has been added to represent Gitea instances, which you can configure and use along side your GitHub runners.

You can essentially create runners for both GitHub and Gitea using the same GARM instance, using the same CLI and the same API. It's simply a matter of adding an endpoint and credentials. The rest is the same as for github.

## Quickstart

This is for testing purposes only. We'll assume you're running on an Ubuntu 24.04 VM or server. You can use anything you'd like, but this quickstart is tailored to get you up and running with the LXD provider. So we'll:

* Initialize LXD
* Create a docker compose yaml
* Deploy Gitea and GARM
* Configure GARM to use Gitea

You will have to install Docker-CE yourself.

### Initialize LXD

If you already have LXD initialized, you can skip this step. Otherwise, simply run:

```bash
sudo lxd init --auto
```

This should set up LXD with default settings that should work on any system.

LXD and Docker sometimes have issues with networking due to some conflicting iptables rules. In most cases, if you have docker installed and notice that you don't have access to the outside world from the containers, run the following command:

```bash
sudo iptables -I DOCKER-USER -j ACCEPT
```

### Create the docker compose

Create a docker compose file in `$HOME/compose.yaml`. This docker compose will deploy both gitea and GARM. If you already have a Gitea >=1.24.0, you can edit this docker compose to only deploy GARM. 

```yaml
networks:
  default:
    external: false

services:
  gitea:
    image: docker.gitea.com/gitea:1.24.0-rc0
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
    restart: always
    networks:
      - default
    volumes:
      - /etc/gitea/gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "80:80"
      - "22:22"
  garm:
    image: ghcr.io/cloudbase/garm:${GARM_VERSION:-nightly}
    container_name: garm
    environment:
      - USER_UID=1000
      - USER_GID=1000
    restart: always
    networks:
      - default
    volumes:
      - /etc/garm:/etc/garm
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
      # Give GARM access to the LXD socket. We need this later in the LXD provider.
      - /var/snap/lxd/common/lxd/unix.socket:/var/snap/lxd/common/lxd/unix.socket
    ports:
      - "9997:9997"
```

Create the folders for Gitea and GARM:

```bash
sudo mkdir -p /etc/gitea /etc/garm
sudo chown 1000:1000 /etc/gitea /etc/garm
```

Create the GARM configuration file:

```bash

sudo tee /etc/garm/config.toml <<EOF
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
secret = "&n~@^QV;&fiTTljy#pWq0>_YE+$%d+O;BMDqnaB)`U4_*iF8snEpEszPyg4N*lI&"
time_to_live = "8760h"

[apiserver]
  bind = "0.0.0.0"
  port = 9997
  use_tls = false

[database]
  backend = "sqlite3"
  # This needs to be changed.
  passphrase = "OsawnUlubmuHontamedOdVurwetEymni"
  [database.sqlite3]
    db_file = "/etc/garm/garm.db"

# This enables the LXD provider. There are other providers available in the image
# in /opt/garm/providers.d. Feel free to use them as well.
[[provider]]
  name = "lxd_local"
  provider_type = "external"
  description = "Local LXD installation"
  [provider.external]
    provider_executable = "/opt/garm/providers.d/garm-provider-lxd"
    config_file = "/etc/garm/garm-provider-lxd.toml"
EOF
```

Create the LXD provider config file:

```bash
sudo tee /etc/garm/garm-provider-lxd.toml <<EOF
# the path to the unix socket that LXD is listening on. This works if garm and LXD
# are on the same system, and this option takes precedence over the "url" option,
# which connects over the network.
unix_socket_path = "/var/snap/lxd/common/lxd/unix.socket"
# When defining a pool for a repository or an organization, you have an option to
# specify a "flavor". In LXD terms, this translates to "profiles". Profiles allow
# you to customize your instances (memory, cpu, disks, nics, etc).
# This option allows you to inject the "default" profile along with the profile selected
# by the flavor.
include_default_profile = false
# instance_type defines the type of instances this provider will create.
#
# Options are:
#
#   * virtual-machine (default)
#   * container
#
instance_type = "container"
# enable/disable secure boot. If the image you select for the pool does not have a
# signed bootloader, set this to false, otherwise your instances won't boot.
secure_boot = false
# Project name to use. You can create a separate project in LXD for runners.
project_name = "default"
# URL is the address on which LXD listens for connections (ex: https://example.com:8443)
url = ""
# garm supports certificate authentication for LXD remote connections. The easiest way
# to get the needed certificates, is to install the lxc client and add a remote. The
# client_certificate, client_key and tls_server_certificate can be then fetched from
# $HOME/snap/lxd/common/config.
client_certificate = ""
client_key = ""
tls_server_certificate = ""
[image_remotes]
    # Image remotes are important. These are the default remotes used by lxc. The names
    # of these remotes are important. When specifying an "image" for the pool, that image
    # can be a hash of an existing image on your local LXD installation or it can be a
    # remote image from one of these remotes. You can specify the images as follows:
    # Example:
    #
    #    * ubuntu:20.04
    #    * ubuntu_daily:20.04
    #    * images:centos/8/cloud
    #
    # Ubuntu images come pre-installed with cloud-init which we use to set up the runner
    # automatically and customize the runner. For non Ubuntu images, you need to use the
    # variant that has "/cloud" in the name. Those images come with cloud-init.
    [image_remotes.ubuntu]
    addr = "https://cloud-images.ubuntu.com/releases"
    public = true
    protocol = "simplestreams"
    skip_verify = false
    [image_remotes.ubuntu_daily]
    addr = "https://cloud-images.ubuntu.com/daily"
    public = true
    protocol = "simplestreams"
    skip_verify = false
    [image_remotes.images]
    addr = "https://images.lxd.canonical.com"
    public = true
    protocol = "simplestreams"
    skip_verify = false
EOF
```

Start the containers:

```bash
docker compose -f $HOME/compose.yaml up --force-recreate --build
```

Create a gitea user:

```bash
docker exec -u=1000 gitea \
    gitea admin user create \
    --username testing \
    --password superSecretPasswordThatYouMustAbsolutelyChange \
    --email admin@example.com \
    --admin \
    --must-change-password=false
```

Feel free to log into Gitea and create an org or repo. We'll need one to configure in GARM later to spin up runners. For the purpose of this document, I'll assume you careated an org called `testorg` and a repo inside that org called `testrepo`.

### Initialize GARM

Before GARM can be used, you need to [create the admin user](/doc/quickstart.md#initializing-garm). If you deployed GARM using docker, you can copy the client from the image:

```bash
sudo docker cp garm:/bin/garm-cli /usr/local/bin/garm-cli
```

Make sure it's executable:

```bash
chmod +x /usr/local/bin/garm-cli
```

Now you can follow [the rest of the steps from the quickstart](/doc/quickstart.md#initializing-garm). Given that this is gitea and everything is local, instead of `http://garm.example.com` you can use one of the IP addresses available on your system to initialize GARM and possibly [set the controller URLs](/doc/using_garm.md#controller-operations). For the purpose of this example, I'll assume that your local IP address is: `10.0.9.5`, and both GARM (port 9997) and Gitea (port 80) are accessible on that IP address.

## Adding a Gitea endpoint

We now have GARM and Gitea installed. Time to add a Gitea endpoint to GARM.

```bash
garm-cli gitea endpoint create \
    --api-base-url http://10.0.9.5/ \
    --base-url http://10.0.9.5/ \
    --description "My first Gitea endpoint" \
    --name local-gitea
```

An `endpoint` tells GARM how it can access a particular forge (github.com, a self hosted GHES or Gitea). Credentials are tied to endpoints and in turn, entities (repos, orgs, enterprises) are tied to credentials.

## Adding a Gitea credential

GARM needs to be able to fetch runner registration tokens, delete runners and set webhooks. We need to create a token that allows these operations on orgs and repos:

```bash
LOGIN=$(curl -s -X POST http://localhost/api/v1/users/testing/tokens \
  -u 'testing:superSecretPasswordThatYouMustAbsolutelyChange' \
  -H "Content-Type: application/json" \
  -d '{"name": "autotoken", "scopes": ["write:repository", "write:organization"]}')
```

If you echo the resulting token, you should see something like:

```bash
ubuntu@gitea-garm:~$ echo $LOGIN
{"id":2,"name":"autotoken","sha1":"34b01d497501c40bde7d4a6052d883b86387ed45","token_last_eight":"6387ed45","scopes":["write:organization","write:repository"]}
```

Now we can create a credential in GARM that uses this token:

```bash
TOKEN=$(echo $LOGIN | jq -r '.sha1')
garm-cli gitea credentials add \
    --endpoint local-gitea \
    --auth-type pat \
    --pat-oauth-token $TOKEN \
    --name autotoken \
    --description "Gitea token"
```

## Adding a repository

If you've created a new repository called `testorg/testrepo` in your gitea for the user `testing`, you can add it to GARM as follows:

```bash
garm-cli repo add \
    --credentials autotoken \
    --name testrepo \
    --owner testorg \
    --random-webhook-secret \
    --install-webhook
```

Make a note of the repo UUID. You will need it when adding a pool.

This will add the repo to GARM as an entity, and automatically install a webhook that will send `workflow_job` webhooks to GARM. GARM uses these webhooks to know when a runner needs to be spun up or when a runner must be removed from the provider (LXD in this case), because it has finished running the job.

The URL that is used by GARM to configure the webhook in gitea is dictated by the `Controller Webhook URL` field in:

```bash
garm-cli controller show
```

You can manually update the controller urls using `garm-cli controller update`, but unless you're using a reverse proxy in front of GARM or want to make it accessible from the internet, you won't need to do this. Consult the [using garm](/doc/using_garm.md) and the [quickstart](/doc/quickstart.md) documents for more information on how to set up GARM and the controller.

## Adding a pool to your Gitea repo

Pools will maintain your runners. Each pool is tied to a provider. You can define as many pools as you want, each using a different provider.

We have the LXD provider configured, so we can use that. You can get a list of configured providers by doing:

```bash
garm-cli provider list
```

The one we configured is called `local_lxd`.

Before we add a pool, there is one more important detail to know. Right now, the default runner installation script is tailored for github. But most providers offer a way to override the runner install template via an opaque JSON field called `extra_specs`. We can set this field when creating the pool or update it any time later.

There is a sample extra specs file [in this github gist](https://gist.github.com/gabriel-samfira/d132169ec41d990bbe17e6097af94c4c). We can fetch it and use it when definig the pool:

```bash
curl -L -o $HOME/gitea-pool-specs.json https://gist.githubusercontent.com/gabriel-samfira/d132169ec41d990bbe17e6097af94c4c/raw/67d226d9115eca5e10b69eac5ecb04be91a48991/gitea-extra-specs.json
```

Now, add the pool:

```bash
garm-cli pool add \
    --repo  theUUIDOfTheRepoAddedAbove \
    --provider-name local_lxd \
    --image ubuntu:24.04 \
    --tags ubuntu-latest \
    --flavor default \
    --enabled=true \
    --min-idle-runners=1 \
    --max-runners=5 \
    --extra-specs-file=$HOME/gitea-pool-specs.json
```

You should now see 1 runner being spun up in LXD. You can check the status of the pool by doing:

```bash
garm-cli runner ls -a
```

To get more details about the runner, run:

```bash
garm-cli runner show RUNNER_NAME_GOES_HERE
```

That's it! You can now use GARM with Gitea. You can add more pools, more repos, more orgs, more endpoints and more providers. 
