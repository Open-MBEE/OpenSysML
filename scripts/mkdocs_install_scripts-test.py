#!/usr/bin/env python3
"""Tests for scripts/mkdocs_install_scripts.py. Run: python3 scripts/mkdocs_install_scripts-test.py"""

import importlib.util
import pathlib
import subprocess
import sys
import tempfile
import textwrap
import unittest

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parent
spec = importlib.util.spec_from_file_location("mkdocs_install_scripts", HERE / "mkdocs_install_scripts.py")
hook = importlib.util.module_from_spec(spec)
spec.loader.exec_module(hook)


class InstallScriptsTest(unittest.TestCase):
    def test_the_repository_has_both_scripts(self):
        self.assertEqual([s.name for s in hook.install_scripts(ROOT)], ["install.sh", "install.ps1"])

    def test_a_missing_script_fails_the_build(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            (root / "install.sh").write_text("#!/bin/sh\n")
            with self.assertRaises(FileNotFoundError) as caught:
                hook.install_scripts(root)
            self.assertIn("install.ps1", str(caught.exception))

    def test_a_site_build_copies_the_scripts_to_its_root(self):
        # A minimal site whose config sits where the repository's does, so the hook
        # resolves the same root; only the scripts' bytes are under test.
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)
            (root / "docs").mkdir()
            (root / "docs" / "index.md").write_text("# Site\n")
            (root / "install.sh").write_text("#!/bin/sh\necho sh\n")
            (root / "install.ps1").write_text("Write-Host ps1\n")
            (root / "mkdocs.yml").write_text(
                textwrap.dedent(
                    f"""\
                    site_name: test
                    docs_dir: docs
                    hooks:
                      - {HERE / "mkdocs_install_scripts.py"}
                    """
                )
            )
            subprocess.run(
                [sys.executable, "-m", "mkdocs", "build", "--strict", "--site-dir", str(root / "site")],
                cwd=root,
                check=True,
                capture_output=True,
            )
            self.assertEqual((root / "site" / "install.sh").read_text(), "#!/bin/sh\necho sh\n")
            self.assertEqual((root / "site" / "install.ps1").read_text(), "Write-Host ps1\n")


if __name__ == "__main__":
    unittest.main()
