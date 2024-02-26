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
    systemctl --user stop $GARM_SERVICE_NAME.service
    rm $HOME/.local/share/systemd/user/${GARM_SERVICE_NAME}.service
fi

if [ -d "$GARM_CONFIG_DIR" ] && [ -f "$GARM_CONFIG_DIR/config.toml" ] && [ -f "$GARM_CONFIG_DIR/garm-provider-lxd.toml" ];then
    rm -rf ${GARM_CONFIG_DIR}
fi