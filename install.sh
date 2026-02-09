#!/bin/sh
# Install alf — audio lf
set -e

PREFIX="${PREFIX:-$HOME/.local}"
LFCONF="${XDG_CONFIG_HOME:-$HOME/.config}/lf"

install -Dm755 aw "$PREFIX/bin/aw"
install -Dm755 alf "$PREFIX/bin/alf"
install -Dm755 alf-scope "$LFCONF/alf-scope"
install -Dm644 alf-rc "$LFCONF/alf-rc"

echo "installed: aw, alf, alf-scope, alf-rc"
echo "run: alf /path/to/samples"
