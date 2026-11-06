#!/usr/bin/env python3
"""Tests for scripts/mkdocs_spec_compliance.py. Run: python3 scripts/mkdocs_spec_compliance-test.py"""

import importlib.util
import pathlib
import unittest
from unittest import mock

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("mkdocs_spec_compliance", HERE / "mkdocs_spec_compliance.py")
hook = importlib.util.module_from_spec(spec)
spec.loader.exec_module(hook)

from mkdocs.config.defaults import MkDocsConfig  # noqa: E402
from mkdocs.structure.files import File, Files  # noqa: E402


def config() -> MkDocsConfig:
    """A configuration carrying only this hook, so its events run as MkDocs runs them."""
    cfg = MkDocsConfig(config_file_path=str(HERE.parent / "mkdocs.yml"))
    cfg.load_dict({
        "site_name": "t",
        "docs_dir": str(HERE.parent / "docs"),
        "site_dir": "/tmp/site",
        "hooks": [str(HERE / "mkdocs_spec_compliance.py")],
    })
    errors, _warnings = cfg.validate()
    assert not errors, errors
    return cfg


def committed_copy(cfg: MkDocsConfig) -> File:
    return File(hook.PAGE, src_dir=cfg.docs_dir, dest_dir=cfg.site_dir, use_directory_urls=True)


class HookTest(unittest.TestCase):
    def test_adds_the_combined_page_from_the_sources(self):
        cfg = config()
        files = cfg.plugins.on_files(Files([]), config=cfg)
        page = files.get_file_from_path(hook.PAGE)
        self.assertIsNotNone(page)
        self.assertEqual(page.content_string, hook.spec_compliance.render())
        self.assertEqual(page.url, "project/spec-compliance/")

    def test_replaces_a_committed_copy(self):
        cfg = config()
        stale = committed_copy(cfg)
        files = cfg.plugins.on_files(Files([stale]), config=cfg)
        pages = [f for f in files if f.src_uri == hook.PAGE]
        self.assertEqual(len(pages), 1)
        self.assertIsNot(pages[0], stale)
        self.assertEqual(pages[0].content_string, hook.spec_compliance.render())

    def test_leaves_the_files_alone_when_the_sources_are_broken(self):
        cfg = config()
        with mock.patch.object(hook.spec_compliance, "render", side_effect=hook.spec_compliance.SourceError("bad")):
            with self.assertLogs(hook.log, level="WARNING"):
                files = hook.on_files(Files([]), cfg)
        self.assertIsNone(files.get_file_from_path(hook.PAGE))


if __name__ == "__main__":
    unittest.main()
