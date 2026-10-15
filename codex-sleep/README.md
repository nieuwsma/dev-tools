<!-- Copyright 2026 Nieuwsma Services LLC
     SPDX-License-Identifier: Apache-2.0 -->

# codex-sleep: resume a Codex session at a clock time

Wait until a usage window resets, then resume a named Codex session in the current directory:

```sh
codex-sleep translation-studio 1:49am
codex-sleep translation-studio 1:49am --caffeinate
codex-sleep --dry-run translation-studio 01:49
```

The command sleeps until that local time plus one minute. A time already passed today waits until tomorrow. It then runs `codex exec resume` with the existing task-continuation prompt. Accepted times are `1:49am`, `1:49pm`, and `01:49`.

`--caffeinate` is optional. It keeps the Mac awake during the wait and the Codex run. Inside tmux, the command already keeps running if the terminal disconnects, so leave the flag off when the Mac is set to stay awake.

## Install just this tool

Copy this entire directory to another Mac, or install from the repo:

```sh
./codex-sleep/install.sh
export PATH="$HOME/.local/bin:$PATH"
codex-sleep --help
```

From a copied `codex-sleep` directory, run `./install.sh`. Installation copies one executable into `~/.local/bin`. The source checkout is not needed afterward. It does not change shell, tmux, network, or machine configuration. Changed existing executables are backed up beside the installed file. Repeat installations of the same content do nothing.

To choose another executable directory:

```sh
./codex-sleep/install.sh --prefix /path/to/bin
```

Put the selected directory in PATH in your own shell configuration. To update, rerun the installer from a newer copy. To uninstall, remove that installed `codex-sleep` file.

Requirements: macOS, Bash, and Codex when a real resume runs. `--dry-run` does not need Codex. Other operating systems have not been verified.

## Commands

| Command | Behavior |
| --- | --- |
| `codex-sleep SESSION TIME` | Sleep, then resume that Codex session here. |
| `codex-sleep SESSION TIME --caffeinate` | Same, while holding the Mac awake. |
| `codex-sleep --dry-run SESSION TIME` | Print the wait and command. Do not sleep or start Codex. |
| `codex-sleep -h`, `codex-sleep --help` | Print usage. |

Run it from the repository Codex should continue. The session argument is a Codex session id or thread name.

## Verification

Run `python3 codex-sleep/tests/smoke.py` from the repo. The test installs a detached copy and checks time math with `--dry-run`. It does not sleep, call Codex, or change your shell configuration.
