#!/usr/bin/env python3
"""Re-run the failed jobs of tracked runs that failed on infrastructure (setup steps, DNS, apt mirrors), once it has recovered.

The tracked runs are infra_runs.json: [{"run": id, "pr": n, "head": full sha, "hold": bool}, ...], in re-run order.
"""
import argparse
import calendar
import json
import os
import re
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from throttle_common import GH, Config  # noqa: E402

SETUP = re.compile(r"install z3|download go modules|provision.*pinned corpora|toolchain|set ?up go|apt", re.I)
APTNET = re.compile(r"apt-get (update|install) failed \(attempt 3 of 3\)\s*\n(?:[^\n]*\n){0,2}##\[error\]Process completed")
MAX_RERUNS = 3


def ts(s):
    return calendar.timegm(time.strptime(s[:19], "%Y-%m-%dT%H:%M:%S")) if s else 0


def hhmm(t):
    return time.strftime("%H:%M:%SZ", time.gmtime(t))


class Tracker:
    def __init__(self, cfg):
        self.cfg = cfg
        self.gh = GH(cfg, self.out)
        self.runs = {int(e["run"]): e for e in cfg.read("infra_runs.json", [])}
        self.order = list(self.runs)
        self.logs, self.reruns, self.prevfail = {}, {}, {}
        self.last_fail, self.recovered_said = 0, False

    def out(self, m):
        with open(self.cfg.file("infra.out"), "a") as f:
            f.write(time.strftime("%Y-%m-%dT%H:%M:%SZ ", time.gmtime()) + m + "\n")

    def classify(self, rid, d):
        """Record each failed job as INFRA, RED or ESCALATE; return the newest setup-step success time."""
        probe = 0
        for j in d["jobs"]:
            for st in j.get("steps", []):
                if SETUP.search(st["name"]) and st.get("conclusion") == "success":
                    probe = max(probe, ts(st.get("completedAt") or j.get("completedAt") or ""))
            if j.get("conclusion") != "failure":
                continue
            fs = [st["name"] for st in j.get("steps", []) if st.get("conclusion") == "failure"]
            if fs == ["Verify required checks"]:
                continue
            key = (rid, d["attempt"], j["databaseId"])
            if key not in self.logs:
                kind = "INFRA" if fs and all(SETUP.search(x) for x in fs) else "RED"
                if kind == "RED":
                    lg = self.gh(["api", f"repos/{self.cfg.repo}/actions/jobs/{j['databaseId']}/logs"], ok_fail=True) or ""
                    if APTNET.search(re.sub(r"(?m)^\S+Z ", "", lg)):
                        kind = "INFRA"
                if kind == "INFRA" and d["attempt"] > 1 and self.prevfail.get((rid, j["name"])) == fs:
                    kind = "ESCALATE"
                self.prevfail[(rid, j["name"])] = fs
                self.logs[key] = kind
                self.out(f"{kind} #{self.runs[rid]['pr']} {rid} attempt {d['attempt']}: job '{j['name']}' failed at {j.get('completedAt')} in {fs}")
            if self.logs[key] == "INFRA":
                self.last_fail = max(self.last_fail, ts(j.get("completedAt")))
        return probe

    def step(self):
        data, probe = {}, 0
        for rid in list(self.runs):
            s = self.gh(["run", "view", str(rid), "--repo", self.cfg.repo, "--json", "status,conclusion,attempt,headSha,jobs"], ok_fail=True)
            if s:
                data[rid] = json.loads(s)
        for rid, d in data.items():
            probe = max(probe, self.classify(rid, d))
        now = time.time()
        quiet = now - self.last_fail
        recovered = bool(self.last_fail) and ((probe > self.last_fail and quiet >= 300) or quiet >= 600)
        if recovered and not self.recovered_said:
            self.recovered_said = True
            self.out("infrastructure looks recovered: " + (
                f"a setup step succeeded at {hhmm(probe)}, 5+ quiet min after the last failure at {hhmm(self.last_fail)}"
                if probe > self.last_fail else f"10 min since the last failure at {hhmm(self.last_fail)}"))
        for rid in [r for r in self.order if r in data]:
            d, m = data[rid], self.runs[rid]
            if d["status"] != "completed":
                if m.get("hold") and not self.reruns.get(rid):
                    break
                continue
            kinds = {self.logs[k] for k in self.logs if k[0] == rid and k[1] == d["attempt"]}
            if d["conclusion"] == "success":
                self.out(f"DONE #{m['pr']} {rid} attempt {d['attempt']}: GREEN")
                self.runs.pop(rid)
                continue
            if "RED" in kinds or "ESCALATE" in kinds or not kinds or d["headSha"] != m["head"] or self.reruns.get(rid, 0) >= MAX_RERUNS:
                self.out(f"FINAL #{m['pr']} {rid} attempt {d['attempt']}: {d['conclusion']} kinds={sorted(kinds)}; no auto-rerun")
                self.runs.pop(rid)
                continue
            if not recovered:
                break
            if self.cfg.frozen() or self.cfg.dry_run:
                self.out(f"{'FROZEN (freeze.json)' if self.cfg.frozen() else 'DRY RUN'}: rerun --failed of {rid} deferred")
                break
            if self.gh(["run", "rerun", str(rid), "--repo", self.cfg.repo, "--failed"], ok_fail=True) is None:
                self.out(f"ERROR #{m['pr']} {rid}: rerun --failed refused")
                self.runs.pop(rid)
                continue
            self.reruns[rid] = self.reruns.get(rid, 0) + 1
            if m.get("hold"):
                self.cfg.write(f"priority_hold_{m['pr']}.json", {"pr": m["pr"], "after": m["head"][:10], "run": rid, "head": m["head"],
                                                                 "label": f"#{m['pr']} {m['head'][:10]} run"})
            self.out(f"RERUN #{m['pr']} {rid}: rerun --failed sent (attempt {d['attempt'] + 1})" + ("; priority hold re-armed" if m.get("hold") else ""))
            break


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--config")
    ap.add_argument("--dry-run", action="store_true", help="classify and log, never re-run")
    ap.add_argument("--once", action="store_true", help="run one pass and exit")
    args = ap.parse_args(argv)
    t = Tracker(Config(args.config, dry_run=args.dry_run))
    while t.runs:
        t.step()
        if args.once:
            return 0
        time.sleep(30)
    t.out("all tracked runs settled; exiting")
    return 0


if __name__ == "__main__":
    sys.exit(main())
