#!/usr/bin/env python3
"""Tests for scripts/mkdocs_release_version.py."""

import importlib.util
import pathlib
import subprocess
import sys
import unittest
from unittest.mock import patch

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location(
    "mkdocs_release_version", HERE / "mkdocs_release_version.py"
)
release_version = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release_version)


class FakePage:
    class File:
        src_uri = "downloads.md"

    def __init__(self):
        self.file = self.File()


class ReleaseVersionTest(unittest.TestCase):
    def test_selects_the_highest_numeric_stable_tag(self):
        self.assertEqual(
            "0.10.0",
            release_version._latest_stable_version(
                ["v0.9.1", "nightly", "v1.0.0-rc1", "v0.10.0", "v0.10.0.1"]
            ),
        )

    def test_uses_git_tags_on_config(self):
        result = subprocess.CompletedProcess(
            ["git"], 0, stdout="v0.9.1\nv0.10.0\nv1.0.0-rc1\n", stderr=""
        )
        config = {}
        with patch.object(release_version.subprocess, "run", return_value=result):
            self.assertIs(config, release_version.on_config(config))
        self.assertEqual("0.10.0", config["release_version"])

    def test_falls_back_to_package_version_when_git_is_unavailable(self):
        config = {}
        with (
            patch.object(
                release_version.subprocess, "run", side_effect=FileNotFoundError("git")
            ),
            self.assertLogs(release_version.log, level="WARNING") as warnings,
        ):
            release_version.on_config(config)
        self.assertEqual("0.9.1", config["release_version"])
        self.assertIn("Git release tags are unavailable", warnings.output[0])

    def test_falls_back_when_no_stable_tag_is_found(self):
        result = subprocess.CompletedProcess(
            ["git"], 0, stdout="nightly\nv1.0.0-rc1\n", stderr=""
        )
        config = {}
        with (
            patch.object(release_version.subprocess, "run", return_value=result),
            self.assertLogs(release_version.log, level="WARNING") as warnings,
        ):
            release_version.on_config(config)
        self.assertEqual("0.9.1", config["release_version"])
        self.assertIn("No stable release tags were found", warnings.output[0])

    def test_substitutes_release_version_tokens(self):
        markdown = (
            "[opensysml-{{release_version}}-windows-amd64.msi]"
            "(https://example.test/v{{release_version}}/"
            "opensysml-{{release_version}}-windows-amd64.msi)"
        )
        rendered = release_version.on_page_markdown(
            markdown, FakePage(), {"release_version": "0.9.1"}
        )
        self.assertEqual(
            "[opensysml-0.9.1-windows-amd64.msi]"
            "(https://example.test/v0.9.1/opensysml-0.9.1-windows-amd64.msi)",
            rendered,
        )

    def test_warns_if_a_release_version_token_remains(self):
        with self.assertLogs(release_version.log, level="WARNING") as warnings:
            rendered = release_version.on_page_markdown(
                release_version.TOKEN,
                FakePage(),
                {"release_version": release_version.TOKEN},
            )
        self.assertEqual(release_version.TOKEN, rendered)
        self.assertIn("Unreplaced", warnings.output[0])


if __name__ == "__main__":
    sys.exit(unittest.main(verbosity=1).result.wasSuccessful() is False)
