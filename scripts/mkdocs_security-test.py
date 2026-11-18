#!/usr/bin/env python3
"""Tests for scripts/mkdocs_security.py. Run: python3 scripts/mkdocs_security-test.py"""

import datetime as dt
import importlib.util
import os
import pathlib
import subprocess
import sys
import tempfile
import textwrap
import unittest

import yaml

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parent
spec = importlib.util.spec_from_file_location("mkdocs_security", HERE / "mkdocs_security.py")
hook = importlib.util.module_from_spec(spec)
spec.loader.exec_module(hook)

POLICY = textwrap.dedent(
    """\
    # Security policy

    Intro.

    ## Reporting a vulnerability

    Report privately: [Report](https://github.com/Acme/Tool/security/advisories/new) or e-mail
    <sec@example.org>. See [`.github/CODEOWNERS`](.github/CODEOWNERS),
    [releasing](docs/project/releasing.md#the-release-provenance) and
    [install](docs/guide/01-install.md), or [CVSS](https://www.first.org/cvss/) and [below](#scope).

    ## Scope

    Everything.
    """
)


def pages_archive(site: pathlib.Path) -> list[str]:
    """The members of the archive pages.yml uploads for site, by the upload action's own rule."""
    workflow = yaml.safe_load((ROOT / ".github" / "workflows" / "pages.yml").read_text(encoding="utf-8"))
    uploads = [
        step.get("with", {})
        for job in workflow["jobs"].values()
        for step in job.get("steps", [])
        if step.get("uses", "").startswith("actions/upload-pages-artifact@")
    ]
    if len(uploads) != 1:
        raise AssertionError(f"pages.yml uploads the Pages artifact {len(uploads)} times, want once")
    # actions/upload-pages-artifact's tar step: dot entries are left out unless the input is true.
    hidden = [] if str(uploads[0].get("include-hidden-files", "false")).lower() == "true" else ["--exclude=.[^/]*"]
    tar = ["tar", "--directory", str(site), "-cf", "-", "--exclude=.git", "--exclude=.github", *hidden, "."]
    archive = subprocess.run(tar, check=True, capture_output=True).stdout
    return subprocess.run(["tar", "-tf", "-"], input=archive, check=True, capture_output=True).stdout.decode().split()


class PolicySectionsTest(unittest.TestCase):
    def test_title_dropped_sections_kept(self):
        out = hook.policy_sections(POLICY)
        self.assertTrue(out.startswith("Intro.\n"))
        self.assertIn("\n## Reporting a vulnerability\n", out)
        self.assertIn("\n## Scope\n", out)
        self.assertNotIn("# Security policy", out)

    def test_repository_links_are_rerooted_to_the_page(self):
        out = hook.policy_sections(POLICY)
        self.assertIn("](../../.github/CODEOWNERS)", out)
        self.assertIn("](../project/releasing.md#the-release-provenance)", out)
        self.assertIn("](01-install.md)", out)
        self.assertIn("](https://www.first.org/cvss/)", out)
        self.assertIn("](#scope)", out)

    def test_the_repository_policy_embeds(self):
        out = hook.policy_sections((ROOT / hook.POLICY).read_text(encoding="utf-8"))
        self.assertIn("\n## Reporting a vulnerability\n", out)
        self.assertIn("\n## Supported versions\n", out)


