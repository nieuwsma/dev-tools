#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "$script_dir/../../lib/repos.sh"

usage() {
  printf 'Usage: repo-status [directory]\n'
  printf 'Report immediate child Git repositories (default: current directory).\n'
}
if [[ ${1:-} == --help || ${1:-} == -h ]]; then
  usage
  exit 0
fi
if [[ $# -gt 1 ]]; then
  usage >&2
  exit 1
fi
workspace_root=$(cd -- "${1:-$PWD}" && pwd)

repo_tools_discover "$workspace_root"
if [[ ${#repos[@]} == 0 ]]; then
  repo_tools_no_repos "$workspace_root"
  exit 0
fi

records=()
errors=0
for repo in "${repos[@]}"; do
  if ! repo_tools_validate_repo "$repo"; then
    errors=1
    continue
  fi
  name=${repo##*/}

  branch=$(git -C "$repo" branch --show-current)
  if [[ -z "$branch" ]]; then
    branch="(detached HEAD)"
  elif ! git -C "$repo" rev-parse --verify HEAD >/dev/null 2>&1; then
    branch="$branch (no commits)"
  fi

  changed=$(git -C "$repo" status --porcelain | wc -l | tr -d ' ')
  if [[ "$changed" == "0" ]]; then
    status="Clean"
  else
    status="Dirty (${changed} paths)"
  fi

  if upstream=$(git -C "$repo" rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null); then
    counts=$(git -C "$repo" rev-list --left-right --count "$upstream...HEAD")
    read -r behind ahead <<< "$counts"
    if [[ "$ahead" == "0" && "$behind" == "0" ]]; then
      tracking="$upstream (up to date)"
    elif [[ "$ahead" == "0" ]]; then
      tracking="$upstream (behind ${behind})"
    elif [[ "$behind" == "0" ]]; then
      tracking="$upstream (ahead ${ahead})"
    else
      tracking="$upstream (ahead ${ahead}, behind ${behind})"
    fi
  else
    upstream=$(git -C "$repo" for-each-ref --format='%(upstream:short)' "refs/heads/$branch")
    if [[ -n "$upstream" ]]; then
      tracking="$upstream (missing upstream ref)"
    else
      tracking="No upstream"
    fi
  fi

  records+=("$name"$'\t'"$branch"$'\t'"$status"$'\t'"$tracking")
done

[[ ${#records[@]} != 0 ]] || exit "$errors"

printf '%-28s  %-36s  %-18s  %s\n' 'Repository' 'Branch' 'Status' 'Tracked branch'
printf '%-28s  %-36s  %-18s  %s\n' '----------------------------' '------------------------------------' '------------------' '----------------------------------------'
for record in "${records[@]}"; do
  IFS=$'\t' read -r name branch status tracking <<< "$record"
  printf '%-28s  %-36s  %-18s  %s\n' "$name" "$branch" "$status" "$tracking"
done
exit "$errors"
