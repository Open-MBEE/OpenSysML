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

try:
    import markdown
    from markdown.extensions.tables import TableProcessor
except ImportError:  # the site's renderer is only needed for the cross-check
    markdown = None

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

Prose naming ✅, ⚠️ and ❌ is not a row.

### Action

**No external referee:** self-assessed.

| Rule | Where | Test | Status |
|---|---|---|---|
| e | x | y | ✅ Faithful |
| f | x | y | ✅ Faithful |

| Not implemented | Why |
|---|---|
| `at` | not declared by the library |
"""

TABLE = "| Rule | Where | Test | Status |\n|---|---|---|---|\n| a | x | y | ✅ Faithful |\n"


class CensusTest(unittest.TestCase):
    def test_counts_the_status_column_and_self_assessed_sections(self):
        c = census.count_rules(PAGE)
        self.assertEqual(c["total"], 8)
        self.assertEqual((c["✅"], c["🧩"], c["⚠"], c["❌"], c["⛔"], c["🚧"]), (3, 1, 2, 1, 1, 0))
        self.assertEqual(c["self-assessed"], 2)

    def test_attributes_a_row_by_its_status_cell(self):
        page = (
            "| RPC | Implementation | Status | Tests |\n|---|---|---|---|\n"
            "| Query | `a|b` was ❌ once | ⛔ Deliberate | ✅ when `x \\| y` holds |\n"
            "| Parse | service.go | ✅ Faithful | t.go |\n"
        )
        c = census.count_rules(page)
        self.assertEqual((c["total"], c["✅"], c["⛔"], c["❌"]), (2, 1, 1, 0))

    def test_a_delimiter_row_of_spaces_is_a_table_as_the_renderer_renders_it(self):
        page = "| Rule | Status |\n|   |   |\n| a | ✅ Faithful |\n"
        self.assertEqual(census.count_rules(page)["total"], 1)
        if markdown is not None:
            self.assertIn("<td>✅ Faithful</td>", markdown.markdown(page, extensions=["tables"]))

    def test_headers_separators_and_prose_are_not_rows(self):
        page = "✅ prose naming ⚠️ and ❌\n\n| header | Status |\n|---|---|\n\n| Not implemented | Why |\n|---|---|\n| `at` | no |\n"
        self.assertEqual(census.count_rules(page)["total"], 0)

    def test_an_ambiguous_row_is_an_error_naming_its_line(self):
        for page, message in (
            (TABLE + "| b | x | y | ✅ Faithful; compiled ⚠️ Approximate |\n",
             "project/spec-compliance.md:4: 2 status markers in the Status column (✅, ⚠); state one"),
            (TABLE + "| b | x | y | ✅ Faithful; was ✅ before |\n",
             "project/spec-compliance.md:4: 2 status markers in the Status column (✅, ✅); state one"),
            (TABLE + "| b | x | y | pending |\n",
             "project/spec-compliance.md:4: no status marker in the Status column"),
            (TABLE + "| b | x | y\n  z | ✅ Faithful |\n",
             "project/spec-compliance.md:4: no status marker in the Status column"),
            (TABLE + "\n| b | x | y | ✅ Faithful |\n",
             "project/spec-compliance.md:5: a row with a status marker outside a table"),
            ("| Rule | Where | Test | Verdict |\n|---|---|---|---|\n| a | x | y | ✅ Faithful |\n",
             "project/spec-compliance.md:3: a row with a status marker in a table without a Status column"),
        ):
            with self.subTest(message=message):
                with self.assertRaises(ValueError) as raised:
                    census.count_rules(page)
                self.assertEqual(str(raised.exception), message)
                with self.assertRaises(ValueError):
                    census.census(PAGE + "\n" + page)

    def test_split_cells_splits_as_the_site_renderer_does(self):
        cases = (
            ("| a | b |", ["a", "b"]),
            ("| a | `x|y` | b |", ["a", "`x|y`", "b"]),
            ("| a \\| b | c |", ["a \\| b", "c"]),
            ("| a | b \\|", ["a", "b \\|"]),
            ("| ``a `b` c | d`` | e |", ["``a `b` c | d``", "e"]),
            ("| `a \\`b\\` c | d` | e |", ["`a \\`b\\` c", "d`", "e"]),
            ("no pipes at all", ["no pipes at all"]),
        )
        for row, cells in cases:
            self.assertEqual(census.split_cells(row), cells, row)
        if markdown is not None:
            for row, cells in cases:
                self.assertEqual([c.strip() for c in renderer_split(row)], cells, row)

    def test_replaces_the_block_and_nothing_else(self):
        out = census.census(PAGE)
        self.assertIn(
            "# Compliance\n\nThe map below tracks 8 semantic rules: **3 ✅ faithful, "
            "1 🧩 extension (UML-derived or OpenSysML, not a SysML v2 claim), 2 ⚠️ approximate, "
            "1 ❌ not implemented, 1 ⛔ deliberate divergence**; 2 of them have no external referee.\n"
            "Read that as progress.\n",
            out,
        )
        self.assertNotIn("doc-counts", out)
        self.assertTrue(out.endswith("| `at` | not declared by the library |\n"))

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

    @unittest.skipIf(markdown is None, "the site's Markdown renderer is not installed")
    def test_real_page_splits_as_the_site_renderer_does(self):
        text = (HERE.parent / "docs" / "project" / "spec-compliance.md").read_text(encoding="utf-8")
        rows = [line for line in text.splitlines() if line.strip(" ").startswith("|")]
        self.assertGreater(len(rows), 1000)
        for row in rows:
            self.assertEqual(census.split_cells(row), [c.strip() for c in renderer_split(row)], row[:80])


def renderer_split(row: str) -> list[str]:
    """Split a row with Python-Markdown's own tables extension, as a bordered table row."""
    md = markdown.Markdown(extensions=["tables"])
    processor = next(p for p in md.parser.blockprocessors if isinstance(p, TableProcessor))
    processor.border = True
    return processor._split_row(row.strip(" "))


if __name__ == "__main__":
    sys.exit(unittest.main(verbosity=1).result.wasSuccessful() is False)
