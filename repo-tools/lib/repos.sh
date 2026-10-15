# Copyright 2026 Nieuwsma Services LLC
# SPDX-License-Identifier: Apache-2.0

# Shared helpers compatible with macOS's Bash 3.2.

# Populate the caller's repos array with immediate child repositories only.
# A .git file is valid for a linked worktree. Do not follow directory symlinks.
repo_tools_discover() {
  local root=$1 directory
  local had_nullglob=false had_dotglob=false
  shopt -q nullglob && had_nullglob=true
  shopt -q dotglob && had_dotglob=true
  shopt -s nullglob dotglob
  repos=()
  for directory in "$root"/*; do
    [[ -d "$directory" && ! -L "$directory" && -e "$directory/.git" ]] || continue
    repos+=("$directory")
  done
  "$had_nullglob" || shopt -u nullglob
  "$had_dotglob" || shopt -u dotglob
}

repo_tools_validate_repo() {
  local repo=$1 inside prefix
  inside=$(git -C "$repo" rev-parse --is-inside-work-tree 2>/dev/null) &&
    prefix=$(git -C "$repo" rev-parse --show-prefix 2>/dev/null) &&
    [[ "$inside" == true && -z "$prefix" ]] && return 0
  printf '%s: invalid Git worktree\n' "${repo##*/}" >&2
  return 1
}

repo_tools_no_repos() {
  printf 'No immediate child Git repositories found under %s\n' "$1"
}
