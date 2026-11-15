#!/usr/bin/env python3
"""Report the state of every stack in stacks.json: bases, predecessor containment, retargets and merge-queue order.

Read-only: it reads GitHub through the guarded gh wrapper and the configured checkout through a git wrapper that refuses
every subcommand but fetch, rev-parse, merge-base and diff. It never pushes, rebases or merges. Its only write is
stack_state.json in the state directory, the heads it saw last.
"""
import argparse
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from throttle_common import GH, Config, stacks, utc_stamp  # noqa: E402

READ_ONLY_GIT = ("fetch", "rev-parse", "merge-base", "diff")
PR_QUERY = ("query($o:String!,$r:String!,$n:Int!){repository(owner:$o,name:$r){pullRequest(number:$n){"
            "number state headRefName headRefOid baseRefName isInMergeQueue mergeQueueEntry{state}}}}")


class Git:
    """git in the configured checkout, limited to subcommands that change no branch, index or work tree."""

    def __init__(self, checkout):
        if not checkout:
            raise RuntimeError("the stack monitor needs `checkout` in the configuration")
        self.checkout = checkout

    def __call__(self, *args, ok_rc=(0,)):
        if args[0] not in READ_ONLY_GIT:
            raise RuntimeError(f"git {args[0]} refused: the stack monitor is read-only")
        p = subprocess.run(["git", "-C", self.checkout, *args], capture_output=True, text=True, timeout=300)
        if p.returncode not in ok_rc:
            raise RuntimeError(f"git {' '.join(args)}: {p.stderr.strip()[:300]}")
        return p


class Monitor:
    def __init__(self, cfg, out=print):
        self.cfg, self.out = cfg, out
        self.gh = GH(cfg, self.log)
        self.git = Git(cfg.checkout)
        self.base = cfg["base_branch"]

    def log(self, msg):
        with open(self.cfg.file("watch.log"), "a") as f:
            f.write(f"{utc_stamp()} STACK-MONITOR {msg}\n")

    def pull(self, n):
        owner, name = self.cfg.repo.split("/", 1)
        d = self.gh.json(["api", "graphql", "-f", f"query={PR_QUERY}", "-f", f"o={owner}", "-f", f"r={name}", "-F", f"n={n}"])
        p = d["data"]["repository"]["pullRequest"]
        if p is None:
            raise RuntimeError(f"#{n}: no such pull request")
        return p

    def contains(self, ancestor, head):
        return self.git("merge-base", "--is-ancestor", ancestor, head, ok_rc=(0, 1)).returncode == 0

    def diff(self, a, b):
        return self.git("diff", "--no-color", a, b).stdout

    def report(self):
        """One report line per finding; any GitHub or git error raises before anything is reported or recorded."""
        chains = stacks(self.cfg)
        prs = {n: self.pull(n) for s in chains for n in s}
        if prs:
            self.git("fetch", "--quiet", "origin", f"+refs/heads/{self.base}:refs/remotes/origin/{self.base}",
                     *(f"refs/pull/{n}/head" for n in prs))
        seen = (self.cfg.read("stack_state.json") or {}).get("heads", {})
        lines = []
        for s in chains:
            for i, n in enumerate(s):
                lines += self.member(s[:i], n, prs, seen.get(str(n)))
        for line in lines:
            self.out(line)
        self.cfg.write("stack_state.json", {"heads": {str(n): p["headRefOid"] for n, p in prs.items()}})
        return lines

    def member(self, before, n, prs, last):
        p = prs[n]
        if p["state"] == "CLOSED":
            return [f"#{n}: CLOSED, not merged"]
        if p["state"] == "MERGED":
            return [f"#{n}: merged"]
        head, actual = p["headRefOid"], p["baseRefName"]
        pred = next((m for m in reversed(before) if prs[m]["state"] != "MERGED"), None)
        merged = next((m for m in reversed(before) if prs[m]["state"] == "MERGED"), None)
        if pred is not None and prs[pred]["state"] == "CLOSED":
            return [f"#{n}: its predecessor #{pred} is CLOSED, not merged; base {actual}, no expected base"]
        expected = prs[pred]["headRefName"] if pred is not None else self.base
        qualifies = actual == expected
        lines = [f"#{n}: base {actual}, expected {expected}" + ("" if qualifies else " (MISMATCH)")]
        lacks = None
        if pred is not None:
            ph = prs[pred]["headRefOid"]
            if self.contains(ph, head):
                lines.append(f"#{n}: contains #{pred} head {ph[:10]}")
            else:
                lacks, qualifies = ph, False
                lines.append(f"#{n}: lost #{pred} head {ph[:10]}; unqualified until the owner runs git merge --no-ff {ph}")
        elif merged is not None and actual == self.base:
            mh = prs[merged]["headRefOid"]
            mb = self.git("merge-base", f"origin/{self.base}", head).stdout.strip()
            same = self.diff(mh, head) == self.diff(mb, head)
            moved = "unchanged" if last == head else f"first seen at {head[:10]}" if last is None else f"changed from {last[:10]} to {head[:10]}"
            lines.append(f"#{n}: retargeted to {self.base} after #{merged} merged; its diff vs the merged #{merged} head {mh[:10]} "
                         f"{'matches' if same else 'differs from'} its diff vs merge-base({self.base}) {mb[:10]}; head {moved}")
        if p["isInMergeQueue"] and (p.get("mergeQueueEntry") or {}).get("state") == "UNMERGEABLE":
            lines.append(f"#{n}: UNMERGEABLE in the merge queue" + (f"; lacks #{pred} head {lacks}" if lacks else ""))
        if pred is not None and prs[pred]["isInMergeQueue"] and qualifies and not p["isInMergeQueue"]:
            lines.append(f"#{n}: queue after #{pred}")
        return lines


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--config")
    args = ap.parse_args(argv)
    cfg = Config(args.config)
    try:
        Monitor(cfg).report()
    except Exception as ex:
        print(f"ERROR: {ex}; nothing reported", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
