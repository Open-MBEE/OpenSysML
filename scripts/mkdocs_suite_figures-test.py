#!/usr/bin/env python3
"""Tests for scripts/mkdocs_suite_figures.py. Run: python3 scripts/mkdocs_suite_figures-test.py"""

import importlib.util
import pathlib
import re
import sys
import unittest

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("mkdocs_suite_figures", HERE / "mkdocs_suite_figures.py")
figures = importlib.util.module_from_spec(spec)
spec.loader.exec_module(figures)

PAGE = """# Compliance

<!-- doc-counts:begin census -->
The map below tracks every rule row on this page.
<!-- doc-counts:end census -->

**Test Coverage:**
- Execution conformance: <!-- doc-counts:begin inventory-conformance -->the conformance cases<!-- doc-counts:end inventory-conformance --> — kept prose
- Test functions: <!-- doc-counts:begin inventory-tests -->the top-level `Test` functions<!-- doc-counts:end inventory-tests --> (kept prose)

| a | x | y | ✅ Faithful |
"""

BLOCKS = {
    "inventory-conformance": "7 conformance cases (all passing: calc×4)",
    "inventory-tests": "10 top-level `Test` functions across the module",
}


class SpliceTest(unittest.TestCase):
    def test_replaces_each_block_and_nothing_else(self):
        out = figures.splice(PAGE, "project/spec-compliance.md", BLOCKS)
        self.assertIn("- Execution conformance: 7 conformance cases (all passing: calc×4) — kept prose\n", out)
        self.assertIn("- Test functions: 10 top-level `Test` functions across the module (kept prose)\n", out)
        self.assertNotIn("inventory-", out)
        # Blocks the build does not render, the census here, are left for their own hook.
        self.assertIn("<!-- doc-counts:begin census -->\n", out)
        self.assertTrue(out.startswith("# Compliance\n\n"))
        self.assertTrue(out.endswith("| a | x | y | ✅ Faithful |\n"))

    def test_missing_or_duplicated_markers_are_an_error(self):
        begin = "<!-- doc-counts:begin inventory-tests -->"
        end = "<!-- doc-counts:end inventory-tests -->"
        for page in (
            PAGE.replace(begin, ""),
            PAGE.replace(end, ""),
            PAGE.replace(begin, begin + begin),
            PAGE + begin + "again" + end + "\n",
            PAGE.replace(begin, "\0").replace(end, begin).replace("\0", end),
        ):
            with self.assertRaises(ValueError):
                figures.splice(page, "project/spec-compliance.md", BLOCKS)

    def test_a_missing_or_failing_go_is_an_error(self):
        with self.assertRaises(ValueError):
            figures.render_site_blocks(command=("no-such-go-toolchain",))
        with self.assertRaises(ValueError):
            figures.render_site_blocks(command=figures.COMMAND + ("-root", str(HERE / "no-such-tree")))

    def test_real_tree_renders_into_the_real_page(self):
        rendered = figures.render_site_blocks()
        self.assertEqual(list(rendered), ["docs/project/spec-compliance.md"])
        text = (HERE.parent / "docs" / "project" / "spec-compliance.md").read_text(encoding="utf-8")
        out = figures.splice(text, "project/spec-compliance.md", rendered["docs/project/spec-compliance.md"])
        self.assertRegex(out, r"- Runtime robustness: [1-9][0-9,]* runtime robustness cases \(first-level subtests across the `TestRuntimeRobustness\*` functions\)")
        self.assertRegex(out, r"\*\*Measured coverage:\*\* [1-9][0-9,]* top-level `Test` functions in `internal/frontend/lsp`")
        self.assertNotIn("doc-counts:begin inventory-", out)
        self.assertNotIn("doc-counts:begin lsp-tests", out)
        self.assertIn("<!-- doc-counts:begin census -->", out)
        for line in text.splitlines():
            if "doc-counts:begin inventory-" in line or "doc-counts:begin lsp-tests" in line:
                self.assertIsNone(re.search(r"-->[^<]*[0-9][^<]*<!-- doc-counts:end", line), f"a figure is committed: {line[:120]}")

    def test_hooks_leave_other_pages_alone(self):
        class File:
            src_uri = "guide/index.md"

        class Page:
            file = File()

        figures._rendered = {"docs/project/spec-compliance.md": BLOCKS}
        try:
            self.assertEqual(figures.on_page_markdown("# Guide\n", Page()), "# Guide\n")
            File.src_uri = "project/spec-compliance.md"
            self.assertNotIn("inventory-", figures.on_page_markdown(PAGE, Page()))
        finally:
            figures._rendered = None


if __name__ == "__main__":
    sys.exit(unittest.main(verbosity=1).result.wasSuccessful() is False)