class SecurityTxtTest(unittest.TestCase):
    def test_contacts_form_first_then_mail(self):
        self.assertEqual(
            hook.contacts(POLICY),
            ["https://github.com/Acme/Tool/security/advisories/new", "mailto:sec@example.org"],
        )

    def test_a_policy_without_both_channels_fails_the_build(self):
        with self.assertRaises(ValueError):
            hook.contacts("# Security policy\n\nE-mail <sec@example.org>.\n")

    def test_fields(self):
        built = dt.datetime(2026, 10, 27, 12, 0, tzinfo=dt.timezone.utc)
        out = hook.security_txt(POLICY, "https://docs.example.org/", "https://github.com/Acme/Tool", built)
        self.assertEqual(
            out.splitlines(),
            [
                "Contact: https://github.com/Acme/Tool/security/advisories/new",
                "Contact: mailto:sec@example.org",
                "Expires: 2027-10-27T12:00:00Z",
                "Policy: https://github.com/Acme/Tool/security/policy",
                "Canonical: https://docs.example.org/.well-known/security.txt",
                "Preferred-Languages: en",
                "Acknowledgments: https://github.com/Acme/Tool/security/advisories",
            ],
        )

    def test_build_time_prefers_source_date_epoch(self):
        saved = os.environ.get("SOURCE_DATE_EPOCH")
        os.environ["SOURCE_DATE_EPOCH"] = "1700000000"
        try:
            self.assertEqual(hook.build_time(ROOT), dt.datetime.fromtimestamp(1700000000, dt.timezone.utc))
        finally:
            if saved is None:
                del os.environ["SOURCE_DATE_EPOCH"]
            else:
                os.environ["SOURCE_DATE_EPOCH"] = saved

    def test_build_time_of_a_checkout_is_its_commit(self):
        if "SOURCE_DATE_EPOCH" in os.environ:
            self.skipTest("SOURCE_DATE_EPOCH set")
        commit = int(subprocess.run(["git", "-C", str(ROOT), "log", "-1", "--format=%ct"], capture_output=True, text=True, check=True).stdout)
        self.assertEqual(hook.build_time(ROOT), dt.datetime.fromtimestamp(commit, dt.timezone.utc))

    def test_a_plain_directory_is_dated_by_the_clock(self):
        if "SOURCE_DATE_EPOCH" in os.environ:
            self.skipTest("SOURCE_DATE_EPOCH set")
        with tempfile.TemporaryDirectory() as tmp:
            before = dt.datetime.now(dt.timezone.utc)
            self.assertGreaterEqual(hook.build_time(pathlib.Path(tmp)), before.replace(microsecond=0))


class SiteBuildTest(unittest.TestCase):
    def test_a_site_build_emits_both_files_and_embeds_the_policy(self):
        # A minimal site with the hook's inputs where the repository keeps them; the
        # tree is not a checkout, so Expires follows SOURCE_DATE_EPOCH.
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            (root / "docs" / "guide").mkdir(parents=True)
            (root / "docs" / "index.md").write_text("# Site\n")
            (root / "docs" / "guide" / "01-install.md").write_text("# Install\n")
            (root / "docs" / "guide" / "security.md").write_text(
                "# Security\n\nAbove.\n\n" + hook.BEGIN + "Pointer for GitHub readers.\n" + hook.END + "\nBelow.\n"
            )
            (root / "SECURITY.md").write_text(POLICY)
            (root / "mkdocs.yml").write_text(
                textwrap.dedent(
                    f"""\
                    site_name: test
                    site_url: https://docs.example.org/
                    repo_url: https://github.com/Acme/Tool
                    docs_dir: docs
                    hooks:
                      - {HERE / "mkdocs_security.py"}
                      - {HERE / "mkdocs_repo_links.py"}
                    """
                )
            )
            subprocess.run(
                [sys.executable, "-m", "mkdocs", "build", "--strict", "--site-dir", str(root / "site")],
                cwd=root,
                check=True,
                capture_output=True,
                env={**os.environ, "SOURCE_DATE_EPOCH": "1800000000"},
            )
            txt = (root / "site" / ".well-known" / "security.txt").read_text()
            self.assertEqual(txt, (root / "site" / "security.txt").read_text())
            self.assertIn("Contact: mailto:sec@example.org\n", txt)
            self.assertIn("Expires: 2028-01-15T08:00:00Z\n", txt)
            # Pages serves what the workflow archives, not site/ itself.
            archived = pages_archive(root / "site")
            self.assertIn("./.well-known/security.txt", archived)
            self.assertIn("./security.txt", archived)
            page = (root / "site" / "guide" / "security" / "index.html").read_text()
            self.assertIn("Above.", page)
            self.assertIn("Below.", page)
            self.assertNotIn("Pointer for GitHub readers.", page)
            self.assertNotIn("Security policy", page)
            self.assertIn('id="reporting-a-vulnerability"', page)
            self.assertIn("https://github.com/Acme/Tool/blob/main/.github/CODEOWNERS", page)
            self.assertIn('href="../01-install/"', page)


if __name__ == "__main__":
    unittest.main()
