---
name: repo-status
description: Report the current branch, working-tree cleanliness, tracked upstream, and ahead/behind divergence for every Git repository directly under this workspace. Use when the user asks where the workspace repositories currently stand; do not use it to modify repositories.
---

# Repo Status

Use this skill for a concise, read-only snapshot of the repositories directly
under the workspace root.

Run [scripts/repo-status.sh](scripts/repo-status.sh). It discovers only
immediate child Git repositories and reports:

- repository name;
- checked-out branch (or detached HEAD);
- working-tree status, including a count of changed paths when dirty;
- configured tracked branch and its local ahead/behind count.

The script deliberately does not fetch remotes. Its divergence figures compare
against the locally recorded upstream reference. If freshness relative to the
remote matters, explain that a `git fetch` is needed and obtain authorization
before doing it.

Report the command output directly, calling out repositories with no upstream,
a detached HEAD, local changes, or nonzero divergence. Do not stage, commit,
reset, clean, switch branches, push, or otherwise mutate a repository as part
of this status request.
