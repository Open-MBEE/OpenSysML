#!/usr/bin/env python3
"""Tests for scripts/ci-verified-tree.py. Run: python3 scripts/ci-verified-tree-test.py"""

from __future__ import annotations

import contextlib
import importlib.util
import io
import os
import pathlib
import unittest
import unittest.mock
import urllib.error

HERE = pathlib.Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("ci_verified_tree", HERE / "ci-verified-tree.py")
verified = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(verified)

SHA = "12c4c3983bb4ee471cbb7371eb44b5d28475838d"
OTHER = "1362cf6237df6e839ad42c0060c136873a3015d7"


def check_run(name, suite, sha=SHA, conclusion="success", status="completed", app="github-actions"):
    return {
        "name": name,
        "status": status,
        "conclusion": conclusion,
        "head_sha": sha,
        "app": {"slug": app},
        "check_suite": {"id": suite},
    }


def full_tier(suite):
    return [check_run("Build and test", suite), check_run("Changed areas", suite)] + [
        check_run(f"Go race tests ({shard})", suite) for shard in ("runtime", "model", "cli")
    ]


def quick_tier(suite):
    return [check_run("Build and test", suite), check_run("Changed areas", suite)] + [
        check_run(f"Go tests ({shard})", suite) for shard in ("runtime", "model export graph")
    ]


def github(runs):
    """A get_json stand-in answering the check-runs endpoints from one list of runs."""

    def get_json(url, headers):
        assert f"/commits/{SHA}/check-runs" in url, url
        selected = runs
        if "check_name=Build+and+test" in url:
            selected = [run for run in runs if run["name"] == "Build and test"]
        return {"total_count": len(selected), "check_runs": selected}

    return get_json


def run(argv, get_json=None, env=None):
    out, err = io.StringIO(), io.StringIO()
    patches = [contextlib.redirect_stdout(out), contextlib.redirect_stderr(err),
               unittest.mock.patch.dict(os.environ, env or {}, clear=True)]
    if get_json is not None:
        patches.append(unittest.mock.patch.object(verified, "get_json", get_json))
    with contextlib.ExitStack() as stack:
        for patch in patches:
            stack.enter_context(patch)
        out.reconfigure = lambda **kwargs: None  # redirected stdout has no buffering to set
        code = verified.main(argv)
    return code, out.getvalue(), err.getvalue()


class GitHubEvidence(unittest.TestCase):
    def test_full_tier_pass_is_verified(self):
        code, out, _ = run(["--sha", SHA], github(full_tier(7)))
        self.assertEqual(code, 0)
        self.assertTrue(out.startswith("verified: GitHub Actions check suite 7 passed 'Build and test'"), out)
        self.assertIn("3 'Go race tests (…)' shards", out)

    def test_quick_tier_pass_alone_is_unverified(self):
        code, out, err = run(["--sha", SHA], github(quick_tier(7)))
        self.assertEqual(code, 0)
        self.assertTrue(out.startswith("unverified: no green full-tier run"), out)
        self.assertIn("quick-tier run", err)

    def test_quick_tier_pass_does_not_borrow_another_suites_shards(self):
        # The quick suite passed; the full one on the same commit ran its shards but failed.
        runs = quick_tier(7) + [check_run("Build and test", 8, conclusion="failure")] + [
            check_run(f"Go race tests ({shard})", 8) for shard in ("runtime", "model")
        ]
        _, out, _ = run(["--sha", SHA], github(runs))
        self.assertTrue(out.startswith("unverified:"), out)

    def test_full_tier_pass_beside_a_quick_one_is_verified(self):
        _, out, _ = run(["--sha", SHA], github(quick_tier(7) + full_tier(8)))
        self.assertTrue(out.startswith("verified: GitHub Actions check suite 8 "), out)

    def test_incomplete_failed_or_foreign_aggregate_is_unverified(self):
        for broken in (
            check_run("Build and test", 7, status="in_progress", conclusion=None),
            check_run("Build and test", 7, conclusion="failure"),
            check_run("Build and test", 7, conclusion="skipped"),
            check_run("Build and test", 7, app="some-other-app"),
            check_run("Build and test", 7, sha=OTHER),
        ):
            runs = [broken] + full_tier(7)[1:]
            _, out, _ = run(["--sha", SHA], github(runs))
            self.assertTrue(out.startswith("unverified:"), (broken, out))

    def test_no_check_runs_is_unverified(self):
        _, out, _ = run(["--sha", SHA], github([]))
        self.assertTrue(out.startswith("unverified: no green full-tier run"), out)

    def test_an_api_error_is_unverified(self):
        def failing(url, headers):
            raise urllib.error.HTTPError(url, 403, "rate limit exceeded", {}, None)

        code, out, _ = run(["--sha", SHA], failing)
        self.assertEqual(code, 0)
        self.assertTrue(out.startswith("unverified: could not read the evidence"), out)
        self.assertIn("rate limit exceeded", out)

    def test_a_malformed_answer_is_unverified(self):
        _, out, _ = run(["--sha", SHA], lambda url, headers: {"unexpected": True})
        self.assertTrue(out.startswith("unverified: could not read the evidence"), out)

    def test_rehearsal_never_reads_the_evidence(self):
        def forbidden(url, headers):
            raise AssertionError("a rehearsal must not consult the APIs")

        _, out, _ = run(["--sha", SHA, "--tag", "v1.0.0", "--rehearsal"], forbidden)
        self.assertTrue(out.startswith("unverified: release rehearsal"), out)

    def test_a_short_sha_is_a_usage_error(self):
        with self.assertRaises(SystemExit) as raised:
            run(["--sha", SHA[:10]])
        self.assertEqual(raised.exception.code, 2)

    def test_token_is_sent_when_present(self):
        seen = {}

        def get_json(url, headers):
            seen.update(headers)
            return {"total_count": 0, "check_runs": []}

        run(["--sha", SHA], get_json, env={"GH_TOKEN": "t0ken"})
        self.assertEqual(seen.get("Authorization"), "Bearer t0ken")


