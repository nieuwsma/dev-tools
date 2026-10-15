<!-- Copyright 2026 Nieuwsma Services LLC
     SPDX-License-Identifier: Apache-2.0 -->

# dev: named development workspaces

Give a work session a name and choose its starting directory independently:

```sh
dev component ~/repo/collection
dev component
dev -l
```

The first launch creates a tmux session with a `development` window: Codex on
the left (about 65%) and a shell on the right (about 35%). Both start in your
chosen directory. Reconnecting preserves the layout and processes, including
sessions created manually. When the left command exits, that pane becomes a shell.

## Install just this tool

Copy this entire directory to another Mac, or install from the repo:

```sh
./dev/install.sh
export PATH="$HOME/.local/bin:$PATH"
dev foo ~/path
```

From a copied `dev` directory, run `./install.sh`. The directory passed to `dev`
must already exist. Installation copies one executable into `~/.local/bin`;
the source checkout is not needed afterward. It does not change shell, tmux,
network, or machine configuration. Changed existing executables are backed up
beside the installed file; repeat installations of the same content do nothing.

To choose another executable directory:

```sh
./dev/install.sh --prefix /path/to/bin
```

Put the selected directory in PATH in your own shell configuration. To update,
rerun the installer from a newer copy. To uninstall, remove that installed `dev`
file; sessions already running remain available through tmux.

Requirements: macOS, Bash, tmux (verified with 3.5a), standard command-line tools,
and a shell. New sessions need Codex or your selected left-pane command.
Python 3 is needed only to discover a nested repository by short name. No
Oh My Zsh, personal hostname, Tailscale, or terminal theme is required. Other
operating systems have not been verified.

## Commands

| Command | Behavior |
| --- | --- |
| `dev SESSION DIRECTORY` | Create or reconnect to a named workspace. Directory can be absolute or relative; quote spaces. |
| `dev SESSION` | Reconnect to an existing session with this exact name first. |
| `dev PROJECT` | Otherwise use the matching top-level directory or discover a uniquely named nested Git repo. |
| `dev COLLECTION/PROJECT` | Use an explicit path below the repository root; session name replaces `/` with `--`. |
| `dev -l`, `dev --list` | List existing local sessions, including detached ones. |
| `dev -h`, `dev --help` | Print usage. |

Names accept letters, numbers, hyphens, and underscores. Separate sessions can
start at the same collection root. Reusing a managed session name with a different
starting directory produces an error. Inside tmux, `dev` switches the current
terminal client; use a fresh terminal window for each project to keep them visible
independently.

For example, `dev collection/component` starts inside that repository
with the session name `collection--component`. `dev component` finds
it by short name only if there is no existing session named `component` and no
top-level `component` directory. Ambiguous nested names list explicit paths.
Discovery recognizes `.git` files and directories, skipping hidden and common
dependency/build directories.

The listing shows the session name, attached/detached status, window count, and
recorded starting directory. For sessions created outside dev it falls back to
tmux's session path. This is distinct from each pane's changing working directory.

## Optional defaults

| Environment variable | Default | Purpose |
| --- | --- | --- |
| `DEV_REPO_ROOT` | `~/repo` | Root used by one-argument project lookup. |
| `DEV_COMMAND` | `codex` | Executable to start in the left pane. |
| `DEV_SHELL` | `$SHELL`, then `/bin/sh` | Shell to use on the right and after the left command exits. |

Example for a work checkout layout:

```sh
export DEV_REPO_ROOT="$HOME/work"
dev billing
dev investigation ~/some/other/directory
```

`DEV_COMMAND` and `DEV_SHELL` each name one executable, with no flags or shell
expressions. For arguments or setup steps, point `DEV_COMMAND` to your own
executable wrapper. The caller's PATH takes precedence; missing tools are also
looked up in `~/.local/bin` and common macOS Homebrew locations. Configuration
changes affect new workspaces; reconnecting does not restart processes.

## tmux and remote access

Press Ctrl+B, release, then an arrow to change panes; `o` cycles panes, `z`
zooms, `c` creates an activity window, comma renames it, `s` selects a session,
and `d` detaches. Mouse behavior and colors depend on your tmux configuration.

Run the launcher on the machine holding the files and processes:

```sh
ssh -t HOST '~/.local/bin/dev foo ~/path'
ssh HOST '~/.local/bin/dev -l'
```

Each host has its own sessions. A disconnect leaves processes running while
the host and tmux server remain alive. Reboots end processes; `dev` creates a
new workspace afterward. Saved Codex conversations can be reopened separately.

## Verification

Run `python3 dev/tests/smoke.py` from the repo. The test installs a detached
copy of this tool and uses a separate tmux socket and harmless stub commands;
it does not attach to or modify personal sessions or shell configuration.
