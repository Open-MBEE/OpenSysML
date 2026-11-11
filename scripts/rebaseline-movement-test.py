#!/usr/bin/env python3
"""Tests for scripts/rebaseline-movement.py. Run: python3 scripts/rebaseline-movement-test.py"""

import importlib.util
import io
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stdout

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("rebaseline_movement", HERE / "rebaseline-movement.py")
movement = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = movement
spec.loader.exec_module(movement)

CORPUS = "tests/corpus/testdata/pilot_corpora_expected.txt"
CENSUS = "internal/semantic/metamodel/derived_census_baseline.json"
STRUCTURE = "internal/translate/export/testdata/structure_differential_expected.txt"
GOLDENS = "tests/export/testdata/convert"


def corpus(**verdicts: int) -> str:
    return "# files: sysml-examples 2\n" + "".join(f"{n}\t{path}\n" for path, n in sorted(verdicts.items()))


def census(**rules: dict) -> str:
    return json.dumps({"roots": [{"name": "training", "rules": rules}], "summary": {"served": 2, "silent": 0}})


class Repo:
    """A throwaway repository with a merge base, two sides and their merge."""

    def __init__(self, root: str):
        self.root = root
        self.git("init", "-q", "-b", "develop")
        self.git("config", "user.email", "t@example.invalid")
        self.git("config", "user.name", "t")

    def git(self, *args: str) -> str:
        return subprocess.run(["git", *args], cwd=self.root, capture_output=True, text=True, check=True).stdout.strip()

    def write(self, files: dict[str, str]) -> None:
        for path, text in files.items():
            full = os.path.join(self.root, path)
            os.makedirs(os.path.dirname(full), exist_ok=True)
            with open(full, "w", encoding="utf-8") as handle:
                handle.write(text)

    def commit(self, files: dict[str, str], message: str) -> str:
        self.write(files)
        self.git("add", "-A")
        self.git("commit", "-q", "-m", message)
        return self.git("rev-parse", "HEAD")


class AttributionTest(unittest.TestCase):
    def test_each_sides_movement_is_explained_and_an_interaction_is_not(self):
        base = {"a": "1", "b": "1", "c": "1", "d": "1"}
        ours = {"a": "2", "b": "1", "c": "1", "d": "2"}
        theirs = {"a": "1", "b": "2", "c": "1", "d": "3"}
        merged = {"a": "2", "b": "2", "c": "1", "d": "3"}
        fresh = {"a": "2", "b": "2", "c": "9", "d": "4"}
        m = movement.attribute("x", base, ours, theirs, merged, fresh)
        # a and b match the merged tree already: not a movement. c moved with no side
        # behind it; d was moved by both sides, so the fresh value is the merge's verdict.
        self.assertEqual((m.moved, m.by_ours, m.by_theirs, m.by_both), (2, 0, 0, 1))
        self.assertEqual(m.unexplained, ["c"])

    def test_a_side_that_moved_a_record_must_also_be_where_the_fresh_run_lands(self):
        m = movement.attribute("x", {"a": "1"}, {"a": "2"}, {"a": "1"}, {"a": "1"}, {"a": "2"})
        self.assertEqual((m.moved, m.by_ours, m.unexplained), (1, 1, []))
        m = movement.attribute("x", {"a": "1"}, {"a": "2"}, {"a": "1"}, {"a": "1"}, {"a": "3"})
        self.assertEqual((m.moved, m.by_ours, m.unexplained), (1, 1, ["a"]))
        m = movement.attribute("x", {}, {}, {"a": "1"}, {"a": "1"}, {"a": "1"})
        self.assertEqual(m.moved, 0)

    def test_parsers_key_every_record_the_way_its_recorder_writes_it(self):
        self.assertEqual(movement.parse_tsv(corpus(**{"x/a.sysml": 0, "x/b.sysml": 3})), {"x/a.sysml": "0", "x/b.sysml": "3"})
        structure = "# header\nroot examples: models 2, skipped 0, matched 10, unmatched 1\n  Type::feature: equal 1, different 2, symbol-missing 0, symbol-extra 0\nroot tests/testdata: models 1, skipped 0, matched 3, unmatched 0\n"
        self.assertEqual(movement.parse_structure(structure), {
            "examples": "models 2, skipped 0, matched 10, unmatched 1",
            "examples\tType::feature": "equal 1, different 2, symbol-missing 0, symbol-extra 0",
            "tests/testdata": "models 1, skipped 0, matched 3, unmatched 0",
        })
        self.assertEqual(movement.structure_figures(structure), {"examples models": 2, "examples skipped": 0, "examples matched": 10, "examples unmatched": 1, "tests/testdata models": 1, "tests/testdata skipped": 0, "tests/testdata matched": 3, "tests/testdata unmatched": 0})
        self.assertEqual(movement.parse_sweep("# h\nread\tProbe::p1.a\tmodel\tP1::a\tconstant 2.5\n"), {"read\tProbe::p1.a\tmodel\tP1::a": "constant 2.5"})
        self.assertEqual(sorted(movement.parse_census(census(**{"A::b": {"ours": 1}}))), ["root\ttraining", "training\tA::b"])
        apijson = json.dumps({"totals": {"files": 3, "elements": {"matched": 5}, "properties": {"oursMissing": 7, "presentDifferent": 2}},
                              "roots": [{"name": "training", "dir": "x", "files": 3, "elements": {}, "properties": {}}],
                              "scorecard": {"Feature.type": [1, 2, 3, 4, 5, 6, 7]}})
        self.assertEqual(sorted(movement.parse_apijson(apijson)), ["root\ttraining", "row\tFeature.type"])
        self.assertEqual(movement.apijson_figures(apijson)["ours-missing"], 7)
        self.assertIsNone(movement.parser_for(GOLDENS))
        self.assertIs(movement.parser_for(CORPUS), movement.parse_tsv)


class RepositoryTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = Repo(self.tmp.name)
        self.base = self.repo.commit({
            CORPUS: corpus(**{"x/a.sysml": 0, "x/b.sysml": 1}),
            CENSUS: census(**{"A::b": {"ours": 1}, "A::c": {"ours": 1}}),
            f"{GOLDENS}/one.golden.ttl": "one\n", f"{GOLDENS}/two.golden.ttl": "two\n",
        }, "base")
        self.repo.git("checkout", "-q", "-b", "feature")
        self.ours = self.repo.commit({CORPUS: corpus(**{"x/a.sysml": 2, "x/b.sysml": 1}), f"{GOLDENS}/one.golden.ttl": "one'\n"}, "ours")
        self.repo.git("checkout", "-q", "develop")
        self.theirs = self.repo.commit({CENSUS: census(**{"A::b": {"ours": 2}, "A::c": {"ours": 1}})}, "theirs")
        self.repo.git("checkout", "-q", "feature")
        self.repo.git("merge", "-q", "--no-ff", "-m", "merge", "develop")
        self.merged = self.repo.git("rev-parse", "HEAD")

    def tearDown(self):
        self.tmp.cleanup()

    def run_script(self, *paths: str) -> tuple[int, str]:
        out = io.StringIO()
        with redirect_stdout(out):
            code = movement.main(["--base", self.base, "--ours", self.ours, "--theirs", self.theirs, "--merged", self.merged, "--tree", self.tmp.name, *paths])
        return code, out.getvalue()

    def test_a_fresh_run_matching_the_merge_moves_nothing(self):
        code, out = self.run_script(CORPUS, CENSUS, GOLDENS)
        self.assertEqual(code, 0)
        self.assertIn("No generated record moved", out)

    def test_a_movement_neither_side_made_stops_the_run(self):
        self.repo.write({CORPUS: corpus(**{"x/a.sysml": 2, "x/b.sysml": 5})})
        code, out = self.run_script(CORPUS, CENSUS, GOLDENS)
        self.assertEqual(code, 1)
        self.assertIn("1 movement(s) the merge does not explain", out)
        self.assertIn("`x/b.sysml`", out)
        self.assertIn("1: 1 → 0", out)
        self.assertIn("5: 0 → 1", out)

    def test_a_regenerated_golden_is_reported_not_attributed(self):
        self.repo.write({f"{GOLDENS}/two.golden.ttl": "two'\n"})
        code, out = self.run_script(GOLDENS)
        self.assertEqual(code, 0)
        self.assertIn(f"`{GOLDENS}`: 1 file(s) regenerated", out)

    def test_the_training_expectation_is_refused(self):
        with self.assertRaises(SystemExit):
            self.run_script(movement.TRAINING)

    def test_the_summary_is_written_where_asked(self):
        path = os.path.join(self.tmp.name, "summary.md")
        out = io.StringIO()
        with redirect_stdout(out):
            movement.main(["--base", self.base, "--ours", self.ours, "--theirs", self.theirs, "--merged", self.merged, "--tree", self.tmp.name, "--summary", path, CORPUS])
        with open(path, encoding="utf-8") as handle:
            self.assertEqual(handle.read(), out.getvalue())


if __name__ == "__main__":
    unittest.main()