def circleci(pipelines, workflows, github_runs=()):
    """A get_json stand-in: GitHub without a full-tier pass, CircleCI from the given pages."""

    def get_json(url, headers):
        if url.startswith(verified.GITHUB_API):
            return github(list(github_runs))(url, headers)
        self_check = headers.get("Circle-Token")
        assert self_check == "cci-token", headers
        if "/pipeline?" in url:
            page = 1 if "page-token" not in url else 2
            items, next_token = pipelines[page - 1]
            return {"items": items, "next_page_token": next_token}
        for pipeline_id, items in workflows.items():
            if f"/pipeline/{pipeline_id}/workflow" in url:
                return {"items": items}
        raise AssertionError(url)

    return get_json


def pipeline(pipeline_id, number, sha):
    return {"id": pipeline_id, "number": number, "trigger_parameters": {"github_app": {"commit_sha": sha}}}


class CircleCIEvidence(unittest.TestCase):
    ENV = {"CIRCLECI_API_TOKEN": "cci-token"}

    def test_green_build_test_on_the_commit_verifies_a_tag(self):
        pages = [([pipeline("p1", 10, OTHER)], "next"), ([pipeline("p2", 9, SHA), pipeline("p3", 8, SHA)], None)]
        flows = {"p2": [{"name": "release", "status": "success"}], "p3": [{"name": "build-test", "status": "success"}]}
        _, out, _ = run(["--sha", SHA, "--tag", "v1.0.0"], circleci(pages, flows), env=self.ENV)
        self.assertEqual(out.strip(), f"verified: CircleCI pipeline 8 on main passed its build-test workflow on {SHA}")

    def test_running_or_failed_build_test_is_unverified(self):
        for status in ("running", "failed", "canceled"):
            pages = [([pipeline("p1", 10, SHA)], None)]
            flows = {"p1": [{"name": "build-test", "status": status}]}
            _, out, _ = run(["--sha", SHA, "--tag", "v1.0.0"], circleci(pages, flows), env=self.ENV)
            self.assertTrue(out.startswith("unverified: no green full-tier run"), (status, out))

    def test_circleci_counts_only_for_a_tag(self):
        pages = [([pipeline("p1", 10, SHA)], None)]
        flows = {"p1": [{"name": "build-test", "status": "success"}]}
        _, out, _ = run(["--sha", SHA], circleci(pages, flows), env=self.ENV)
        self.assertTrue(out.startswith("unverified:"), out)

    def test_without_a_token_only_github_counts(self):
        def no_circleci(url, headers):
            assert url.startswith(verified.GITHUB_API), url
            return {"total_count": 0, "check_runs": []}

        _, out, err = run(["--sha", SHA, "--tag", "v1.0.0"], no_circleci)
        self.assertTrue(out.startswith("unverified:"), out)
        self.assertIn("CIRCLECI_API_TOKEN is not set", err)

    def test_a_circleci_error_is_unverified(self):
        def failing(url, headers):
            if url.startswith(verified.GITHUB_API):
                return {"total_count": 0, "check_runs": []}
            raise urllib.error.HTTPError(url, 401, "Unauthorized", {}, None)

        _, out, _ = run(["--sha", SHA, "--tag", "v1.0.0"], failing, env=self.ENV)
        self.assertTrue(out.startswith("unverified: could not read the evidence"), out)


if __name__ == "__main__":
    unittest.main()
