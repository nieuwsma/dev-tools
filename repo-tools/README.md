<!-- Copyright 2026 Nieuwsma Services LLC
     SPDX-License-Identifier: Apache-2.0 -->

# repo-tools

Bash commands for coordinating work across a directory of Git repositories.
They scan immediate child repositories, including hidden directories and linked
worktrees. Ordinary folders, directory symlinks, the current repo itself, and
more deeply nested repos are skipped.

| Command | Purpose |
| --- | --- |
| `repo-status [directory]` | Report branch, cleanliness, upstream, and local divergence. Defaults to the current directory. |
| `repo-branch [--dry-run] NAME` | Create and switch to the same new branch in each child repo. |
| `repo-push [--dry-run]` | Push each current branch and establish missing tracking. |

## Install

Requires Bash 3.2 or newer and Git with `git switch` support. From dev-tools:

```sh
./repo-tools/install.sh
export PATH="$HOME/.local/bin:$PATH"
```

Or copy only `repo-tools/` elsewhere and run `./install.sh` inside it. All three
commands share `lib/repos.sh`, so keep the package together rather than copying
an individual source script. The installer copies the four runtime files to
`~/.local/share/dev-tools/repo-tools` and installs three launchers in
`~/.local/bin`. The checkout is not needed afterward. Existing changed installs
are backed up beside the files they replace; repeating an unchanged installation
does not create more backups. Shell configuration is not modified.

For another bin directory, use `./repo-tools/install.sh --prefix /path/to/bin`.
The shared bundle is installed under the bin directory's sibling `share/dev-tools/`
directory. Add the chosen bin directory to PATH. To update, rerun the installer.
To uninstall, remove the three installed launchers and their shared bundle.
If a shell function already has one of these names, remove that function from
your shell configuration and start a new shell so the installed command is used.

## Use

From a collection directory containing the Git repos you want to operate on:

```sh
repo-status
repo-status /path/to/collection
repo-branch --dry-run feature/next-wave
repo-branch feature/next-wave
# Make and commit changes separately inside the individual repositories.
repo-push --dry-run
repo-push
```

These tools do not create commits. `repo-push` never force-pushes and leaves
uncommitted changes local. Its dry run previews destinations without contacting
remotes. Both mutating tools validate all repositories before starting; if an
operation subsequently fails, earlier successful operations remain in place.
Run `--help` on any command for usage. Source scripts can also be invoked with
Bash directly from the package without installation.

Detailed [branch behavior](repo-branch/README.md) and
[push behavior](repo-push/README.md) describe validation and failure handling.
Status compares locally recorded upstream references and does not fetch. Its
[optional skill instructions](repo-status/SKILL.md) are included as a separate
artifact; installing the commands does not register a skill with any agent.

## Tests

Python 3 tests use temporary repositories and local bare remotes only:

```sh
python3 repo-tools/repo-status/test_repo_status.py
python3 repo-tools/repo-branch/test_repo_branch.py
python3 repo-tools/repo-push/test_repo_push.py
python3 repo-tools/tests/test_install.py
```

No tests run the mutating commands against your real repositories.
