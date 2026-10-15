#!/bin/bash
# Copyright 2026 Nieuwsma Services LLC
# SPDX-License-Identifier: Apache-2.0

# Copy the shared repo-tools bundle and install three independent entry points.
set -euo pipefail

destination="$HOME/.local/bin"
case ${1:-} in
    -h|--help)
        echo 'Usage: install.sh [--prefix BIN_DIRECTORY]'
        echo 'Installs repo-status, repo-branch, repo-push, and their shared runtime.'
        exit 0 ;;
    --prefix)
        [[ $# == 2 && -n $2 ]] || { echo '--prefix needs a directory' >&2; exit 1; }
        destination=$2 ;;
    '') ;;
    *) echo 'Usage: install.sh [--prefix BIN_DIRECTORY]' >&2; exit 1 ;;
esac
here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
mkdir -p -- "$destination"
destination=$(cd -- "$destination" && pwd -P)
shared_parent="$(dirname -- "$destination")/share/dev-tools"
mkdir -p -- "$shared_parent"
bundle="$shared_parent/repo-tools"
staged=$(mktemp -d "$shared_parent/.repo-tools-install.XXXXXXXX")
wrapper=''
trap 'rm -rf -- "$staged"; if [[ -n $wrapper ]]; then rm -f -- "$wrapper"; fi' EXIT
runtime=(lib/repos.sh repo-status/scripts/repo-status.sh repo-branch/repo-branch.sh repo-push/repo-push.sh)
for relative in "${runtime[@]}"; do
    mkdir -p -- "$staged/$(dirname -- "$relative")"
    cp -p -- "$here/$relative" "$staged/$relative"
done

if [[ -d $bundle && ! -L $bundle ]] && diff -qr "$staged" "$bundle" >/dev/null; then
    echo "Already installed runtime: $bundle"
else
    if [[ -e $bundle || -L $bundle ]]; then
        backup=$(mktemp -d "$shared_parent/repo-tools.before-install.XXXXXXXX")
        mv -- "$bundle" "$backup/runtime"
        echo "Runtime backup: $backup/runtime"
    fi
    mv -- "$staged" "$bundle"
    echo "Installed runtime: $bundle"
fi

for name in repo-status repo-branch repo-push; do
    case $name in
        repo-status) relative=repo-status/scripts/repo-status.sh ;;
        *) relative="$name/$name.sh" ;;
    esac
    wrapper=$(mktemp "$destination/.repo-tools-command.XXXXXXXX")
    {
        printf '#!/bin/bash\n'
        printf 'exec /bin/bash %q "$@"\n' "$bundle/$relative"
    } > "$wrapper"
    chmod 755 "$wrapper"
    target="$destination/$name"
    if [[ ! -L $target && -x $target ]] && cmp -s "$wrapper" "$target"; then
        echo "Already installed: $target"
    else
        if [[ -e $target || -L $target ]]; then
            backup=$(mktemp "${target}.before-install.XXXXXXXX")
            cp -p -- "$target" "$backup"
            echo "Command backup: $backup"
        fi
        mv -f -- "$wrapper" "$target"
        echo "Installed: $target"
    fi
    rm -f -- "$wrapper"
    wrapper=''
done
