#!/usr/bin/env python3
"""Tests for scripts/release-provenance.py. Run: python3 scripts/release-provenance-test.py"""

from __future__ import annotations

import importlib.util
import json
import pathlib
import tempfile
import unittest
import unittest.mock

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("release_provenance", HERE / "release-provenance.py")
provenance = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(provenance)

A = "a" * 64
B = "b" * 64
COMMIT = "0123456789abcdef0123456789abcdef01234567"

MANIFEST = f"""{A}  sysml-linux-amd64.tar.gz
{B}  opensysml-0.9.0-py3-none-any.whl
"""

ENV = {
    "CIRCLE_TAG": "v0.9.0",
    "CIRCLE_SHA1": COMMIT,
    "CIRCLE_BUILD_URL": "https://circleci.com/gh/Open-MBEE/OpenSysML/1234",
    "CIRCLE_PROJECT_ID": "eeb0dddd-237f-4f02-9e51-8e24caef589d",
    "CIRCLE_ORGANIZATION_ID": "1169df8b-0b59-400f-82d2-c9d8e98bdb62",
    "CIRCLE_WORKFLOW_ID": "wf-1",
    "CIRCLE_JOB": "build-release",
    "CIRCLE_PROJECT_USERNAME": "Open-MBEE",
    "CIRCLE_PROJECT_REPONAME": "OpenSysML",
}


def build(**overrides):
    env = dict(ENV)
    env.update(overrides)
    return provenance.Build.from_env(env)


class SubjectsTest(unittest.TestCase):
    def test_every_manifest_line_is_a_subject_with_its_digest(self):
        self.assertEqual(
            provenance.subjects(MANIFEST),
            [
                {"name": "sysml-linux-amd64.tar.gz", "digest": {"sha256": A}},
                {"name": "opensysml-0.9.0-py3-none-any.whl", "digest": {"sha256": B}},
            ],
        )

    def test_blank_lines_and_binary_mode_markers_are_accepted(self):
        self.assertEqual(
            [s["name"] for s in provenance.subjects(f"\n{A} *x.zip\n\n")], ["x.zip"]
        )

    def test_names_with_spaces_are_kept_whole(self):
        self.assertEqual(provenance.subjects(f"{A}  a b.tar.gz")[0]["name"], "a b.tar.gz")

    def test_an_empty_manifest_is_refused(self):
        with self.assertRaisesRegex(provenance.ProvenanceError, "no artifacts"):
            provenance.subjects("\n")

    def test_a_malformed_line_is_refused(self):
        for line in (f"{A[:63]}  short.tar.gz", f"{A.upper()}  upper.tar.gz", "x.tar.gz", f"{A}"):
            with self.subTest(line=line):
                with self.assertRaisesRegex(provenance.ProvenanceError, "line 1"):
                    provenance.subjects(line)

    def test_a_name_listed_twice_is_refused(self):
        with self.assertRaisesRegex(provenance.ProvenanceError, "twice"):
            provenance.subjects(f"{A}  x\n{B}  x\n")


class BuildTest(unittest.TestCase):
    def test_every_variable_is_required(self):
        for name in provenance.REQUIRED_ENV:
            with self.subTest(name=name):
                with self.assertRaisesRegex(provenance.ProvenanceError, name):
                    build(**{name: ""})

    def test_a_tag_that_is_not_a_release_is_refused(self):
        with self.assertRaisesRegex(provenance.ProvenanceError, "release tag"):
            build(CIRCLE_TAG="develop")

    def test_a_commit_that_is_not_a_full_hash_is_refused(self):
        with self.assertRaisesRegex(provenance.ProvenanceError, "commit hash"):
            build(CIRCLE_SHA1=COMMIT[:7])


class StatementTest(unittest.TestCase):
    def test_the_statement_describes_the_tag_commit_and_job(self):
        got = provenance.statement(MANIFEST, build())
        self.assertEqual(got["_type"], "https://in-toto.io/Statement/v1")
        self.assertEqual(got["predicateType"], "https://slsa.dev/provenance/v1")
        self.assertEqual(len(got["subject"]), 2)
        definition = got["predicate"]["buildDefinition"]
        self.assertEqual(definition["buildType"], provenance.BUILD_TYPE)
        self.assertEqual(
            definition["externalParameters"],
            {
                "repository": "https://github.com/Open-MBEE/OpenSysML",
                "ref": "refs/tags/v0.9.0",
                "job": "build-release",
            },
        )
        self.assertEqual(
            definition["resolvedDependencies"],
            [
                {
                    "uri": "git+https://github.com/Open-MBEE/OpenSysML@refs/tags/v0.9.0",
                    "digest": {"gitCommit": COMMIT},
                }
            ],
        )
        self.assertEqual(
            definition["internalParameters"],
            {
                "organization": ENV["CIRCLE_ORGANIZATION_ID"],
                "project": ENV["CIRCLE_PROJECT_ID"],
                "workflow": "wf-1",
            },
        )
        run = got["predicate"]["runDetails"]
        self.assertEqual(
            run["builder"]["id"],
            "https://circleci.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d",
        )
        self.assertEqual(
            run["metadata"],
            {"invocationId": ENV["CIRCLE_BUILD_URL"]},
        )

    def test_render_is_json_ending_in_a_newline(self):
        text = provenance.render(MANIFEST, build())
        self.assertTrue(text.endswith("}\n"))
        self.assertEqual(json.loads(text), provenance.statement(MANIFEST, build()))


class MainTest(unittest.TestCase):
    def test_writes_the_statement_and_refuses_without_a_build(self):
        with tempfile.TemporaryDirectory() as tmp:
            manifest = pathlib.Path(tmp, "SHA256SUMS.txt")
            manifest.write_text(MANIFEST)
            out = pathlib.Path(tmp, "provenance.intoto.json")
            with unittest.mock.patch.dict("os.environ", ENV, clear=True):
                self.assertEqual(
                    provenance.main(["--manifest", str(manifest), "--out", str(out)]), 0
                )
            self.assertEqual(len(json.loads(out.read_text())["subject"]), 2)
            out.unlink()
            with unittest.mock.patch.dict("os.environ", {}, clear=True):
                self.assertEqual(
                    provenance.main(["--manifest", str(manifest), "--out", str(out)]), 1
                )
            self.assertFalse(out.exists())

    def test_a_missing_manifest_is_an_error_not_a_traceback(self):
        with tempfile.TemporaryDirectory() as tmp:
            out = pathlib.Path(tmp, "out.json")
            with unittest.mock.patch.dict("os.environ", ENV, clear=True):
                self.assertEqual(
                    provenance.main(["--manifest", f"{tmp}/missing", "--out", str(out)]), 1
                )


if __name__ == "__main__":
    unittest.main()
