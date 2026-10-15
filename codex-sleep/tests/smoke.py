#!/usr/bin/env python3
# Copyright 2026 Nieuwsma Services LLC
# SPDX-License-Identifier: Apache-2.0

"""Install and time-math checks. No sleep, Codex, or live shell changes."""
from pathlib import Path
import hashlib
import os
import shutil
import subprocess
import tempfile


def epoch(stamp):
    output = subprocess.check_output(
        ["date", "-j", "-f", "%Y-%m-%d %H:%M:%S", stamp, "+%s"], text=True
    )
    return int(output.strip())


def dry_run(command, *args):
    result = subprocess.run([command, "--dry-run", *args], capture_output=True, text=True)
    return result


def run():
    source = Path(__file__).resolve().parents[1]
    protected = [Path.home() / name for name in (".zshrc", ".tmux.conf")]
    before = {p: hashlib.sha256(p.read_bytes()).digest() for p in protected if p.exists()}
    with tempfile.TemporaryDirectory(prefix="codex-sleep-smoke-") as staging:
        stage = Path(staging).resolve()
        copied = stage / "copied-tool"
        shutil.copytree(source, copied)
        target = stage / "installed"
        install = [str(copied / "install.sh"), "--prefix", str(target)]
        subprocess.run(install, cwd="/", check=True)
        subprocess.run(install, cwd="/", check=True)
        assert not list(target.glob("codex-sleep.before-install.*"))
        installed = target / "codex-sleep"
        installed.write_text("old version\n")
        subprocess.run(install, cwd="/", check=True)
        assert len(list(target.glob("codex-sleep.before-install.*"))) == 1
        installed.chmod(0o644)
        subprocess.run(install, cwd="/", check=True, stdout=subprocess.DEVNULL)
        assert os.access(installed, os.X_OK)
        installed.unlink()
        installed.symlink_to(copied / "codex-sleep")
        subprocess.run(install, cwd="/", check=True, stdout=subprocess.DEVNULL)
        assert not installed.is_symlink()
        shutil.rmtree(copied)

        help_text = subprocess.check_output([str(installed), "--help"], text=True)
        assert "Usage:" in help_text
        assert "--caffeinate" in help_text

        invalid = subprocess.run([str(installed), "session", "25:99"], capture_output=True, text=True)
        assert invalid.returncode == 1
        assert "Invalid time" in invalid.stderr

        now = epoch("2026-10-14 12:00:00")
        afternoon = dry_run(str(installed), "translation-studio", "1:49pm", "--now", str(now))
        assert afternoon.returncode == 0, afternoon.stderr
        fields = dict(line.split(": ", 1) for line in afternoon.stdout.splitlines() if ": " in line)
        assert fields["session"] == "translation-studio"
        assert int(fields["resume_at"]) == epoch("2026-10-14 13:50:00")
        assert fields["wait_seconds"] == "6600"
        assert fields["caffeinate"] == "no"
        assert fields["command"].startswith("codex ")
        assert "caffeinate" not in fields["command"]

        morning = dry_run(
            str(installed), "--caffeinate", "translation-studio", "1:49am", "--now", str(now)
        )
        assert morning.returncode == 0, morning.stderr
        fields = dict(line.split(": ", 1) for line in morning.stdout.splitlines() if ": " in line)
        assert int(fields["resume_at"]) == epoch("2026-10-15 01:50:00")
        assert fields["wait_seconds"] == "49800"
        assert fields["caffeinate"] == "yes"
        assert fields["command"].startswith("caffeinate -i codex ")

        pinned = subprocess.run(
            [str(installed), "translation-studio", "1:49am", "--now", str(now)],
            capture_output=True, text=True,
        )
        assert pinned.returncode == 1
        assert "--now is only valid with --dry-run" in pinned.stderr
    assert all(hashlib.sha256(p.read_bytes()).digest() == digest for p, digest in before.items())
    print("codex-sleep smoke: ok")


if __name__ == "__main__":
    run()
