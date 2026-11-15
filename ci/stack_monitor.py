#!/usr/bin/env python3
"""Report the state of every stack in stacks.json: bases, predecessor containment, retargets and merge-queue order.

Each pass first adds stacked open PRs to stacks.json: a PR whose base is the head branch of another open PR of the
repository joins that PR's chain. Read-only otherwise: it reads GitHub through the guarded gh wrapper and the configured
checkout through a git wrapper that refuses every subcommand but fetch, rev-parse, merge-base and diff. It never pushes,
rebases or merges. Its only writes are stacks.json and stack_state.json (the heads it saw last) in the state directory.
"""
import argparse
import json
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

    def open_prs(self):
        """Every open PR (all pages) as {"n", "head", "base", "repo"}; repo is the head's repository, None when deleted."""
        out = self.gh(["api", "--paginate", f"repos/{self.cfg.repo}/pulls?state=open&per_page=100", "--jq",
                       ".[] | {n: .number, head: .head.ref, base: .base.ref, repo: .head.repo.full_name}"])
        return [json.loads(line) for line in out.splitlines() if line.strip()]

    def auto_add(self):
        """Join each open PR whose base is another open same-repository PR's head branch to that PR's chain, and return one
        line per addition or refusal. A fork (the predecessor already has another successor), a PR already in a chain
        under another predecessor, or an ambiguous predecessor is reported and left unchanged."""
        # GitHub repository names are case-insensitive, and the API answers with the canonical spelling.
        repo = self.cfg.repo.lower()
        prs = sorted((p for p in self.open_prs() if (p["repo"] or "").lower() == repo), key=lambda p: p["n"])
        by_head = {}
        for p in prs:
            by_head.setdefault(p["head"], []).append(p["n"])
        data = self.cfg.read("stacks.json") or {}
        chains = [list(s) for s in data.get("stacks", [])]
        lines = []

        def chain_of(n):
            return next((s for s in chains if n in s), None)
        for p in prs:
            n, preds = p["n"], [m for m in by_head.get(p["base"], []) if m != p["n"]]
            if p["base"] == self.base or not preds:
                continue
            if len(preds) > 1:
                lines.append(f"#{n}: base {p['base']} is the head of several open PRs {preds}; stacks.json not changed")
                continue
            pred = preds[0]
            cp, cn = chain_of(pred), chain_of(n)
            if cp is not None and cp is cn and cp.index(n) == cp.index(pred) + 1:
                continue
            if cn is not None and cn.index(n) > 0:
                lines.append(f"#{n}: builds on #{pred} but follows #{cn[cn.index(n) - 1]} in stacks.json; not changed")
                continue
            if cp is not None and cp[-1] != pred:
                lines.append(f"#{n}: builds on #{pred}, which already has successor #{cp[cp.index(pred) + 1]} (a fork); not changed")
                continue
            if cp is not None and cp is cn:
                lines.append(f"#{n}: builds on #{pred}, which follows it in the same chain (a cycle); not changed")
                continue
            if cp is not None and cn is not None:
                cp.extend(cn)
                chains.remove(cn)
                lines.append(f"stack: #{n} joined after #{pred}, concatenating their chains: {cp}")
            elif cp is not None:
                cp.append(n)
                lines.append(f"stack: #{n} appended after #{pred}: {cp}")
            elif cn is not None:
                cn.insert(0, pred)
                lines.append(f"stack: #{pred} prepended before #{n}: {cn}")
            else:
                chains.append([pred, n])
                lines.append(f"stack: new chain {[pred, n]}")
        if any(line.startswith("stack: ") for line in lines):
            self.cfg.write("stacks.json", dict(data, stacks=chains))
        for line in lines:
            self.log(line)
        return lines

    def report(self):
        """Stack additions, then one report line per finding; any GitHub or git error raises before anything more is
        reported or recorded."""
        added = self.auto_add()
        chains = stacks(self.cfg)
        prs = {n: self.pull(n) for s in chains for n in s}
        if prs:
            self.git("fetch", "--quiet", "origin", f"+refs/heads/{self.base}:refs/remotes/origin/{self.base}",
                     *(f"refs/pull/{n}/head" for n in prs))
        seen = (self.cfg.read("stack_state.json") or {}).get("heads", {})
        lines = list(added)
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
