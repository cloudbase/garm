#  GitHub Actions Runners Manager (garm)

Welcome to garm!

Garm enables you to create and automatically maintain pools of [self-hosted GitHub runners](https://docs.github.com/en/actions/hosting-your-own-runners/about-self-hosted-runners), with autoscaling that can be used inside your github workflow runs. 

## Installing

To install garm, simply run:

```bash
git clone https://github.com/cloudbase/garm
cd garm
go install ./...
```

You should now have both ```garm``` and ```garm-cli``` in your ```$GOPATH/bin``` folder.

## Configuring

The ```garm``` configuration is a simple ```toml```. A sample of the config file can be found in [the testdata folder](/testdata/config.toml).

There are two URLs that you need to give special consideration to, besides the ones used by the CLI to interact with the system. Those tow API endpoints are:

  * Webhook endpoint

```
POST /webhooks
```

  * Instance callback URL

 ```
POST /api/v1/callbacks/status
```


### Webhooks endpoint

API endpoint:

```
POST /webhooks
```

This API endpoint must be added to your github repository or organization, and must be publicly accessible. There is no authentication on this URL. Validation of the workflow POST body is done, if a secret is configured (highly recommended) when defining the repository or organization in ```garm```. Optionally, you can place a reverse proxy in front of it, and configure [basic auth](https://docs.nginx.com/nginx/admin-guide/security-controls/configuring-http-basic-authentication/).


### The callback_url option

If you want your runners to be able to call back home and update their status as they install, you will need to configure the ```callback_url``` option in the ```garm``` server config. This URL needs to point to the following API endpoint:

```
POST /api/v1/callbacks/status
```

While not critical, this allows instances to call back home, set their own status as installation procedes and send back messages which can be viewed by running:

```bash
garm-cli runner show <runner_name>
```

For example:

```bash
garm-cli runner show garm-f5227755-129d-4e2d-b306-377a8f3a5dfe
+-----------------+--------------------------------------------------------------------------------------------------------------------------------------------------+
| FIELD           | VALUE                                                                                                                                            |
+-----------------+--------------------------------------------------------------------------------------------------------------------------------------------------+
| ID              | 1afb407b-e9f7-4d75-a410-fc4a8c2dbe6c                                                                                                             |
| Provider ID     | garm-f5227755-129d-4e2d-b306-377a8f3a5dfe                                                                                                        |
| Name            | garm-f5227755-129d-4e2d-b306-377a8f3a5dfe                                                                                                        |
| OS Type         | linux                                                                                                                                            |
| OS Architecture | amd64                                                                                                                                            |
| OS Name         | ubuntu                                                                                                                                           |
| OS Version      | focal                                                                                                                                            |
| Status          | running                                                                                                                                          |
| Runner Status   | idle                                                                                                                                             |
| Pool ID         | 98f438b9-5549-4eaf-9bb7-1781533a455d                                                                                                             |
| Status Updates  | 2022-05-05T11:32:41: downloading tools from https://github.com/actions/runner/releases/download/v2.290.1/actions-runner-linux-x64-2.290.1.tar.gz |
|                 | 2022-05-05T11:32:43: extracting runner                                                                                                           |
|                 | 2022-05-05T11:32:47: installing dependencies                                                                                                     |
|                 | 2022-05-05T11:32:55: configuring runner                                                                                                          |
|                 | 2022-05-05T11:32:59: installing runner service                                                                                                   |
|                 | 2022-05-05T11:33:00: starting service                                                                                                            |
|                 | 2022-05-05T11:33:00: runner successfully installed                                                                                               |
+-----------------+--------------------------------------------------------------------------------------------------------------------------------------------------+
```

This URL if set, must be accessible by the instance. If you wish to restrict access to it, a reverse proxy can be configured to accept requests only from networks in which the runners ```garm``` manages will be spun up. This URL doesn't need to be globally accessible, it just needs to be accessible by the instances.

For example, in a scenario where you expose the API endpoint directly, this setting could look like the following:

```toml
callback_url = "https://garm.example.com/api/v1/callbacks/status"
```

Authentication is done using a short-lived (15 minutes) JWT token, that gets generated for a particular instance that we are spinning up. That JWT token only has access to update it's own status. No other API endpoints will work with that JWT token.

There is a sample ```nginx``` config [in the testdata folder](/testdata/nginx-server.conf). Feel free to customize it whichever way you see fit.

### Configuring GitHub webhooks

Garm is designed to auto-scale github runners based on a few simple rules:

  * A minimum idle runner count can be set for a pool. Garm will attempt to maintain that minimum of idle runners, ready to be used by your workflows.
  * A maximum number of runners for a pool. This is a hard limit of runners a pool will create, regardless of minimum idle runners.
  * When a runner is scheduled by github, ```garm``` will automatically spin up a new runner to replace it, obeying the maximum hard limit defined.

To achieve this, ```garm``` leverages [GitHub webhooks](https://docs.github.com/en/developers/webhooks-and-events/webhooks/about-webhooks).

In the webhook configuration page under ```Content type``` you will need to select ```application/json```, set the proper webhook URL and, really important, **make sure you configure a webhook secret**. Click on ```Let me select individual events``` and select ```Workflow jobs``` (should be at the bottom). You can send everything if you want, but any events ```garm``` doesn't care about will simply be ignored.

The webhook secret must be secure. Use something like this to generate one:

```bash
gabriel@rossak:~$ function generate_secret () { 
    tr -dc 'a-zA-Z0-9!@#$%^&*()_+?><~\`;' < /dev/urandom | head -c 64;
    echo ''
}

gabriel@rossak:~$ generate_secret
9Q<fVm5dtRhUIJ>*nsr*S54g0imK64(!2$Ns6C!~VsH(p)cFj+AMLug%LM!R%FOQ
```

You can use the same function to generate a proper ```JWT``` secret for the config. The database passphrase used to encrypt sensitive data before being saved in the database must be 32 characters in size.

### Configuring github credentials

Garm needs a [Personal Access Token (PAT)](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token) to create runner registration tokens, list current self hosted runners and potentially remove them if they become orphaned (the VM was manually removed on the provider).

From the list of scopes, you will need to select:

  * ```workflow``` - for access to repository level workflows
  * ```admin:org``` - if you plan on using this with an organization to which you have access

The resulting token must be configured in the ```[[github]]``` section of the config. Sample as follows:

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

The double paranthesis means that this is an array. You can specify the ```[[github]]``` section multiple times, with different tokens from different users, or with different access levels. You will then be able to list the available credentials using the API, and reference these credentials when adding repositories or organizations.

The API will only ever return the name and description to the API consumer.

### Database configuration

Garm currently supports two database backends:

  * SQLite3
  * MySQL

You can choose either one of these. For most cases, ```SQLite3``` should do, but feel free to go with MySQL if you wish.

```toml
[database]
  # Turn on/off debugging for database queries.
  debug = false
  # Database backend to use. Currently supported backends are:
  #   * sqlite3
  #   * mysql
  backend = "sqlite3"
  # the passphrase option is a temporary measure by which we encrypt the webhook
  # secret that gets saved to the database, using AES256. In the future, secrets
  # will be saved to something like Barbican or Vault, eliminating the need for
  # this.
  passphrase = "n<$n&P#L*TWqOh95_bN5J1r4mhxY7R84HZ%pvM#1vxJ<7~q%YVsCwU@Z60;7~Djo"
  [database.mysql]
    # If MySQL is used, these are the credentials and connection information used
    # to connect to the server instance.
    # database username
    username = ""
    # Database password
    password = ""
    # hostname to connect to
    hostname = ""
    # database name
    database = ""
  [database.sqlite3]
    # Path on disk to the sqlite3 database file.
    db_file = "/home/runner/file.db"
```

### Provider configuration

Garm was designed to be extensible. The database layer as well as the providers are defined as interfaces. Currently the only implementation of a provider is for [LXD](https://linuxcontainers.org/lxd/introduction/), but will be extended to include more providers in the future. LXD is the simplest cloud-like system you can easily set up on any GNU/Linux machine, which allows you to create both containers and Virtual Machines.

Garm leverages the virtual machines feature of LXD to create the runners, and the provider itself allows you to separate those machines from the rest of your LXD workloads, by using LXD projects. Here is a sample config section for an LXD provider:

```toml
# Currently, providers are defined statically in the config. This is due to the fact
# that we have not yet added support for storing secrets in something like Barbican
# or Vault. This will change in the future. However, for now, it's important to remember
# that once you create a pool using one of the providers defined here, the name of that
# provider must not be changes, or the pool will no longer work. Make sure you remove any
# pools before removing or changing a provider.
[[provider]]
  # An arbitrary string describing this provider.
  name = "lxd_local"
  # Provider type. Garm is designed to allow creating providers which are used to spin
  # up compute resources, which in turn will run the github runner software.
  # Currently, LXD is the only supprted provider, but more will be written in the future.
  provider_type = "lxd"
  # A short description of this provider. The name, description and provider types will
  # be included in the information returned by the API when listing available providers.
  description = "Local LXD installation"
  [provider.lxd]
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
    [provider.lxd.image_remotes]
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
      [provider.lxd.image_remotes.ubuntu]
        addr = "https://cloud-images.ubuntu.com/releases"
        public = true
        protocol = "simplestreams"
        skip_verify = false
      [provider.lxd.image_remotes.ubuntu_daily]
        addr = "https://cloud-images.ubuntu.com/daily"
        public = true
        protocol = "simplestreams"
        skip_verify = false
      [provider.lxd.image_remotes.images]
        addr = "https://images.linuxcontainers.org"
        public = true
        protocol = "simplestreams"
        skip_verify = false
```

You can choose to connect to a local LXD server by using the ```unix_socket_path``` option, or you can connect to a remote LXD cluster/server by using the ```url``` option. If both are specified, the unix socket takes precedence. The config file is fairly well commented, but I will add a note about remotes.

By default, garm does not load any image remotes. You get to choose which remotes you add (if any). Image remotes are a repository of images that LXD uses to create new instances, either virtual machines or containers. In the absence of any remote, garm will attempt to find the image you configure for a pool of runners, on the LXD server we're connecting to. If one is present, it will be used, otherwise it will attempt to look for it in one of the configured remotes.

The sample config file has the following remotes configured:

  * https://cloud-images.ubuntu.com/releases (ubuntu) - Official Ubuntu images
  * https://cloud-images.ubuntu.com/daily (ubuntu_daily) - Official Ubuntu images, daily build
  * https://images.linuxcontainers.org (images) - Comunity maintained images for various operating systems

When creating a new pool, you'll be able to specify which image you would like to use. To use these remotes (if you add them to your provider config), you can use the following syntax:

```
image_remote:image_tag
```

For example, if you want to launch a runner on an Ubuntu 20.04, the image name would be ```ubuntu:20.04```. For a daily image it would be ```ubuntu_daily:20.04```. And for one of the unnoficial images it would be ```images:centos/8-Stream/cloud```. Note, that for unofficial images, you need to use the tags that have ```/cloud``` in the name. These images come pre-installed with ```cloud-init``` which we leverage to set up the runners automatically.

You can also create your own image remote, where you can host your own custom images. If you want to build your own images, have a look at [distrobuilder](https://github.com/lxc/distrobuilder).

Image remotes in the ```garm``` config, is a map of strings to remote settins. The name of the remote is the last bit of string in the section header. For example, the following section ```[provider.lxd.image_remotes.ubuntu_daily]```, defines the image remote named **ubuntu_daily**. Use this name to reference images inside that remote.


## Running garm

Create a folder for the config:

```bash
mkdir $HOME/garm
```

Create a config file for ```garm```:

```bash
cp ./testdata/config.toml $HOME/garm/config.toml
```

Customize the config whichever way you want, then run ```garm```:

```bash
garm -config $HOME/garm/config.toml
```

This will start the API and migrate the database. Note, if you're using MySQL, you will need to create a database, grant access to a user and configure those credentials in the ```config.toml``` file.

## First run

Before you can use ```garm```, you need to initialize it. This means we need to create an admin user, and login:

```bash
ubuntu@experiments:~$ garm-cli init --name="local_garm" --url https://garm.example.com
Username: admin
Email: root@localhost
✔ Password: *************█
+----------+--------------------------------------+
| FIELD    | VALUE                                |
+----------+--------------------------------------+
| ID       | ef4ab6fd-1252-4d5a-ba5a-8e8bd01610ae |
| Username | admin                                |
| Email    | root@localhost                       |
| Enabled  | true                                 |
+----------+--------------------------------------+
```

Alternatively you can run this in non-interactive mode. See ```garm-cli init -h``` for details.

## Enabling bash completion

Before we begin, let's make our lives a little easier and set up bash completion. The wonderful [cobra](https://github.com/spf13/cobra) library gives us completion for free:

```bash
mkdir $HOME/.bash_completion.d
echo 'source $HOME/.bash_completion.d/* >/dev/null 2>&1|| true' >> $HOME/.bash_completion
```

Now generate the completion file:

```bash
garm-cli completion bash > $HOME/.bash_completion.d/garm
```

Completion for multipiple shells is available:

```bash
ubuntu@experiments:~$ garm-cli completion
Generate the autocompletion script for garm-cli for the specified shell.
See each sub-command's help for details on how to use the generated script.

Usage:
  garm-cli completion [command]

Available Commands:
  bash        Generate the autocompletion script for bash
  fish        Generate the autocompletion script for fish
  powershell  Generate the autocompletion script for powershell
  zsh         Generate the autocompletion script for zsh

Flags:
  -h, --help   help for completion

Global Flags:
      --debug   Enable debug on all API calls

Use "garm-cli completion [command] --help" for more information about a command.
```

## Adding a repository/organization

To add a repository, we need credentials. Let's list the available credentials currently configured. These credentials are added to ```garm``` using the config file (see above), but we need to reference them by name when creating a repo.

```bash
ubuntu@experiments:~$ garm-cli credentials list
+---------+------------------------------+
| NAME    | DESCRIPTION                  |
+---------+------------------------------+
| gabriel | github token or user gabriel |
+---------+------------------------------+
```

Now we can add a repository to ```garm```:

```bash
ubuntu@experiments:~$ garm-cli repository create \
      --credentials=gabriel \
      --owner=gabriel-samfira \
      --name=scripts \
      --webhook-secret="super secret webhook secret you configured in github webhooks"
+-------------+--------------------------------------+
| FIELD       | VALUE                                |
+-------------+--------------------------------------+
| ID          | 77258e1b-81d2-4821-bdd7-f6923a026455 |
| Owner       | gabriel-samfira                      |
| Name        | scripts                              |
| Credentials | gabriel                              |
+-------------+--------------------------------------+
```

To add an organization, use the following command:

```bash
ubuntu@experiments:~$ garm-cli organization create \
      --credentials=gabriel \
      --name=gsamfira \
      --webhook-secret="$SECRET"
+-------------+--------------------------------------+
| FIELD       | VALUE                                |
+-------------+--------------------------------------+
| ID          | 7f0b83d5-3dc0-42de-b189-f9bbf1ae8901 |
| Name        | gsamfira                             |
| Credentials | gabriel                              |
+-------------+--------------------------------------+
```

## Creating a pool

Pools are objects that define one type of worker and rules by which that pool of workers will be maintained. You can have multiple pools of different types of instances. Each pool can have different images, be on different providers and have different tags.

Before we can create a pool, we need to list the available providers. Providers are defined in the config (see above), but we need to reference them by name in the pool.

```bash
ubuntu@experiments:~$ garm-cli provider list 
+-----------+------------------------+------+
| NAME      | DESCRIPTION            | TYPE |
+-----------+------------------------+------+
| lxd_local | Local LXD installation | lxd  |
+-----------+------------------------+------+
```

Now we can create a pool for repo ```gabriel-samfira/scripts```:

```bash
ubuntu@experiments:~$ garm-cli pool add \
      --repo=77258e1b-81d2-4821-bdd7-f6923a026455 \
      --flavor="default" \
      --image="ubuntu:20.04" \
      --provider-name="lxd_local" \
      --tags="ubuntu,simple-runner,repo-runner" \
      --enabled=false
+------------------+-------------------------------------------------------------+
| FIELD            | VALUE                                                       |
+------------------+-------------------------------------------------------------+
| ID               | fb25f308-7ad2-4769-988e-6ec2935f642a                        |
| Provider Name    | lxd_local                                                   |
| Image            | ubuntu:20.04                                                |
| Flavor           | default                                                     |
| OS Type          | linux                                                       |
| OS Architecture  | amd64                                                       |
| Max Runners      | 5                                                           |
| Min Idle Runners | 1                                                           |
| Tags             | ubuntu, simple-runner, repo-runner, self-hosted, x64, linux |
| Belongs to       | gabriel-samfira/scripts                                     |
| Level            | repo                                                        |
| Enabled          | false                                                       |
+------------------+-------------------------------------------------------------+
```

There are a bunch of things going on here, so let's break it down. We created a pool for repo ```gabriel-samfira/scripts``` (identified by the ID ```77258e1b-81d2-4821-bdd7-f6923a026455```). This pool has the following characteristics:

  * flavor=default - The **flavor** describes the hardware aspects of an instance. In LXD terms, this translates to [profiles](https://linuxcontainers.org/lxd/docs/master/profiles/). In LXD, profiles describe how much memory, CPU, NICs and disks a particular instance will get. Much like the flavors in OpenStack or any public cloud provider
  * image=ubuntu:20.04 - The image describes the operating system that will be spun up on the provider. LXD fetches these images from one of the configured remotes, or from the locally cached images. On AWS, this would be an AMI (for example).
  * provider-name=lxd_local - This is the provider on which we'll be spinning up runners. You can have as many providers defined as you wish, and you can reference either one of them when creating a pool.
  * tags="ubuntu,simple-runner,repo-runner" - This list of tags will be added to all runners maintained by this pool. These are the tags you can use to target whese runners in your workflows. By default, the github runner will automatically add a few default tags (self-hosted, x64, linux in the above example)
  * enabled=false - This option creates the pool in **disabled** state. When disabled, no new runners will be spun up.

By default, a pool is created with a max worker count of ```5``` and a minimum idle runner count of ```1```. This means that this pool will create by default one runner, and will automatically add more, as jobs are triggered on github. The idea is to have at least one runner ready to accept a workflow job. The pool will keep adding workers until the max runner count is reached. Once a workflow job is complete, the runner is automatically deleted, and replaced.

To update the pool, we cam use the following command:

```bash
ubuntu@experiments:~$ garm-cli pool update fb25f308-7ad2-4769-988e-6ec2935f642a --enabled=true
+------------------+-------------------------------------------------------------+
| FIELD            | VALUE                                                       |
+------------------+-------------------------------------------------------------+
| ID               | fb25f308-7ad2-4769-988e-6ec2935f642a                        |
| Provider Name    | lxd_local                                                   |
| Image            | ubuntu:20.04                                                |
| Flavor           | default                                                     |
| OS Type          | linux                                                       |
| OS Architecture  | amd64                                                       |
| Max Runners      | 5                                                           |
| Min Idle Runners | 1                                                           |
| Tags             | ubuntu, simple-runner, repo-runner, self-hosted, x64, linux |
| Belongs to       | gabriel-samfira/scripts                                     |
| Level            | repo                                                        |
| Enabled          | true                                                        |
+------------------+-------------------------------------------------------------+
```

Now, if we list the runners, we should see one being created:

```bash
ubuntu@experiments:~$ garm-cli runner ls fb25f308-7ad2-4769-988e-6ec2935f642a
+-------------------------------------------+----------------+---------------+--------------------------------------+
| NAME                                      | STATUS         | RUNNER STATUS | POOL ID                              |
+-------------------------------------------+----------------+---------------+--------------------------------------+
| garm-edeb8f46-ab09-4ed9-88fc-2731ecf9aabe | pending_create | pending       | fb25f308-7ad2-4769-988e-6ec2935f642a |
+-------------------------------------------+----------------+---------------+--------------------------------------+
```

We can also do a show on that runner to get more info:

```bash
ubuntu@experiments:~$ garm-cli runner show garm-edeb8f46-ab09-4ed9-88fc-2731ecf9aabe
+-----------------+-------------------------------------------+
| FIELD           | VALUE                                     |
+-----------------+-------------------------------------------+
| ID              | 089d63c9-5567-4318-a3a6-e065685c975b      |
| Provider ID     | garm-edeb8f46-ab09-4ed9-88fc-2731ecf9aabe |
| Name            | garm-edeb8f46-ab09-4ed9-88fc-2731ecf9aabe |
| OS Type         | linux                                     |
| OS Architecture | amd64                                     |
| OS Name         | ubuntu                                    |
| OS Version      | focal                                     |
| Status          | running                                   |
| Runner Status   | pending                                   |
| Pool ID         | fb25f308-7ad2-4769-988e-6ec2935f642a      |
+-----------------+-------------------------------------------+
```

If we check out LXD, we can see the instance was created and is currently being bootstrapped:

```bash
ubuntu@experiments:~$ lxc list
+-------------------------------------------+---------+-------------------------+------+-----------------+-----------+
|                   NAME                    |  STATE  |          IPV4           | IPV6 |      TYPE       | SNAPSHOTS |
+-------------------------------------------+---------+-------------------------+------+-----------------+-----------+
| garm-edeb8f46-ab09-4ed9-88fc-2731ecf9aabe | RUNNING | 10.247.246.219 (enp5s0) |      | VIRTUAL-MACHINE | 0         |
+-------------------------------------------+---------+-------------------------+------+-----------------+-----------+
```

It might take a couple of minutes for the runner to come online, as the instance will do a full upgrade, then download the runner and install it. But once the installation is done you should see something like this:

```bash
ubuntu@experiments:~$ garm-cli runner show garm-edeb8f46-ab09-4ed9-88fc-2731ecf9aabe
+-----------------+--------------------------------------------------------------------------------------------------------------------------------------------------+
| FIELD           | VALUE                                                                                                                                            |
+-----------------+--------------------------------------------------------------------------------------------------------------------------------------------------+
| ID              | 089d63c9-5567-4318-a3a6-e065685c975b                                                                                                             |
| Provider ID     | garm-edeb8f46-ab09-4ed9-88fc-2731ecf9aabe                                                                                                        |
| Name            | garm-edeb8f46-ab09-4ed9-88fc-2731ecf9aabe                                                                                                        |
| OS Type         | linux                                                                                                                                            |
| OS Architecture | amd64                                                                                                                                            |
| OS Name         | ubuntu                                                                                                                                           |
| OS Version      | focal                                                                                                                                            |
| Status          | running                                                                                                                                          |
| Runner Status   | idle                                                                                                                                             |
| Pool ID         | fb25f308-7ad2-4769-988e-6ec2935f642a                                                                                                             |
| Status Updates  | 2022-05-06T13:21:54: downloading tools from https://github.com/actions/runner/releases/download/v2.291.1/actions-runner-linux-x64-2.291.1.tar.gz |
|                 | 2022-05-06T13:21:56: extracting runner                                                                                                           |
|                 | 2022-05-06T13:21:58: installing dependencies                                                                                                     |
|                 | 2022-05-06T13:22:07: configuring runner                                                                                                          |
|                 | 2022-05-06T13:22:12: installing runner service                                                                                                   |
|                 | 2022-05-06T13:22:12: starting service                                                                                                            |
|                 | 2022-05-06T13:22:13: runner successfully installed                                                                                               |
+-----------------+--------------------------------------------------------------------------------------------------------------------------------------------------+
```

If we list the runners for this pool, we should see one runner with a ```RUNNER STATUS``` of ```idle```:

```bash
ubuntu@experiments:~$ garm-cli runner ls fb25f308-7ad2-4769-988e-6ec2935f642a
+-------------------------------------------+---------+---------------+--------------------------------------+
| NAME                                      | STATUS  | RUNNER STATUS | POOL ID                              |
+-------------------------------------------+---------+---------------+--------------------------------------+
| garm-edeb8f46-ab09-4ed9-88fc-2731ecf9aabe | running | idle          | fb25f308-7ad2-4769-988e-6ec2935f642a |
+-------------------------------------------+---------+---------------+--------------------------------------+
```

## Updating a pool

Let's update the pool and request that it maintain a number of minimum idle runners equal to 3:

```bash
ubuntu@experiments:~$ garm-cli pool update fb25f308-7ad2-4769-988e-6ec2935f642a \
      --min-idle-runners=3 \
      --max-runners=10
+------------------+----------------------------------------------------------------------------------+
| FIELD            | VALUE                                                                            |
+------------------+----------------------------------------------------------------------------------+
| ID               | fb25f308-7ad2-4769-988e-6ec2935f642a                                             |
| Provider Name    | lxd_local                                                                        |
| Image            | ubuntu:20.04                                                                     |
| Flavor           | default                                                                          |
| OS Type          | linux                                                                            |
| OS Architecture  | amd64                                                                            |
| Max Runners      | 10                                                                               |
| Min Idle Runners | 3                                                                                |
| Tags             | ubuntu, simple-runner, repo-runner, self-hosted, x64, linux                      |
| Belongs to       | gabriel-samfira/scripts                                                          |
| Level            | repo                                                                             |
| Enabled          | true                                                                             |
| Instances        | garm-edeb8f46-ab09-4ed9-88fc-2731ecf9aabe (089d63c9-5567-4318-a3a6-e065685c975b) |
+------------------+----------------------------------------------------------------------------------+
```

Now if we list runners we should see 2 more in ```pending``` state:

```bash
ubuntu@experiments:~$ garm-cli runner ls fb25f308-7ad2-4769-988e-6ec2935f642a
+-------------------------------------------+---------+---------------+--------------------------------------+
| NAME                                      | STATUS  | RUNNER STATUS | POOL ID                              |
+-------------------------------------------+---------+---------------+--------------------------------------+
| garm-edeb8f46-ab09-4ed9-88fc-2731ecf9aabe | running | idle          | fb25f308-7ad2-4769-988e-6ec2935f642a |
+-------------------------------------------+---------+---------------+--------------------------------------+
| garm-bc180c6c-6e31-4c7b-8ce1-da0ffd76e247 | running | pending       | fb25f308-7ad2-4769-988e-6ec2935f642a |
+-------------------------------------------+---------+---------------+--------------------------------------+
| garm-37c5daf4-18c5-47fc-95de-8c1656889093 | running | pending       | fb25f308-7ad2-4769-988e-6ec2935f642a |
+-------------------------------------------+---------+---------------+--------------------------------------+
```

We can see them in LXC as well:

```bash
ubuntu@experiments:~$ lxc list
+-------------------------------------------+---------+-------------------------+------+-----------------+-----------+
|                   NAME                    |  STATE  |          IPV4           | IPV6 |      TYPE       | SNAPSHOTS |
+-------------------------------------------+---------+-------------------------+------+-----------------+-----------+
| garm-37c5daf4-18c5-47fc-95de-8c1656889093 | RUNNING |                         |      | VIRTUAL-MACHINE | 0         |
+-------------------------------------------+---------+-------------------------+------+-----------------+-----------+
| garm-bc180c6c-6e31-4c7b-8ce1-da0ffd76e247 | RUNNING |                         |      | VIRTUAL-MACHINE | 0         |
+-------------------------------------------+---------+-------------------------+------+-----------------+-----------+
| garm-edeb8f46-ab09-4ed9-88fc-2731ecf9aabe | RUNNING | 10.247.246.219 (enp5s0) |      | VIRTUAL-MACHINE | 0         |
+-------------------------------------------+---------+-------------------------+------+-----------------+-----------+
```

Once they transition to ```idle```, you should see them in your repo settings, under ```Actions --> Runners```.


The procedure is identical for organizations. Have a look at the garm-cli help:


```bash
ubuntu@experiments:~$ garm-cli -h
CLI for the github self hosted runners manager.

Usage:
  garm-cli [command]

Available Commands:
  completion   Generate the autocompletion script for the specified shell
  credentials  List configured credentials
  help         Help about any command
  init         Initialize a newly installed garm
  login        Log into a manager
  organization Manage organizations
  pool         List pools
  provider     Interacts with the providers API resource.
  repository   Manage repositories
  runner       List runners in a pool

Flags:
      --debug   Enable debug on all API calls
  -h, --help    help for garm-cli

Use "garm-cli [command] --help" for more information about a command.
```

## Security considerations

Garm does not apply any ACLs of any kind to the instances it creates. That task remains in the responsability of the user. [Here is a guide for creating ACLs in LXD](https://linuxcontainers.org/lxd/docs/master/howto/network_acls/). You can of course use ```iptables``` or ```nftables``` to create any rules you wish. I recommend you create a separate isolated lxd bridge for runners, and secure it using ACLs/iptables/nftables.

You must make sure that the code that runs as part of the workflows is trusted, and if that cannot be done, you must make sure that any malitious code that will be pulled in by the actions and run as part of a workload, is as contained as possible. There is a nice article about [securing your workflow runs here](https://blog.gitguardian.com/github-actions-security-cheat-sheet/).