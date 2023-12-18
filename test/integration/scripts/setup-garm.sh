#!/usr/bin/env bash
set -o errexit

DIR="$(dirname $0)"
BINARIES_DIR="$PWD/bin"
CONTRIB_DIR="$PWD/contrib"
CONFIG_DIR="$PWD/test/integration/config"
CONFIG_DIR_PROV="$PWD/test/integration/provider"
PROVIDER_BIN_DIR="/opt/garm/providers.d/lxd"

if [[ ! -f $BINARIES_DIR/garm ]] || [[ ! -f $BINARIES_DIR/garm-cli ]]; then
    echo "ERROR: Please build GARM binaries first"
    exit 1
fi

if [[ -z $GH_OAUTH_TOKEN ]]; then echo "ERROR: The env variable GH_OAUTH_TOKEN is not set"; exit 1; fi
if [[ -z $CREDENTIALS_NAME ]]; then echo "ERROR: The env variable CREDENTIALS_NAME is not set"; exit 1; fi
if [[ -z $GARM_BASE_URL ]]; then echo "ERROR: The env variable GARM_BASE_URL is not set"; exit 1; fi

# Generate a random 32-char secret for JWT_AUTH_SECRET and DB_PASSPHRASE.
function generate_secret() {
    (tr -dc 'a-zA-Z0-9!@#$%^&*()_+?><~\`;' < /dev/urandom | head -c 32) 2>/dev/null
}

# Wait for a port to open at a given address.
function wait_open_port() {
    local ADDRESS="$1"
    local PORT="$2"
    local TIMEOUT=30
    SECONDS=0
    while true; do
        if [[ $SECONDS -gt $TIMEOUT ]]; then
            echo "ERROR: Port $PORT didn't open at $ADDRESS within $TIMEOUT seconds"
            return 1
        fi
        nc -v -w 5 -z "$ADDRESS" "$PORT" &>/dev/null && break || sleep 1
    done
    echo "Port $PORT at address $ADDRESS is open"
}

export JWT_AUTH_SECRET="$(generate_secret)"
export DB_PASSPHRASE="$(generate_secret)"

# Group "adm" is the LXD daemon group as set by the "canonical/setup-lxd" GitHub action.
sudo useradd --shell /usr/bin/false --system --groups adm --no-create-home garm

export LXD_PROVIDER_EXECUTABLE="$PROVIDER_BIN_DIR/garm-provider-lxd"
export LXD_PROVIDER_CONFIG="/etc/garm/garm-provider-lxd.toml"
sudo cp $CONFIG_DIR/garm-provider-lxd.toml $LXD_PROVIDER_CONFIG

sudo mkdir -p $PROVIDER_BIN_DIR
git clone https://github.com/cloudbase/garm-provider-lxd ~/garm-provider-lxd
pushd ~/garm-provider-lxd
go build -o $LXD_PROVIDER_EXECUTABLE
popd

sudo mkdir -p /etc/garm
cat $CONFIG_DIR/config.toml | envsubst | sudo tee /etc/garm/config.toml
sudo chown -R garm:garm /etc/garm

sudo mkdir /etc/garm/test-provider
sudo touch $CONFIG_DIR_PROV/config
sudo cp $CONFIG_DIR_PROV/* /etc/garm/test-provider  

sudo mv $BINARIES_DIR/* /usr/local/bin/
sudo cp $CONTRIB_DIR/garm.service /etc/systemd/system/garm.service

sudo systemctl daemon-reload
sudo systemctl start garm

wait_open_port 127.0.0.1 9997

echo "GARM is up and running"
