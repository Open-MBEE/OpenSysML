#!/usr/bin/env python3
"""Tests for scripts/spec_compliance.py. Run: python3 scripts/spec_compliance-test.py"""

from __future__ import annotations

import importlib.util
import pathlib
import sys
import tempfile
import unittest

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("spec_compliance", HERE / "spec_compliance.py")
sc = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(sc)


def write(root: pathlib.Path, rel: str, text: str) -> pathlib.Path:
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")
    return path


class Fixture(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.tmp.name) / "spec-compliance"
        self.root.mkdir()
        write(self.root, "00-preamble.md", "# Title\n\nIntro.\n")
        write(self.root, "01-status.md", "## Status\n\n| a | b |\n")
        write(self.root, "02-map/00-how-to-read.md", "## Map\n\n### How to Read\n\nText.\n")
        write(self.root, "02-map/01-action.md", "### Action\n\nRows.\n")
        write(self.root, "03-files.md", "## Files\n\nList.\n")
        write(self.root, "README.md", "Not a source.\n")

    def tearDown(self) -> None:
        self.tmp.cleanup()


class RenderTests(Fixture):
    def test_sources_in_path_order_skipping_readme(self) -> None:
        names = [p.relative_to(self.root).as_posix() for p in sc.sources(self.root)]
        self.assertEqual(
            names,
            ["00-preamble.md", "01-status.md", "02-map/00-how-to-read.md", "02-map/01-action.md", "03-files.md"],
        )

    def test_render_joins_with_one_blank_line(self) -> None:
        self.assertEqual(
            sc.render(self.root),
            "# Title\n\nIntro.\n\n## Status\n\n| a | b |\n\n## Map\n\n### How to Read\n\nText.\n\n"
            "### Action\n\nRows.\n\n## Files\n\nList.\n",
        )

    def test_render_is_deterministic(self) -> None:
        first = sc.render(self.root)
        self.assertEqual(sc.render(self.root), first)

    def test_locate_names_the_source_holding_a_heading(self) -> None:
        found = [p.name for p in sc.locate("action", self.root)]
        self.assertEqual(found, ["01-action.md"])
        self.assertEqual(sc.locate("nowhere", self.root), [])


class SourceErrorTests(Fixture):
    def assert_rejected(self, fragment: str) -> None:
        with self.assertRaises(sc.SourceError) as ctx:
            sc.render(self.root)
        self.assertIn(fragment, str(ctx.exception))

    def test_stray_file_rejected(self) -> None:
        write(self.root, "notes.md", "## Notes\n")
        self.assert_rejected("notes.md")

    def test_unnumbered_directory_rejected(self) -> None:
        write(self.root, "extra/00-x.md", "## X\n")
        self.assert_rejected("extra")

    def test_nested_directory_rejected(self) -> None:
        write(self.root, "02-map/03-deep/00-x.md", "## X\n")
        self.assert_rejected("03-deep")

    def test_empty_section_directory_rejected(self) -> None:
        (self.root / "04-empty").mkdir()
        self.assert_rejected("04-empty")

    def test_missing_trailing_newline_rejected(self) -> None:
        write(self.root, "03-files.md", "## Files\n\nList.")
        self.assert_rejected("exactly one newline")

    def test_doubled_trailing_newline_rejected(self) -> None:
        write(self.root, "03-files.md", "## Files\n\nList.\n\n")
        self.assert_rejected("exactly one newline")

    def test_source_without_heading_rejected(self) -> None:
        write(self.root, "03-files.md", "List.\n")
        self.assert_rejected("starts with a heading")

    def test_missing_directory_rejected(self) -> None:
        with self.assertRaises(sc.SourceError):
            sc.sources(self.root / "absent")

    def test_empty_directory_rejected(self) -> None:
        empty = pathlib.Path(self.tmp.name) / "empty"
        empty.mkdir()
        with self.assertRaises(sc.SourceError):
            sc.sources(empty)


class RepositoryTests(unittest.TestCase):
    """The checked-in sources render, twice identically, and the tracked tree holds no page."""

    def test_checked_in_sources_render_identically_twice(self) -> None:
        first = sc.render()
        self.assertEqual(first, sc.render())
        self.assertTrue(first.startswith("# SysML v2 Specification Compliance\n"))
        self.assertIn("\n## Detailed Semantic Compliance Map\n", first)

    def test_every_top_level_section_is_a_source_boundary(self) -> None:
        page = sc.render()
        headings = sum(1 for line in page.splitlines() if line.startswith("## "))
        starts = sum(1 for p in sc.sources() if sc.read_source(p).startswith("## "))
        self.assertEqual(headings, starts)

    def test_check_accepts_the_repository(self) -> None:
        self.assertEqual(sc.check(require_page=False, forbid_page=False), [])


if __name__ == "__main__":
    unittest.main()
