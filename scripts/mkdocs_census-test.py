#!/usr/bin/env python3
"""Tests for scripts/mkdocs_census.py. Run: python3 scripts/mkdocs_census-test.py"""

import importlib.util
import pathlib
import sys
import unittest

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("mkdocs_census", HERE / "mkdocs_census.py")
census = importlib.util.module_from_spec(spec)
spec.loader.exec_module(census)

PAGE = """# Compliance

<!-- doc-counts:begin census -->
The map below tracks every rule row on this page; the census is counted at build time.
<!-- doc-counts:end census -->
Read that as progress.

### Calc

| Rule | Where | Test | Status |
|---|---|---|---|
| a | x | y | ✅ Faithful |
| b | x | y | ⚠️ Approximate |
| c | x | y | ❌ Not implemented |
| d | x | y | ⛔ Deliberate |
| g | x | y | 🧩 Extension (UML-derived) |
| notes | mentions ✅ and ❌ together | y | ⚠️ Approximate |

### Action

**No external referee:** self-assessed.

| Rule | Where | Test | Status |
|---|---|---|---|
| e | x | y | ✅ Faithful |
| f | x | y | ✅ Faithful |
"""


class CensusTest(unittest.TestCase):
    def test_counts_one_marker_per_row_and_self_assessed_sections(self):
        c = census.count_rules(PAGE)
        self.assertEqual(c["total"], 7)
        self.assertEqual((c["✅"], c["🧩"], c["⚠"], c["❌"], c["⛔"], c["🚧"]), (3, 1, 1, 1, 1, 0))
        self.assertEqual(c["self-assessed"], 2)

    def test_replaces_the_block_and_nothing_else(self):
        out = census.census(PAGE)
        self.assertIn(
            "# Compliance\n\nThe map below tracks 7 semantic rules: **3 ✅ faithful, "
            "1 🧩 extension (UML-derived or OpenSysML, not a SysML v2 claim), 1 ⚠️ approximate, "
            "1 ❌ not implemented, 1 ⛔ deliberate divergence**; 2 of them have no external referee.\n"
            "Read that as progress.\n",
            out,
        )
        self.assertNotIn("doc-counts", out)
        self.assertTrue(out.endswith("| f | x | y | ✅ Faithful |\n"))

    def test_page_without_block_or_rows_is_an_error(self):
        without_block = PAGE.replace("<!-- doc-counts:begin census -->\n", "")
        without_rows = PAGE.split("### Calc")[0]
        with self.assertRaises(ValueError):
            census.census(without_block)
        with self.assertRaises(ValueError):
            census.census(without_rows)

    def test_malformed_markers_are_an_error(self):
        begin, end = census.BEGIN, census.END
        for page in (
            PAGE.replace(begin, begin + begin),
            PAGE.replace(end, end + end),
            PAGE + begin + "again\n" + end,
            PAGE.replace(begin, "\0").replace(end, begin).replace("\0", end),
            PAGE.replace(begin, "prose " + begin),
            PAGE.replace(end, end.rstrip("\n") + " prose\n"),
        ):
            with self.assertRaises(ValueError):
                census.census(page)

    def test_known_failure_rows_are_an_error(self):
        with_known_failure = PAGE.replace("⛔ Deliberate", "🚧 Known failure")
        with self.assertRaises(ValueError):
            census.census(with_known_failure)

    def test_real_page_renders(self):
        text = (HERE.parent / "docs" / "project" / "spec-compliance.md").read_text(encoding="utf-8")
        out = census.census(text)
        self.assertRegex(out, r"The map below tracks [1-9][0-9]* semantic rules: \*\*")
        self.assertNotIn("doc-counts:begin census", out)


if __name__ == "__main__":
    sys.exit(unittest.main(verbosity=1).result.wasSuccessful() is False)
