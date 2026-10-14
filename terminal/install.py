#!/usr/bin/env python3
"""Install optional zsh/tmux styling. Does not install dev or require a known host."""

import argparse
from datetime import datetime
import json
from pathlib import Path
import re
import shutil

BEGIN = "# >>> dev-tools terminal identity >>>"
END = "# <<< dev-tools terminal identity <<<"
DEFAULTS = {"prompt_color": "yellow", "status_fg": "colour231", "status_bg": "colour238",
            "active_fg": "colour231", "active_bg": "colour24"}


def prepare(path, content):
    original = path.read_text() if path.exists() else ""
    if original.count(BEGIN) != original.count(END) or original.count(BEGIN) > 1:
        raise ValueError(f"Malformed managed block in {path}")
    block = BEGIN + "\n" + content.rstrip() + "\n" + END + "\n"
    if BEGIN in original:
        start = original.index(BEGIN)
        finish = original.index(END) + len(END)
        if finish < start:
            raise ValueError(f"Malformed managed block in {path}")
        if original[finish:finish + 1] == "\n":
            finish += 1
        updated = original[:start] + block + original[finish:]
    else:
        updated = original.rstrip("\n") + "\n\n" + block
    return original, updated


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--home", type=Path, default=Path.home(), help="configuration destination")
    parser.add_argument("--profile", type=Path, help="optional JSON color profile")
    args = parser.parse_args()
    palette = DEFAULTS.copy()
    try:
        if args.profile:
            supplied = json.loads(args.profile.read_text())
            if not isinstance(supplied, dict) or set(supplied) - set(DEFAULTS):
                raise ValueError("Profile must contain only the documented color fields")
            palette.update(supplied)
        for key, color in palette.items():
            if not isinstance(color, str) or not re.fullmatch(r"[A-Za-z][A-Za-z0-9]*|#[0-9a-fA-F]{6}", color):
                raise ValueError(f"Invalid color for {key}")
        here = Path(__file__).resolve().parent
        prompt = (here / "prompt.zsh").read_text().replace("@PROMPT_COLOR@", palette["prompt_color"])
        tmux = (here / "tmux.conf").read_text()
        fields = {"@MACHINE_COLOR@": "status_bg", "@MACHINE_FOREGROUND@": "status_fg",
                  "@ACTIVE_FOREGROUND@": "active_fg", "@ACTIVE_BACKGROUND@": "active_bg"}
        for token, key in fields.items():
            tmux = tmux.replace(token, palette[key])
        # Validate both existing blocks before modifying either file.
        changes = [(args.home / name, prepare(args.home / name, content))
                   for name, content in ((".zshrc", prompt), (".tmux.conf", tmux))]
    except (OSError, ValueError) as error:
        parser.error(str(error))
    args.home.mkdir(parents=True, exist_ok=True)
    for path, (original, updated) in changes:
        if original == updated:
            print(f"Already configured: {path}")
            continue
        if path.exists():
            stamp = datetime.now().strftime("%Y%m%d-%H%M%S-%f")
            backup = path.with_name(f"{path.name}.before-terminal-identity-{stamp}")
            shutil.copy2(path, backup)
            print(f"Backup: {backup}")
        path.write_text(updated)
        print(f"Updated: {path}")


if __name__ == "__main__":
    main()
