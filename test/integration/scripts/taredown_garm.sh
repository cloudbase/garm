#!/bin/bash

if [ -f "$GITHUB_ENV" ];then
    source $GITHUB_ENV
fi

if [ -z $GARM_CONFIG_DIR ]; then
    echo "ERROR: GARM_CONFIG_DIR is not set"
    exit 1
fi

if [ -z $GARM_SERVICE_NAME ]; then
    echo "ERROR: GARM_SERVICE_NAME is not set"
    exit 1
fi

if [ -f "$HOME/.local/share/systemd/user/${GARM_SERVICE_NAME}.service" ];then
    sudo systemctl stop $GARM_SERVICE_NAME@${RUN_USER}
    sudo systemctl disable $GARM_SERVICE_NAME@${RUN_USER}
    sudo rm /lib/systemd/system/${GARM_SERVICE_NAME}@.service
    sudo systemctl daemon-reload
fi

if [ -d "$GARM_CONFIG_DIR" ] && [ -f "$GARM_CONFIG_DIR/config.toml" ] && [ -f "$GARM_CONFIG_DIR/garm-provider-lxd.toml" ];then
    rm -rf ${GARM_CONFIG_DIR}
fi