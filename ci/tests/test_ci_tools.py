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
import stack_monitor  # noqa: E402
import starve_sweep  # noqa: E402
import throttle_common  # noqa: E402
import watcher  # noqa: E402

REPO = "Example/Repo"
FAKE_GH = """#!/usr/bin/env python3
import json, os, re, sys
args = " ".join(sys.argv[1:])
with open(os.environ["FAKE_GH_LOG"], "a") as f:
    f.write(args + "\\n")
for rule in json.load(open(os.environ["FAKE_GH_RULES"])):
    pat, out, rc = rule[:3]
    if not re.search(pat, args):
        continue
    if len(rule) > 3:
        mark = os.path.join(os.path.dirname(os.environ["FAKE_GH_LOG"]), rule[3])
        if os.path.exists(mark):
            continue
        open(mark, "w").close()
    (sys.stdout if rc == 0 else sys.stderr).write(out if isinstance(out, str) else json.dumps(out))
    sys.exit(rc)
sys.stderr.write("no rule for: " + args)
sys.exit(1)
"""
FAKE_GIT = FAKE_GH.replace("FAKE_GH_", "FAKE_GIT_").replace('args = " ".join(sys.argv[1:])', 'args = " ".join(sys.argv[3:])')


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
        self.sleep = sleep.start()
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

    def test_auth_failure_is_retried_once_then_fails_closed(self):
        token = os.path.join(self.dir, "token")
        with open(token, "w") as f:
            f.write("old\n")
        cfg = throttle_common.Config(self.config)
        cfg.values["gh_token_file"] = token
        g = throttle_common.GH(cfg, lambda m: None)
        self.rules([[r"^run list", "HTTP 401: Bad credentials (https://api.github.com/graphql)", 1, "first"], [r"^run list", "[]", 0]])
        with mock.patch.object(throttle_common.subprocess, "run", wraps=subprocess.run) as run:
            self.assertEqual(g.json(["run", "list"]), [])
            self.assertEqual([c.kwargs["env"]["GH_TOKEN"] for c in run.call_args_list], ["old", "old"])
        # subprocess.run polls a slow child through the same patched time.sleep, so this is not always the last call.
        self.sleep.assert_any_call(5)
        self.assertEqual(self.calls(), ["run list", "run list"])
        self.rules([[r"^run cancel", "To get started with GitHub CLI, please run:  gh auth login", 1]])
        self.assertIsNone(g(["run", "cancel", "1"], ok_fail=True))
        self.assertEqual(self.calls().count("run cancel 1"), 2)
        self.rules([[r"^run cancel", "HTTP 401: Bad credentials", 1]])
        self.sleep.side_effect = lambda _: self.write("freeze.json", {"reason": "outage"})
        self.assertIsNone(g(["run", "cancel", "2"], ok_fail=True))
        self.assertEqual(self.calls().count("run cancel 2"), 1)
        os.remove(os.path.join(self.dir, "freeze.json"))
        self.rules([[r"^run view", "HTTP 404: Not Found", 1]])
        self.assertIsNone(g(["run", "view", "1"], ok_fail=True))
        self.assertEqual(self.calls().count("run view 1"), 1)

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

    def test_an_active_build_with_nothing_waiting_admits_reruns_up_to_the_cap(self):
        self.write("config.json", {"repo": REPO, "cap": 3, "checkout": self.dir})
        self.write("tiers.json", {"tiers": [[8]]})
        self.write("auth_rerun.json", [{"pr": 7, "run": 555, "head": "0000000000"}, {"pr": 8, "run": 556, "head": "0000000000"}])
        mg = {"id": 900, "name": "PR", "head_branch": "gh-readonly-queue/develop/pr-5-abc", "head_sha": "e" * 40, "status": "in_progress"}
        mq_rules = [[r"^run list .*--status in_progress --event merge_group", [{"databaseId": 900, "status": "in_progress"}], 0],
                    [r"^api repos/Example/Repo/actions/runs/900/jobs", 0, 0], [r"^run rerun 55[56] --failed", "", 0]]
        self.pass_rules([], mg_runs=[mg], extra=mq_rules)
        log = self.run_once()
        self.assertNotIn("merge-queue first", log)
        self.assertEqual([c.split()[2] for c in self.calls() if c.startswith("run rerun")], ["556", "555"])
        self.assertEqual(self.read("auth_rerun.json"), [])

    def test_an_unreadable_merge_queue_caps_admission_at_one(self):
        self.write("config.json", {"repo": REPO, "cap": 3, "checkout": self.dir})
        self.write("auth_rerun.json", [{"pr": 7, "run": 555, "head": "0000000000"}, {"pr": 8, "run": 556, "head": "0000000000"}])
        self.pass_rules([], extra=[[r"^api repos/Example/Repo/actions/runs\?event=merge_group&per_page=30", "HTTP 502", 1],
                                   [r"^run rerun 55[56] --failed", "", 0]])
        self.assertEqual(watcher.main(["--config", self.config, "--once"]), 0)
        self.assertEqual([c.split()[2] for c in self.calls() if c.startswith("run rerun")], ["555"])
        self.assertEqual(self.read("auth_rerun.json"), [{"pr": 8, "run": 556, "head": "0000000000"}])

    def test_queue_entries_between_builds_hold_reruns(self):
        self.write("auth_rerun.json", [{"pr": 7, "run": 555, "head": "0000000000"}])
        self.pass_rules([], mq=[{"position": 1, "state": "QUEUED", "pullRequest": {"number": 5, "headRefOid": "f" * 40}}])
        log = self.run_once()
        self.assertIn("merge queue holds 1 entry (#5 next) between builds", log)
        self.assertFalse([c for c in self.calls() if c.startswith("run rerun")])

    def test_waiting_merge_queue_admits_nothing_through_a_free_slot(self):
        self.write("ci_prio.json", [7])
        self.write("auth_rerun.json", [{"pr": 8, "run": 556, "head": "0000000000"}])
        mg = {"id": 900, "name": "PR", "head_branch": "gh-readonly-queue/develop/pr-5-abc", "head_sha": "e" * 40, "status": "in_progress"}
        self.pass_rules([run(101, 7, "feat/seven")], mg_runs=[mg], extra=[
            [r"^run list .*--status in_progress --event merge_group", [{"databaseId": 900, "status": "in_progress"}], 0],
            [r"^api repos/Example/Repo/actions/runs/900/jobs", 1, 0],
            [r"^api repos/Example/Repo/actions/runs\?branch=feat/eight", {"workflow_runs": [
                {"id": 80, "name": "PR", "status": "queued", "run_attempt": 1, "created_at": stamp(5)}]}, 0]])
        log = self.run_once()
        self.assertIn("merge-queue first: merge_group run 900 has 1 job(s) waiting for runners; no ordinary slot granted (#7 not admitted)", log)
        self.assertNotIn("took a free slot", log)
        self.assertEqual(self.read("state.json")["holders"], {})
        self.assertEqual(self.read("auth_rerun.json"), [{"pr": 8, "run": 556, "head": "0000000000"}])
        self.assertFalse([c for c in self.calls() if c.startswith(("run rerun", "run cancel", "pr ready", "pr close", "pr reopen"))])
        os.remove(os.path.join(self.dir, "auth_rerun.json"))
        st = watcher.load()
        st["queue"] = [{"pr": 8, "run": 556, "kind": "rerun"}]
        watcher.MQ_HELD.clear()
        watcher.start_next(st)
        self.assertIn("(#8 not admitted)", self.text("watch.log"))
        self.assertEqual((st["holders"], [e["pr"] for e in st["queue"]]), ({}, [8]))

    def test_waiting_merge_queue_still_counts_a_started_pinned_run(self):
        self.write("pinned.json", [7])
        mg = {"id": 900, "name": "PR", "head_branch": "gh-readonly-queue/develop/pr-5-abc", "head_sha": "e" * 40, "status": "in_progress"}
        self.pass_rules([run(101, 7, "feat/seven", "in_progress")], mg_runs=[mg], extra=[
            [r"^run list .*--status in_progress --event merge_group", [{"databaseId": 900, "status": "in_progress"}], 0],
            [r"^api repos/Example/Repo/actions/runs/900/jobs", 1, 0],
            [r"^run view 101 --repo Example/Repo --json status,jobs",
             {"status": "in_progress", "jobs": [{"name": "Changed areas", "status": "completed", "conclusion": "success"},
                                                {"name": "Build and test", "status": "in_progress"}]}, 0]])
        log = self.run_once()
        self.assertIn("#7: pinned PR; run 101 holds a slot", log)
        self.assertNotIn("(#7 not admitted)", log)
        self.assertEqual(self.read("state.json")["holders"]["7"]["kind"], "pinned")

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

    def stack_rules(self, pred_runs, extra=()):
        self.pass_rules([run(101, 7, "feat/seven")], extra=list(extra) + [
            [r"^api repos/Example/Repo/actions/runs\?head_sha=0+8&event=pull_request", {"workflow_runs": pred_runs}, 0]])

    def test_stack_successor_waits_for_a_run_on_its_predecessor_head(self):
        self.write("stacks.json", {"stacks": [[8, 7]]})
        self.stack_rules([])
        log = self.run_once()
        self.assertIn("stack: #7 takes no slot: its predecessor #8 has no pull_request run on its head 0000000000 yet", log)
        self.assertNotIn("took a free slot", log)
        self.assertEqual(self.read("state.json")["holders"], {})

    def test_stack_successor_is_admitted_once_its_predecessor_head_has_a_run(self):
        self.write("stacks.json", {"stacks": [[8, 7]]})
        self.stack_rules([{"id": 900, "name": "PR"}])
        log = self.run_once()
        self.assertIn("#7: new run 101 took a free slot", log)
        self.assertNotIn("stack:", log)

    def test_stack_skips_merged_predecessors_to_the_nearest_open_one(self):
        self.write("stacks.json", {"stacks": [[6, 8, 7]]})
        merged = dict(pull(8, "feat/eight"), state="closed", merged=True)
        self.stack_rules([], extra=[[r"^api repos/Example/Repo/pulls/8$", merged, 0],
                                    [r"^api repos/Example/Repo/pulls/6$", pull(6, "feat/six"), 0],
                                    [r"^api repos/Example/Repo/actions/runs\?head_sha=0+6&event=pull_request", {"workflow_runs": []}, 0]])
        log = self.run_once()
        self.assertIn("stack: #7 takes no slot: its predecessor #6 has no pull_request run on its head 0000000000 yet", log)
        self.assertFalse([c for c in self.calls() if f"head_sha={8:040d}" in c])

    def test_stack_holds_an_authorized_rerun_and_keeps_it(self):
        self.write("stacks.json", {"stacks": [[8, 7]]})
        self.write("auth_rerun.json", [{"pr": 7, "run": 556, "head": "0000000000"}])
        self.pass_rules([], extra=[[r"^api repos/Example/Repo/actions/runs\?head_sha=0+8&event=pull_request", {"workflow_runs": []}, 0],
                                   [r"^run rerun 556 --repo", "", 0]])
        self.assertIn("stack: #7 takes no slot", self.run_once())
        self.assertFalse([c for c in self.calls() if c.startswith("run rerun")])
        self.assertEqual([e["run"] for e in self.read("auth_rerun.json")], [556])

    def test_a_held_authorized_rerun_leaves_the_slot_to_the_queue(self):
        self.write("stacks.json", {"stacks": [[8, 7]]})
        self.write("auth_rerun.json", [{"pr": 7, "run": 556, "head": "0000000000"}])
        self.rules([[r"^api repos/Example/Repo/pulls/8$", pull(8, "feat/eight"), 0],
                    [r"^api repos/Example/Repo/actions/runs\?head_sha=0+8&event=pull_request", {"workflow_runs": []}, 0]])
        watcher.configure(self.cfg())
        st = {"queue": [{"pr": 9, "run": 900, "kind": "rerun"}], "holders": {}}
        with mock.patch.object(watcher, "outage_hold", return_value=None), mock.patch.object(watcher, "review_gate", return_value=True), \
                mock.patch.object(watcher, "_start_next", return_value=True) as nxt:
            self.assertTrue(watcher.start_next(st))
        nxt.assert_called_once()
        self.assertFalse([c for c in self.calls() if c.startswith("run rerun")])

    def test_stack_rank_is_its_best_tier_then_chain_index(self):
        self.write("tiers.json", {"tiers": [[7]]})
        self.write("stacks.json", {"stacks": [[5, 7], [9, 8]]})
        watcher.configure(self.cfg())
        self.assertEqual(sorted([8, 9, 6, 7, 5], key=watcher.tier_rank), [5, 7, 6, 9, 8])
        sw = starve_sweep.Sweeper(self.cfg())
        self.assertEqual([watcher.tier_rank(n) for n in (5, 7, 9, 8)], [sw.rank({"pr": n}) for n in (5, 7, 9, 8)])
        self.assertEqual(watcher.tier_rank(8), (1, 8, 1))

    def test_tiers_order_and_exemptions_come_only_from_exempt_json(self):
        watcher.configure(self.cfg())
        self.assertEqual(watcher.tier_rank(9), (0, 9, 0))
        self.write("tiers.json", {"tiers": [[30, 10], [20]]})
        self.assertEqual(sorted([5, 20, 10, 30, 40], key=watcher.tier_rank), [30, 10, 20, 5, 40])
        self.assertEqual(watcher.tier_members(0, 1), {10, 20, 30})
        self.write("config.json", {"repo": REPO, "exempt_body_tag": "Requested by: @someone"})
        with self.assertRaisesRegex(SystemExit, "exempt_body_tag. was removed"):
            self.cfg()
        self.write("config.json", {"repo": REPO})
        self.write("exempt.json", [20])
        self.write("review_gate.json", {})
        watcher.configure(self.cfg())
        self.assertEqual(watcher.EXEMPT_PRS, {20})
        self.assertTrue(watcher.review_gate(20))
        self.assertEqual(self.calls(), [])


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
            [r"^run list .*--event merge_group", [], 0],
            [r"^api graphql", queue_nodes([]), 0],
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
        self.assertFalse(watcher.review_clear(7))
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
        with mock.patch.object(watcher.subprocess, "run") as run:
            self.assertEqual([watcher.probe_code(u) for u in ("-K/etc/passwd", "file:///etc/passwd", "http://example.org/", "https://[::1/asset", None)], ["err"] * 5)
            run.return_value.stdout = "206\n"
            self.assertEqual(watcher.probe_code("https://example.org/asset"), "206")
        self.assertEqual(run.call_count, 1)
        self.assertEqual(run.call_args.args[0][-2:], ["--url", "https://example.org/asset"])
        os.remove(os.path.join(self.dir, "outage.json"))
        self.assertIsNone(watcher.start_next(st))
        self.assertIn("release hold (release_hold.json): release in progress", self.log())
        watcher.wait_for_new(st)
        self.assertIn("fast poll: new queued run(s) [555]; running a pass now", self.log())

    def cancelled_view(self, rid, head=SHA7, minutes=1):
        return [rf"^run view {rid} --repo Example/Repo --json conclusion,status,jobs", {
            "status": "completed", "conclusion": "cancelled", "headSha": head, "attempt": 1, "startedAt": stamp(90),
            "jobs": [{"name": "Go", "conclusion": "cancelled", "startedAt": stamp(minutes), "completedAt": stamp(0)}]}, 0]

    def test_finish_holder_words_a_cancelled_run_by_what_became_of_its_pr(self):
        self.use([self.cancelled_view(1, head=SHA8), self.cancelled_view(2, head=f"{9:040d}"), self.cancelled_view(3), self.cancelled_view(4, minutes=61),
                  [r"^api repos/Example/Repo/pulls/9$", dict(pull(9, "feat/nine"), mergeable_state="dirty"), 0],
                  [r"^run rerun 3 ", "", 0]])
        st = watcher.load()
        watcher.CANCEL_ATT["3"] = 1
        watcher.PINNED.add(7)
        watcher.finish_holder(st, 7, {"run": 1})
        watcher.finish_holder(st, 9, {"run": 2, "retries": 2})
        watcher.finish_holder(st, 7, {"run": 3})
        watcher.finish_holder(st, 7, {"run": 4})
        log = self.log()
        self.assertIn("#7: run 1 cancelled on stale head 00000000 (PR head 00000000); not re-queued", log)
        self.assertIn("#9: run 2 attempt 1 was CANCELLED (not by me; cancelled jobs: Go); PR is dirty, parked until its head changes", log)
        self.assertIn("#7: pinned run 3 attempt 1 was CANCELLED (by the watcher); re-ran it immediately on current head 00000000", log)
        self.assertIn("#7: run 4 attempt 1 was TIMEOUT (Go, 61 min) (job timeout-minutes) after holding a slot; not re-run (maintainer decides), dropped", log)
        self.assertEqual((st["parked"]["9"]["retries"], st["holders"]["7"]["run"]), (2, 3))

    def test_finish_holder_names_the_base_and_the_authorized_exception(self):
        red = [("Go", "failure"), ("Build and test", "failure")]
        self.use([self.view(1, "success"), self.view(2, "failure", red), self.view(3, "failure", red + [("Docs", "failure")]),
                  self.view(4, "failure", red), self.view(5, "success"), self.view(6, "success"),
                  [r"^api repos/Example/Repo/pulls/9$", dict(pull(9, "feat/nine"), base={"ref": "feat/seven"}), 0],
                  [r"^api repos/Example/Repo/pulls/10$", dict(pull(10, "feat/ten"), mergeable_state="dirty"), 0],
                  [r"^api repos/Example/Repo/compare/b+\.\.\.feat/seven", {"commits": [
                      {"sha": "c" * 40, "parents": [{"sha": "b" * 40}], "commit": {"committer": {"date": "2000-01-01T00:00:00Z"}}}]}, 0],
                  [r"compare/n+\.\.\.0+7 --jq", {"s": "behind"}, 0],
                  [r"^run view 9 --repo Example/Repo --json conclusion,status,jobs", "", 1],
                  [r"^run view 8 --repo Example/Repo --json conclusion,status,jobs", {"status": "in_progress", "conclusion": None}, 0]])
        self.write("fail_ok.json", {"2": {"jobs": ["Go", "Build and test"], "why": "accepted", "label": "EXCUSED"},
                                    "3": {"jobs": ["Go", "Build and test"], "why": "accepted"},
                                    "4": {"jobs": ["Go", "Build and test"], "why": "accepted"}})
        self.write("unreleased.json", [6])
        self.write("release.json", [{"pr": 7, "sha": "0000"}])
        st = watcher.load()
        watcher.finish_holder(st, 9, {"run": 1})
        watcher.finish_holder(st, 7, {"run": 2})
        watcher.finish_holder(st, 7, {"run": 3})
        watcher.finish_holder(st, 10, {"run": 4})
        self.write("need_base.json", "n" * 40)
        watcher.finish_holder(st, 7, {"run": 5})
        os.remove(os.path.join(self.dir, "need_base.json"))
        watcher.finish_holder(st, 7, {"run": 6})
        watcher.finish_holder(st, 7, {"run": 9})
        watcher.finish_holder(st, 7, {"run": 8})
        log = self.log()
        self.assertIn("#9: run 1 (base=feat/seven@bbbbbbbbbb) finished GREEN (base bbbbbbbbbb; feat/seven now cccccccccc, freshness supplied by the merge queue)", log)
        self.assertIn("#7: run 2 (base bbbbbbbbbb) finished EXCUSED (Go, Build and test; accepted); every other job success/skipped", log)
        self.assertIn("#7: run 3 (base bbbbbbbbbb) finished RED (failure); failed jobs: Go, Build and test, Docs; dropped", log)
        self.assertIn("#10: run 4 (base bbbbbbbbbb) finished RED only on Go, Build and test; validated under the authorized exception (accepted); PR is dirty at head", log)
        self.assertIn("#7: run 5 (base bbbbbbbbbb, head lacks nnnnnnnnnn) finished GREEN on bbbbbbbbbb, head lacks nnnnnnnnnn; maintainer decides", log)
        self.assertIn("#7: run 6 (base bbbbbbbbbb) finished GREEN [UNRELEASED run: re-run without a release", log)
        self.assertIn("#7: run 9 finished (could not read result)", log)
        self.assertIn("#7: run 8 still in_progress; keeps its slot", log)
        self.assertEqual((st["stale_bt"]["7"], st["holders"]["7"]), ({"head": SHA7, "green": 6}, {"run": 8}))

    def test_finish_holder_queues_the_current_head_after_a_run_on_a_stale_one(self):
        self.use([self.view(1, "success", head=SHA8),
                  [r"actions/runs\?head_sha=0+7&event=pull_request", {"workflow_runs": [
                      {"id": 70, "name": "PR", "status": "completed", "conclusion": "cancelled", "created_at": stamp(5)}]}, 0]])
        st = watcher.load()
        st["cancelled"] = [70]
        watcher.finish_holder(st, 7, {"run": 1})
        self.assertIn("#7: finished run 1 was on stale head 00000000; current head 00000000's pre-cancelled run 70 queued (by tier, position 1/1)", self.log())
        self.assertEqual(st["queue"], [{"pr": 7, "run": 70, "sha": SHA7, "kind": "rerun"}])

    def test_start_next_keeps_or_drops_an_entry_it_cannot_start(self):
        def runs(n, **kw):
            return [rf"^api repos/Example/Repo/actions/runs\?branch=feat/{n}&", {"workflow_runs": [
                dict({"id": n * 10, "name": "PR", "status": "completed", "conclusion": "cancelled", "run_attempt": 1, "created_at": stamp(5)}, **kw)]}, 0]
        self.write("strict_base.json", {"sha": "b" * 40})
        self.write("reopen_ok.json", [18])
        self.use([[r"^api repos/Example/Repo/pulls/11$", dict(pull(11, "feat/11"), state="closed", merged=True), 0],
                  [r"^api repos/Example/Repo/pulls/13$", pull(13, "feat/13"), 0],
                  [r"^api repos/Example/Repo/pulls/14$", dict(pull(14, "feat/14"), draft=True, mergeable_state="unknown"), 0],
                  [r"^api repos/Example/Repo/pulls/15$", dict(pull(15, "feat/15"), draft=True), 0],
                  [r"^api repos/Example/Repo/pulls/16$", pull(16, "feat/16"), 0], runs(16, conclusion="failure"),
                  [r"^api repos/Example/Repo/pulls/17$", pull(17, "feat/17"), 0], runs(17, conclusion="success"),
                  [r"^api repos/Example/Repo/pulls/18$", pull(18, "feat/18"), 0], runs(18),
                  [r"^api repos/Example/Repo/pulls/19$", pull(19, "feat/19"), 0], runs(19, status="in_progress", conclusion=None),
                  [r"^api repos/Example/Repo/pulls/12$", pull(12, "feat/12"), 0],
                  [rf"compare/b+\.\.\.{13:040d} --jq", {"s": "behind"}, 0],
                  [rf"compare/b+\.\.\.{12:040d} --jq", "", 1],
                  [r"compare/b+\.\.\.\d+ --jq", {"s": "ahead"}, 0],
                  [r"^api repos/Example/Repo/actions/runs/190 --jq", {"a": 1, "b": "feat/19", "e": "pull_request"}, 0],
                  [r"^run cancel 190 ", "", 0]])
        st = watcher.load()
        watcher.HOLDS["7"] = ["0000"]
        watcher.NOCANCEL.add(16)
        watcher.cancel(190)
        st["cancelled"] = [190]
        st["queue"] = [{"pr": n, "run": None, "kind": "ready" if n == 15 else "rerun"} for n in (7, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19)]
        for _ in range(3):
            watcher._start_next(st)
        log = self.log()
        for line in ("#7: owner announced a push; skipped until a new head lands", "#10: could not fetch PR; dropped from queue", "#11: merged; skipped",
                     "#13: head 0000000000 lacks develop bbbbbbbbbb; parked (needs develop merge)", "#14: mergeable_state still unknown; not marking ready this pass",
                     "#15: draft (owner's WIP); taken out of the queue, its runs are never cancelled or re-run",
                     "#16: latest PR run 160 on head 00000000 is failure (not cancelled by me); not re-run, dropped",
                     "#17: latest PR run 170 on head 00000000 is success, not cancelled; dropped",
                     "#18: refs/pull/18/merge not yet on current develop or without the runner pin; close/reopen deferred",
                     "#12: could not compare head with bbbbbbbbbb; retrying next pass"):
            self.assertIn(line, log)
        self.assertEqual([e["pr"] for e in st["queue"]], [7, 12, 14, 18, 19])
        st["queue"] = [e for e in st["queue"] if e["pr"] == 19]
        watcher._start_next(st)
        self.assertIn("#19: my cancel of run 190 still settling; retry next pass", self.log())
        self.assertEqual((st["queue"], st["holders"], list(st["parked"])), ([], {}, ["13"]))

    def test_the_fast_poll_ends_for_a_finished_run_and_a_failing_sweep_is_logged(self):
        self.use([[r"^run view 300 --repo Example/Repo --json status$", {"status": "completed"}, 0],
                  [r"^run view 301 --repo Example/Repo --json status$", {"status": "in_progress"}, 0],
                  [r"^run list --repo Example/Repo --status queued -L 50", [], 0],
                  [r"^api repos/Example/Repo/pulls/8$", dict(pull(8, "feat/eight"), draft=True), 0],
                  [r"^api repos/Example/Repo/pulls/9$", dict(pull(9, "feat/nine"), state="closed"), 0],
                  [r"^api repos/Example/Repo/actions/runs/102 --jq", {"a": 1, "b": "feat/eight", "e": "pull_request"}, 0],
                  [r"^api repos/Example/Repo/actions/runs/102$", {"head_sha": SHA8, "pull_requests": [{"number": 8}]}, 0],
                  [r"^run view 10[23] --repo Example/Repo --json status,jobs", {"status": "queued", "jobs": []}, 0],
                  [r"^run cancel 102 ", "", 0]], interval_seconds=1)
        st = watcher.load()
        self.write("state.json", {"holders": {"8": {"run": 300}, "7": {"run": 301}}})
        self.write("solo.json", {"pr": 7, "run": 301})
        watcher.NOCANCEL.update([7, 8])
        watcher.wait_for_new(st)
        self.assertIn("fast poll: #8's run 300 completed; running a pass now", self.log())
        self.write("solo.json", {"pr": 7, "run": 300})
        watcher.wait_for_new(st)
        self.assertIn("fast poll: #7's solo run 300 completed; running a pass now", self.log())
        os.remove(os.path.join(self.dir, "solo.json"))
        watcher.NOCANCEL.clear()
        self.write("review_gate.json", {})
        self.write("cancel_auth.json", [102])
        st["holders"]["8"] = {"run": 102}

        def sweep(extra=()):
            self.rules([[r"runs\?event=pull_request&status=queued", {"workflow_runs": [
                {"id": 102, "name": "PR", "head_sha": SHA8, "pull_requests": [{"number": 8, "base": {"ref": "develop"}}]},
                {"id": 103, "name": "PR", "head_sha": f"{9:040d}", "pull_requests": [{"number": 9, "base": {"ref": "develop"}}]},
                *extra]}, 0], [r"runs\?event=pull_request&status=in_progress", {"workflow_runs": []}, 0]] + self.read("gh.rules"))
        sweep()
        watcher.wait_for_new(st)
        self.assertIn("fast poll: cancelled unreleased run(s) during preflight; running a pass now", self.log())
        self.assertIn("#8: draft head 0000000000 not released; run 102 caught with only the preflight running; cancelled, held until released", self.log())
        self.assertEqual((st["holders"], st["queue"]), ({}, []))
        sweep([{"id": 104, "name": "PR", "head_sha": SHA7, "pull_requests": [{}]}])
        watcher.wait_for_new(st)
        self.assertIn("ERROR gate_sweep Traceback", self.log())


