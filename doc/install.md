# Install

## Build from source

You need to have Go install, then run:

```bash
git clone https://github.com/cloudbase/garm
cd garm
go install ./...
```

You should now have both ```garm``` and ```garm-cli``` in your ```$GOPATH/bin``` folder.

## Install the service

Add a new system user:

```bash
useradd --shell /usr/bin/false \
    --system \
    --groups lxd \
    --no-create-home garm
```

Copy the binary from your ```$GOPATH``` to somewhere in the system ```$PATH```:

```bash
sudo cp $(go env GOPATH)/bin/garm /usr/local/bin/garm
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