#!/usr/bin/env python3
"""While a merge-queue run waits for runners, cancel ordinary pull_request runs that would delay it and queue them for re-run.

Runs only while starve_cancel.json exists: that file is the maintainer's standing authorization for these cancels.
"""
import argparse
import datetime
import os
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from throttle_common import GH, Config, heavy_job_started, protected_branch, utc_stamp  # noqa: E402

WAITING = ("queued", "waiting")
ACTIVE = ("queued", "in_progress", "waiting", "pending", "requested")


class Sweeper:
    def __init__(self, cfg):
        self.cfg = cfg
        self.gh = GH(cfg, self.log)

    def log(self, msg):
        with open(self.cfg.file("watch.log"), "a") as f:
            f.write(f"{utc_stamp()} STARVE-SWEEP {msg}\n")

    def api(self, path):
        return self.gh.json(["api", path])

    def rank(self, e):
        """tiers.json order, then every other PR by age (PR number), as the watcher ranks them."""
        tiers = (self.cfg.read("tiers.json") or {}).get("tiers", [])
        for i, t in enumerate(tiers):
            if e["pr"] in t:
                return (i, t.index(e["pr"]))
        return (len(tiers), e["pr"])

    def merge_group_runs(self):
        runs = {}
        for st in ACTIVE:
            for r in self.api(f"repos/{self.cfg.repo}/actions/runs?event=merge_group&status={st}&per_page=50")["workflow_runs"]:
                runs[r["id"]] = r
        return list(runs.values())

    def jobs(self, rid):
        return self.api(f"repos/{self.cfg.repo}/actions/runs/{rid}/jobs?per_page=100")["jobs"]

    def queue_state(self):
        """(waiting, starved, failing): a merge-group job is queued or waiting; one has waited longer than the grace
        period; the merge-group runs with a job concluded "failure". A failing run's waiting jobs count for neither."""
        waiting = starved = False
        failing = []
        now = datetime.datetime.now(datetime.timezone.utc)
        for r in self.merge_group_runs():
            jobs = self.jobs(r["id"])
            bad = [j for j in jobs if j.get("conclusion") == "failure"]
            if bad:
                failing.append((r, bad[0].get("name")))
                continue
            waiting = waiting or r["status"] == "queued"
            # A run with no jobs at all has nothing waiting for a runner, so it is never starved.
            for j in jobs:
                if j["status"] not in WAITING:
                    continue
                waiting = True
                created = j.get("created_at") or r.get("created_at")
                if created and (now - datetime.datetime.fromisoformat(created.replace("Z", "+00:00"))).total_seconds() >= self.cfg["starve_grace_seconds"]:
                    starved = True
        return waiting, starved, failing

    def cancel_failing_group(self, r, job):
        """The one exception to "a merge-group run is never cancelled": one of its jobs concluded "failure", so the group
        cannot merge and its remaining jobs only hold runners. The merge queue re-builds it; it is not queued for re-run."""
        done = self.gh(["run", "cancel", str(r["id"]), "-R", self.cfg.repo], ok_fail=True) is not None
        self.log(f"merge-group run {r['id']}: job {job!r} concluded failure; {'cancelled' if done else 'not cancelled'}")

    def closeness(self, jobs):
        """(no completed non-preflight, non-skipped job, unfinished jobs): lower is closer to finishing. Until a
        non-preflight job completes, the matrix may not have expanded, so the unfinished count understates the work left."""
        unexpanded = not any(j.get("name") not in self.cfg["preflight_jobs"] and j["status"] == "completed"
                             and j.get("conclusion") != "skipped" for j in jobs)
        return unexpanded, sum(1 for j in jobs if j["status"] != "completed")

    def cancel_and_queue(self, r, why):
        pr, repo = r["pull_requests"][0]["number"], self.cfg.repo
        done = self.gh(["run", "cancel", str(r["id"]), "-R", repo], ok_fail=True) is not None
        if done:
            with self.cfg.locked("auth_rerun.json"):
                todo = [e for e in self.cfg.read("auth_rerun.json", []) if e["pr"] != pr]
                todo.append({"pr": pr, "run": r["id"], "head": r["head_sha"][:10]})
                self.cfg.write("auth_rerun.json", sorted(todo, key=self.rank))
        self.log(f"#{pr}: run {r['id']} ({r['head_sha'][:10]}, {why}) {'cancelled; queued for re-run under the cap' if done else 'not cancelled'}")

    def ordinary_runs(self):
        """(started, unstarted, cancelling) active ordinary runs, each started or unstarted one as (closeness, rank, run).
        A run with no jobs uses no runner and GitHub refuses to retry it once cancelled; a run with a job concluded
        "cancelled" is already being cancelled. Neither is kept or cancelled."""
        repo, workflows = self.cfg.repo, set(self.cfg["workflows"])
        cand = {}
        for st in ACTIVE:
            for r in self.api(f"repos/{repo}/actions/runs?event=pull_request&status={st}&per_page=100")["workflow_runs"]:
                cand[r["id"]] = r
        started, unstarted, cancelling = [], [], 0
        for r in cand.values():
            if r["event"] != "pull_request" or r.get("name") not in workflows or not r.get("pull_requests") \
                    or protected_branch(self.cfg, r["head_branch"], r["event"]):
                continue
            jobs = self.jobs(r["id"])
            if not jobs or all(j["status"] == "completed" for j in jobs):
                continue
            if any(j.get("conclusion") == "cancelled" for j in jobs):
                cancelling += 1
                continue
            e = (self.closeness(jobs), self.rank({"pr": r["pull_requests"][0]["number"]}), r)
            (started if heavy_job_started(self.cfg, jobs) else unstarted).append(e)
        return started, unstarted, cancelling

    def idle_cap(self):
        """The configured cap, or the higher cap of wide_cap.json while the watcher keeps that file."""
        w = self.cfg.read("wide_cap.json")
        return max(self.cfg["cap"], int(w["cap"])) if w else self.cfg["cap"]

    def sweep(self):
        """One pass; any API error raises before anything more is cancelled."""
        waiting, starved, failing = self.queue_state()
        started, unstarted, cancelling = self.ordinary_runs()
        fails = "yes (%s)" % ", ".join(str(r["id"]) for r, _ in failing) if failing else "no"
        self.log(f"heartbeat: mq waiting={'yes' if waiting else 'no'} starved={'yes' if starved else 'no'} failing={fails}; "
                 f"ordinary runs: {len(started)} started, {len(unstarted)} unstarted, {cancelling} being cancelled")
        for r, job in failing:
            self.cancel_failing_group(r, job)
        if waiting:
            for _, _, r in unstarted:
                self.cancel_and_queue(r, "no job started")
            if starved and len(started) > 1:
                started.sort(key=lambda e: (e[0], e[1]))
                (_, unfinished), _, keep = started[0]
                self.log(f"merge group starved: keeping run {keep['id']} (#{keep['pull_requests'][0]['number']}, {unfinished} job(s) unfinished)")
                for (_, unfinished), _, r in started[1:]:
                    self.cancel_and_queue(r, f"started, {unfinished} job(s) unfinished, merge group starved")
            return
        cap, active = self.idle_cap(), started + unstarted
        if len(active) <= cap:
            return
        # Tier first, then closest to finishing: the lowest tier goes first and, within a tier, the run furthest from finishing.
        active.sort(key=lambda e: (e[1][0], e[0], e[1]))
        kept = ", ".join("run %d (#%d)" % (r["id"], r["pull_requests"][0]["number"]) for _, _, r in active[:cap]) or "none"
        self.log(f"over the cap with the merge queue idle: {len(active)} ordinary runs active, keeping {kept} (cap {cap})")
        for (_, unfinished), _, r in reversed(active[cap:]):
            self.cancel_and_queue(r, f"{unfinished} job(s) unfinished, over the cap of {cap} with the merge queue idle")


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--config")
    ap.add_argument("--dry-run", action="store_true", help="log the cancels instead of sending them")
    ap.add_argument("--once", action="store_true", help="run one pass and exit")
    args = ap.parse_args(argv)
    cfg = Config(args.config, dry_run=args.dry_run)
    s = Sweeper(cfg)
    while cfg.exists("starve_cancel.json"):
        try:
            s.sweep()
        except Exception as ex:
            s.log(f"error, nothing more cancelled this pass: {ex}")
        if args.once:
            return 0
        time.sleep(60)
    s.log("starve_cancel.json absent; sweeper exiting")
    return 0


if __name__ == "__main__":
    sys.exit(main())
