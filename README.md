#  GitHub Actions Runners Manager (garm)

[![Go Tests](https://github.com/cloudbase/garm/actions/workflows/go-tests.yml/badge.svg)](https://github.com/cloudbase/garm/actions/workflows/go-tests.yml)

Welcome to garm!

Garm enables you to create and automatically maintain pools of [self-hosted GitHub runners](https://docs.github.com/en/actions/hosting-your-own-runners/about-self-hosted-runners), with autoscaling that can be used inside your github workflow runs.

The goal of ```garm``` is to be simple to set up, simple to configure and simple to use. It is a single binary that can run on any GNU/Linux machine without any other requirements other than the providers it creates the runners in. It is intended to be easy to deploy in any environment and can create runners in any system you can write a provider for. There is no complicated setup process and no extremely complex concepts to understant. Once set up, it's meant to stay out of your way.  

Garm supports creating pools on either GitHub itself or on your own deployment of [GitHub Enterprise Server](https://docs.github.com/en/enterprise-server@3.5/admin/overview/about-github-enterprise-server). For instructions on how to use ```garm``` with GHE, see the [credentials](/doc/github_credentials.md) section of the documentation.

## Installing

## Build from source

You need to have Go installed, then run:

```bash
git clone https://github.com/cloudbase/garm
cd garm
go install ./...
```
You should now have both ```garm``` and ```garm-cli``` in your ```$GOPATH/bin``` folder.

If you have docker/podman installed, you can also build statically linked binaries by running:

```bash
make
```

The ```garm``` and ```garm-cli``` binaries will be built and copied to the ```bin/``` folder in your current working directory.

## Install the service

Add a new system user:

```bash
useradd --shell /usr/bin/false \
    --system \
    --groups lxd \
    --no-create-home garm
```

The ```lxd``` group is only needed if you have a local LXD install and want to connect to the unix socket to use it. If you're connecting to a remote LXD server over TCP, you can skip adding the ```garm``` user to the ```lxd``` group.

Copy the binary to somewhere in the system ```$PATH```:

```bash
sudo cp $(go env GOPATH)/bin/garm /usr/local/bin/garm
```

Or if you built garm using ```make```:

```bash
sudo cp ./bin/garm /usr/local/bin/garm
```

Create the config folder:

```bash
sudo mkdir -p /etc/garm
```

Copy the config template:

```bash
sudo cp ./testdata/config.toml /etc/garm/
```

Copy the external provider (optional):

```bash
sudo cp -a ./contrib/providers.d /etc/garm/
```

Copy the systemd service file:

```bash
sudo cp ./contrib/garm.service /etc/systemd/system/
```

Change permissions on config folder:

```bash
sudo chown -R garm:garm /etc/garm
sudo chmod 750 -R /etc/garm
```

Enable the service:

```bash
sudo systemctl enable garm
```

Customize the config in ```/etc/garm/config.toml```, and start the service:

```bash
sudo systemctl start garm
```

## Configuration

The ```garm``` configuration is a simple ```toml```. A sample of the config file can be found in [the testdata folder](/testdata/config.toml).

There are 3 major sections of the config that require your attention:

  * [Github credentials section](/doc/github_credentials.md)
  * [Providers section](/doc/providers.md)
  * [The database section](/doc/database.md)

Once you've configured your database, providers and github credentials, you'll need to configure your [webhooks and the callback_url](/doc/webhooks_and_callbacks.md).

At this point, you should be done. Have a look at the [running garm document](/doc/running_garm.md) for usage instructions and available features.

If you would like to use ```garm``` with a different IaaS than the ones already available, have a loot at the [writing an external provider](/doc/external_provider.md) page.


## Security considerations

Garm does not apply any ACLs of any kind to the instances it creates. That task remains in the responsability of the user. [Here is a guide for creating ACLs in LXD](https://linuxcontainers.org/lxd/docs/master/howto/network_acls/). You can of course use ```iptables``` or ```nftables``` to create any rules you wish. I recommend you create a separate isolated lxd bridge for runners, and secure it using ACLs/iptables/nftables.

You must make sure that the code that runs as part of the workflows is trusted, and if that cannot be done, you must make sure that any malitious code that will be pulled in by the actions and run as part of a workload, is as contained as possible. There is a nice article about [securing your workflow runs here](https://blog.gitguardian.com/github-actions-security-cheat-sheet/).

## Write your own provider

The providers are interfaces between ```garm``` and a particular IaaS in which we spin up GitHub Runners. These providers can be either **native** or **external**. The **native** providers are written in ```Go```, and must implement [the interface defined here](https://github.com/cloudbase/garm/blob/main/runner/common/provider.go#L22-L39). **External** providers can be written in any language, as they are in the form of an external executable that ```garm``` calls into.

There is currently one **native** provider for [LXD](https://linuxcontainers.org/lxd/) and two **external** providers for [Openstack and Azure](/contrib/providers.d/).

If you want to write your own provider, you can choose to write a native one, or implement an **external** one. The easiest one to write is probably an **external** provider. Please see the [Writing an external provider](/doc/external_provider.md) document for details. Also, feel free to inspect the two available external providers in this repository.