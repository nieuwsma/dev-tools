#!/usr/bin/env python3
# Copyright 2026 Nieuwsma Services LLC
# SPDX-License-Identifier: Apache-2.0

"""Integration checks against a disposable tmux server; no real agent calls."""
from pathlib import Path
import hashlib
import os
import shlex
import shutil
import subprocess
import tempfile
import time


def run():
    source = Path(__file__).resolve().parents[1]
    tmux = shutil.which("tmux")
    assert tmux, "tmux is required for the integration checks"
    protected = [Path.home() / name for name in (".zshrc", ".tmux.conf")]
    before = {p: hashlib.sha256(p.read_bytes()).digest() for p in protected if p.exists()}
    with tempfile.TemporaryDirectory(prefix="dev-smoke-") as staging:
        stage = Path(staging).resolve()
        copied = stage / "copied-tool"
        shutil.copytree(source, copied)
        target = stage / "installed"
        command = [str(copied / "install.sh"), "--prefix", str(target)]
        subprocess.run(command, cwd="/", check=True)
        subprocess.run(command, cwd="/", check=True)
        assert not list(target.glob("dev.before-install.*"))
        (target / "dev").write_text("old version\n")
        subprocess.run(command, cwd="/", check=True)
        assert len(list(target.glob("dev.before-install.*"))) == 1
        # Equal content must still repair permissions and detach checkout symlinks.
        (target / "dev").chmod(0o644)
        subprocess.run(command, cwd="/", check=True, stdout=subprocess.DEVNULL)
        assert os.access(target / "dev", os.X_OK)
        (target / "dev").unlink()
        (target / "dev").symlink_to(copied / "dev")
        subprocess.run(command, cwd="/", check=True, stdout=subprocess.DEVNULL)
        assert not (target / "dev").is_symlink()
        shutil.rmtree(copied)
        launcher = target / "dev"
        helpers = stage / "helpers"
        helpers.mkdir()
        sock = stage / "tmux.sock"
        wrapper = helpers / "tmux"
        wrapper.write_text(
            '#!/bin/bash\nif [[ $1 == attach-session || $1 == switch-client ]]; then\n'
            ' printf "%s\\n" "$*" >> "$DEV_TEST_CONNECTIONS"\n exit 0\nfi\n'
            f'exec {shlex.quote(tmux)} -S {shlex.quote(str(sock))} -f /dev/null "$@"\n'
        )
        agent = helpers / "agent with spaces"
        agent.write_text('#!/bin/sh\nprintf "%s\\n" "$PWD" >> "$DEV_TEST_LOG"\n')
        shell = helpers / "test-shell"
        shell.write_text('#!/bin/sh\nexec /bin/sleep 120\n')
        for executable in (wrapper, agent, shell):
            executable.chmod(0o755)
        repo_root = stage / "repositories"
        for name in ("collection/component", "a/ambiguous", "b/ambiguous"):
            (repo_root / name / ".git").mkdir(parents=True)
        outside = stage / "outside directory"
        outside.mkdir()
        env = dict(os.environ, PATH=str(helpers) + os.pathsep + os.environ["PATH"],
                   DEV_REPO_ROOT=str(repo_root), DEV_COMMAND=str(agent), DEV_SHELL=str(shell),
                   DEV_TEST_LOG=str(stage / "agent.log"), DEV_TEST_CONNECTIONS=str(stage / "connections"))
        env.pop("TMUX", None)
        client = [tmux, "-S", str(sock)]

        def launch(*args, ok=True, extra=None):
            result = subprocess.run([str(launcher), *args], env=env | (extra or {}),
                                    capture_output=True, text=True)
            assert (result.returncode == 0) == ok, result.stderr
            return result.stdout + result.stderr

        try:
            assert "No sessions yet" in launch("-l")
            assert not sock.exists(), "listing should not start a server"
            launch("work", str(outside))
            for _ in range(60):
                if (stage / "agent.log").exists():
                    break
                time.sleep(0.05)
            assert (stage / "agent.log").read_text().strip() == str(outside)
            inspect = client + ["list-panes", "-t", "work", "-F",
                                "#{pane_id}|#{pane_width}|#{pane_active}|#{pane_current_path}"]
            panes = subprocess.check_output(inspect, text=True)
            left, right = [line.split("|") for line in panes.splitlines()]
            assert left[3] == right[3] == str(outside)
            assert int(left[1]) > int(right[1]) and left[2] == "1"
            launch("work")
            assert subprocess.check_output(inspect, text=True) == panes
            launch("work", str(repo_root), ok=False)
            launch("work", extra={"TMUX": "test"})
            assert "switch-client -t =work" in (stage / "connections").read_text()
            launch("component")
            launch("collection/component")
            assert "dev a/ambiguous" in launch("ambiguous", ok=False)
            launch("../invalid", ok=False)
            launch("missing", ok=False)
            listed = launch("-l")
            assert listed == launch("--list") and "START DIRECTORY" in listed
            assert str(outside) in listed and "detached" in listed
            assert "collection--component" in listed
            assert len(subprocess.check_output(client + ["list-sessions"], text=True).splitlines()) == 2
        finally:
            subprocess.run(client + ["kill-server"], check=False, capture_output=True)
    assert all(hashlib.sha256(p.read_bytes()).digest() == digest for p, digest in before.items())
    print("PASS: independent install, backup, PATH precedence, configurable command/shell/root, "
          "quoted paths, split, reconnect, discovery, listing, and unchanged personal configs")


if __name__ == "__main__":
    run()
