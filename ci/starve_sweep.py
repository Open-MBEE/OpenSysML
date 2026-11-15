#!/usr/bin/env python3
"""While a merge-queue run waits for runners, cancel ordinary pull_request runs with no started job and queue them for re-run.

Runs only while starve_cancel.json exists: that file is the maintainer's standing authorization for these cancels.
"""
import argparse
import os
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from throttle_common import GH, Config, protected_branch, utc_stamp  # noqa: E402

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

    def mq_waiting(self):
        repo = self.cfg.repo
        for st in ACTIVE:
            for r in self.api(f"repos/{repo}/actions/runs?event=merge_group&status={st}&per_page=50")["workflow_runs"]:
                if r["status"] == "queued":
                    return True
                jobs = self.api(f"repos/{repo}/actions/runs/{r['id']}/jobs?per_page=100")["jobs"]
                if any(j["status"] in WAITING for j in jobs):
                    return True
        return False

    def sweep(self):
        """One pass; any API error raises before anything is cancelled for that run."""
        if not self.mq_waiting():
            return
        repo, workflows = self.cfg.repo, set(self.cfg["workflows"])
        cand = {}
        for st in ACTIVE:
            for r in self.api(f"repos/{repo}/actions/runs?event=pull_request&status={st}&per_page=100")["workflow_runs"]:
                cand[r["id"]] = r
        for r in cand.values():
            if r["event"] != "pull_request" or r.get("name") not in workflows or not r.get("pull_requests") \
                    or protected_branch(self.cfg, r["head_branch"], r["event"]):
                continue
            jobs = self.api(f"repos/{repo}/actions/runs/{r['id']}/jobs?per_page=100")["jobs"]
            # A run with no jobs uses no runner, and GitHub refuses to retry it once cancelled.
            if not jobs or any(j["status"] != "queued" for j in jobs):
                continue
            pr = r["pull_requests"][0]["number"]
            done = self.gh(["run", "cancel", str(r["id"]), "-R", repo], ok_fail=True) is not None
            if done:
                todo = [e for e in self.cfg.read("auth_rerun.json", []) if e["pr"] != pr]
                todo.append({"pr": pr, "run": r["id"], "head": r["head_sha"][:10]})
                self.cfg.write("auth_rerun.json", sorted(todo, key=self.rank))
            self.log(f"#{pr}: run {r['id']} ({r['head_sha'][:10]}, no job started) "
                     f"{'cancelled; queued for re-run under the cap' if done else 'not cancelled'}")


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
