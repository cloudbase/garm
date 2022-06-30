#!/bin/sh

GARM_SOURCE="/build/garm"
BIN_DIR="$GARM_SOURCE/bin"
git config --global --add safe.directory "$GARM_SOURCE"

[ ! -d "$BIN_DIR" ] && mkdir -p "$BIN_DIR"

export CGO_ENABLED=1
USER_ID=${USER_ID:-$UID}
USER_GROUP=${USER_GROUP:-$(id -g)}

cd $GARM_SOURCE/cmd/garm
go build -mod vendor -o $BIN_DIR/garm -tags osusergo,netgo,sqlite_omit_load_extension -ldflags "-linkmode external -extldflags '-static' -s -w -X main.Version=$(git describe --always --dirty)" .

cd $GARM_SOURCE/cmd/garm-cli
go build -mod vendor -o $BIN_DIR/garm-cli -tags osusergo,netgo -ldflags "-linkmode external -extldflags '-static' -s -w -X garm/cmd/garm-cli/cmd.Version=$(git describe --always --dirty)" .
GOOS=windows CGO_ENABLED=0 go build -mod vendor -o $BIN_DIR/garm-cli.exe -ldflags "-s -w -X garm/cmd/garm-cli/cmd.Version=$(git describe --always --dirty)" .

chown $USER_ID:$USER_GROUP -R "$BIN_DIR"