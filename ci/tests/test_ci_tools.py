#!/usr/bin/env python3
"""Offline tests of the CI throttle tools against a stub `gh` that answers from rules and records every call."""
import base64
import datetime
import importlib
import io
import json
import os
import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from unittest import mock

CI = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, CI)

import infra_rerun  # noqa: E402
import starve_sweep  # noqa: E402
import throttle_common  # noqa: E402
import watcher  # noqa: E402

REPO = "Example/Repo"
FAKE_GH = """#!/usr/bin/env python3
import json, os, re, sys
args = " ".join(sys.argv[1:])
with open(os.environ["FAKE_GH_LOG"], "a") as f:
    f.write(args + "\\n")
for pat, out, rc in json.load(open(os.environ["FAKE_GH_RULES"])):
    if re.search(pat, args):
        sys.stdout.write(out if isinstance(out, str) else json.dumps(out))
        sys.exit(rc)
sys.stderr.write("no rule for: " + args)
sys.exit(1)
"""


def stamp(minutes_ago=0):
    t = datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(minutes=minutes_ago)
    return t.strftime("%Y-%m-%dT%H:%M:%SZ")


def queue_nodes(nodes):
    return {"data": {"repository": {"mergeQueue": {"entries": {"nodes": nodes}}}}}


class Stub(unittest.TestCase):
    """A state directory with a configuration, and `gh` on PATH replaced by the rule-driven stub."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.dir = self.tmp.name
        bindir = os.path.join(self.dir, "bin")
        os.mkdir(bindir)
        with open(os.path.join(bindir, "gh"), "w") as f:
            f.write(FAKE_GH)
        os.chmod(os.path.join(bindir, "gh"), 0o755)
        self.config = os.path.join(self.dir, "config.json")
        self.write("config.json", {"repo": REPO, "cap": 1, "checkout": self.dir})
        self.calls_file = os.path.join(self.dir, "gh.calls")
        self.rules_file = os.path.join(self.dir, "gh.rules")
        self.env = {k: os.environ.get(k) for k in ("PATH", "FAKE_GH_LOG", "FAKE_GH_RULES", "CI_THROTTLE_CONFIG")}
        os.environ.update(PATH=bindir + os.pathsep + os.environ["PATH"], FAKE_GH_LOG=self.calls_file, FAKE_GH_RULES=self.rules_file)
        os.environ.pop("CI_THROTTLE_CONFIG", None)
        self.rules([])
        sleep = mock.patch("time.sleep")
        sleep.start()
        self.addCleanup(sleep.stop)

    def tearDown(self):
        for k, v in self.env.items():
            if v is None:
                os.environ.pop(k, None)
            else:
                os.environ[k] = v
        self.tmp.cleanup()

    def write(self, name, value):
        with open(os.path.join(self.dir, name), "w") as f:
            json.dump(value, f)

    def read(self, name):
        with open(os.path.join(self.dir, name)) as f:
            return json.load(f)

    def text(self, name):
        p = os.path.join(self.dir, name)
        if not os.path.exists(p):
            return ""
        with open(p) as f:
            return f.read()

    def rules(self, rules):
        with open(self.rules_file, "w") as f:
            json.dump(rules, f)

    def calls(self):
        return self.text("gh.calls").splitlines()

    def cfg(self, dry_run=False):
        return throttle_common.Config(self.config, dry_run=dry_run)


class CommonTest(Stub):
    def test_config_defaults_and_state_dir(self):
        cfg = self.cfg()
        self.assertEqual((cfg.owner, cfg.name, cfg.state_dir), ("Example", "Repo", self.dir))
        self.assertEqual(cfg["base_branch"], "develop")
        self.assertEqual(cfg.read("absent.json", [1]), [1])

    def test_config_from_environment_and_bad_repo(self):
        os.environ["CI_THROTTLE_CONFIG"] = self.config
        self.assertEqual(throttle_common.Config().repo, REPO)
        self.write("config.json", {"repo": "no-slash"})
        with self.assertRaises(SystemExit):
            self.cfg()

    def test_protected_branches(self):
        cfg = self.cfg()
        for branch, event in (("gh-readonly-queue/develop/pr-1-abc", "pull_request"), ("x", "merge_group"), ("develop", None),
                              ("main", None), ("release/1.2.0", None), ("hotfix/x", None), ("v0.12.1", None)):
            self.assertTrue(throttle_common.protected_branch(cfg, branch, event), branch)
        for branch in ("feat/x", "fix/v2", "version", None):
            self.assertFalse(throttle_common.protected_branch(cfg, branch, "pull_request"), branch)

    def test_mutations_are_recognised(self):
        for args in (["run", "cancel", "1"], ["run", "rerun", "1"], ["pr", "close", "1"], ["pr", "ready", "1"],
                     ["api", "-X", "POST", "x"], ["api", "--method", "PATCH", "x"]):
            self.assertTrue(throttle_common.mutating(args), args)
        for args in (["run", "list"], ["api", "repos/x"], ["pr", "view", "1"]):
            self.assertFalse(throttle_common.mutating(args), args)

    def test_freeze_and_dry_run_block_mutations_only(self):
        self.rules([["", "{}", 0]])
        msgs = []
        throttle_common.GH(self.cfg(dry_run=True), msgs.append)(["run", "cancel", "1"])
        self.write("freeze.json", {})
        gh = throttle_common.GH(self.cfg(), msgs.append)
        self.assertIsNone(gh(["run", "rerun", "1"]))
        self.assertEqual(gh.json(["api", "repos/x"]), {})
        self.assertEqual(self.calls(), ["api repos/x"])
        self.assertTrue(msgs[0].startswith("DRY RUN") and msgs[1].startswith("FROZEN"), msgs)

    def test_gh_failure_raises_unless_allowed(self):
        msgs = []
        gh = throttle_common.GH(self.cfg(), msgs.append)
        with self.assertRaises(RuntimeError):
            gh(["api", "repos/x"])
        self.assertIsNone(gh(["api", "repos/x"], ok_fail=True))
        self.assertIn("no rule for", msgs[0])

    def test_get_command(self):
        out = io.StringIO()
        with redirect_stdout(out):
            self.assertEqual(throttle_common.main(["--config", self.config, "get", "repo"]), 0)
            self.assertEqual(throttle_common.main(["--config", self.config, "get", "workflows"]), 0)
        self.assertEqual(out.getvalue().splitlines(), [REPO, '["PR"]'])
        with redirect_stderr(io.StringIO()):
            self.assertEqual(throttle_common.main(["get"]), 2)
            self.assertEqual(throttle_common.main(["--config", self.config, "get", "nothing"]), 1)

    def test_shell_tools_read_the_configuration(self):
        self.write("ready.txt", [])
        os.remove(os.path.join(self.dir, "ready.txt"))
        p = subprocess.run([os.path.join(CI, "drycheck.sh"), "--config", self.config], capture_output=True, text=True, timeout=60)
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertIn("no " + self.dir + "/ready.txt", p.stdout)


def pr(n, branch, draft=False, body=""):
    return {"number": n, "title": f"PR {n}", "headRefName": branch, "headRefOid": f"{n:040d}", "author": {"login": "someone"},
            "body": body, "createdAt": stamp(60), "isDraft": draft, "baseRefName": "develop", "mergeable": "MERGEABLE"}


def pull(n, branch):
    return {"number": n, "state": "open", "mergeable_state": "clean", "draft": False, "body": "",
            "head": {"sha": f"{n:040d}", "ref": branch}, "base": {"ref": "develop"}}


def run(rid, n, branch, status="queued"):
    return {"databaseId": rid, "headBranch": branch, "event": "pull_request", "workflowName": "PR", "createdAt": stamp(1),
            "startedAt": stamp(1), "headSha": f"{n:040d}", "status": status, "attempt": 1}


class WatcherTest(Stub):
    def setUp(self):
        super().setUp()
        importlib.reload(watcher)

    def pass_rules(self, runs, mg_runs=(), mq=(), extra=()):
        prs = [pr(7, "feat/seven"), pr(8, "feat/eight")]
        rules = list(extra) + [
            [r"^pr list .*--limit 300", prs, 0],
            [r"^pr list .*-L 200", prs, 0],
            [r"^api repos/Example/Repo/actions/runs\?event=merge_group&per_page=30", {"workflow_runs": list(mg_runs)}, 0],
            [r"^run list .*--event merge_group", [], 0],
            [r"^api graphql", queue_nodes(list(mq)), 0],
            [r"^api repos/Example/Repo/commits/develop --jq", {"s": "d" * 40, "m": "Merge"}, 0],
            [r"^run list --repo Example/Repo --status queued -L 100", runs, 0],
            [r"^run list --repo Example/Repo --status \w+ -L 100", [], 0],
            [r"^run view \d+ --repo Example/Repo --json status,jobs", {"status": "queued", "jobs": [{"name": "Changed areas", "status": "queued"}]}, 0],
            [r"^api repos/Example/Repo/pulls/7$", pull(7, "feat/seven"), 0],
            [r"^api repos/Example/Repo/pulls/8$", pull(8, "feat/eight"), 0],
            [r"^api repos/Example/Repo/actions/runs/102 --jq", {"a": 1, "b": "feat/eight", "e": "pull_request"}, 0],
            [r"^api repos/Example/Repo/actions/runs/102$", {"head_sha": f"{8:040d}", "pull_requests": [{"number": 8}]}, 0],
            [r"^run cancel 102 ", "", 0],
        ]
        self.rules(rules)

    def run_once(self, *flags):
        self.assertEqual(watcher.main(["--config", self.config, "--once", *flags]), 0)
        log = self.text("watch.log")
        self.assertNotIn("ERROR", log)
        return log

    def test_over_cap_run_is_not_cancelled_without_authorization(self):
        self.pass_rules([run(101, 7, "feat/seven"), run(102, 8, "feat/eight")])
        log = self.run_once()
        self.assertIn("#7: new run 101 took a free slot", log)
        self.assertIn("run(s) [102] would be cancelled by the throttle; not cancelled", log)
        self.assertFalse([c for c in self.calls() if c.startswith("run cancel")])
        self.assertEqual(list(self.read("state.json")["holders"]), ["7"])

    def test_authorized_over_cap_run_is_cancelled_and_queued(self):
        self.write("cancel_auth.json", [102])
        self.pass_rules([run(101, 7, "feat/seven"), run(102, 8, "feat/eight")])
        log = self.run_once()
        self.assertIn("run cancel 102 --repo Example/Repo", self.calls())
        self.assertIn("#8: cancelled queued run(s) [102] over the cap", log)
        self.assertEqual([e["pr"] for e in self.read("state.json")["queue"]], [8])

    def test_dry_run_and_freeze_send_no_cancel(self):
        self.write("cancel_auth.json", [102])
        self.pass_rules([run(101, 7, "feat/seven"), run(102, 8, "feat/eight")])
        self.assertIn("DRY RUN: would run gh run cancel 102", self.run_once("--dry-run"))
        os.remove(os.path.join(self.dir, "state.json"))
        self.write("freeze.json", {})
        self.assertIn("FROZEN (freeze.json): skipped gh run cancel 102", self.run_once())
        self.assertFalse([c for c in self.calls() if c.startswith("run cancel")])

    def test_blocked_pr_finished_holder_and_dirty_holder(self):
        self.write("cancel_auth.json", [102, 103])
        self.write("blocked.json", {"prs": [8], "why": "held for a release"})
        self.write("state.json", {"holders": {"7": {"run": 100, "since": 0, "seen_active": True}, "9": {"run": 103, "since": 0}},
                                  "queue": [], "parked": {}, "cancelled": [], "order": []})
        dirty9 = dict(pull(9, "feat/nine"), mergeable_state="dirty")
        self.pass_rules([run(102, 8, "feat/eight"), run(103, 9, "feat/nine")], extra=[
            [r"^pr list .*-L 200", [pr(7, "feat/seven"), pr(8, "feat/eight"), pr(9, "feat/nine")], 0],
            [r"^run view 100 --repo Example/Repo --json status,conclusion$", {"status": "completed", "conclusion": "success"}, 0],
            [r"^run view 100 --repo Example/Repo --json conclusion,status,jobs", {"status": "completed", "conclusion": "success",
                                                                                 "headSha": f"{7:040d}", "attempt": 1, "jobs": []}, 0],
            [r"^api repos/Example/Repo/actions/runs/100/jobs", {"jobs": []}, 0],
            [r"^api repos/Example/Repo/pulls/9$", dirty9, 0],
            [r"^api repos/Example/Repo/actions/runs/103 --jq", {"a": 1, "b": "feat/nine", "e": "pull_request"}, 0],
            [r"^api repos/Example/Repo/actions/runs/103$", {"head_sha": f"{9:040d}", "pull_requests": [{"number": 9}]}, 0],
            [r"^run cancel 103 ", "", 0]])
        log = self.run_once()
        self.assertIn("#8: held for a release; cancelled not-started run(s) [102], not queued", log)
        self.assertIn("#7: run 100 (base unread (develop)) finished GREEN", log)
        self.assertIn("#9: head 00000000 is DIRTY; cancelled its run(s) [103] and parked until a new non-dirty head lands", log)
        self.assertEqual(sorted(c for c in self.calls() if c.startswith("run cancel")),
                         ["run cancel 102 --repo Example/Repo", "run cancel 103 --repo Example/Repo"])

    def test_started_means_a_heavy_job_and_frozen_cancel_is_not_a_cancel(self):
        watcher.configure(self.cfg())
        self.rules([[r"^run view 1 ", {"status": "in_progress", "jobs": [{"name": "Changed areas", "status": "completed", "conclusion": "success"},
                                                                       {"name": "Go", "status": "queued"}, {"name": "Docs", "status": "completed", "conclusion": "skipped"}]}, 0],
                    [r"^run view 2 ", {"status": "in_progress", "jobs": [{"name": "Changed areas", "status": "completed", "conclusion": "success"},
                                                                       {"name": "Go", "status": "in_progress"}]}, 0],
                    [r"^run view 3 ", {"status": "in_progress", "jobs": []}, 0],
                    [r"runs/4 --jq", {"a": 1, "b": "feat/x", "e": "pull_request"}, 0]])
        self.assertEqual([watcher.jobs_started({"databaseId": n}) for n in (1, 2, 3, 9)], [False, True, True, True])
        self.write("freeze.json", {})
        self.assertFalse(watcher.cancel(4))

    def test_cancel_refuses_merge_queue_tag_and_unreadable_runs(self):
        watcher.configure(self.cfg())
        self.rules([[r"runs/1 --jq", {"a": 1, "b": "gh-readonly-queue/develop/pr-5-x", "e": "merge_group"}, 0],
                    [r"runs/2 --jq", {"a": 1, "b": "v1.2.3", "e": "push"}, 0],
                    [r"^run cancel", "", 0]])
        self.assertFalse(watcher.cancel(1))
        self.assertFalse(watcher.cancel(2))
        self.assertFalse(watcher.cancel(3))
        self.assertFalse([c for c in self.calls() if c.startswith("run cancel")])

    def test_github_error_fails_closed(self):
        self.write("cancel_auth.json", [102])
        self.rules([[r"^api graphql", queue_nodes([]), 0]])
        watcher.main(["--config", self.config, "--once"])
        self.assertIn("ERROR", self.text("watch.log"))
        self.assertFalse([c for c in self.calls() if c.startswith(("run cancel", "run rerun", "api -X"))])

    def test_merge_queue_first_holds_authorized_reruns(self):
        self.write("auth_rerun.json", [{"pr": 7, "run": 555, "head": "0000000000"}])
        mg = {"id": 900, "name": "PR", "head_branch": "gh-readonly-queue/develop/pr-5-abc", "head_sha": "e" * 40, "status": "in_progress"}
        self.pass_rules([], mg_runs=[mg], extra=[
            [r"^run list .*--status in_progress --event merge_group", [{"databaseId": 900, "status": "in_progress"}], 0],
            [r"^api repos/Example/Repo/actions/runs/900/jobs", 2, 0]])
        log = self.run_once()
        self.assertIn("MERGE QUEUE: #5 queue build run 900 started", log)
        self.assertIn("merge-queue first: merge_group run 900 has 2 job(s) waiting for runners; no ordinary slot granted", log)
        self.assertFalse([c for c in self.calls() if c.startswith("run rerun")])

    def test_queue_entries_between_builds_hold_reruns(self):
        self.write("auth_rerun.json", [{"pr": 7, "run": 555, "head": "0000000000"}])
        self.pass_rules([], mq=[{"position": 1, "state": "QUEUED", "pullRequest": {"number": 5, "headRefOid": "f" * 40}}])
        log = self.run_once()
        self.assertIn("merge queue holds 1 entry (#5 next) between builds", log)
        self.assertFalse([c for c in self.calls() if c.startswith("run rerun")])

    def test_authorized_rerun_starts_when_the_queue_is_idle_and_waits_under_dry_run_and_freeze(self):
        self.write("auth_rerun.json", [{"pr": 7, "run": 555, "head": "0000000000"}])
        self.pass_rules([], extra=[[r"^run rerun 555 --failed", "", 0]])
        self.assertIn("DRY RUN: authorized re-run of 555 deferred", self.run_once("--dry-run"))
        self.write("freeze.json", {})
        self.run_once()
        self.assertEqual(len(self.read("auth_rerun.json")), 1)
        os.remove(os.path.join(self.dir, "freeze.json"))
        self.assertIn("#7: started authorized re-run (failed jobs only) of run 555", self.run_once())
        self.assertIn("run rerun 555 --failed --repo Example/Repo", self.calls())
        self.assertEqual(self.read("auth_rerun.json"), [])
        os.remove(os.path.join(self.dir, "state.json"))
        self.write("auth_rerun.json", [{"pr": 7, "run": 556, "head": "0000000000", "full": True}])
        self.pass_rules([], extra=[[r"^run rerun 556 --repo", "", 0]])
        self.assertIn("#7: started authorized re-run (all jobs) of run 556", self.run_once())
        self.assertIn("run rerun 556 --repo Example/Repo", self.calls())

    def test_tiers_order_and_gate_the_exemption(self):
        watcher.configure(self.cfg())
        self.assertEqual(watcher.tier_rank(9), (0, 9))
        self.write("tiers.json", {"tiers": [[30, 10], [20]]})
        self.assertEqual(sorted([5, 20, 10, 30, 40], key=watcher.tier_rank), [30, 10, 20, 5, 40])
        self.assertEqual(watcher.tier_members(0, 1), {10, 20, 30})
        self.write("config.json", {"repo": REPO, "exempt_body_tag": "Requested by: @someone"})
        watcher.configure(self.cfg())
        self.assertTrue(watcher.exempt_mark(30, "x\nRequested by: @someone"))
        self.assertFalse(watcher.exempt_mark(20, "Requested by: @someone"))
        self.assertIn("EXEMPTION applied", self.text("watch.log"))


SHA7, SHA8 = f"{7:040d}", f"{8:040d}"
MERGE_LOG = f"2026-01-01T00:00:00Z HEAD is now at abc Merge {SHA7} into {'b' * 40}\n"


class WatcherPartsTest(Stub):
    """The watcher's decisions one function at a time, each against the GitHub answers it reads."""

    def setUp(self):
        super().setUp()
        importlib.reload(watcher)

    def use(self, rules, **config):
        self.write("config.json", dict({"repo": REPO, "cap": 2}, **config))
        watcher.configure(self.cfg())
        self.rules(list(rules) + [
            [r"^api repos/Example/Repo/pulls/7$", pull(7, "feat/seven"), 0],
            [r"^api repos/Example/Repo/pulls/8$", pull(8, "feat/eight"), 0],
            [r"^api repos/Example/Repo/actions/runs/\d+/jobs\?per_page=100$", {"jobs": [{"id": 1, "conclusion": "success"}]}, 0],
            [r"^api repos/Example/Repo/actions/jobs/1/logs", MERGE_LOG, 0],
            [r"^api repos/Example/Repo/compare/b+\.\.\.develop", {"commits": []}, 0],
        ])

    def log(self):
        return self.text("watch.log")

    def view(self, rid, conclusion, jobs=(), head=SHA7):
        return [rf"^run view {rid} --repo Example/Repo --json conclusion,status,jobs", {
            "status": "completed", "conclusion": conclusion, "headSha": head, "attempt": 1, "startedAt": stamp(30),
            "jobs": [{"name": n, "conclusion": c, "databaseId": 50, "steps": []} for n, c in jobs]}, 0]

    def test_finish_holder_reports_each_outcome(self):
        self.use([self.view(1, "success"), self.view(2, "failure", [("Go", "failure"), ("Build and test", "failure")]),
                  self.view(3, "cancelled", [("Go", "cancelled")]),
                  self.view(4, "failure", [("Python client tests", "failure"), ("Build and test", "failure")]),
                  self.view(5, "failure", [("Go", "failure"), ("Build and test", "failure")]),
                  [r"actions/jobs/50/logs", "FAILED tests/test_x.py::test_flaky - boom\n", 0]],
                 known_flakes=[{"job": "Python client tests", "test": "test_flaky", "note": "a known flake"}])
        self.write("fail_ok.json", {"5": {"jobs": ["Go", "Build and test"], "why": "accepted"}})
        st = watcher.load()
        for rid in (1, 2, 3, 4, 5):
            watcher.finish_holder(st, 7, {"run": rid})
        log = self.log()
        self.assertIn(f"#7: run 1 (base {'b' * 10}) finished GREEN", log)
        self.assertIn("run 2 (base bbbbbbbbbb) finished RED (failure); failed jobs: Go, Build and test; dropped", log)
        self.assertIn("run 3 attempt 1 was CANCELLED (not by me) after holding a slot; not re-run (maintainer decides), dropped", log)
        self.assertIn("known flake (a known flake)", log)
        self.assertIn("RED only on Go, Build and test; validated under the authorized exception (accepted)", log)
        watcher.finish_holder(st, 7, {"run": None})
        self.assertIn("no PR run appeared within 15 min", self.log())

    def test_finish_holder_requeues_a_green_draft_and_parks_a_dirty_pr(self):
        self.use([self.view(1, "success"), self.view(2, "success", head=SHA8),
                  [r"^api repos/Example/Repo/pulls/9$", dict(pull(9, "feat/nine"), draft=True), 0],
                  [r"^api repos/Example/Repo/pulls/10$", dict(pull(10, "feat/ten"), mergeable_state="dirty"), 0]])
        st = watcher.load()
        watcher.finish_holder(st, 9, {"run": 1})
        watcher.finish_holder(st, 10, {"run": 1})
        self.assertIn("quick tier only: PR is a draft, not validated); re-queued for un-draft", self.log())
        self.assertIn("PR is dirty at head", self.log())
        self.assertEqual(([e["pr"] for e in st["queue"]], list(st["parked"])), ([9], ["10"]))

    def test_start_next_parks_drops_adopts_and_reopens(self):
        reopen = [r"^api repos/Example/Repo/actions/runs\?branch=feat/seven", {"workflow_runs": [
            {"id": 70, "name": "PR", "status": "completed", "conclusion": "cancelled", "run_attempt": 1, "created_at": stamp(5)}]}, 0]
        self.use([reopen,
                  [r"^api repos/Example/Repo/actions/runs\?branch=feat/eight", {"workflow_runs": [
                      {"id": 80, "name": "PR", "status": "queued", "run_attempt": 1, "created_at": stamp(5)}]}, 0],
                  [r"^api repos/Example/Repo/actions/runs\?branch=", {"workflow_runs": []}, 0],
                  [r"^api repos/Example/Repo/pulls/11$", dict(pull(11, "feat/eleven"), mergeable_state="dirty"), 0],
                  [r"^api repos/Example/Repo/pulls/12$", dict(pull(12, "feat/twelve"), draft=True), 0],
                  [r"^api repos/Example/Repo/pulls/13$", dict(pull(13, "feat/thirteen"), draft=True), 0],
                  [r"^api repos/Example/Repo/pulls/14$", pull(14, "feat/fourteen"), 0],
                  [r"^pr ready 13 ", "", 0],
                  [r"git/ref/pull/7/merge --jq", {"s": "m" * 40}, 0],
                  [r"commits/develop --jq", {"s": "b" * 40}, 0],
                  [r"commits/m+ --jq", {"s": "b" * 40}, 0],
                  [r"contents/.github/workflows/pr.yml\?ref=m+ --jq", {"c": base64.b64encode(b"runs-on: ubuntu-24.04").decode()}, 0],
                  [r"^pr (close|reopen) 7 ", "", 0]],
                 runner_pin={"file": ".github/workflows/pr.yml", "require": "ubuntu-24.04", "forbid": "ubuntu-latest"})
        st = watcher.load()
        st["queue"] = [{"pr": n, "run": None, "kind": "rerun"} for n in (11, 12, 14, 8)]
        watcher.READY_OK.add(13)
        st["queue"].append({"pr": 13, "run": None, "kind": "ready"})
        for _ in range(3):
            watcher._start_next(st)
        log = self.log()
        self.assertIn("#11: merge conflicts (dirty)", log)
        self.assertIn("#12: draft (not approved to un-draft)", log)
        self.assertIn("#14: no PR run on current head", log)
        self.assertIn("#8: run 80 already active on current head; holding slot", log)
        self.assertIn("#13: slot granted, marked ready for review (gh pr ready)", log)
        self.assertEqual(sorted(st["holders"]), ["13", "8"])
        st["queue"] = [{"pr": 7, "run": 70, "kind": "rerun"}]
        watcher._start_next(st)
        self.assertIn("waiting for an authorized close/reopen", self.log())
        self.write("reopen_ok.json", [7])
        watcher._start_next(st)
        self.assertIn("closed and reopened (authorized)", self.log())
        self.assertEqual([c for c in self.calls() if c.startswith("pr ")], ["pr ready 13 --repo Example/Repo",
                                                                           "pr close 7 --repo Example/Repo", "pr reopen 7 --repo Example/Repo"])
        self.assertEqual(self.read("reopen_ok.json"), [])

    def test_review_threads_beyond_one_page_are_not_clear(self):
        self.use([[r"^api graphql", {"data": {"repository": {"pullRequest": {"reviewThreads": {
            "totalCount": 101, "nodes": [{"isResolved": True}] * 100}}}}}, 0]])
        self.assertFalse(watcher.review_clear(7, SHA7))
        self.assertIn("101 review threads, more than one page", self.log())

    def test_unreadable_pr_keeps_its_authorized_rerun_and_dry_run_keeps_reopen(self):
        self.write("auth_rerun.json", [{"pr": 99, "run": 5, "head": "0"}])
        self.use([], cap=2)
        self.assertIsNone(watcher.start_authorized(watcher.load()))
        self.assertEqual(len(self.read("auth_rerun.json")), 1)
        self.assertIn("PR unreadable; authorized re-run of 5 kept", self.log())
        self.write("reopen_ok.json", [7])
        self.use([[r"^api repos/Example/Repo/actions/runs\?branch=feat/seven", {"workflow_runs": [
            {"id": 70, "name": "PR", "status": "completed", "conclusion": "cancelled", "run_attempt": 1, "created_at": stamp(5)}]}, 0],
                  [r"git/ref/pull/7/merge --jq", {"s": "m" * 40}, 0], [r"commits/develop --jq", {"s": "b" * 40}, 0],
                  [r"commits/m+ --jq", {"s": "b" * 40}, 0]])
        watcher.CFG.dry_run = True
        st = watcher.load()
        st["queue"] = [{"pr": 7, "run": 70, "kind": "rerun"}]
        watcher._start_next(st)
        self.assertEqual((self.read("reopen_ok.json"), [e["pr"] for e in st["queue"]]), ([7], [7]))
        self.assertFalse([c for c in self.calls() if c.startswith("pr ")])

    def test_review_gate_and_gate_sweep(self):
        self.write("review_gate.json", {"since": "2026-01-01"})
        self.write("release.json", [{"pr": 7, "sha": "0000"}])
        self.write("cancel_auth.json", [102])
        self.use([[r"^api graphql", {"data": {"repository": {"pullRequest": {"reviewThreads": {"nodes": [{"isResolved": True}]}}}}}, 0],
                  [r"runs\?event=pull_request&status=queued", {"workflow_runs": [
                      {"id": 102, "name": "PR", "head_sha": SHA8, "pull_requests": [{"number": 8, "base": {"ref": "develop"}}]}]}, 0],
                  [r"runs\?event=pull_request&status=in_progress", {"workflow_runs": []}, 0],
                  [r"^run view 102 --repo Example/Repo --json status,jobs", {"status": "in_progress", "jobs": [
                      {"name": "Changed areas", "status": "in_progress"}, {"name": "Go", "status": "queued"}]}, 0],
                  [r"^api repos/Example/Repo/actions/runs/102 --jq", {"a": 1, "b": "feat/eight", "e": "pull_request"}, 0],
                  [r"^api repos/Example/Repo/actions/runs/102$", {"head_sha": SHA8, "pull_requests": [{"number": 8}]}, 0],
                  [r"^run cancel 102 ", "", 0]])
        self.assertTrue(watcher.review_gate(7))
        self.assertFalse(watcher.review_gate(8))
        st = watcher.load()
        self.assertTrue(watcher.gate_sweep(st))
        self.assertIn("#8: head 0000000000 not released; run 102 caught with only the preflight running", self.log())
        self.assertEqual([e["pr"] for e in st["queue"]], [8])

    def test_holds_and_caps(self):
        self.use([[r"^run view 300 --repo Example/Repo --json status", {"status": "in_progress", "conclusion": None}, 0],
                  [r"^run view 301 --repo Example/Repo --json status", {"status": "completed", "conclusion": "success"}, 0]])
        self.write("priority_hold_7.json", {"pr": 7, "run": 300, "head": SHA7, "label": "#7 run"})
        self.assertTrue(watcher.priority_hold())
        self.write("priority_hold_7.json", {"pr": 7, "run": 301, "head": SHA7, "label": "#7 run"})
        self.assertFalse(watcher.priority_hold())
        self.assertIn("#7 run 301 completed: success; its runner-priority hold lifted", self.log())
        st = watcher.load()
        self.write("wide_cap.json", {"prs": [7], "cap": 4, "seen": {}, "done": []})
        st["holders"] = {"7": {"run": 5}}
        self.assertEqual(watcher.solo_cap(st), 4)
        st["holders"] = {}
        self.assertEqual(watcher.solo_cap(st), 2)
        self.write("solo.json", {"pr": 7, "run": None})
        self.assertEqual(watcher.solo_cap(st), 1)
        st["holders"] = {"7": {"run": 301}}
        self.assertEqual(watcher.solo_cap(st), 1)
        st["holders"] = {}
        self.assertEqual(watcher.solo_cap(st), 2)
        self.assertIn("sole-priority hold ended", self.log())
        self.write("freeze.json", {})
        self.assertEqual(watcher.solo_cap(st), 0)

    def test_stale_required_check_is_rerun_once_nothing_runs_on_the_head(self):
        self.use([[r"^api repos/Example/Repo/pulls/7$", dict(pull(7, "feat/seven"), auto_merge={"merge_method": "merge"}), 0],
                  [r"check-runs\?check_name=Build%20and%20test", {"check_runs": [
                      {"status": "completed", "conclusion": "failure", "details_url": "https://x/actions/runs/401/job/1"}]}, 0],
                  [r"runs\?head_sha=0+7&per_page=50 --jq", [], 0],
                  [r"^run view 400 --repo Example/Repo --json conclusion", {"conclusion": "success"}, 0],
                  [r"^run rerun 401 --failed", "", 0]])
        st = watcher.load()
        st["stale_bt"] = {"7": {"head": SHA7, "green": 400}}
        watcher.stale_bt(st)
        self.assertIn("rerun --failed started (cancel-protected, runner-priority hold)", self.log())
        self.assertEqual((self.read("ext_ok.json"), self.read("priority_hold_7.json")["run"]), ([401], 401))

    def test_ci_priority_admits_flags_and_follows_tiers(self):
        self.use([[r"^pr view 7 .*files,state", {"state": "OPEN", "files": [{"path": "a"}]}, 0],
                  [r"^pr view 8 .*files,state", {"state": "OPEN", "files": [{"path": "a"}]}, 0],
                  [r"^pr view 9 .*--json files$", {"files": [{"path": ".github/workflows/pr.yml"}]}, 0],
                  [r"^pr view 9 .*files,state", {"state": "OPEN", "files": [{"path": ".github/workflows/pr.yml"}]}, 0]],
                 ci_priority={"after_pr": 5, "title_re": "^ci(\\(|:|!)", "path_re": "^\\.github/workflows/"})

        def p(n, title):
            return {"number": n, "title": title, "baseRefName": "develop", "headRefOid": f"{n:040d}"}
        self.assertTrue(watcher.ci_mark(p(7, "ci: faster")))
        self.assertFalse(watcher.ci_mark(p(8, "ci(x): overlap")))
        self.assertTrue(watcher.ci_mark(p(9, "touch the workflow")))
        self.assertFalse(watcher.ci_mark(p(4, "ci: too old")))
        self.assertEqual((self.read("ci_prio.json"), self.read("ci_flagged.json")), ([7, 9], [8]))
        self.write("tiers.json", {"tiers": [[1], [9]]})
        self.assertFalse(watcher.ci_mark(p(7, "ci: faster")))
        self.assertTrue(watcher.ci_mark(p(9, "touch the workflow")))

    def test_merge_queue_reporting(self):
        self.use([[r"runs\?event=merge_group&per_page=30", {"workflow_runs": [
            {"id": 900, "name": "PR", "head_branch": "gh-readonly-queue/develop/pr-5-x", "head_sha": "e" * 40, "status": "completed", "conclusion": "failure"},
            {"id": 901, "name": "PR", "head_branch": "gh-readonly-queue/develop/pr-6-x", "head_sha": "f" * 40, "status": "completed", "conclusion": "success"}]}, 0],
                  [r"^run view 900 --repo Example/Repo --json jobs", {"jobs": [{"name": "Go", "conclusion": "failure", "steps": [{"name": "Test", "conclusion": "failure"}]}]}, 0],
                  [r"^api graphql", queue_nodes([]), 0]])
        st = watcher.load()
        st["mq"] = {"runs": {"900": "active", "901": "active"}, "entries": ["5@eeeeeeeeee"]}
        self.assertEqual(watcher.mq_track(st), 0)
        log = self.log()
        self.assertIn("MERGE QUEUE: #5 queue build run 900 RED: Go [Test]", log)
        self.assertIn("MERGE QUEUE: #6 queue build run 901 GREEN", log)
        self.assertIn("MERGE QUEUE: #5 eeeeeeeeee left the queue", log)

    def test_cancel_paths_and_small_gates(self):
        self.write("cancel_auth.json", [102])
        self.write("release_hold.json", {"reason": "release in progress", "exempt": []})
        self.use([[r"^api repos/Example/Repo/actions/runs/102 --jq", {"a": 1, "b": "feat/eight", "e": "pull_request"}, 0],
                  [r"^run cancel 102 ", "", 0],
                  [r"runs/9/jobs\?per_page=1 --jq", 0, 0],
                  [r"^run list --repo Example/Repo --status queued -L 50", [{"databaseId": 555, "workflowName": "PR", "event": "pull_request"}], 0]])
        st = watcher.load()
        g = [{"databaseId": 102, "workflowName": "PR", "headSha": SHA8}]
        self.assertTrue(watcher.hold_cancel(st, "8", g))
        self.assertIn("started during the release hold; cancelled and requeued at 0000000000 (release in progress)", self.log())
        self.assertTrue(watcher.ghost({"databaseId": 9, "createdAt": stamp(20), "status": "queued"}))
        self.write("outage.json", {"probes": ["file:///nonexistent/ci-throttle-probe"]})
        self.assertIsNone(watcher.start_next(st))
        self.assertIn("no slot granted", self.log())
        os.remove(os.path.join(self.dir, "outage.json"))
        self.assertIsNone(watcher.start_next(st))
        self.assertIn("release hold (release_hold.json): release in progress", self.log())
        watcher.wait_for_new(st)
        self.assertIn("fast poll: new queued run(s) [555]; running a pass now", self.log())


