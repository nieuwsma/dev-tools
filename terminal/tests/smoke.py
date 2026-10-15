#!/usr/bin/env python3
# Copyright 2026 Nieuwsma Services LLC
# SPDX-License-Identifier: Apache-2.0

"""Generate independent terminal configs and load them in disposable tmux servers."""
from pathlib import Path
import json
import os
import shutil
import subprocess
import sys
import tempfile


def run():
    source = Path(__file__).resolve().parents[1]
    tmux = shutil.which("tmux")
    assert tmux
    with tempfile.TemporaryDirectory(prefix="terminal-smoke-") as staging:
        stage = Path(staging).resolve()
        copied = stage / "standalone"
        shutil.copytree(source, copied)
        for profile, expected in ((None, "fg=colour231,bg=colour238"),
                                  ("green", "fg=black,bg=green"),
                                  ("red", "fg=colour231,bg=colour88")):
            destination = stage / (profile or "unrelated-work-host")
            destination.mkdir()
            (destination / ".zshrc").write_text("# Existing shell settings\n")
            (destination / ".tmux.conf").write_text("# Existing tmux settings\n")
            command = [sys.executable, str(copied / "install.py"), "--home", str(destination)]
            if profile:
                colors = {
                    "green": {"prompt_color": "green", "status_fg": "black", "status_bg": "green", "active_fg": "green", "active_bg": "black"},
                    "red": {"prompt_color": "red", "status_fg": "colour231", "status_bg": "colour88", "active_fg": "colour231", "active_bg": "colour52"},
                }
                profile_path = stage / f"{profile}.json"
                profile_path.write_text(json.dumps(colors[profile]))
                command += ["--profile", str(profile_path)]
            subprocess.run(command, check=True, stdout=subprocess.DEVNULL)
            first = {p: p.read_bytes() for p in destination.glob(".*") if p.is_file()}
            subprocess.run(command, check=True, stdout=subprocess.DEVNULL)
            assert first == {p: p.read_bytes() for p in destination.glob(".*") if p.is_file()}
            assert not (destination / ".local/bin/dev").exists()
            env = dict(os.environ, TERM="xterm-256color")
            env.pop("TMUX", None)
            subprocess.run(["zsh", "-f", "-c", '''
PROMPT='original theme '
source "$1"
first=$PROMPT
source "$1"
[[ $first == $PROMPT && $PROMPT == *'original theme ' ]] || exit 1
machine_identity_title
''', "check", str(destination / ".zshrc")], env=env, check=True, stdout=subprocess.DEVNULL)
            client = [tmux, "-S", str(stage / "test.sock")]
            try:
                subprocess.run(client + ["-f", str(destination / ".tmux.conf"), "new-session", "-d",
                                         "-s", "check", "/bin/sleep 120"], check=True)
                actual = subprocess.check_output(client + ["show-options", "-gv", "status-style"], text=True)
                assert actual.strip() == expected, actual
            finally:
                subprocess.run(client + ["kill-server"], check=False, capture_output=True)
        malformed = stage / "malformed"
        malformed.mkdir()
        (malformed / ".zshrc").write_text("# unchanged\n")
        (malformed / ".tmux.conf").write_text("# <<< dev-tools terminal identity <<<\n")
        result = subprocess.run([sys.executable, str(copied / "install.py"), "--home", str(malformed)],
                                capture_output=True)
        assert result.returncode and (malformed / ".zshrc").read_text() == "# unchanged\n"
    print("PASS: standalone generic install, both profiles, idempotence, zsh titles without Oh My Zsh, "
          "tmux parsing, and malformed-block protection")


if __name__ == "__main__":
    run()
