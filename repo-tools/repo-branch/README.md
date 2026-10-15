<!-- Copyright 2026 Nieuwsma Services LLC
     SPDX-License-Identifier: Apache-2.0 -->

# repo-branch

Create and switch to the same new branch name in every immediate child Git
repository of your current directory:

```zsh
repo-branch feature/next-wave
repo-branch --dry-run feature/next-wave
```

Each branch starts at that repository's current HEAD. Staged, unstaged, and
untracked changes stay in place. No upstream is configured; the tool does not
fetch, push, commit, or stash.

Only immediate child directories containing `.git` are considered, including
hidden directories and linked Git worktrees. Ordinary directories and symlinks
are skipped. The current directory itself and deeper repositories are never
modified.

All repositories are checked before changes begin. An existing branch, missing
initial commit, invalid worktree, unresolved conflict, or unfinished Git
operation aborts the entire preflight without creating branches. If a later
switch fails (for example, due to a lock or hook), execution stops and reports
how many repositories were already switched; successful switches are retained.

Run `repo-branch --help` for usage. Run `repo-status` afterward to see the branches.