class WatcherPassTest(Stub):
    """One pass over a prepared state directory, for the decisions the tests above do not reach."""
    run_once = WatcherTest.run_once

    def setUp(self):
        super().setUp()
        importlib.reload(watcher)

    def state(self, **kw):
        self.write("state.json", dict({"holders": {}, "queue": [], "parked": {}, "cancelled": [], "order": []}, **kw))

    def world(self, runs=(), prs=(7, 8), started=(), extra=(), cap=1):
        """Rules for open PRs (numbers or pr() dicts) and their queued runs; the runs listed in started have a heavy job executing."""
        self.write("config.json", {"repo": REPO, "cap": cap, "checkout": self.dir})
        prs = [p if isinstance(p, dict) else pr(p, f"feat/{p}") for p in prs]
        rules = list(extra) + [[r"^pr list ", prs, 0],
                               [r"^api repos/Example/Repo/actions/runs\?event=merge_group&per_page=30", {"workflow_runs": []}, 0],
                               [r"^run list .*--event merge_group", [], 0],
                               [r"^api graphql", queue_nodes([]), 0],
                               [r"^api repos/Example/Repo/commits/develop --jq", {"s": "d" * 40, "m": "Merge"}, 0],
                               [r"^run list --repo Example/Repo --status queued -L 100", list(runs), 0],
                               [r"^run list --repo Example/Repo --status \w+ -L 100", [], 0],
                               [r"^api repos/Example/Repo/actions/runs\?branch=", {"workflow_runs": []}, 0]]
        for p in prs:
            rules.append([rf"^api repos/Example/Repo/pulls/{p['number']}$", dict(pull(p["number"], p["headRefName"]), draft=p["isDraft"]), 0])
        for r in runs:
            rid, n = r["databaseId"], int(r["headSha"])
            rules += [[rf"^api repos/Example/Repo/actions/runs/{rid} --jq", {"a": 1, "b": r["headBranch"], "e": "pull_request"}, 0],
                      [rf"^api repos/Example/Repo/actions/runs/{rid}$", {"head_sha": r["headSha"], "pull_requests": [{"number": n}]}, 0],
                      [rf"^run cancel {rid} ", "", 0]]
            if rid in started:
                rules.append([rf"^run view {rid} --repo Example/Repo --json (status,)?jobs", {"status": "in_progress", "jobs": [
                    {"name": "Go", "status": "in_progress"}, {"name": "Docs", "status": "completed", "conclusion": "success"}]}, 0])
        rules.append([r"^run view \d+ --repo Example/Repo --json status,jobs", {"status": "queued", "jobs": [{"name": "Changed areas", "status": "queued"}]}, 0])
        self.rules(rules)

    def said(self, log, *lines):
        for line in lines:
            self.assertIn(line, log)

    def holder(self, rid, **kw):
        return dict({"run": rid, "since": 0, "seen_active": True}, **kw)

    def test_a_pass_reports_what_moved_since_the_last_one(self):
        self.state(develop_head="a" * 40, mergeable={"7": "MERGEABLE"}, baseref={"8": "develop", "9": "develop"},
                   parked={"5": {"pr": 5, "parked_sha": "x"}, "7": {"pr": 7, "parked_sha": "old", "kind": "rerun"},
                           "8": {"pr": 8, "parked_sha": "old"}, "9": {"pr": 9, "parked_sha": "old"}})
        self.write("strict_base.json", {"sha": "b" * 40})
        self.world(prs=[dict(pr(7, "feat/7"), mergeable="CONFLICTING"), dict(pr(8, "feat/8"), baseRefName="feat/7"),
                        dict(pr(9, "feat/9"), baseRefName="main")],
                   extra=[[r"^api repos/Example/Repo/pulls/8$", dict(pull(8, "feat/8"), mergeable_state="dirty"), 0],
                          [rf"compare/b+\.\.\.{9:040d} --jq", {"s": "behind"}, 0],
                          [r"compare/b+\.\.\.\d+ --jq", {"s": "ahead"}, 0]])
        self.said(self.run_once(), "develop moved aaaaaaaaaa -> dddddddddd: Merge",
                  "#7 turned DIRTY (CONFLICTING) at head 0000000000 against develop dddddddddd",
                  "#8: base branch changed develop -> feat/7 (#7's branch; stacked behind it)",
                  "#5: parked PR no longer open; dropped",
                  "#7: head moved to 00000000, no longer dirty; back in queue (unparked, position 1/1)",
                  "#8: head moved to 00000000 but still dirty; stays parked",
                  "#9: head moved to 00000000 but still lacks develop bbbbbbbbbb; stays parked",
                  "slot free for #8 (1 free; #8 is parked, dirty): it may push its prepared head now")

    def test_approved_drafts_are_queued_parked_or_marked_ready(self):
        self.write("ready_ok.json", [10, 11, 12])
        self.state(queue=[{"pr": 12, "run": None, "kind": "rerun"}])
        self.world(prs=[pr(10, "feat/10", draft=True), dict(pr(11, "feat/11", draft=True), mergeable="CONFLICTING"), pr(12, "feat/12", draft=True)],
                   extra=[[r"^pr ready 10 ", "", 0]], cap=3)
        self.run_once()
        self.said(self.run_once(), "#10: draft on develop queued for un-draft (by tier, position 1/1)",
                  "#12: draft on develop queued for un-draft (by tier, position 2/2)",
                  "#11: draft is CONFLICTING at 00000000; parked until a new non-dirty head lands",
                  "#10: slot granted, marked ready for review (gh pr ready)", "#12: gh pr ready failed; dropped")
        self.assertEqual(self.read("state.json")["holders"]["10"]["kind"], "ready")

    def test_a_head_without_the_strict_base_is_parked(self):
        self.write("strict_base.json", {"sha": "b" * 40})
        self.write("cancel_auth.json", [102])
        self.write("nocancel.json", [9])
        self.state(queue=[{"pr": 7, "run": None, "kind": "rerun"}])
        self.world([run(102, 8, "feat/8"), run(103, 9, "feat/9")], prs=(7, 8, 9), extra=[[r"compare/b+\.\.\.\d+ --jq", {"s": "behind"}, 0]])
        self.run_once()
        self.said(self.run_once(), "needs develop merge (head lacks bbbbbbbbbb); parked until a new head contains it: #7 (0000000000)",
                  "#8: head 0000000000 lacks develop bbbbbbbbbb; cancelled not-yet-started run(s) [102]; parked (needs develop merge)",
                  "#9: run(s) [103] on 0000000000 lack develop bbbbbbbbbb; not cancelled (never cancelled before start)")
        self.assertEqual(sorted(self.read("state.json")["parked"]), ["7", "8"])

    def test_a_gate_holds_every_pr_outside_its_allow_list_until_its_pr_merges(self):
        self.write("gate.json", {"pr": 7, "allow": [7]})
        self.write("cancel_auth.json", [102, 103])
        runs = [run(102, 8, "feat/8"), run(103, 9, "feat/9")]
        self.world(runs, prs=(7, 8, 9), extra=[[r"^api repos/Example/Repo/pulls/9$", dict(pull(9, "feat/9"), mergeable_state="dirty"), 0]])
        self.said(self.run_once(), "#8: held until #7 merges; cancelled run(s) [102]; queued (by tier)",
                  "#9: held until #7 merges; cancelled run(s) [103]; PR is dirty, parked")
        self.world(prs=(7, 8), extra=[[r"^api repos/Example/Repo/pulls/7$", dict(pull(7, "feat/7"), merged=True), 0]])
        self.said(self.run_once(), "#7 merged; hold released, filling free slots in normal order")
        self.assertFalse(os.path.exists(os.path.join(self.dir, "gate.json")))

    def test_a_newer_run_takes_over_its_prs_slot(self):
        self.state(holders={"7": self.holder(100, kind="ready"), "8": self.holder(None)})
        self.world([run(101, 7, "feat/7"), run(102, 8, "feat/8")], cap=2)
        self.said(self.run_once(), "#7: new head 00000000; run 101 takes over the slot from 100", "#7: ready event started run 101")
        self.assertEqual({k: h["run"] for k, h in self.read("state.json")["holders"].items()}, {"7": 101, "8": 102})

    def test_an_unreleased_new_head_gives_its_slot_back(self):
        self.write("review_gate.json", {})
        self.write("cancel_auth.json", [101, 102])
        self.state(holders={"7": self.holder(100), "8": self.holder(102)})
        self.world([run(101, 7, "feat/7"), run(102, 8, "feat/8")], cap=2,
                   extra=[[r"runs\?event=pull_request&status=", {"workflow_runs": []}, 0],
                          [r"^run view 101 --repo Example/Repo --json status,jobs", {"status": "queued", "jobs": [{"name": "Go", "status": "pending"}]}, 0]])
        self.said(self.run_once(), "#7: new head 0000000000 not released; cancelled queued run 101, slot freed; held under the review gate (by tier)",
                  "#8: head 0000000000 not released; run(s) [102] had only the preflight done, no job executing; cancelled, slot freed, held under the review gate (by tier)",
                  "2 slot(s) free; no released head waiting (top held: #7, queued, no runnable head); no invitation sent")

    def test_a_dirty_holder_keeps_a_started_or_protected_run(self):
        self.write("nocancel.json", [8])
        self.state(holders={"7": self.holder(101), "8": self.holder(102)})
        dirty = [[rf"^api repos/Example/Repo/pulls/{n}$", dict(pull(n, f"feat/{n}"), mergeable_state="dirty"), 0] for n in (7, 8)]
        self.world([run(101, 7, "feat/7"), run(102, 8, "feat/8")], started=[101], extra=dirty, cap=2)
        self.said(self.run_once(), "#7: head 00000000 is DIRTY but run(s) [101] already started; not cancelled, still holding a slot",
                  "#8: head 00000000 is DIRTY; run(s) [102] not cancelled (never cancelled before start), still holding a slot")
        self.assertEqual(self.run_once().count("is DIRTY"), 2)

    def test_a_started_run_is_adopted_and_a_branch_without_a_pr_is_cancelled(self):
        self.write("cancel_auth.json", [104])
        orphan = dict(run(104, 0, "feat/gone"))
        self.world([run(101, 7, "feat/7"), orphan], started=[101], cap=2,
                   extra=[[r"^api repos/Example/Repo/actions/runs/104$", {"head_sha": "0" * 40, "pull_requests": []}, 0]])
        self.said(self.run_once(), "#7: run 101 already executing jobs (1 done / 1 in progress / 0 queued); never cancelled once started; holding a slot",
                  "cancelled run(s) [104] on branch:feat/gone (no open PR); not queued")

    def test_pinned_reserved_and_sole_priority_runs_take_their_slots(self):
        self.write("pinned.json", ["feat/7"])
        self.write("reserve.json", {"pr": 8})
        self.world([run(101, 7, "feat/7"), run(102, 8, "feat/8")])
        self.said(self.run_once(), "#7: pinned PR; run 101 holds a slot (never cancelled for the cap)",
                  "ATTENTION #8: reserved PR pushed while 1 runs were active; run 102 NOT cancelled, now 2 active",
                  "#8: reserved slot filled by run 102; reservation cleared")
        os.remove(os.path.join(self.dir, "pinned.json"))
        self.state(queue=[{"pr": 7, "run": None, "kind": "rerun"}])
        self.write("reserve.json", {"pr": 8})
        self.world()
        self.said(self.run_once(), "slot reserved for #8; not starting [7]")
        os.remove(os.path.join(self.dir, "reserve.json"))
        self.state()
        self.write("solo.json", {"pr": 8, "run": None})
        self.write("cancel_auth.json", [101])
        self.world([run(101, 7, "feat/7"), run(102, 8, "feat/8")], cap=2)
        self.said(self.run_once(), "#8: sole-priority run 102 took the slot", "#7: cancelled not-yet-started run(s) [101] over the cap; queued (by tier)")

    def test_a_protected_run_over_the_cap_waits_uncancelled(self):
        self.write("nocancel.json", [8])
        self.world([run(101, 7, "feat/7"), run(102, 8, "feat/8")])
        self.run_once()
        log = self.run_once()
        self.said(log, "#8: run 102 on 0000000000 queued at position 1/1, NOT cancelled; takes the first free slot (slots: #7 101)")
        self.assertEqual(log.count("NOT cancelled"), 1)

    def test_holders_over_the_cap_lose_the_lowest_ranked_unstarted_run(self):
        self.write("cancel_auth.json", [101, 102])
        self.state(holders={"7": self.holder(101), "8": self.holder(102)}, cancelled=[103], cancelled_att={"103": 1})
        self.world([run(101, 7, "feat/7"), run(102, 8, "feat/8"), run(103, 9, "feat/9")], prs=(7, 8, 9),
                   extra=[[r"^api -X POST repos/Example/Repo/actions/runs/103/force-cancel", "", 0]])
        self.said(self.run_once(), "#8: cancelled not-yet-started run(s) [102] over the cap; queued (by tier)",
                  "#9: runs [103] still active after my cancel; force-cancel sent")
        self.assertEqual(list(self.read("state.json")["holders"]), ["7"])

    def test_a_block_releases_a_finished_holder_and_lifts_once_its_run_executes(self):
        self.write("blocked.json", {"prs": [8], "until": 7})
        self.state(holders={"7": self.holder(101), "8": self.holder(100)}, parked={"8": {"pr": 8, "parked_sha": SHA8}})
        self.world([run(101, 7, "feat/7")], started=[101], cap=2,
                   extra=[[r"^run view 100 --repo Example/Repo --json conclusion,jobs", {"conclusion": "failure", "jobs": [{"name": "Go", "conclusion": "failure"}]}, 0]])
        self.said(self.run_once(), "#8: run 100 finished failure; failed jobs: Go; slot released (blocked until #7's run is executing; not re-queued)",
                  "#7: run 101 is executing; block on [8] lifted")
        self.assertEqual(self.read("state.json")["parked"], {})

    def test_a_jobless_ghost_and_a_runner_priority_run(self):
        ghost = dict(run(101, 7, "feat/7"), createdAt=stamp(20))
        self.write("priority_hold_8.json", {"pr": 8, "run": 102, "head": SHA8, "label": "#8 run"})
        self.state(holders={"7": self.holder(101)})
        self.world([ghost, run(102, 8, "feat/8")], extra=[[r"runs/101/jobs\?per_page=1 --jq", 0, 0],
                                                         [r"^run view 102 --repo Example/Repo --json status,conclusion", {"status": "queued"}, 0]])
        self.said(self.run_once(), "#7: run 101 is a jobless ghost (0 jobs); slot no longer counted",
                  "#8: runner-priority run 102 holds a slot (exempt from its own hold)")


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
            [r"runs\?status=queued", {"workflow_runs": queued}, 0],
            [r"runs\?status=", {"workflow_runs": []}, 0],
            [r"runs/11/jobs", {"jobs": [{"status": "queued"}]}, 0],
            [r"runs/12/jobs", {"jobs": [{"status": "in_progress"}]}, 0],
            [r"runs/14/jobs", {"jobs": [{"status": "queued"}]}, 0],
            [r"runs/15/jobs", {"jobs": []}, 0],
            [r"runs/16/jobs", {"jobs": [{"name": "Changed areas", "status": "completed", "conclusion": "success"}, {"name": "Go", "status": "queued"}]}, 0],
            [r"^run cancel 1[146] ", "", 0],
        ])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual([c for c in self.calls() if c.startswith("run cancel")], ["run cancel 11 -R Example/Repo", "run cancel 14 -R Example/Repo",
                                                                                     "run cancel 16 -R Example/Repo"])
        self.assertEqual([e["pr"] for e in self.read("auth_rerun.json")], [9, 4, 7, 10])

    def starved_rules(self, mg_age, mg_jobs, cancel_fails=(), extra=()):
        self.write("starve_cancel.json", {"by": "maintainer", "rule": "keep the merge PR run"})
        self.write("tiers.json", {"tiers": [[6]]})

        def r(rid, n):
            return {"id": rid, "event": "pull_request", "name": "PR", "head_branch": f"feat/{n}", "head_sha": f"{rid:040d}",
                    "pull_requests": [{"number": n}]}

        def jobs(*sts):
            return {"jobs": [{"name": f"j{i}", "status": s, "conclusion": "success" if s == "completed" else None} for i, s in enumerate(sts)]}
        mg = {"id": 900, "status": "in_progress", "created_at": stamp(mg_age), "head_branch": "gh-readonly-queue/develop/pr-1-x"}
        rules = list(extra) + [[rf"^run cancel {rid} ", "boom", 1] for rid in cancel_fails] + [
            [r"event=merge_group&status=in_progress", {"workflow_runs": [mg]}, 0],
            [r"event=merge_group&status=", {"workflow_runs": []}, 0],
            [r"runs/900/jobs", {"jobs": mg_jobs}, 0],
            [r"runs\?status=in_progress", {"workflow_runs": [r(21, 4), r(22, 5), r(23, 6), r(24, 7)]}, 0],
            [r"runs\?status=", {"workflow_runs": []}, 0],
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

    def test_starved_merge_group_keeps_an_expanded_run_over_an_unexpanded_one(self):
        done = {"name": "j", "status": "completed", "conclusion": "success"}
        self.starved_rules(10, [{"status": "queued"}], extra=[
            [r"runs/21/jobs", {"jobs": [{"name": "Changed areas", "status": "completed", "conclusion": "success"}, {"name": "Go", "status": "in_progress"}]}, 0],
            [r"runs/22/jobs", {"jobs": [done] * 23 + [{"name": "k", "status": "in_progress"}] + [{"name": "k", "status": "queued"}] * 5}, 0],
            [r"runs/2[34]/jobs", {"jobs": [{"name": "k", "status": "queued"}]}, 0]])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(sorted(self.cancels()), ["21", "23", "24"])
        self.assertIn("merge group starved: keeping run 22 (#5, 6 job(s) unfinished)", self.text("watch.log"))

    def test_group_with_only_its_preflight_done_is_starved(self):
        self.starved_rules(10, [{"name": "Changed areas", "status": "completed", "conclusion": "success"}, {"name": "Go", "status": "queued"}])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(sorted(self.cancels()), ["21", "23", "24"])

    def test_waiting_group_within_the_grace_period_leaves_started_runs_alone(self):
        self.starved_rules(1, [{"status": "in_progress", "created_at": stamp(10)}, {"status": "queued", "created_at": stamp(10)}])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(self.cancels(), [])
        self.write("config.json", {"repo": REPO, "cap": 4, "checkout": self.dir})
        self.starved_rules(10, [])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(self.cancels(), [])
        self.assertNotIn("starved: keeping", self.text("watch.log"))

    def test_a_job_queued_past_the_grace_period_starves_a_partly_started_group(self):
        # A downstream job is created late; the run's age decides, not the job's.
        self.starved_rules(10, [{"status": "in_progress", "created_at": stamp(10)}, {"status": "queued", "created_at": stamp(1)}])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(sorted(self.cancels()), ["21", "23", "24"])

    def test_every_event_is_a_candidate_except_protected_runs(self):
        self.cap(2)

        def r(rid, event, branch, n=None):
            return {"id": rid, "event": event, "name": "Any", "head_branch": branch, "head_sha": f"{rid:040d}",
                    "pull_requests": [{"number": n}] if n else []}
        runs = [r(22, "pull_request", "feat/5", 5), r(31, "schedule", "develop"), r(32, "push", "develop"), r(33, "push", "v1.2.3"),
                r(34, "pull_request", "release/1.0", 9), r(37, "push", "gh-readonly-queue/develop/pr-1-x"), r(38, "dynamic", "feat/d"),
                r(35, "workflow_dispatch", "feat/x"), r(36, "pull_request", "develop", 8)]
        busy = {"jobs": [{"name": "j0", "status": "completed", "conclusion": "success"}, {"name": "j1", "status": "in_progress"}]}
        self.starved_rules(10, [], extra=[[r"runs\?status=in_progress", {"workflow_runs": runs}, 0], [r"runs/3[0-9]/jobs", busy, 0]])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(self.cancels(), ["35"])
        for rid in (31, 32, 33, 34, 37, 38):
            self.assertFalse([c for c in self.calls() if f"runs/{rid}/jobs" in c], rid)
        log = self.text("watch.log")
        self.assertIn("keeping run 22 (#5), run 36 (#8) (cap 2)", log)
        self.assertIn("run 35 (workflow_dispatch on feat/x, no PR, 0000000000", log)
        self.assertIn("cancelled; not queued for re-run (no PR)", log)
        self.assertIsNone(self.cfg().read("auth_rerun.json"))

    def test_re_cancelling_the_same_run_and_head_keeps_a_full_rerun_full(self):
        self.write("auth_rerun.json", [{"pr": 4, "run": 21, "head": "0000000000", "workflow": "PR", "full": True},
                                       {"pr": 5, "run": 99, "head": "0000000000", "workflow": "PR", "full": True}])
        self.starved_rules(1, [{"status": "queued"}], extra=[[r"runs\?status=in_progress", {"workflow_runs": [
            {"id": rid, "event": "pull_request", "name": "PR", "head_branch": f"feat/{n}", "head_sha": f"{rid:040d}",
             "pull_requests": [{"number": n}]} for rid, n in ((21, 4), (22, 5))]}, 0],
            [r"runs/2[12]/jobs", {"jobs": [{"name": "Go", "status": "queued"}]}, 0]])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(sorted(self.cancels()), ["21", "22"])
        self.assertEqual(sorted(self.read("auth_rerun.json"), key=lambda e: e["pr"]), [
            {"pr": 4, "run": 21, "head": "0000000000", "workflow": "PR", "full": True},
            {"pr": 5, "run": 22, "head": "0000000000", "workflow": "PR"}])

    def test_a_prs_runs_of_two_workflows_both_keep_a_rerun(self):
        def r(rid, name):
            return {"id": rid, "event": "pull_request", "name": name, "head_branch": "feat/5", "head_sha": f"{rid:040d}",
                    "pull_requests": [{"number": 5}]}
        self.starved_rules(1, [{"status": "queued"}], extra=[
            [r"runs\?status=in_progress", {"workflow_runs": [r(41, "PR"), r(42, "Docs")]}, 0],
            [r"runs/4[12]/jobs", {"jobs": [{"name": "Go", "status": "queued"}]}, 0]])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(sorted(self.cancels()), ["41", "42"])
        self.assertEqual(sorted((e["run"], e["workflow"]) for e in self.read("auth_rerun.json")), [(41, "PR"), (42, "Docs")])

    def test_failed_starvation_cancel_is_not_queued(self):
        self.starved_rules(10, [{"status": "queued"}], cancel_fails=(23,))
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual([e["pr"] for e in self.read("auth_rerun.json")], [4, 7])
        self.assertIn("#6: run 23", self.text("watch.log"))
        self.assertIn("merge group starved) not cancelled", self.text("watch.log"))

    def cap(self, n):
        self.write("config.json", {"repo": REPO, "cap": n, "checkout": self.dir})

    def test_idle_queue_over_the_cap_keeps_cap_runs_by_tier_then_closest_to_finishing(self):
        self.cap(2)
        self.starved_rules(10, [], cancel_fails=(24,), extra=[[r"runs/21/jobs", {"jobs": [{"name": "Go", "status": "queued"}]}, 0]])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        log = self.text("watch.log")
        self.assertIn("heartbeat: mq waiting=no starved=no failing=no; ordinary runs: 3 started, 1 unstarted, 0 being cancelled", log)
        self.assertIn("over the cap with the merge queue idle: 4 ordinary runs active, keeping run 23 (#6), run 22 (#5) (cap 2)", log)
        self.assertEqual(self.cancels(), ["21", "24"])
        self.assertEqual(self.read("auth_rerun.json"), [{"pr": 4, "run": 21, "head": "0000000000", "workflow": "PR"}])
        self.assertIn("#7: run 24 (0000000000, 2 job(s) unfinished, over the cap of 2 with the merge queue idle) not cancelled", log)

    def test_idle_queue_over_the_cap_keeps_a_started_run_over_an_unstarted_one_of_the_same_tier(self):
        self.cap(1)
        self.starved_rules(10, [], extra=[
            [r"runs\?status=in_progress", {"workflow_runs": [
                {"id": rid, "event": "pull_request", "name": "PR", "head_branch": f"feat/{n}", "head_sha": f"{rid:040d}",
                 "pull_requests": [{"number": n}]} for rid, n in ((21, 4), (22, 5))]}, 0],
            [r"runs/21/jobs", {"jobs": [{"name": "Go", "status": "queued"}]}, 0],
            [r"runs/22/jobs", {"jobs": [{"name": f"j{i}", "status": s} for i, s in enumerate(("in_progress", "queued", "queued"))]}, 0]])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertIn("keeping run 22 (#5) (cap 1)", self.text("watch.log"))
        self.assertEqual(self.cancels(), ["21"])

    def test_idle_queue_within_the_cap_cancels_nothing(self):
        self.cap(4)
        self.starved_rules(10, [])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(self.cancels(), [])
        self.assertNotIn("over the cap", self.text("watch.log"))

    def test_idle_queue_follows_a_raised_cap(self):
        self.cap(1)
        self.write("wide_cap.json", {"prs": [5], "cap": 3, "seen": {}, "done": []})
        self.starved_rules(10, [])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(self.cancels(), ["21"])
        self.assertIn("keeping run 23 (#6), run 22 (#5), run 24 (#7) (cap 3)", self.text("watch.log"))

    def test_idle_queue_keeps_an_expanded_run_over_an_unexpanded_one_in_the_same_tier(self):
        self.cap(1)
        done = {"name": "j", "status": "completed", "conclusion": "success"}
        self.starved_rules(10, [], extra=[
            [r"runs/21/jobs", {"jobs": [{"name": "Changed areas", "status": "completed", "conclusion": "success"},
                                        {"name": "Docs", "status": "completed", "conclusion": "skipped"}, {"name": "Go", "status": "in_progress"}]}, 0],
            [r"runs/22/jobs", {"jobs": [done] * 23 + [{"name": "k", "status": "in_progress"}] + [{"name": "k", "status": "queued"}] * 5}, 0],
            [r"runs/2[34]/jobs", {"jobs": [done]}, 0]])
        self.write("tiers.json", {"tiers": []})
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(self.cancels(), ["21"])
        self.assertIn("keeping run 22 (#5) (cap 1)", self.text("watch.log"))

    def test_runs_already_being_cancelled_are_neither_kept_nor_cancelled(self):
        cancelling = [r"runs/22/jobs", {"jobs": [{"name": "j0", "status": "completed", "conclusion": "cancelled"}, {"name": "j1", "status": "in_progress"}]}, 0]
        self.starved_rules(10, [{"status": "queued"}], extra=[cancelling])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(sorted(self.cancels()), ["21", "23"])
        log = self.text("watch.log")
        self.assertIn("merge group starved: keeping run 24 (#7, 2 job(s) unfinished)", log)
        self.assertIn("ordinary runs: 3 started, 0 unstarted, 1 being cancelled", log)
        os.remove(self.calls_file)
        os.remove(os.path.join(self.dir, "auth_rerun.json"))
        self.starved_rules(10, [], extra=[cancelling])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(self.cancels(), ["21", "24"])
        self.assertIn("keeping run 23 (#6) (cap 1)", self.text("watch.log"))

    def test_a_failing_merge_group_is_the_only_one_cancelled_and_is_not_queued(self):
        self.cap(4)
        old = {"status": "queued", "created_at": stamp(10)}
        self.starved_rules(10, [{"name": "Go", "status": "completed", "conclusion": "failure"}, old],
                           extra=[[r"^run cancel 900 ", "", 0]])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(self.cancels(), ["900"])
        self.assertIsNone(self.cfg().read("auth_rerun.json"))
        log = self.text("watch.log")
        self.assertIn("heartbeat: mq waiting=no starved=no failing=yes (900)", log)
        self.assertIn("merge-group run 900: job 'Go' concluded failure; cancelled", log)
        for concluded in ("cancelled", "skipped"):
            os.remove(self.calls_file)
            self.starved_rules(10, [{"name": "Go", "status": "completed", "conclusion": concluded}, old])
            self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
            self.assertEqual(sorted(self.cancels()), ["21", "23", "24"], concluded)
        os.remove(self.calls_file)
        self.starved_rules(10, [], extra=[[r"runs/900/jobs", "HTTP 502", 1]])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertEqual(self.cancels(), [])
        self.assertIn("error, nothing more cancelled this pass", self.text("watch.log"))

    def test_one_heartbeat_per_pass(self):
        self.cap(4)
        self.starved_rules(1, [{"status": "queued", "created_at": stamp(1)}])
        for _ in range(3):
            self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        beats = [line for line in self.text("watch.log").splitlines() if "heartbeat:" in line]
        self.assertEqual(len(beats), 3)
        self.assertTrue(beats[0].endswith("STARVE-SWEEP heartbeat: mq waiting=yes starved=no failing=no; ordinary runs: 4 started, 0 unstarted, 0 being cancelled"))

    def test_idle_queue_api_error_and_absent_authorization(self):
        self.rules([[r"event=merge_group", {"workflow_runs": []}, 0], [r"runs\?status=", {"workflow_runs": []}, 0]])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertIn("starve_cancel.json absent", self.text("watch.log"))
        self.write("starve_cancel.json", {"by": "maintainer"})
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertIn("heartbeat: mq waiting=no starved=no failing=no; ordinary runs: 0 started, 0 unstarted", self.text("watch.log"))
        self.assertNotIn("error", self.text("watch.log"))
        self.rules([])
        self.assertEqual(starve_sweep.main(["--config", self.config, "--once"]), 0)
        self.assertIn("error, nothing more cancelled this pass", self.text("watch.log"))
        self.assertFalse([c for c in self.calls() if c.startswith("run cancel")])


A, B = "a" * 40, "b" * 40


def gql(n, state="OPEN", ref=None, head=None, base="develop", queued=None):
    return {"data": {"repository": {"pullRequest": {
        "number": n, "state": state, "headRefName": ref or f"feat/{n}", "headRefOid": head or f"{n:040d}", "baseRefName": base,
        "isInMergeQueue": queued is not None, "mergeQueueEntry": {"state": queued} if queued else None}}}}


class StackMonitorTest(Stub):
    def setUp(self):
        super().setUp()
        with open(os.path.join(self.dir, "bin", "git"), "w") as f:
            f.write(FAKE_GIT)
        os.chmod(os.path.join(self.dir, "bin", "git"), 0o755)
        self.git_calls, self.git_rules = os.path.join(self.dir, "git.calls"), os.path.join(self.dir, "git.rules")
        for k in ("FAKE_GIT_LOG", "FAKE_GIT_RULES"):
            self.env[k] = os.environ.get(k)
        os.environ.update(FAKE_GIT_LOG=self.git_calls, FAKE_GIT_RULES=self.git_rules)
        self.write("stacks.json", {"stacks": [[10, 11]]})

    def setup(self, pulls, git, open_prs=()):
        self.rules([[rf"^api graphql .* -F n={n}$", g, 0] for n, g in pulls.items()] + [
            [r"^api --paginate repos/Example/Repo/pulls\?state=open&per_page=100 --jq ",
             "".join(json.dumps(p) + "\n" for p in open_prs), 0]])
        with open(self.git_rules, "w") as f:
            json.dump(list(git) + [[r"^fetch --quiet origin \+refs/heads/develop:refs/remotes/origin/develop refs/pull/", "", 0]], f)

    def report(self):
        lines = []
        stack_monitor.Monitor(self.cfg(), out=lines.append).report()
        gits = [line.split()[0] for line in self.text("git.calls").splitlines()]
        self.assertLessEqual(set(gits), {"fetch", "merge-base", "diff"}, gits)
        self.assertFalse([c for c in self.calls() if throttle_common.mutating(c.split())])
        return lines

    def test_open_predecessor_contained_and_queued_means_queue_after(self):
        self.setup({10: gql(10, ref="feat/ten", head=A, queued="QUEUED"), 11: gql(11, head=B, base="feat/ten")},
                   [[rf"^merge-base --is-ancestor {A} {B}$", "", 0]])
        self.assertEqual(self.report(), ["#10: base develop, expected develop", "#11: base feat/ten, expected feat/ten",
                                         "#11: contains #10 head aaaaaaaaaa", "#11: queue after #10"])

    def test_lost_predecessor_head_is_unqualified_and_a_wrong_base_is_flagged(self):
        self.setup({10: gql(10, ref="feat/ten", head=A, queued="QUEUED"), 11: gql(11, head=B)},
                   [[rf"^merge-base --is-ancestor {A} {B}$", "", 1]])
        lines = self.report()
        self.assertIn("#11: base develop, expected feat/ten (MISMATCH)", lines)
        self.assertIn(f"#11: lost #10 head aaaaaaaaaa; unqualified until the owner runs git merge --no-ff {A}", lines)
        self.assertNotIn("#11: queue after #10", lines)

    def test_retarget_after_the_predecessor_merged_compares_diffs_and_the_head(self):
        mb = "c" * 40
        git = [[rf"^merge-base origin/develop {B}$", mb + "\n", 0], [rf"^diff --no-color {A} {B}$", "same", 0],
               [rf"^diff --no-color {mb} {B}$", "same", 0]]
        self.setup({10: gql(10, state="MERGED", head=A), 11: gql(11, head=B)}, git)
        lines = self.report()
        self.assertEqual(lines[:2], ["#10: merged", "#11: base develop, expected develop"])
        self.assertEqual(lines[2], "#11: retargeted to develop after #10 merged; its diff vs the merged #10 head aaaaaaaaaa matches "
                                   "its diff vs merge-base(develop) cccccccccc; head first seen at bbbbbbbbbb")
        self.assertTrue(self.report()[2].endswith("; head unchanged"))
        d = "d" * 40
        self.setup({10: gql(10, state="MERGED", head=A), 11: gql(11, head=d)},
                   [[rf"^merge-base origin/develop {d}$", mb, 0], [rf"^diff --no-color {A} {d}$", "one", 0], [rf"^diff --no-color {mb} {d}$", "two", 0]])
        line = self.report()[2]
        self.assertIn("differs from its diff vs merge-base(develop)", line)
        self.assertTrue(line.endswith("; head changed from bbbbbbbbbb to dddddddddd"))

    def test_closed_not_merged_member_is_reported(self):
        self.setup({10: gql(10, state="CLOSED"), 11: gql(11, base="feat/10")}, [])
        self.assertEqual(self.report(), ["#10: CLOSED, not merged", "#11: its predecessor #10 is CLOSED, not merged; base feat/10, no expected base"])

    def test_queued_member_turning_unmergeable_names_the_head_it_lacks(self):
        self.setup({10: gql(10, ref="feat/ten", head=A), 11: gql(11, head=B, base="feat/ten", queued="UNMERGEABLE")},
                   [[rf"^merge-base --is-ancestor {A} {B}$", "", 1]])
        self.assertIn(f"#11: UNMERGEABLE in the merge queue; lacks #10 head {A}", self.report())

    def auto_add(self, stacks_, open_prs):
        self.write("stacks.json", {"stacks": stacks_})
        self.setup({}, [], [{"n": n, "head": h, "base": b, "repo": repo} for n, h, b, repo in open_prs])
        lines = stack_monitor.Monitor(self.cfg(), out=lambda _: None).auto_add()
        return lines, self.read("stacks.json")["stacks"]

    def test_auto_add_appends_prepends_concatenates_and_creates_chains(self):
        def p(n, base="develop", repo=REPO):
            return (n, f"feat/{n}", base, repo)
        lines, chains = self.auto_add([[10, 11], [20, 21], [40, 41]], [
            p(10), p(11, "feat/10"), p(12, "feat/11"),              # append after the tail 11
            p(19), p(20, "feat/19"), p(21, "feat/20"),             # prepend 19 before the head 20
            p(30), p(31, "feat/30"),                               # new chain [30, 31]
            p(40, "feat/31"), p(41, "feat/40"),                    # concatenate [30, 31] + [40, 41]
            p(50, "feat/30", "someone/fork")])                     # a PR from another repository is ignored
        self.assertEqual(chains, [[10, 11, 12], [19, 20, 21], [30, 31, 40, 41]])
        self.assertEqual(lines, ["stack: #12 appended after #11: [10, 11, 12]", "stack: #19 prepended before #20: [19, 20, 21]",
                                 "stack: new chain [30, 31]", "stack: #40 joined after #31, concatenating their chains: [30, 31, 40, 41]"])
        self.assertFalse([c for c in self.calls() if throttle_common.mutating(c.split())])

    def test_auto_add_reports_a_fork_or_a_different_predecessor_and_changes_nothing(self):
        before = {"stacks": [[10, 11], [20, 21]], "note": "kept"}
        self.write("stacks.json", before)
        self.setup({}, [], [{"n": n, "head": f"feat/{n}", "base": b, "repo": REPO} for n, b in
                            ((10, "develop"), (11, "feat/10"), (12, "feat/10"), (20, "develop"), (21, "feat/12"))])
        lines = stack_monitor.Monitor(self.cfg(), out=lambda _: None).auto_add()
        self.assertEqual(lines, ["#12: builds on #10, which already has successor #11 (a fork); not changed",
                                 "#21: builds on #12 but follows #20 in stacks.json; not changed"])
        self.assertEqual(self.read("stacks.json"), before)
        self.assertNotIn("stacks.json.tmp", os.listdir(self.dir))

    def test_auto_add_refuses_a_cycle_without_recording_any_part_of_it(self):
        lines, chains = self.auto_add([], [(1, "feat/1", "feat/2", REPO), (2, "feat/2", "feat/1", REPO),
                                           (3, "feat/3", "feat/1", REPO)])
        self.assertEqual(chains, [[1, 3]])
        self.assertEqual(lines, ["#1: builds on #2, whose branch builds on #1's (a cycle); not changed",
                                 "#2: builds on #1, whose branch builds on #2's (a cycle); not changed",
                                 "stack: new chain [1, 3]"])

    def test_auto_add_matches_the_repository_name_case_insensitively(self):
        lines, chains = self.auto_add([], [(1, "feat/1", "develop", REPO.upper()), (2, "feat/2", "feat/1", REPO.upper())])
        self.assertEqual((lines, chains), (["stack: new chain [1, 2]"], [[1, 2]]))

    def test_git_refuses_every_mutating_subcommand(self):
        g = stack_monitor.Git(self.dir)
        for sub in ("push", "rebase", "merge", "commit", "reset", "checkout"):
            with self.assertRaisesRegex(RuntimeError, "read-only"):
                g(sub, "origin")
        self.assertEqual(self.text("git.calls"), "")

    def test_a_github_error_fails_closed_without_recording_heads(self):
        self.rules([[r"^api graphql", "HTTP 502", 1]])
        with redirect_stderr(io.StringIO()) as err:
            self.assertEqual(stack_monitor.main(["--config", self.config]), 1)
        self.assertIn("nothing reported", err.getvalue())
        self.assertFalse(os.path.exists(os.path.join(self.dir, "stack_state.json")))
        self.assertEqual(self.text("git.calls"), "")


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
