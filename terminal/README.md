# Optional terminal identity

Add a hostname to an existing zsh prompt, a host/session label and active-pane
directory to tmux, and a terminal window title. This package is independent of
dev. It does not install a launcher or replace your whole shell configuration.

## Install on any Mac

Copy this directory or install from a checkout:

```sh
python3 terminal/install.py
source ~/.zshrc
# Only when a tmux server is already running:
tmux source-file ~/.tmux.conf
```

Requirements: Python 3, zsh, and tmux (verified with 3.5a). Oh My Zsh is optional;
an existing prompt's directory and Git information is preserved. Title handling
works with zsh directly. A generic installation uses a gray bar with white text,
a blue active-window background, and a yellow hostname. No hostname is rejected.

Installation adds/replaces one managed block in `.zshrc` and `.tmux.conf`, backing
up changed files alongside their originals. Reinstallation is safe; sourcing
the prompt repeatedly does not duplicate the hostname. Existing window and pane
numbers are not retroactively changed. Automatic activity-window renaming is
disabled; rename a window with Ctrl+B, then comma.

## Optional color profiles

Pass an external JSON file to customize the colors:

```sh
python3 terminal/install.py --profile /path/to/profile.json
```

Example profile:

```json
{
  "prompt_color": "green",
  "status_fg": "black",
  "status_bg": "green",
  "active_fg": "green",
  "active_bg": "black"
}
```

Profiles accept `prompt_color` (a zsh color), `status_fg`, `status_bg`, `active_fg`,
and `active_bg` (tmux colors). Missing fields use generic defaults. Examples of
colors: `green`, `colour231`, `colour88`, and `#870000`.
Use `--home DIRECTORY` to generate configurations in a staging directory.
Profiles can live outside this checkout; no machine inventory or network
configuration is required.

The source `prompt.zsh` and `tmux.conf` are templates: use the installer to replace
color placeholders rather than sourcing/copying the templates directly.

## Use and recover

The tmux bar identifies the host and work session; `cwd` shows the active pane's
current directory. If you SSH inside a local tmux pane, the remote prompt names
the remote Mac while the surrounding bar still identifies the local tmux host.
The terminal receives the requested host/session title; its preferences control
how that title is displayed.

Mouse support allows clicking panes and resizing. In iTerm, hold Option while
dragging to select text, then Cmd+C. For keyboard navigation, press Ctrl+B,
release it, then an arrow. The literal `>` key opens a menu.

To undo installation, restore the generated backup files and reload the settings.
Alternatively remove the block between the `dev-tools terminal identity`
markers from each config, then open a fresh shell and reload tmux. Keep any personal configuration snapshots separately from this tools repository.

## Verification

From a checkout, run `python3 terminal/tests/smoke.py`. It creates generic
sample profiles in a temporary directory, checks zsh startup without Oh My Zsh,
and loads the generated configs into separate tmux servers. No personal profile
files or live configuration changes are required.
