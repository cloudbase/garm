#!/usr/bin/env bash
set -o errexit

DIR="$(dirname $0)"
BINARIES_DIR="$PWD/bin"
CONTRIB_DIR="$PWD/contrib"
export CONFIG_DIR="$PWD/test/integration/config"
export CONFIG_DIR_PROV="$PWD/test/integration/provider"
export GARM_CONFIG_DIR=${GARM_CONFIG_DIR:-$(mktemp -d)}
export PROVIDER_BIN_DIR="$GARM_CONFIG_DIR/providers.d/lxd"
export IS_GH_WORKFLOW=${IS_GH_WORKFLOW:-"true"}
export LXD_PROVIDER_LOCATION=${LXD_PROVIDER_LOCATION:-""}
export RUN_USER=${RUN_USER:-$USER}
export GARM_PORT=${GARM_PORT:-"9997"}
export GARM_SERVICE_NAME=${GARM_SERVICE_NAME:-"garm"}
export GARM_CONFIG_FILE=${GARM_CONFIG_FILE:-"${GARM_CONFIG_DIR}/config.toml"}

if [ -f "$GITHUB_ENV" ];then
    echo "export GARM_CONFIG_DIR=${GARM_CONFIG_DIR}" >> $GITHUB_ENV
    echo "export GARM_SERVICE_NAME=${GARM_SERVICE_NAME}" >> $GITHUB_ENV
fi

if [[ ! -f $BINARIES_DIR/garm ]] || [[ ! -f $BINARIES_DIR/garm-cli ]]; then
    echo "ERROR: Please build GARM binaries first"
    exit 1
fi


if [[ -z $GH_TOKEN ]]; then echo "ERROR: The env variable GH_TOKEN is not set"; exit 1; fi
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

if [ $IS_GH_WORKFLOW == "true" ]; then
    # Group "adm" is the LXD daemon group as set by the "canonical/setup-lxd" GitHub action.
    sudo useradd --shell /usr/bin/false --system --groups adm --no-create-home garm
fi

sudo mkdir -p ${GARM_CONFIG_DIR}
sudo mkdir -p $PROVIDER_BIN_DIR
sudo chown -R $RUN_USER:$RUN_USER ${PROVIDER_BIN_DIR}
sudo chown -R $RUN_USER:$RUN_USER ${GARM_CONFIG_DIR}

export LXD_PROVIDER_EXECUTABLE="$PROVIDER_BIN_DIR/garm-provider-lxd"
export LXD_PROVIDER_CONFIG="${GARM_CONFIG_DIR}/garm-provider-lxd.toml"
sudo cp $CONFIG_DIR/garm-provider-lxd.toml $LXD_PROVIDER_CONFIG

function clone_and_build_lxd_provider() {
    git clone https://github.com/cloudbase/garm-provider-lxd ~/garm-provider-lxd
    pushd ~/garm-provider-lxd
    go build -o $LXD_PROVIDER_EXECUTABLE
    popd
}

if [ $IS_GH_WORKFLOW == "true" ]; then
    clone_and_build_lxd_provider
else
    if [ -z "$LXD_PROVIDER_LOCATION" ];then
        clone_and_build_lxd_provider
    else
        cp $LXD_PROVIDER_LOCATION $LXD_PROVIDER_EXECUTABLE
    fi

fi

cat $CONFIG_DIR/config.toml | envsubst | sudo tee ${GARM_CONFIG_DIR}/config.toml > /dev/null
sudo chown -R $RUN_USER:$RUN_USER ${GARM_CONFIG_DIR}

sudo mkdir -p ${GARM_CONFIG_DIR}/test-provider
sudo touch $CONFIG_DIR_PROV/config
sudo cp $CONFIG_DIR_PROV/* ${GARM_CONFIG_DIR}/test-provider

sudo mv $BINARIES_DIR/* /usr/local/bin/
mkdir -p $HOME/.local/share/systemd/user/
cat $CONFIG_DIR/garm.service| envsubst | sudo tee /lib/systemd/system/${GARM_SERVICE_NAME}@.service > /dev/null
sudo chown -R $RUN_USER:$RUN_USER ${GARM_CONFIG_DIR}

sudo systemctl daemon-reload
sudo systemctl enable ${GARM_SERVICE_NAME}@${RUN_USER}
sudo systemctl restart ${GARM_SERVICE_NAME}@${RUN_USER}
wait_open_port 127.0.0.1 ${GARM_PORT}

echo "GARM is up and running"
echo "GARM config file is $GARM_CONFIG_FILE"
echo "GARM service name is $GARM_SERVICE_NAME"
