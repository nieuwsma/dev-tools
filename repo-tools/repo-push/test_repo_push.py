import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).with_name('repo-push.sh')


def git(repo, *args):
    return subprocess.check_output(['git', '-C', str(repo), *args], text=True,
                                   stderr=subprocess.PIPE).strip()


class RepoPushTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='repo-push-test-')
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.root = self.base / 'workspace'
        self.root.mkdir()
        self.remotes = self.base / 'remotes'
        self.remotes.mkdir()

    def remote(self, name):
        path = self.remotes / name
        path.mkdir()
        git(path, 'init', '--bare', '-q')
        return path

    def repo(self, name, with_remote=True):
        repo = self.root / name
        repo.mkdir(parents=True)
        git(repo, 'init', '-q', '-b', 'wave')
        git(repo, 'config', 'user.name', 'Test')
        git(repo, 'config', 'user.email', 'test@example.invalid')
        git(repo, 'config', 'commit.gpgsign', 'false')
        (repo / 'file').write_text('initial\n')
        git(repo, 'add', 'file')
        git(repo, 'commit', '-qm', 'initial')
        remote = self.remote(name.replace('/', '-')) if with_remote else None
        if remote:
            git(repo, 'remote', 'add', 'origin', str(remote))
        return repo, remote

    def run_tool(self, *args):
        env = dict(os.environ, PWD=str(self.root))
        return subprocess.run(['bash', str(SCRIPT), *args], cwd=self.root,
                              env=env, capture_output=True, text=True)

    def assert_unpushed(self, remote):
        self.assertEqual(git(remote, 'for-each-ref', '--format=%(refname)', 'refs/heads'), '')

    def test_creates_upstreams_and_preserves_uncommitted_changes(self):
        a, ar = self.repo('a repo')
        b, br = self.repo('.hidden')
        (a / 'file').write_text('staged\n')
        git(a, 'add', 'file')
        (a / 'file').write_text('unstaged\n')
        (a / 'untracked').write_text('keep')
        status = git(a, 'status', '--porcelain')
        head = git(a, 'rev-parse', 'HEAD')
        result = self.run_tool()
        self.assertEqual(result.returncode, 0, result.stderr)
        for repo, remote in ((a, ar), (b, br)):
            self.assertEqual(git(remote, 'rev-parse', 'refs/heads/wave'), git(repo, 'rev-parse', 'HEAD'))
            self.assertEqual(git(repo, 'rev-parse', '--abbrev-ref', '@{upstream}'), 'origin/wave')
        self.assertEqual(git(a, 'status', '--porcelain'), status)
        self.assertEqual(git(a, 'rev-parse', 'HEAD'), head)
        self.assertEqual(git(ar, 'show', 'wave:file'), 'initial')

    def test_existing_upstream_with_different_name_and_push_defaults(self):
        repo, remote = self.repo('repo')
        git(repo, 'push', '-qu', 'origin', 'wave:published')
        git(repo, 'config', 'push.default', 'matching')
        git(repo, 'config', 'remote.origin.mirror', 'true')
        git(repo, 'config', 'push.followTags', 'true')
        git(repo, 'config', 'tag.gpgsign', 'false')
        git(repo, 'tag', '-am', 'tag', 'unrelated-tag')
        git(repo, 'branch', 'other')
        git(repo, 'commit', '--allow-empty', '-qm', 'next')
        result = self.run_tool()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(git(remote, 'rev-parse', 'published'), git(repo, 'rev-parse', 'HEAD'))
        self.assertEqual(git(remote, 'for-each-ref', '--format=%(refname)'), 'refs/heads/published')
        self.assertEqual(git(repo, 'config', 'branch.wave.merge'), 'refs/heads/published')

    def test_dry_run_is_local_and_nonmutating(self):
        repo, remote = self.repo('repo')
        git(repo, 'remote', 'set-url', 'origin', '/nonexistent/remote')
        config = (repo / '.git' / 'config').read_bytes()
        result = self.run_tool('--dry-run')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('origin/wave (set upstream)', result.stdout)
        self.assertEqual((repo / '.git' / 'config').read_bytes(), config)
        self.assert_unpushed(remote)

    def test_preflight_aborts_all_for_missing_remote(self):
        _, remote = self.repo('a')
        self.repo('b', with_remote=False)
        result = self.run_tool()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Nothing pushed', result.stderr)
        self.assert_unpushed(remote)

    def test_detached_head_aborts_all(self):
        _, remote = self.repo('a')
        repo, _ = self.repo('b')
        git(repo, 'checkout', '--detach', '-q')
        self.assertNotEqual(self.run_tool().returncode, 0)
        self.assert_unpushed(remote)

    def test_only_immediate_children_and_linked_worktrees(self):
        source, remote = self.repo('ordinary/source')
        git(source, 'worktree', 'add', '-qb', 'linked-wave', str(self.root / 'worktree'))
        nested, nested_remote = self.repo('ordinary/nested')
        (self.root / 'symlink').symlink_to(nested, target_is_directory=True)
        result = self.run_tool()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(git(remote, 'for-each-ref', '--format=%(refname)'), 'refs/heads/linked-wave')
        self.assert_unpushed(nested_remote)

    def test_remote_selection_and_ambiguous_remotes(self):
        repo, origin = self.repo('repo')
        other = self.remote('other')
        git(repo, 'remote', 'add', 'other', str(other))
        self.assertIn('origin/wave', self.run_tool('--dry-run').stdout)
        git(repo, 'config', 'remote.pushDefault', 'other')
        self.assertIn('other/wave', self.run_tool('--dry-run').stdout)
        git(repo, 'config', 'branch.wave.pushRemote', 'origin')
        self.assertIn('origin/wave', self.run_tool('--dry-run').stdout)
        git(repo, 'config', '--unset', 'branch.wave.pushRemote')
        git(repo, 'config', '--unset', 'remote.pushDefault')
        git(repo, 'remote', 'rename', 'origin', 'first')
        self.assertNotEqual(self.run_tool().returncode, 0)
        self.assert_unpushed(origin)
        self.assert_unpushed(other)
        git(repo, 'remote', 'remove', 'other')
        self.assertIn('first/wave', self.run_tool('--dry-run').stdout)

    def test_non_fast_forward_stops_without_force_or_later_pushes(self):
        repo, remote = self.repo('a')
        _, later = self.repo('b')
        git(repo, 'push', '-qu', 'origin', 'wave')
        original = git(repo, 'rev-parse', 'HEAD')
        git(repo, 'commit', '--allow-empty', '-qm', 'remote-only')
        git(repo, 'push', '-q', 'origin', 'wave')
        remote_head = git(repo, 'rev-parse', 'HEAD')
        git(repo, 'reset', '--hard', '-q', original)
        git(repo, 'commit', '--allow-empty', '-qm', 'local-only')
        result = self.run_tool()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('push failed; stopping', result.stderr)
        self.assertEqual(git(remote, 'rev-parse', 'wave'), remote_head)
        self.assert_unpushed(later)

    def test_unborn_and_invalid_upstream_abort(self):
        _, remote = self.repo('a')
        unborn = self.root / 'b'
        unborn.mkdir()
        git(unborn, 'init', '-q')
        self.assertNotEqual(self.run_tool().returncode, 0)
        self.assert_unpushed(remote)

    def test_current_repo_and_empty_workspace_are_skipped(self):
        git(self.root, 'init', '-q')
        result = self.run_tool()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('No immediate child', result.stdout)
        self.assertNotEqual(self.run_tool('unexpected').returncode, 0)

    def test_help(self):
        result = self.run_tool('--help')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('without contacting remotes', result.stdout)


if __name__ == '__main__':
    unittest.main()
