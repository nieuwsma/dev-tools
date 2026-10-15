#!/bin/bash
# Copyright 2026 Nieuwsma Services LLC
# SPDX-License-Identifier: Apache-2.0

# Install only codex-sleep. No shell, tmux, or machine settings are changed.
set -euo pipefail

destination="$HOME/.local/bin"
case ${1:-} in
    -h|--help)
        echo 'Usage: install.sh [--prefix BIN_DIRECTORY]'
        echo 'Default: ~/.local/bin. Installs only codex-sleep; add this directory to PATH.'
        exit 0 ;;
    --prefix)
        [[ $# == 2 && -n $2 ]] || { echo '--prefix needs a directory' >&2; exit 1; }
        destination=$2 ;;
    '') ;;
    *) echo 'Usage: install.sh [--prefix BIN_DIRECTORY]' >&2; exit 1 ;;
esac
here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
mkdir -p -- "$destination"
target="$destination/codex-sleep"
if [[ -e $target || -L $target ]]; then
    if [[ ! -L $target && -x $target ]] && cmp -s "$here/codex-sleep" "$target"; then
        echo "Already installed: $target"
        exit 0
    fi
    backup=$(mktemp "${target}.before-install.XXXXXXXX")
    cp -p -- "$target" "$backup"
    echo "Backup: $backup"
fi
# Rename a new file into place, so an in-use launcher can finish reading its copy.
staged=$(mktemp "$destination/.codex-sleep-install.XXXXXXXX")
trap 'rm -f -- "$staged"' EXIT
install -m 755 "$here/codex-sleep" "$staged"
mv -f -- "$staged" "$target"
echo "Installed: $target"
