# Everyday terminal commands

A running reference for the tools and terminal techniques in this repo.

## Work sessions

```sh
dev task ~/path          # Name the work independently from its starting directory
dev task                # Reconnect to an existing session
dev -l                  # List sessions on this machine
dev --help              # Show usage
```

New sessions start Codex (or `DEV_COMMAND`) beside a shell. Use a separate
terminal window per work session. Full behavior and optional settings:
[dev](../dev/README.md).

To resume a named Codex session when a usage window resets, from that repo:

```sh
codex-sleep translation-studio 1:49am
codex-sleep translation-studio 1:49am --caffeinate
```

The second form holds the Mac awake. Leave it off inside tmux when the Mac
already stays awake. Full behavior: [codex-sleep](../codex-sleep/README.md).

## tmux navigation

Press Ctrl+B, release, then press the second key:

| Key | Action |
| --- | --- |
| Arrow / `o` | Move between panes / cycle panes. |
| `z` | Zoom/unzoom the active pane. |
| `c` | Create an activity window. |
| `,` | Rename the current window. |
| `n` / `p` | Next/previous window. |
| `w` / `s` | Choose a window / session. |
| `%` / `"` | Split left/right / top/bottom. |
| `d` | Detach, leaving processes running. |
| `[` | Enter tmux scrollback. |

In scrollback, use arrows or Page Up/Down (Fn+Up/Down on a Mac). Press Esc to
return to the application. To enable mouse support immediately, press Ctrl+B,
then `:`, enter `set -g mouse on`, and press Enter. Put `set -g mouse on` in
`~/.tmux.conf` to make it persistent, or install the optional terminal package.

## Copy text in iTerm

Hold **Option** while dragging to select text, then press **Cmd+C**. Paste with
**Cmd+V**. Option bypasses application mouse reporting. For wrapped text or
adjacent panes, zoom the pane first with Ctrl+B, then `z`.

To copy text from a shell on your local Mac:

```sh
printf '%s' 'path/to/file.md' | pbcopy
```

`pbcopy` affects the clipboard of the Mac running it. For remote file contents,
run this on your local Mac, replacing HOST and the path:

```sh
ssh HOST 'cat ~/path/to/file.md' | pbcopy
```

## Remote work

```sh
ssh -t HOST '~/.local/bin/dev task ~/path'
ssh HOST '~/.local/bin/dev -l'
```

Each host has its own sessions and files. `-t` provides a terminal when attaching.

## Check context and reload

```sh
hostname -s
pwd
git status
git diff
source ~/.zshrc
tmux source-file ~/.tmux.conf
```

[Optional terminal setup](../terminal/README.md) adds a hostname prompt, session
labels, current-directory display, mouse support, and window titles. Personal
network details and host profiles are intentionally external to these tools.


## Collections of Git repositories

Install the [repo-tools package](../repo-tools/README.md), then run from a
collection directory:

```sh
repo-status
repo-status /path/to/collection
repo-branch --dry-run feature/example
repo-branch feature/example
# Commit changes separately inside the repositories when ready.
repo-push --dry-run
repo-push
```

Only immediate child Git repositories are included. `repo-status` does not fetch;
its divergence is based on local upstream refs. The dry runs make no branch or
remote changes. A real `repo-push` pushes existing commits; it does not commit
uncommitted work or force-push.
