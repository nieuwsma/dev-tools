#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "$script_dir/../lib/repos.sh"

usage() {
  printf 'Usage: repo-push [--dry-run]\n'
  printf 'Push the current branch in immediate child Git repositories of $PWD.\n'
  printf '%s\n' '--dry-run previews destinations without contacting remotes.'
}

choose_push_remote() {
  local repo=$1 branch=$2 remote candidate
  local available=()
  remote=$(git -C "$repo" config --get "branch.$branch.pushRemote" || true)
  if [[ -z "$remote" ]]; then
    remote=$(git -C "$repo" config --get remote.pushDefault || true)
  fi
  if [[ -n "$remote" ]]; then
    printf '%s\n' "$remote"
    return 0
  fi
  while IFS= read -r candidate; do
    [[ -z "$candidate" ]] && continue
    if [[ "$candidate" == origin ]]; then
      printf 'origin\n'
      return 0
    fi
    available+=("$candidate")
  done < <(git -C "$repo" remote)
  if [[ ${#available[@]} == 1 ]]; then
    printf '%s\n' "${available[0]}"
    return 0
  fi
  printf '%s: no unambiguous push remote; configure origin or remote.pushDefault\n' "${repo##*/}" >&2
  return 1
}

# Emit remote, destination ref, and whether to establish tracking. Existing
# upstreams take precedence, including upstream branches with different names.
resolve_push_destination() {
  local repo=$1 branch=$2 remote target set_upstream=false
  remote=$(git -C "$repo" config --get "branch.$branch.remote" || true)
  target=$(git -C "$repo" config --get "branch.$branch.merge" || true)
  if [[ -z "$remote" || -z "$target" ]]; then
    set_upstream=true
    target="refs/heads/$branch"
    remote=$(choose_push_remote "$repo" "$branch") || return 1
  fi
  if [[ "$remote" == . ]] || ! git -C "$repo" remote get-url --push "$remote" >/dev/null 2>&1; then
    printf '%s: upstream/push remote is not a configured external remote: %s\n' "${repo##*/}" "$remote" >&2
    return 1
  fi
  if [[ "$target" != refs/heads/* ]] || ! git check-ref-format "$target" >/dev/null 2>&1; then
    printf '%s: invalid upstream branch: %s\n' "${repo##*/}" "$target" >&2
    return 1
  fi
  printf '%s\t%s\t%s\n' "$remote" "$target" "$set_upstream"
}

# Append one validated entry to the push plan. No network operations occur here.
prepare_push() {
  local repo=$1 branch destination remote target set_upstream
  repo_tools_validate_repo "$repo" || return 1
  branch=$(git -C "$repo" symbolic-ref --quiet --short HEAD || true)
  if [[ -z "$branch" ]]; then
    printf '%s: detached HEAD; switch to a branch first\n' "${repo##*/}" >&2
    return 1
  fi
  if ! git -C "$repo" rev-parse --verify "refs/heads/$branch^{commit}" >/dev/null 2>&1; then
    printf '%s: no commits to push\n' "${repo##*/}" >&2
    return 1
  fi
  destination=$(resolve_push_destination "$repo" "$branch") || return 1
  IFS=$'\t' read -r remote target set_upstream <<< "$destination"
  branches+=("$branch")
  remotes+=("$remote")
  targets+=("$target")
  set_upstreams+=("$set_upstream")
}

push_repo() {
  local repo=$1 branch=$2 remote=$3 target=$4 set_upstream=$5
  local note=''
  local options=(--no-force)
  if "$set_upstream"; then
    options+=(--set-upstream)
    note=' (set upstream)'
  fi
  if "$dry_run"; then
    printf '%s: would push %s -> %s/%s%s\n' "${repo##*/}" "$branch" "$remote" "${target#refs/heads/}" "$note"
    return 0
  fi
  printf '%s: pushing %s -> %s/%s%s\n' "${repo##*/}" "$branch" "$remote" "${target#refs/heads/}" "$note"
  # Push exactly this branch. Override settings that could push unrelated refs.
  git -C "$repo" -c push.followTags=false -c push.recurseSubmodules=no \
    -c "remote.$remote.mirror=false" push "${options[@]}" -- "$remote" "refs/heads/$branch:$target"
}

dry_run=false
case ${1:-} in
  --help|-h) usage; exit 0 ;;
  --dry-run) dry_run=true; shift ;;
esac
if [[ $# != 0 ]]; then
  usage >&2
  exit 1
fi

repo_tools_discover "$PWD"
if [[ ${#repos[@]} == 0 ]]; then
  repo_tools_no_repos "$PWD"
  exit 0
fi

branches=()
remotes=()
targets=()
set_upstreams=()
errors=0
for repo in "${repos[@]}"; do
  prepare_push "$repo" || errors=1
done
if [[ "$errors" != 0 ]]; then
  printf 'Nothing pushed. Resolve the errors above and retry.\n' >&2
  exit 1
fi

completed=0
for ((i=0; i<${#repos[@]}; i++)); do
  if push_repo "${repos[$i]}" "${branches[$i]}" "${remotes[$i]}" "${targets[$i]}" "${set_upstreams[$i]}"; then
    completed=$((completed + 1))
  else
    printf '%s: push failed; stopping. %s repositories already processed successfully.\n' "${repos[$i]##*/}" "$completed" >&2
    exit 1
  fi
done
