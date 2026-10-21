"""Release contracts and a local reproduction of the triggering-commit race."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[1]


class ReleaseWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.release = yaml.safe_load((ROOT / '.github/workflows/release.yml').read_text())
        self.publish = yaml.safe_load((ROOT / '.github/workflows/goreleaser.yml').read_text())
        self.job = self.publish['jobs']['goreleaser']

    def step(self, name):
        return next(step for step in self.job['steps'] if step['name'] == name)

    def test_publishing_requires_release_please_output(self):
        self.assertEqual({'workflow_call'}, set(self.publish['on']))
        call = self.release['jobs']['goreleaser']
        self.assertEqual('release-please', call['needs'])
        self.assertEqual("${{ needs.release-please.outputs.component_release_created == 'true' }}", call['if'])
        self.assertEqual('./.github/workflows/goreleaser.yml', call['uses'])
        self.assertEqual('${{ needs.release-please.outputs.component_tag_name }}', call['with']['tag'])
        self.assertTrue(self.publish['on']['workflow_call']['inputs']['tag']['required'])

    def test_fallback_token_does_not_need_another_event(self):
        jobs = self.release['jobs']
        fallback = '${{ secrets.RELEASE_PLEASE_TOKEN || secrets.GITHUB_TOKEN }}'
        self.assertEqual(fallback, jobs['release-please']['secrets']['token'])
        self.assertEqual(fallback, jobs['goreleaser']['secrets']['github-token'])
        self.assertFalse(jobs['release-please']['with']['cancel-in-progress'])
        self.assertEqual('${{ secrets.github-token || github.token }}', self.step('Run GoReleaser')['env']['GITHUB_TOKEN'])

    def test_publish_lock_is_shared_across_tags(self):
        lock = self.job['concurrency']
        self.assertEqual('${{ github.repository }}:goreleaser-publish', lock['group'])
        self.assertFalse(lock['cancel-in-progress'])

    def test_exact_tag_and_history_are_used(self):
        checkout = self.step('Checkout release tag')['with']
        self.assertEqual('refs/tags/${{ inputs.tag }}', checkout['ref'])
        self.assertEqual(0, checkout['fetch-depth'])
        self.assertFalse(checkout['persist-credentials'])
        self.assertEqual('${{ inputs.tag }}', self.step('Run GoReleaser')['env']['GORELEASER_CURRENT_TAG'])

    def test_homebrew_keeps_scoped_app_and_legacy_token_fallback(self):
        secrets = self.release['jobs']['goreleaser']['secrets']
        self.assertEqual('${{ secrets.PRIVATE_KEY }}', secrets['private-key'])
        self.assertEqual('${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}', secrets['homebrew-tap-token'])
        app = self.step('Create GitHub App token for Homebrew tap')['with']
        self.assertEqual('matt-riley', app['owner'])
        self.assertEqual('homebrew-tools', app['repositories'])
        self.assertEqual('write', app['permission-contents'])
        args = self.step('Compute GoReleaser args')
        fallback = "${{ steps.app-token.outputs.token || secrets.homebrew-tap-token || '' }}"
        self.assertEqual(fallback, args['env']['TAP_TOKEN'])
        self.assertEqual(fallback, self.step('Run GoReleaser')['env']['HOMEBREW_TAP_GITHUB_TOKEN'])
        for token, expected in [('', 'release --clean --skip=homebrew'), ('test-token', 'release --clean')]:
            with self.subTest(token_present=bool(token)), tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / 'output'
                env = dict(os.environ, TAP_TOKEN=token, GITHUB_OUTPUT=str(output))
                subprocess.run(['bash', '-e', '-c', args['run']], env=env, capture_output=True, check=True)
                self.assertEqual('value=' + expected, output.read_text().strip())

    def test_tag_created_on_another_commit_checks_out_the_release(self):
        with tempfile.TemporaryDirectory() as directory:
            repo = Path(directory)

            def git(*args):
                return subprocess.run(
                    ['git', '-c', 'commit.gpgsign=false', '-c', 'core.hooksPath=/dev/null', *args],
                    cwd=repo, text=True, capture_output=True, check=True,
                ).stdout.strip()

            git('init')
            git('config', 'user.name', 'Release fixture')
            git('config', 'user.email', 'release@example.invalid')
            (repo / 'version').write_text('before release')
            git('add', 'version')
            git('commit', '-m', 'triggering commit')
            triggering = git('rev-parse', 'HEAD')
            (repo / 'version').write_text('release commit')
            git('commit', '-am', 'release commit')
            released = git('rev-parse', 'HEAD')
            git('tag', '-a', 'v0.2.0', '-m', 'release')
            git('checkout', '--detach', triggering)

            step = self.step('Verify tagged checkout')
            self.assertEqual('${{ inputs.tag }}', step['env']['RELEASE_TAG'])
            env = dict(os.environ, RELEASE_TAG='v0.2.0')

            def verify():
                return subprocess.run(['bash', '-e', '-c', step['run']], cwd=repo, env=env, capture_output=True)

            self.assertNotEqual(0, verify().returncode, 'the original triggering checkout must fail')
            ref = self.step('Checkout release tag')['with']['ref'].replace('${{ inputs.tag }}', 'v0.2.0')
            git('checkout', '--detach', ref)
            self.assertEqual(0, verify().returncode)
            self.assertEqual(released, git('rev-parse', 'HEAD'))
            self.assertEqual('release commit', (repo / 'version').read_text())
            env['RELEASE_TAG'] = 'missing-tag'
            self.assertNotEqual(0, verify().returncode, 'a missing tag must fail before publishing')


if __name__ == '__main__':
    unittest.main()
