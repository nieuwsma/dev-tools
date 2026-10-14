import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).with_name('repo-branch.sh')


def git(repo, *args):
    return subprocess.check_output(['git', '-C', str(repo), *args], text=True).strip()


class RepoBranchTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='repo-branch-test-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def repo(self, name):
        repo = self.root / name
        repo.mkdir(parents=True)
        git(repo, 'init', '-q', '-b', 'main')
        git(repo, 'config', 'user.name', 'Test')
        git(repo, 'config', 'user.email', 'test@example.invalid')
        git(repo, 'config', 'commit.gpgsign', 'false')
        (repo / 'file').write_text('initial\n')
        git(repo, 'add', 'file')
        git(repo, 'commit', '-qm', 'initial')
        return repo

    def run_tool(self, *args):
        env = dict(os.environ, PWD=str(self.root))
        return subprocess.run(['bash', str(SCRIPT), *args], cwd=self.root,
                              env=env, capture_output=True, text=True)

    def test_immediate_only_and_preserves_changes(self):
        a = self.repo('a repo')
        b = self.repo('.hidden')
        nested = self.repo('ordinary/nested')
        (self.root / 'plain').mkdir()
        (self.root / 'linked').symlink_to(nested, target_is_directory=True)
        (a / 'file').write_text('staged\n')
        git(a, 'add', 'file')
        (a / 'file').write_text('unstaged\n')
        (a / 'untracked').write_text('keep me')
        before = git(a, 'status', '--porcelain')
        result = self.run_tool('feature/wave')
        self.assertEqual(result.returncode, 0, result.stderr)
        for repo in (a, b):
            self.assertEqual(git(repo, 'branch', '--show-current'), 'feature/wave')
        self.assertEqual(git(nested, 'branch', '--show-current'), 'main')
        self.assertEqual(git(a, 'status', '--porcelain'), before)
        self.assertEqual((a / 'file').read_text(), 'unstaged\n')
        self.assertEqual((a / 'untracked').read_text(), 'keep me')

    def test_collision_aborts_all(self):
        a, b = self.repo('a'), self.repo('b')
        git(b, 'branch', 'wave')
        result = self.run_tool('wave')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('No branches created', result.stderr)
        self.assertEqual(git(a, 'branch', '--show-current'), 'main')
        self.assertEqual(git(a, 'branch', '--list', 'wave'), '')

    def test_dry_run_and_invalid_names(self):
        repo = self.repo('repo')
        self.assertEqual(self.run_tool('--dry-run', 'wave').returncode, 0)
        self.assertEqual(git(repo, 'branch', '--list', 'wave'), '')
        for name in ('bad name', '-option', '@{-1}', 'HEAD'):
            self.assertNotEqual(self.run_tool(name).returncode, 0, name)
        self.assertEqual(git(repo, 'branch', '--show-current'), 'main')

    def test_linked_worktree(self):
        repo = self.repo('ordinary/source')
        worktree = self.root / 'worktree'
        git(repo, 'worktree', 'add', '-qb', 'working', str(worktree))
        result = self.run_tool('wave')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(git(worktree, 'branch', '--show-current'), 'wave')
        self.assertEqual(git(repo, 'branch', '--show-current'), 'main')

    def test_empty_workspace_and_current_repo_ignored(self):
        self.assertEqual(self.run_tool('wave').returncode, 0)
        git(self.root, 'init', '-q', '-b', 'main')
        self.assertEqual(self.run_tool('wave').returncode, 0)
        self.assertEqual(git(self.root, 'branch', '--show-current'), 'main')

    def test_unfinished_operation_aborts_all(self):
        a, b = self.repo('a'), self.repo('b')
        (b / '.git' / 'MERGE_HEAD').write_text(git(b, 'rev-parse', 'HEAD') + '\n')
        result = self.run_tool('wave')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(git(a, 'branch', '--list', 'wave'), '')

    def test_unborn_repo_aborts_all(self):
        a = self.repo('a')
        empty = self.root / 'b'
        empty.mkdir()
        git(empty, 'init', '-q')
        self.assertNotEqual(self.run_tool('wave').returncode, 0)
        self.assertEqual(git(a, 'branch', '--list', 'wave'), '')


if __name__ == '__main__':
    unittest.main()
