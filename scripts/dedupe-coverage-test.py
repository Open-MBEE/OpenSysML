#!/usr/bin/env python3
"""Tests for scripts/dedupe-coverage.py. Run: python3 scripts/dedupe-coverage-test.py"""

from __future__ import annotations

import contextlib
import importlib.util
import io
import os
import pathlib
import sys
import tempfile
import unittest

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("dedupe_coverage", HERE / "dedupe-coverage.py")
dedupe = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(dedupe)

PROFILE = """mode: atomic
a/a.go:1.1,2.2 1 0
b/b.go:3.3,4.4 2 5
a/a.go:1.1,2.2 1 3

b/b.go:3.3,4.4 2 1
"""


class DedupeCoverageTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.previous = os.getcwd()
        os.chdir(self.tmp.name)
        self.addCleanup(os.chdir, self.previous)

    def run_main(self, *argv: str) -> tuple[int, str, str]:
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            sys.argv = ["dedupe-coverage.py", *argv]
            try:
                status = dedupe.main()
            finally:
                sys.argv = [sys.argv[0]]
        return status, out.getvalue(), err.getvalue()

    def write(self, content: str) -> str:
        path = pathlib.Path("coverage.txt")
        path.write_text(content)
        return str(path)

    def test_keeps_the_highest_count_of_each_block_in_first_seen_order(self):
        path = self.write(PROFILE)
        status, out, err = self.run_main(path)
        self.assertEqual((status, err), (0, ""))
        self.assertIn("2 block(s)", out)
        self.assertEqual(
            pathlib.Path(path).read_text(),
            "mode: atomic\na/a.go:1.1,2.2 1 3\nb/b.go:3.3,4.4 2 5\n",
        )

    def test_a_profile_without_a_mode_line_is_refused(self):
        path = self.write("a/a.go:1.1,2.2 1 0\n")
        status, _, err = self.run_main(path)
        self.assertEqual(status, 1)
        self.assertIn("no mode line", err)
        self.assertEqual(pathlib.Path(path).read_text(), "a/a.go:1.1,2.2 1 0\n")

    def test_a_malformed_block_names_its_line(self):
        path = self.write("mode: set\na/a.go:1.1,2.2 1 many\n")
        status, _, err = self.run_main(path)
        self.assertEqual(status, 1)
        self.assertIn(":2: not a coverage block", err)

    def test_a_missing_profile_is_reported(self):
        status, _, err = self.run_main("absent.txt")
        self.assertEqual(status, 1)
        self.assertIn("absent.txt", err)

    def test_a_path_outside_the_working_tree_is_refused(self):
        outside = pathlib.Path(self.tmp.name).parent / "elsewhere.txt"
        status, _, err = self.run_main(str(outside))
        self.assertEqual(status, 2)
        self.assertIn("outside", err)

    def test_usage_needs_exactly_one_argument(self):
        for argv in ((), ("a", "b")):
            status, _, err = self.run_main(*argv)
            self.assertEqual(status, 2)
            self.assertIn("usage:", err)


if __name__ == "__main__":
    unittest.main()
