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

## Adding a repository/organization/enterprise

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

To add an enterprise, use the following command:

```bash
ubuntu@experiments:~$ garm-cli enterprise create \
      --credentials=gabriel \
      --name=gsamfira \
      --webhook-secret="$SECRET"
+-------------+--------------------------------------+
| FIELD       | VALUE                                |
+-------------+--------------------------------------+
| ID          | 0925033b-049f-4334-a460-c26f979d2356 |
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
  debug-log    Stream garm log
  enterprise   Manage enterprise
  help         Help about any command
  init         Initialize a newly installed garm
  organization Manage organizations
  pool         List pools
  profile      Add, delete or update profiles
  provider     Interacts with the providers API resource.
  repository   Manage repositories
  runner       List runners in a pool
  version      Print version and exit

Flags:
      --debug   Enable debug on all API calls
  -h, --help    help for garm-cli

Use "garm-cli [command] --help" for more information about a command.

```
