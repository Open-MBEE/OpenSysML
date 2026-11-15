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
        """(waiting, starved): a merge-group run has queued or waiting jobs; one older than the grace period has started none."""
        waiting = starved = False
        now = datetime.datetime.now(datetime.timezone.utc)
        for r in self.merge_group_runs():
            jobs = None
            if r["status"] == "queued":
                waiting = True
            else:
                jobs = self.jobs(r["id"])
                waiting = waiting or any(j["status"] in WAITING for j in jobs)
            created = r.get("created_at")
            if created and (now - datetime.datetime.fromisoformat(created.replace("Z", "+00:00"))).total_seconds() >= self.cfg["starve_grace_seconds"]:
                jobs = self.jobs(r["id"]) if jobs is None else jobs
                # A run with no jobs at all is a GitHub ghost holding no runner, not a starved build.
                starved = starved or (bool(jobs) and not heavy_job_started(self.cfg, jobs))
        return waiting, starved

    def cancel_and_queue(self, r, why):
        pr, repo = r["pull_requests"][0]["number"], self.cfg.repo
        done = self.gh(["run", "cancel", str(r["id"]), "-R", repo], ok_fail=True) is not None
        if done:
            with self.cfg.locked("auth_rerun.json"):
                todo = [e for e in self.cfg.read("auth_rerun.json", []) if e["pr"] != pr]
                todo.append({"pr": pr, "run": r["id"], "head": r["head_sha"][:10]})
                self.cfg.write("auth_rerun.json", sorted(todo, key=self.rank))
        self.log(f"#{pr}: run {r['id']} ({r['head_sha'][:10]}, {why}) {'cancelled; queued for re-run under the cap' if done else 'not cancelled'}")

    def sweep(self):
        """One pass; any API error raises before anything more is cancelled."""
        waiting, starved = self.queue_state()
        if not (waiting or starved):
            return
        repo, workflows = self.cfg.repo, set(self.cfg["workflows"])
        cand = {}
        for st in ACTIVE:
            for r in self.api(f"repos/{repo}/actions/runs?event=pull_request&status={st}&per_page=100")["workflow_runs"]:
                cand[r["id"]] = r
        running = []
        for r in cand.values():
            if r["event"] != "pull_request" or r.get("name") not in workflows or not r.get("pull_requests") \
                    or protected_branch(self.cfg, r["head_branch"], r["event"]):
                continue
            jobs = self.jobs(r["id"])
            # A run with no jobs uses no runner, and GitHub refuses to retry it once cancelled.
            if not jobs:
                continue
            if waiting and not heavy_job_started(self.cfg, jobs):
                self.cancel_and_queue(r, "no job started")
            elif starved and any(j["status"] == "in_progress" for j in jobs):
                unfinished = sum(1 for j in jobs if j["status"] != "completed")
                running.append((unfinished, self.rank({"pr": r["pull_requests"][0]["number"]}), r))
        if len(running) > 1:
            running.sort(key=lambda x: (x[0], x[1]))
            keep = running[0][2]
            self.log(f"merge group starved: keeping run {keep['id']} (#{keep['pull_requests'][0]['number']}, {running[0][0]} job(s) unfinished)")
            for unfinished, _, r in running[1:]:
                self.cancel_and_queue(r, f"started, {unfinished} job(s) unfinished, merge group starved")


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