class StarveSweepTest(Stub):
    def test_cancels_only_unstarted_ordinary_runs_with_jobs(self):
        self.write("starve_cancel.json", {"by": "maintainer", "rule": "the merge queue never waits for runners"})
        self.write("tiers.json", {"tiers": [[9]]})
        self.write("auth_rerun.json", [{"pr": 9, "run": 1, "head": "a"}])

        def r(rid, n, branch, name="PR"):
            return {"id": rid, "event": "pull_request", "name": name, "head_branch": branch, "head_sha": f"{rid:040d}",
                    "pull_requests": [{"number": n}]}
        queued = [r(11, 4, "feat/a"), r(12, 5, "feat/b"), r(13, 6, "release/1.0.0"), r(14, 7, "feat/c", "Docs"), r(15, 8, "feat/d"),
                  r(16, 10, "feat/e")]
        self.rules([
            [r"event=merge_group&status=queued", {"workflow_runs": [{"id": 900, "status": "queued"}]}, 0],
            [r"event=merge_group&status=", {"workflow_runs": []}, 0],
            [r"runs/900/jobs", {"jobs": []}, 0],
            [r"event=pull_request&status=queued", {"workflow_runs": queued}, 0],
            [r"event=pull_request&status=", {"workflow_runs": []}, 0],
            [r"runs/11/jobs", {"jobs": [{"status": "queued"}]}, 0],
            [r"runs/12/jobs", {"jobs": [{"status": "in_progress"}]}, 0],
            [r"runs/15/jobs", {"jobs": []}, 0],
            [r"runs/16/jobs", {"jobs": [{"name": "Changed areas", "status": "completed", "conclusion": "success"}, {"name": "Go", "status": "queued"}]}, 0],
            [r"^run cancel 1[16] ", "", 0],
        ])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual([c for c in self.calls() if c.startswith("run cancel")], ["run cancel 11 -R Example/Repo", "run cancel 16 -R Example/Repo"])
        self.assertEqual([e["pr"] for e in self.read("auth_rerun.json")], [9, 4, 10])

    def starved_rules(self, mg_age, mg_jobs, cancel_fails=()):
        self.write("starve_cancel.json", {"by": "maintainer", "rule": "keep the merge PR run"})
        self.write("tiers.json", {"tiers": [[6]]})

        def r(rid, n):
            return {"id": rid, "event": "pull_request", "name": "PR", "head_branch": f"feat/{n}", "head_sha": f"{rid:040d}",
                    "pull_requests": [{"number": n}]}

        def jobs(*sts):
            return {"jobs": [{"name": f"j{i}", "status": s, "conclusion": "success" if s == "completed" else None} for i, s in enumerate(sts)]}
        mg = {"id": 900, "status": "in_progress", "created_at": stamp(mg_age), "head_branch": "gh-readonly-queue/develop/pr-1-x"}
        rules = [[rf"^run cancel {rid} ", "boom", 1] for rid in cancel_fails] + [
            [r"event=merge_group&status=in_progress", {"workflow_runs": [mg]}, 0],
            [r"event=merge_group&status=", {"workflow_runs": []}, 0],
            [r"runs/900/jobs", {"jobs": mg_jobs}, 0],
            [r"event=pull_request&status=in_progress", {"workflow_runs": [r(21, 4), r(22, 5), r(23, 6), r(24, 7)]}, 0],
            [r"event=pull_request&status=", {"workflow_runs": []}, 0],
            [r"runs/21/jobs", jobs("in_progress", "queued", "queued"), 0],
            [r"runs/22/jobs", jobs("completed", "in_progress"), 0],
            [r"runs/23/jobs", jobs("in_progress", "queued"), 0],
            [r"runs/24/jobs", jobs("completed", "in_progress", "queued"), 0],
            [r"^run cancel \d+ ", "", 0]]
        self.rules(rules)

    def cancels(self):
        return [c.split()[2] for c in self.calls() if c.startswith("run cancel")]

    def test_starved_merge_group_keeps_the_run_closest_to_finishing(self):
        self.starved_rules(10, [{"status": "queued"}, {"status": "queued"}])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(sorted(self.cancels()), ["21", "23", "24"])
        self.assertEqual([e["pr"] for e in self.read("auth_rerun.json")], [6, 4, 7])
        self.assertIn("merge group starved: keeping run 22 (#5, 1 job(s) unfinished)", self.text("watch.log"))

    def test_group_with_only_its_preflight_done_is_starved(self):
        self.starved_rules(10, [{"name": "Changed areas", "status": "completed", "conclusion": "success"}, {"name": "Go", "status": "queued"}])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(sorted(self.cancels()), ["21", "23", "24"])

    def test_waiting_group_within_the_grace_period_leaves_started_runs_alone(self):
        self.starved_rules(10, [{"status": "in_progress", "created_at": stamp(10)}, {"status": "queued", "created_at": stamp(1)}])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.starved_rules(10, [])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(self.cancels(), [])

    def test_a_job_queued_past_the_grace_period_starves_a_partly_started_group(self):
        self.starved_rules(10, [{"status": "in_progress", "created_at": stamp(10)}, {"status": "queued", "created_at": stamp(5)}])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(sorted(self.cancels()), ["21", "23", "24"])

    def test_failed_starvation_cancel_is_not_queued(self):
        self.starved_rules(10, [{"status": "queued"}], cancel_fails=(23,))
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual([e["pr"] for e in self.read("auth_rerun.json")], [4, 7])
        self.assertIn("#6: run 23", self.text("watch.log"))
        self.assertIn("merge group starved) not cancelled", self.text("watch.log"))

    def test_idle_queue_api_error_and_absent_authorization(self):
        self.rules([[r"event=merge_group", {"workflow_runs": []}, 0]])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertIn("starve_cancel.json absent", self.text("watch.log"))
        self.write("starve_cancel.json", {"by": "maintainer"})
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.rules([])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertIn("error, nothing more cancelled this pass", self.text("watch.log"))
        self.assertFalse([c for c in self.calls() if c.startswith("run cancel")])


