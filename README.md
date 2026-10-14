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
| [terminal](terminal/README.md) | Optional managed blocks in `.zshrc` and `.tmux.conf`, with backups. |

Terminal appearance is optional and independent of dev. It accepts an external
JSON color profile; personal machine profiles and full shell snapshots should
live outside this repo.

Supported target: macOS. Other platforms have not been verified. Each package
lists its own requirements, installation, update, and removal instructions.

## Everyday reference

[Command cheat sheet](docs/command-cheatsheet.md): launch, reconnect, scrolling,
mouse support, copying text, and remote terminals.

## Verify

With tmux, Python 3, and zsh available:

```sh
python3 dev/tests/smoke.py
python3 terminal/tests/smoke.py
```

The checks use temporary installations, separate tmux sockets, generated color
profiles, and harmless command stubs. They do not attach to existing sessions
or change your live shell/terminal configuration.
