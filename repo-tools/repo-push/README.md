# repo-push

Push the current branch's existing commits in every immediate child Git repo:

```zsh
repo-push --dry-run
repo-push
```

The tool does not create commits or stage, stash, fetch, pull, merge, or force
push. Staged, unstaged, and untracked changes stay local. Only the current branch
is pushed, not other branches, tags, or submodules.

Existing upstreams determine the remote and destination branch, even if the
upstream branch has a different name. If no upstream is configured, the tool
pushes to a branch of the same name and sets tracking on success. It chooses
the remote in this order: `branch.<name>.pushRemote`, `remote.pushDefault`,
`origin`, or the sole configured remote. Ambiguous or missing remotes abort
preflight; configure `git config remote.pushDefault <remote>` in that repo.

Only immediate child directories containing `.git` are considered, including
hidden directories and linked Git worktrees. Ordinary folders, symlinks, the
current directory itself, and deeper repositories are skipped.

All local destinations are validated before the first push. Detached HEADs,
repositories without commits, invalid Git worktrees, and invalid upstreams or
remotes abort preflight without pushing anything. `--dry-run` only previews
destinations: it does not contact remotes or change tracking settings, and cannot
verify authentication, permissions, or whether a remote will accept the push.

Network errors and rejected pushes stop execution. Earlier successful pushes
are retained and their count is reported. A non-fast-forward rejection requires
you to reconcile that repo manually before retrying; the tool never force pushes.

Run `repo-status` afterward to inspect branch tracking and divergence.