class InfraRerunTest(Stub):
    def view(self, attempt=1, conclusion="failure", step="Install Z3", done=None):
        done = done or stamp(30)
        return {"status": "completed", "conclusion": conclusion, "attempt": attempt, "headSha": "a" * 40,
                "jobs": [{"name": "Go", "databaseId": 5, "conclusion": "failure" if conclusion == "failure" else "success", "completedAt": done,
                          "steps": [{"name": step, "conclusion": "failure" if conclusion == "failure" else "success", "completedAt": done}]}]}

    def test_reruns_an_infra_failure_after_recovery_and_respects_freeze(self):
        self.write("infra_runs.json", [{"run": 77, "pr": 3, "head": "a" * 40, "hold": True}])
        self.write("freeze.json", {})
        self.rules([[r"^run view 77", self.view(), 0], [r"^run rerun 77", "", 0]])
        self.assertEqual(infra_rerun.main(["--config", self.config, "--once"]), 0)
        self.assertIn("rerun --failed of 77 deferred", self.text("infra.out"))
        os.remove(os.path.join(self.dir, "freeze.json"))
        self.assertEqual(infra_rerun.main(["--config", self.config, "--once"]), 0)
        self.assertIn("run rerun 77 --repo Example/Repo --failed", self.calls())
        self.assertEqual(self.read("priority_hold_3.json")["run"], 77)

    def test_red_and_green_runs_are_never_rerun(self):
        self.write("infra_runs.json", [{"run": 78, "pr": 3, "head": "a" * 40}, {"run": 79, "pr": 4, "head": "a" * 40}])
        self.rules([[r"^run view 78", self.view(step="Run tests"), 0], [r"^run view 79", self.view(conclusion="success"), 0],
                    [r"actions/jobs/5/logs", "FAIL x\n", 0]])
        self.assertEqual(infra_rerun.main(["--config", self.config]), 0)
        out = self.text("infra.out")
        self.assertIn("FINAL #3 78", out)
        self.assertIn("DONE #4 79 attempt 1: GREEN", out)
        self.assertIn("all tracked runs settled", out)
        self.assertFalse([c for c in self.calls() if c.startswith("run rerun")])


if __name__ == "__main__":
    unittest.main()
