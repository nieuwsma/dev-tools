<!-- Copyright 2026 Nieuwsma Services LLC
     SPDX-License-Identifier: Apache-2.0 -->

# Development tools

Small, independently installable tools for terminal-based development. Install
only the package you want; each package works without the rest of this checkout.

## Start a named development workspace

```sh
./dev/install.sh
export PATH="$HOME/.local/bin:$PATH"
dev foo ~/path
dev foo
dev -l
```

The directory must exist. A new tmux workspace starts Codex on the left and a
shell on the right. Reconnect by name, or list existing sessions with `dev -l`.
Installation copies one executable into `~/.local/bin`; you can remove the
checkout afterward. Requirements and optional defaults: [dev/README.md](dev/README.md).

To take just this tool elsewhere, copy `dev/` and run its `./install.sh`.

## Packages

| Package | Installation changes |
| --- | --- |
| [dev](dev/README.md) | One executable in your selected bin directory. |
| [repo-tools](repo-tools/README.md) | Three commands in your bin directory plus a shared runtime under its sibling `share/dev-tools/` directory. |
| [codex-sleep](codex-sleep/README.md) | One executable in your selected bin directory. |
| [kat](kat/README.md) | One executable in your selected bin directory. The installer builds it with Go. |
| [terminal](terminal/README.md) | Optional managed blocks in `.zshrc` and `.tmux.conf`, with backups. |

Terminal appearance is optional and independent of dev. It accepts an external
JSON color profile; personal machine profiles and full shell snapshots should
live outside this repo.

Supported target: macOS. Other platforms have not been verified. Each package
lists its own requirements, installation, update, and removal instructions.

## Coordinate a collection of repositories

```sh
./repo-tools/install.sh
cd /path/to/collection
repo-status
repo-branch --dry-run feature/example
repo-push --dry-run
```

The package includes `repo-status`, `repo-branch`, and `repo-push`. It scans only
immediate child repositories. Branch creation and pushing are explicit operations;
read [repo-tools/README.md](repo-tools/README.md) for behavior and validation.

## Resume Codex when a usage window resets

```sh
./codex-sleep/install.sh
codex-sleep translation-studio 1:49am
codex-sleep translation-studio 1:49am --caffeinate
```

Run it from the repository Codex should continue. It sleeps until that local
time plus one minute, then resumes the named session. Add `--caffeinate` only
when this Mac might sleep. Details: [codex-sleep/README.md](codex-sleep/README.md).

## See which local Kind apps are expensive

```sh
./kat/install.sh
kat
kat --json --filter rack
```

`kat` needs Go to build and `kubectl` to run. It reads every `kind-*` context.
Details: [kat/README.md](kat/README.md).

## Everyday reference

[Command cheat sheet](docs/command-cheatsheet.md): launch, reconnect, scrolling,
mouse support, copying text, and remote terminals.

## Verify

With tmux, Python 3, and zsh available:

```sh
python3 dev/tests/smoke.py
python3 codex-sleep/tests/smoke.py
go test ./kat/
python3 terminal/tests/smoke.py
python3 repo-tools/repo-status/test_repo_status.py
python3 repo-tools/repo-branch/test_repo_branch.py
python3 repo-tools/repo-push/test_repo_push.py
python3 repo-tools/tests/test_install.py
```

The checks use temporary installations, separate tmux sockets, generated color
profiles, harmless command stubs, and local-only bare Git remotes. They do not attach to existing sessions
or change your live shell/terminal configuration.

## License

Copyright 2026 Nieuwsma Services LLC.

Licensed under the [Apache License, Version 2.0](LICENSE). See [NOTICE](NOTICE).
