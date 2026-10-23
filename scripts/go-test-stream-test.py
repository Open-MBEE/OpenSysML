#!/usr/bin/env python3
"""Tests for scripts/go-test-stream.py. Run: python3 scripts/go-test-stream-test.py"""

from __future__ import annotations

import importlib.util
import io
import json
import pathlib
import unittest

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("go_test_stream", HERE / "go-test-stream.py")
go_test_stream = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(go_test_stream)


def event(action: str, package: str, test: str | None = None, output: str | None = None) -> str:
    record = {"Action": action, "Package": package}
    if test is not None:
        record["Test"] = test
    if output is not None:
        record["Output"] = output
    return json.dumps(record) + "\n"


class WriteCounter(io.StringIO):
    """A StringIO that counts flushes, to show text is released per event."""

    def __init__(self):
        super().__init__()
        self.flushes = 0

    def flush(self):
        self.flushes += 1
        super().flush()


class GoTestStreamTest(unittest.TestCase):
    def run_stream(self, *lines: str) -> tuple[int, WriteCounter]:
        out = WriteCounter()
        status = go_test_stream.stream(iter(lines), out)
        return status, out

    def test_re_emits_the_verbose_text_in_order_and_passes(self):
        status, out = self.run_stream(
            event("run", "a", "TestA"),
            event("output", "a", "TestA", "=== RUN   TestA\n"),
            event("output", "a", "TestA", "--- PASS: TestA (0.01s)\n"),
            event("pass", "a", "TestA"),
            event("output", "a", output="PASS\n"),
            event("output", "a", output="ok  \ta\t0.012s\n"),
            event("pass", "a"),
        )
        self.assertEqual(status, 0)
        self.assertEqual(out.getvalue(), "=== RUN   TestA\n--- PASS: TestA (0.01s)\nPASS\nok  \ta\t0.012s\n")
        self.assertEqual(out.flushes, 4)

    def test_a_failed_package_fails_the_run(self):
        status, out = self.run_stream(
            event("output", "a", output="ok  \ta\t0.012s\n"),
            event("pass", "a"),
            event("output", "b", "TestB", "--- FAIL: TestB (0.00s)\n"),
            event("fail", "b", "TestB"),
            event("output", "b", output="FAIL\tb\t0.5s\n"),
            event("fail", "b"),
        )
        self.assertEqual(status, 1)
        self.assertIn("--- FAIL: TestB", out.getvalue())
        self.assertIn("FAIL\tb\t0.5s\n", out.getvalue())

    def test_a_failed_test_without_its_package_summary_fails_the_run(self):
        status, _ = self.run_stream(
            event("output", "b", "TestB", "--- FAIL: TestB (0.00s)\n"),
            event("fail", "b", "TestB"),
        )
        self.assertEqual(status, 1)

    def test_a_skipped_package_passes(self):
        status, out = self.run_stream(
            event("output", "a", output="?   \ta\t[no test files]\n"),
            event("skip", "a"),
        )
        self.assertEqual(status, 0)
        self.assertEqual(out.getvalue(), "?   \ta\t[no test files]\n")

    def test_no_package_summary_fails_the_run(self):
        status, out = self.run_stream()
        self.assertEqual(status, 1)
        self.assertEqual(out.getvalue(), "")

    def test_text_that_is_not_an_event_is_passed_through(self):
        status, out = self.run_stream(
            "# a/b [a/b.test]\n",
            "./b_test.go:3:2: undefined: x\n",
            event("output", "a/b", output="FAIL\ta/b [build failed]\n"),
            event("fail", "a/b"),
        )
        self.assertEqual(status, 1)
        self.assertEqual(
            out.getvalue(), "# a/b [a/b.test]\n./b_test.go:3:2: undefined: x\nFAIL\ta/b [build failed]\n"
        )


if __name__ == "__main__":
    unittest.main()
