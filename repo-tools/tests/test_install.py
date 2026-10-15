# Copyright 2026 Nieuwsma Services LLC
# SPDX-License-Identifier: Apache-2.0

from pathlib import Path
import hashlib
import os
import shutil
import subprocess
import tempfile
import unittest


SOURCE = Path(__file__).resolve().parents[1]


class InstallationTests(unittest.TestCase):
    def test_copied_bundle_survives_source_removal_and_preserves_configs(self):
        protected = [Path.home() / name for name in ('.zshrc', '.tmux.conf')]
        before = {p: hashlib.sha256(p.read_bytes()).digest() for p in protected if p.exists()}
        with tempfile.TemporaryDirectory(prefix='repo-tools-install-') as temporary:
            staging = Path(temporary).resolve()
            copied = staging / 'copied'
            shutil.copytree(SOURCE, copied)
            prefix = staging / "a 'quoted' directory" / 'bin'
            command = [str(copied / 'install.sh'), '--prefix', str(prefix)]
            subprocess.run(command, cwd='/', check=True, stdout=subprocess.DEVNULL)
            subprocess.run(command, cwd='/', check=True, stdout=subprocess.DEVNULL)
            self.assertEqual(list(staging.rglob('*.before-install.*')), [])
            # Replacing changed runtime and entry points creates recoverable backups.
            runtime = prefix.parent / 'share/dev-tools/repo-tools'
            (runtime / 'lib/repos.sh').write_text('# old helper\n')
            (prefix / 'repo-status').write_text('# old launcher\n')
            subprocess.run(command, cwd='/', check=True, stdout=subprocess.DEVNULL)
            self.assertEqual(len(list((prefix.parent / 'share/dev-tools').glob('repo-tools.before-install.*'))), 1)
            self.assertEqual(len(list(prefix.glob('repo-status.before-install.*'))), 1)
            shutil.rmtree(copied)
            workspace = staging / 'workspace'
            workspace.mkdir()
            repo = workspace / 'example repo'
            repo.mkdir()
            remote = staging / 'remote.git'

            def git(*args):
                return subprocess.check_output(['git', '-C', str(repo), *args], text=True,
                                               stderr=subprocess.PIPE).strip()

            git('init', '-q', '-b', 'main')
            git('config', 'user.name', 'Test')
            git('config', 'user.email', 'test@example.invalid')
            git('config', 'commit.gpgsign', 'false')
            (repo / 'file').write_text('initial\n')
            git('add', 'file')
            git('commit', '-qm', 'initial')
            subprocess.run(['git', 'init', '--bare', '-q', str(remote)], check=True)
            git('remote', 'add', 'origin', str(remote))

            def run(name, *args):
                result = subprocess.run([str(prefix / name), *args], cwd=workspace,
                                        capture_output=True, text=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                return result.stdout

            for name in ('repo-status', 'repo-branch', 'repo-push'):
                self.assertTrue(os.access(prefix / name, os.X_OK))
                self.assertIn('Usage:', run(name, '--help'))
            self.assertIn('example repo', run('repo-status'))
            run('repo-branch', '--dry-run', 'feature/example')
            self.assertEqual(git('branch', '--show-current'), 'main')
            run('repo-branch', 'feature/example')
            self.assertEqual(git('branch', '--show-current'), 'feature/example')
            run('repo-push', '--dry-run')
            run('repo-push')
            self.assertEqual(git('rev-parse', '--abbrev-ref', '@{upstream}'), 'origin/feature/example')
        self.assertTrue(all(hashlib.sha256(p.read_bytes()).digest() == digest
                            for p, digest in before.items()))


if __name__ == '__main__':
    unittest.main()
