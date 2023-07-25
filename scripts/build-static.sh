#!/bin/sh

GARM_SOURCE="/build/garm"
BIN_DIR="$GARM_SOURCE/bin"
git config --global --add safe.directory "$GARM_SOURCE"

[ ! -d "$BIN_DIR" ] && mkdir -p "$BIN_DIR"

export CGO_ENABLED=1
USER_ID=${USER_ID:-$UID}
USER_GROUP=${USER_GROUP:-$(id -g)}

mkdir -p $BIN_DIR/amd64 $BIN_DIR/arm64
cd $GARM_SOURCE/cmd/garm
go build -mod vendor \
    -o $BIN_DIR/amd64/garm \
    -tags osusergo,netgo,sqlite_omit_load_extension \
    -ldflags "-linkmode external -extldflags '-static' -s -w -X main.Version=$(git describe --tags --match='v[0-9]*' --dirty --always)" .
CC=aarch64-linux-musl-gcc GOARCH=arm64 go build \
    -mod vendor \
    -o $BIN_DIR/arm64/garm \
    -tags osusergo,netgo,sqlite_omit_load_extension \
    -ldflags "-linkmode external -extldflags '-static' -s -w -X main.Version=$(git describe --tags --match='v[0-9]*' --dirty --always)" .
# GOOS=windows CC=x86_64-w64-mingw32-cc go build -mod vendor \
#     -o $BIN_DIR/amd64/garm.exe \
#     -tags osusergo,netgo,sqlite_omit_load_extension \
#     -ldflags "-s -w -X main.Version=$(git describe --tags --match='v[0-9]*' --dirty --always)" .

cd $GARM_SOURCE/cmd/garm-cli
go build -mod vendor \
    -o $BIN_DIR/amd64/garm-cli \
    -tags osusergo,netgo,sqlite_omit_load_extension \
    -ldflags "-linkmode external -extldflags '-static' -s -w -X github.com/cloudbase/garm/cmd/garm-cli/cmd.Version=$(git describe --tags --match='v[0-9]*' --dirty --always)" .
CC=aarch64-linux-musl-gcc GOARCH=arm64 go build -mod vendor \
    -o $BIN_DIR/arm64/garm-cli \
    -tags osusergo,netgo,sqlite_omit_load_extension \
    -ldflags "-linkmode external -extldflags '-static' -s -w -X github.com/cloudbase/garm/cmd/garm-cli/cmd.Version=$(git describe --tags --match='v[0-9]*' --dirty --always)" .
# GOOS=windows CGO_ENABLED=0 go build -mod vendor \
#     -o $BIN_DIR/amd64/garm-cli.exe \
#     -tags osusergo,netgo,sqlite_omit_load_extension \
#     -ldflags "-s -w -X github.com/cloudbase/garm/cmd/garm-cli/cmd.Version=$(git describe --tags --match='v[0-9]*' --dirty --always)" .

chown $USER_ID:$USER_GROUP -R "$BIN_DIR"
