#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "$script_dir/../lib/repos.sh"

usage() {
  printf 'Usage: repo-branch [--dry-run] <new-branch-name>\n'
  printf 'Create and switch branches in immediate child Git repositories of $PWD.\n'
}

dry_run=false
if [[ ${1:-} == --help || ${1:-} == -h ]]; then
  usage
  exit 0
fi
if [[ ${1:-} == --dry-run ]]; then
  dry_run=true
  shift
fi
if [[ $# != 1 ]]; then
  usage >&2
  exit 1
fi
branch=$1
if [[ "$branch" == -* || "$branch" == HEAD ]] || ! git check-ref-format "refs/heads/$branch" >/dev/null 2>&1; then
  printf 'Invalid branch name: %s\n' "$branch" >&2
  exit 1
fi

repo_tools_discover "$PWD"
if [[ ${#repos[@]} == 0 ]]; then
  repo_tools_no_repos "$PWD"
  exit 0
fi

# Validate every repository before creating any branches. Local changes are
# preserved because each new branch starts at that repository's current HEAD.
errors=0
for repo in "${repos[@]}"; do
  name=${repo##*/}
  if ! repo_tools_validate_repo "$repo"; then
    errors=1
    continue
  fi
  if ! git -C "$repo" rev-parse --verify HEAD >/dev/null 2>&1; then
    printf '%s: no current commit to branch from\n' "$name" >&2
    errors=1
  fi
  if git -C "$repo" show-ref --verify --quiet "refs/heads/$branch"; then
    printf '%s: branch already exists: %s\n' "$name" "$branch" >&2
    errors=1
  fi
  for operation in MERGE_HEAD CHERRY_PICK_HEAD REVERT_HEAD rebase-merge rebase-apply sequencer BISECT_START; do
    operation_path=$(git -C "$repo" rev-parse --git-path "$operation")
    [[ "$operation_path" == /* ]] || operation_path="$repo/$operation_path"
    if [[ -e "$operation_path" ]]; then
      printf '%s: unfinished Git operation (%s)\n' "$name" "$operation" >&2
      errors=1
    fi
  done
  if [[ -n $(git -C "$repo" ls-files --unmerged) ]]; then
    printf '%s: unresolved conflicts\n' "$name" >&2
    errors=1
  fi
done
if [[ "$errors" != 0 ]]; then
  printf 'No branches created. Resolve the errors above and retry.\n' >&2
  exit 1
fi

completed=0
for repo in "${repos[@]}"; do
  name=${repo##*/}
  if "$dry_run"; then
    printf '%s: would create and switch to %s\n' "$name" "$branch"
    continue
  fi
  if git -C "$repo" switch --quiet --no-track -c "$branch"; then
    printf '%s: created and switched to %s\n' "$name" "$branch"
    completed=$((completed + 1))
  else
    printf '%s: failed; stopping. %s repositories already switched.\n' "$name" "$completed" >&2
    exit 1
  fi
done
