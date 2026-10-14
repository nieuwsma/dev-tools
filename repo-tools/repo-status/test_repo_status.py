from pathlib import Path
import os
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).with_name('scripts') / 'repo-status.sh'


def git(repo, *args):
    return subprocess.check_output(['git', '-C', str(repo), *args], text=True,
                                   stderr=subprocess.PIPE).strip()


class RepoStatusTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='repo-status-test-')
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.root = self.base / 'workspace'
        self.root.mkdir()

    def repo(self, name, commit=True):
        repo = self.root / name
        repo.mkdir(parents=True)
        git(repo, 'init', '-q', '-b', 'main')
        git(repo, 'config', 'user.name', 'Test')
        git(repo, 'config', 'user.email', 'test@example.invalid')
        git(repo, 'config', 'commit.gpgsign', 'false')
        if commit:
            (repo / 'file').write_text('initial\n')
            git(repo, 'add', 'file')
            git(repo, 'commit', '-qm', 'initial')
        return repo

    def run_tool(self, *args, env=None):
        return subprocess.run(['bash', str(SCRIPT), *args], cwd=self.root,
                              env=env, capture_output=True, text=True)

    def status(self):
        return self.run_tool(str(self.root))

    def tracking_repo(self):
        repo = self.repo('tracked')
        remote = self.base / 'remote.git'
        remote.mkdir()
        git(remote, 'init', '--bare', '-q')
        git(repo, 'remote', 'add', 'origin', str(remote))
        git(repo, 'push', '-qu', 'origin', 'main')
        return repo, remote

    def test_shallow_discovery_hidden_spaces_and_linked_worktrees(self):
        self.repo('a repo')
        self.repo('.hidden')
        source = self.repo('ordinary/nested-source')
        git(source, 'worktree', 'add', '-qb', 'linked', str(self.root / 'worktree'))
        (self.root / 'symlink').symlink_to(source, target_is_directory=True)
        result = self.status()
        self.assertEqual(result.returncode, 0, result.stderr)
        for name in ('a repo', '.hidden', 'worktree'):
            self.assertIn(name, result.stdout)
        for name in ('nested-source', 'ordinary', 'symlink'):
            self.assertNotIn(name, result.stdout)

    def test_empty_workspace_and_current_repo_are_ignored(self):
        git(self.root, 'init', '-q')
        result = self.status()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('No immediate child', result.stdout)

    def test_repo_without_commits(self):
        self.repo('empty', commit=False)
        result = self.status()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('main (no commits)', result.stdout)
        self.assertIn('No upstream', result.stdout)

    def test_detached_head(self):
        repo = self.repo('detached')
        git(repo, 'checkout', '-q', '--detach')
        result = self.status()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('(detached HEAD)', result.stdout)

    def test_unconfigured_upstream(self):
        self.repo('untracked')
        result = self.status()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('No upstream', result.stdout)

    def test_missing_upstream_ref(self):
        repo, _ = self.tracking_repo()
        git(repo, 'update-ref', '-d', 'refs/remotes/origin/main')
        result = self.status()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('origin/main (missing upstream ref)', result.stdout)

    def test_divergence(self):
        repo, _ = self.tracking_repo()
        self.assertIn('origin/main (up to date)', self.status().stdout)
        original = git(repo, 'rev-parse', 'HEAD')
        git(repo, 'commit', '--allow-empty', '-qm', 'remote commit')
        remote_head = git(repo, 'rev-parse', 'HEAD')
        self.assertIn('origin/main (ahead 1)', self.status().stdout)
        git(repo, 'update-ref', 'refs/remotes/origin/main', remote_head)
        git(repo, 'reset', '--hard', '-q', original)
        self.assertIn('origin/main (behind 1)', self.status().stdout)
        git(repo, 'commit', '--allow-empty', '-qm', 'local commit')
        result = self.status()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('origin/main (ahead 1, behind 1)', result.stdout)

    def test_local_changes_are_preserved(self):
        repo = self.repo('dirty')
        (repo / 'file').write_text('staged\n')
        git(repo, 'add', 'file')
        (repo / 'file').write_text('unstaged\n')
        (repo / 'untracked').write_text('keep')
        before = git(repo, 'status', '--porcelain')
        head = git(repo, 'rev-parse', 'HEAD')
        result = self.status()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('Dirty (2 paths)', result.stdout)
        self.assertEqual(git(repo, 'status', '--porcelain'), before)
        self.assertEqual(git(repo, 'rev-parse', 'HEAD'), head)

    def test_invalid_metadata_does_not_hide_healthy_repos(self):
        self.repo('healthy')
        broken = self.root / 'broken'
        broken.mkdir()
        (broken / '.git').write_text('gitdir: /nonexistent/repo-status-test\n')
        result = self.status()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('broken: invalid Git worktree', result.stderr)
        self.assertIn('healthy', result.stdout)

    def test_default_workspace_is_current_directory(self):
        self.repo('current-workspace-repo')
        result = self.run_tool()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('current-workspace-repo', result.stdout)

    def test_help_and_extra_arguments(self):
        result = self.run_tool('--help')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('current directory', result.stdout)
        self.assertNotEqual(self.run_tool(str(self.root), 'extra').returncode, 0)


if __name__ == '__main__':
    unittest.main()
