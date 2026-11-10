#!/bin/bash
set -euo pipefail

REPO=peauc/lazydocker-ng

# allow specifying different destination directory
DIR="${DIR:-"$HOME/.local/bin"}"

# map different architecture variations to the available binaries
MACHINE=$(uname -m)
case $MACHINE in
    x86_64|amd64) ARCH=x86_64 ;;
    aarch64*|arm64) ARCH=arm64 ;;
    *)
        echo "lazydocker-ng: unsupported architecture '$MACHINE': release binaries are built for x86_64 and arm64 (since v0.29.0)" >&2
        exit 1
        ;;
esac

# prepare the download URL
TAG=$(curl -fsSL -H 'Accept: application/json' "https://github.com/$REPO/releases/latest" | sed -e 's/.*"tag_name":"\([^"]*\)".*/\1/')
if [ -z "$TAG" ]; then
    echo "lazydocker-ng: could not find the latest release of $REPO" >&2
    exit 1
fi
FILE="lazydocker-ng_${TAG#v}_$(uname -s)_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/${TAG}/${FILE}"

# install/update the local binary
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fL -o "$tmp/lazydocker-ng.tar.gz" "$URL"
tar xzf "$tmp/lazydocker-ng.tar.gz" -C "$tmp" lazydocker-ng
mkdir -p "$DIR"
install -m 755 "$tmp/lazydocker-ng" "$DIR/lazydocker-ng"
echo "installed lazydocker-ng $TAG to $DIR"
