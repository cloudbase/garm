#!/bin/bash

echo $GARM_REF

VERSION=$(git describe --tags --match='v[0-9]*' --dirty --always)
RELEASE="$PWD/release"

[ ! -d "$RELEASE" ] && mkdir -p "$RELEASE"

if [ ! -z "$GARM_REF" ]; then
    VERSION=$(git describe --tags --match='v[0-9]*' --always $GARM_REF)
fi

echo $VERSION

if [ ! -d "build/$VERSION" ]; then
    echo "missing build/$VERSION"
    exit 1
fi

# Windows

if [ ! -d "build/$VERSION/windows/amd64" ];then
    echo "missing build/$VERSION/windows/amd64"
    exit 1
fi

WINDOWS_FILES=("garm.exe" "garm-cli.exe")

for file in ${WINDOWS_FILES[@]};do
    if [ ! -f "build/$VERSION/windows/amd64/$file" ];then
        echo "missing build/$VERSION/windows/amd64/$file"
        exit 1
    fi

    pushd build/$VERSION/windows/amd64
    zip ${file%%.exe}-windows-amd64.zip $file
    sha256sum ${file%%.exe}-windows-amd64.zip > ${file%%.exe}-windows-amd64.zip.sha256
    mv ${file%%.exe}-windows-amd64.zip $RELEASE
    mv ${file%%.exe}-windows-amd64.zip.sha256 $RELEASE
    popd
done

# Linux
OS_ARCHES=("amd64" "arm64")
FILES=("garm" "garm-cli")

for arch in ${OS_ARCHES[@]};do
    for file in ${FILES[@]};do
        if [ ! -f "build/$VERSION/linux/$arch/$file" ];then
            echo "missing build/$VERSION/linux/$arch/$file"
            exit 1
        fi

        pushd build/$VERSION/linux/$arch
        tar czf ${file}-linux-$arch.tgz $file
        sha256sum ${file}-linux-$arch.tgz > ${file}-linux-$arch.tgz.sha256
        mv ${file}-linux-$arch.tgz $RELEASE
        mv ${file}-linux-$arch.tgz.sha256 $RELEASE
        popd
    done
done
