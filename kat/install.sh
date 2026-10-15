#!/bin/bash
# Copyright 2026 Nieuwsma Services LLC
# SPDX-License-Identifier: Apache-2.0

# Install only kat. No shell, tmux, or machine settings are changed.
set -euo pipefail

destination="$HOME/.local/bin"
case ${1:-} in
    -h|--help)
        echo 'Usage: install.sh [--prefix BIN_DIRECTORY]'
        echo 'Default: ~/.local/bin. Builds kat with the local Go toolchain.'
        exit 0 ;;
    --prefix)
        [[ $# == 2 && -n $2 ]] || { echo '--prefix needs a directory' >&2; exit 1; }
        destination=$2 ;;
    '') ;;
    *) echo 'Usage: install.sh [--prefix BIN_DIRECTORY]' >&2; exit 1 ;;
esac
here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
command -v go >/dev/null || { echo 'kat install needs Go on PATH' >&2; exit 1; }
mkdir -p -- "$destination"
target="$destination/kat"
staged=$(mktemp "$destination/.kat-install.XXXXXXXX")
trap 'rm -f -- "$staged"' EXIT
(
    cd -- "$here"
    go build -o "$staged" .
)
chmod 755 "$staged"
if [[ -e $target || -L $target ]]; then
    if [[ ! -L $target && -x $target ]] && cmp -s "$staged" "$target"; then
        echo "Already installed: $target"
        exit 0
    fi
    backup=$(mktemp "${target}.before-install.XXXXXXXX")
    cp -p -- "$target" "$backup"
    echo "Backup: $backup"
fi
mv -f -- "$staged" "$target"
trap - EXIT
echo "Installed: $target"
