#!/usr/bin/env python3
"""Throttle and watch loop for a repository's pull-request CI runs; see ci/README.md."""
import argparse
import base64
import datetime
import functools
import glob
import json
import os
import re
import subprocess
import sys
import time
import traceback
import urllib.parse

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from throttle_common import GH, Config, heavy_job_started, priority, protected_branch, stack_of, utc_stamp  # noqa: E402

CFG = None
GH_RUN = None
REPO = DIR = STATE = LOG = OUTBOX = None
BASE_BRANCH = MAIN_WF = REQUIRED = None
CAP = BASE_CAP = INTERVAL = FAST = 0
WORKFLOWS = set()
PREFLIGHT_JOBS = set()
IGNORED_PRS = set()
EXEMPT_PRS = set()
HOLDS = {}
PINNED = set()
NOCANCEL = set()
READY_OK = set()
ACTIVE = ["queued", "in_progress", "pending", "waiting", "requested"]
F_STATE, F_HOLD = "state.json", "hold.json"
F_CI_PRIO, F_EXT_OK = "ci_prio.json", "ext_ok.json"
F_REVIEW_GATE, F_AUTH_RERUN = "review_gate.json", "auth_rerun.json"


def configure(cfg):
    """Bind the module to one configuration; every path, name and limit below comes from it."""
    global CFG, GH_RUN, REPO, DIR, STATE, LOG, OUTBOX, BASE_BRANCH, MAIN_WF, REQUIRED
    global CAP, BASE_CAP, INTERVAL, FAST, WORKFLOWS, PREFLIGHT_JOBS, CI_PRS, CI_FLAGGED, CI_AFTER
    CFG, REPO, DIR = cfg, cfg.repo, cfg.state_dir
    STATE, LOG, OUTBOX = cfg.file(F_STATE), cfg.file("watch.log"), cfg.file("outbox.jsonl")
    GH_RUN = GH(cfg, log)
    BASE_BRANCH, REQUIRED = cfg["base_branch"], cfg["required_check"]
    WORKFLOWS = set(cfg["workflows"])
    MAIN_WF = cfg["workflows"][0]
    PREFLIGHT_JOBS = set(cfg["preflight_jobs"])
    CAP = BASE_CAP = int(cfg["cap"])
    INTERVAL, FAST = int(cfg["interval_seconds"]), int(cfg["fast_poll_seconds"])
    EXEMPT_PRS.clear()
    EXEMPT_PRS.update(cfg.read("exempt.json", []))
    CI_PRS = set(cfg.read(F_CI_PRIO, []))
    CI_FLAGGED = set(cfg.read("ci_flagged.json", []))
    CI_AFTER = (cfg["ci_priority"] or {}).get("after_pr")


def now():
    return time.time()


def parse_utc(stamp):
    """A GitHub timestamp (2026-01-02T03:04:05Z) as an aware datetime."""
    return datetime.datetime.fromisoformat(stamp.replace("Z", "+00:00"))


def ts():
    return utc_stamp()


def log(msg):
    with open(LOG, "a") as f:
        f.write(f"{ts()} {msg}\n")


def report(msg):
    log("REPORT " + msg)
    with open(OUTBOX, "a") as f:
        f.write(json.dumps({"ts": ts(), "msg": msg}) + "\n")


def gh(args, ok_fail=False):
    return GH_RUN(args, ok_fail)


def gh_json(args, ok_fail=False):
    out = gh(args, ok_fail)
    return None if out is None else json.loads(out or "null")


def exempt(branch):
    return protected_branch(CFG, branch)


def load_json(path):
    with open(path) as f:
        return json.load(f)


def dump_json(path, value):
    with open(path, "w") as f:
        json.dump(value, f, indent=1)


def load():
    st = CFG.read(F_STATE, {})
    for k, v in (("holders", {}), ("queue", []), ("parked", {}), ("cancelled", []), ("order", [])):
        st.setdefault(k, v)
    return st


def save(st):
    tmp = STATE + ".tmp"
    with open(tmp, "w") as f:
        json.dump(st, f, indent=1)
    os.replace(tmp, STATE)


def active_runs():
    runs = {}
    for s in ACTIVE:
        out = gh_json(["run", "list", "--repo", REPO, "--status", s, "-L", "100", "--json",
                       "databaseId,headBranch,event,workflowName,createdAt,startedAt,headSha,status,attempt"])
        for r in out or []:
            runs[r["databaseId"]] = r
    return [r for r in runs.values()
            if r["event"] == "pull_request" and r["workflowName"] in WORKFLOWS and not exempt(r["headBranch"]) and not ghost(r)]


GHOSTS = set()


def ghost(r):
    """Release authority: a run still queued with 0 jobs after 10 min (outage ghost) is not counted against the cap."""
    rid = r["databaseId"]
    if rid in GHOSTS:
        return True
    age = datetime.datetime.now(datetime.timezone.utc) - parse_utc(r["createdAt"])
    if r.get("status") != "queued" or age.total_seconds() < 600:
        return False
    if gh_json(["api", f"repos/{REPO}/actions/runs/{rid}/jobs?per_page=1", "--jq", ".total_count"], ok_fail=True) == 0:
        GHOSTS.add(rid)
        log(f"run {rid}: queued {int(age.total_seconds() // 60)} min with 0 jobs; ghost, not counted against the cap")
        return True
    return False


def run_pr(run, branch_map):
    n = branch_map.get(run["headBranch"])
    if n:
        return n
    d = gh_json(["api", f"repos/{REPO}/actions/runs/{run['databaseId']}"], ok_fail=True)
    prs = (d or {}).get("pull_requests") or []
    return prs[0]["number"] if prs else None


def jobs_started(run):
    d = gh_json(["run", "view", str(run["databaseId"]), "--repo", REPO, "--json", "status,jobs"], ok_fail=True)
    if d is None:
        return True
    js = d.get("jobs", [])
    if not js:
        return d.get("status") not in ("queued", "waiting", "pending", "requested")
    return heavy_job_started(CFG, js)


CANCEL_ATT = {}


def mine(r, cancelled):
    rid = r["databaseId"]
    if rid in CFG.read(F_EXT_OK, []):
        return False
    return rid in cancelled and CANCEL_ATT.get(str(rid), r.get("attempt")) == r.get("attempt")


def cancel(run_id):
    a = gh_json(["api", f"repos/{REPO}/actions/runs/{run_id}", "--jq", "{a: .run_attempt, b: .head_branch, e: .event}"], ok_fail=True)
    if not a or protected_branch(CFG, a.get("b"), a.get("e")):
        log(f"run {run_id}: {'unreadable' if not a else 'merge-queue, tag or protected-branch run'}; never cancelled")
        return False
    CANCEL_ATT[str(run_id)] = a["a"]
    if gh(["run", "cancel", str(run_id), "--repo", REPO], ok_fail=True) is None \
            and gh(["api", "-X", "POST", f"repos/{REPO}/actions/runs/{run_id}/force-cancel"], ok_fail=True) is None:
        log(f"run {run_id}: cancel not sent or refused; still counted as live")
        return False
    log(f"cancelled {run_id}")
    return True


def dev_flake(d, bad):
    """The configured known flake whose job is the only failure (besides the required aggregate), or None."""
    for f in CFG["known_flakes"]:
        if not bad or set(bad) - {REQUIRED} != {f["job"]}:
            continue
        j = next((j for j in d.get("jobs", []) if j.get("name") == f["job"] and j.get("conclusion") == "failure"), None)
        out = gh(["api", f"repos/{REPO}/actions/jobs/{j['databaseId']}/logs"], ok_fail=True) if j and j.get("databaseId") else None
        fails = [line for line in (out or "").splitlines() if "FAILED " in line and "::" in line]
        if fails and all(f["test"] in line for line in fails):
            return f"known flake ({f.get('note') or f['test']}); not a PR failure, not re-run"
    return None


def preflight_only(run):
    """No heavy job executing or done; the preflight (any status) never counts (cancellable for a held head)."""
    d = gh_json(["run", "view", str(run["databaseId"]), "--repo", REPO, "--json", "status,jobs"], ok_fail=True)
    if not d or d.get("status") == "completed":
        return False
    js = d.get("jobs", [])
    return all(j.get("name") in PREFLIGHT_JOBS or j.get("status") == "queued" or (
        j.get("status") == "completed" and j.get("conclusion") in ("cancelled", "skipped")) for j in js)


CANCEL_DENIED = set()


def cancel_authorized(ids):
    """Coordinator rule: no run, queued or started, is cancelled without its explicit authorization (cancel_auth.json run ids)."""
    auth = set(CFG.read("cancel_auth.json", []))
    deny = sorted(set(ids) - auth)
    if deny:
        new = [r for r in deny if r not in CANCEL_DENIED]
        CANCEL_DENIED.update(deny)
        if new:
            report(f"run(s) {new} would be cancelled by the throttle; not cancelled (no coordinator authorization), left to run/queue")
        return False
    return True


def hold_cancel(st, k, g):
    """Release authority, for the hold only: a new ordinary PR-push run is cancelled even after its jobs started, and requeued at its exact head."""
    hold = CFG.read("release_hold.json")
    if hold is None or not k.isdigit():
        return False
    n = int(k)
    ext = set(CFG.read(F_EXT_OK, []))
    if n in set(hold.get("exempt", [])) | EXEMPT_PRS | IGNORED_PRS | set(PINNED) or any(r["databaseId"] in ext | set(HELD_RUNS) for r in g):
        return False
    if not cancel_authorized([r["databaseId"] for r in g]):
        return False
    for r in g:
        if not cancel(r["databaseId"]):
            continue
        if r["databaseId"] not in st.setdefault("cancelled", []):
            st["cancelled"].append(r["databaseId"])
    run = next((r for r in g if r["workflowName"] == MAIN_WF), g[0])
    e = next((e for e in st["queue"] if e["pr"] == n), None)
    if e:
        e.update({"run": run["databaseId"], "sha": run["headSha"][:10], "kind": "rerun"})
    else:
        st["queue"].append({"pr": n, "run": run["databaseId"], "sha": run["headSha"][:10], "kind": "rerun"})
    report(f"#{k}: run(s) {[r['databaseId'] for r in g]} started during the release hold; cancelled and requeued at {run['headSha'][:10]} ({hold.get('reason') or 'release hold'})")
    return True


def gated(k):
    return CFG.exists(F_REVIEW_GATE) and str(k).isdigit() and not review_gate(int(k))


def cancel_all(ids, st=None, allow_preflight=False):
    """Cancel ids only if none has started at the last moment; False (nothing cancelled) otherwise."""
    if not cancel_authorized(ids):
        return False
    ext = set(CFG.read(F_EXT_OK, []))
    if ext & set(ids):
        report(f"run(s) {sorted(ext & set(ids))} were started on the maintainer's order; not cancelled")
        return False
    nb = {((pull(n) or {}).get("head") or {}).get("ref") for n in EXEMPT_PRS}
    held = [rid for rid in ids if None in nb or (gh_json(["api", f"repos/{REPO}/actions/runs/{rid}", "--jq", "{b: .head_branch}"], ok_fail=True) or {"b": None}).get("b") in nb]
    if held:
        log(f"run(s) {held} belong to {sorted(EXEMPT_PRS)} (exempt from the hold policy) or could not be read; not cancelled")
        return False
    late = [rid for rid in ids if jobs_started({"databaseId": rid})
            and not (allow_preflight and preflight_only({"databaseId": rid}))]
    if late:
        report(f"run(s) {late} started just before the cancel; not cancelled (never cancelled once started)")
        return False
    for rid in ids:
        rr = gh_json(["api", f"repos/{REPO}/actions/runs/{rid}"], ok_fail=True)
        if rr is None or protected_branch(CFG, rr.get("head_branch"), rr.get("event")) \
                or any(released(x["number"], rr["head_sha"]) for x in rr.get("pull_requests") or []):
            log(f"run {rid}: released, protected or unreadable at the last moment; not cancelled")
            return False
    done = True
    for rid in ids:
        if not cancel(rid):
            done = False
        elif st is not None:
            st["cancelled"].append(rid)
    return done


def restart_ok():
    f = f"{DIR}/restart_ok.json"
    return set(load_json(f)) if os.path.exists(f) else set()


def pull(pr):
    p = None
    for _ in range(4):
        p = gh_json(["api", f"repos/{REPO}/pulls/{pr}"], ok_fail=True)
        if p and p.get("mergeable_state") not in (None, "unknown"):
            break
        time.sleep(5)
    return p


def park(st, entry, sha):
    st["queue"][:] = [e for e in st["queue"] if e["pr"] != entry["pr"]]
    entry = dict(entry)
    entry["parked_sha"] = sha
    st.setdefault("parked", {})[str(entry["pr"])] = entry


HASBASE = {}


def strict_base():
    """The commit every released head must contain (strict_base.json {"sha": ...}), or None."""
    return (CFG.read("strict_base.json") or {}).get("sha")


def merge_fresh(pr, bref):
    """Release authority: retrigger only when refs/pull/N/merge sits on the current base head; a stale merge ref replays the old workflow."""
    m = gh_json(["api", f"repos/{REPO}/git/ref/pull/{pr}/merge", "--jq", "{s: .object.sha}"], ok_fail=True)
    b = gh_json(["api", f"repos/{REPO}/commits/{bref}", "--jq", "{s: .sha}"], ok_fail=True)
    if not m or not b:
        return False
    p = gh_json(["api", f"repos/{REPO}/commits/{m['s']}", "--jq", "{s: .parents[0].sha}"], ok_fail=True)
    return bool(p) and p["s"] == b["s"]


def stale_ok(pr, head):
    """Release authority: a pinned merge ref on an older base may be reopened for an exact head checked CLEAN with no overlap against the newer base."""
    f = f"{DIR}/stale_ok.json"
    return os.path.exists(f) and any(head.startswith(v) for k, v in load_json(f).items() if int(k) == int(pr))


def pin_ok(pr):
    """The merge ref's workflow file must carry the configured runner pin; a stack base without it replays the old runner image."""
    pin = CFG["runner_pin"]
    if not pin:
        return True
    m = gh_json(["api", f"repos/{REPO}/git/ref/pull/{pr}/merge", "--jq", "{s: .object.sha}"], ok_fail=True)
    c = m and gh_json(["api", f"repos/{REPO}/contents/{pin['file']}?ref={m['s']}", "--jq", "{c: .content}"], ok_fail=True)
    if not c:
        return False
    t = base64.b64decode(c["c"]).decode()
    return pin["require"] in t and not (pin.get("forbid") and pin["forbid"] in t)


def has_base(sha):
    base = strict_base()
    if not base:
        return True
    if (base, sha) not in HASBASE:
        d = gh_json(["api", f"repos/{REPO}/compare/{base}...{sha}", "--jq", "{s: .status}"], ok_fail=True)
        if not d:
            return None
        HASBASE[(base, sha)] = d["s"] in ("ahead", "identical")
    return HASBASE[(base, sha)]


def tiers():
    """tiers.json {"tiers": [[pr, ...], ...]}: the binding priority order, or None when absent."""
    t = CFG.read("tiers.json")
    return None if t is None else t["tiers"]


def tier_rank(pr):
    """Listed PRs in tier and list order; every other PR follows by age (PR number); a stack ranks as its best member."""
    return priority(CFG, int(pr))


def tier_members(*levels):
    """The PRs of the given tiers (0 is the first), or None without tiers.json."""
    ts_ = tiers()
    if ts_ is None:
        return None
    return {n for i in levels if i < len(ts_) for n in ts_[i]}


def qkey(e):
    return tier_rank(e["pr"])


def resort(st):
    st["queue"].sort(key=qkey)


def outranks(st, entry):
    eligible = [e for e in st["queue"] if str(e["pr"]) not in HOLDS and e["pr"] != entry["pr"] and runnable(e)]
    return not eligible or qkey(entry) < min(qkey(e) for e in eligible)


def enqueue(st, entry):
    q = st["queue"]
    parked = st.setdefault("parked", {}).pop(str(entry["pr"]), None)
    where = "unparked" if parked else "by tier"
    if parked and parked.get("kind") == "ready":
        entry["kind"] = "ready"
    for i, e in enumerate(q):
        if e["pr"] == entry["pr"]:
            if e.get("kind") == "ready":
                entry["kind"] = "ready"
            q[i] = entry
            where = "replaced"
            break
    else:
        q.append(entry)
    resort(st)
    return where


MERGE_RE = re.compile(r"HEAD is now at \w+ Merge ([0-9a-f]{40}) into ([0-9a-f]{40})")


def base_note(rid, started, ref):
    """(base, base_branch_at_start) from a job's checkout log; None if unreadable."""
    jobs = (gh_json(["api", f"repos/{REPO}/actions/runs/{rid}/jobs?per_page=100"], ok_fail=True) or {}).get("jobs", [])
    base = None
    for j in [j for j in jobs if j.get("conclusion") not in ("skipped", None)][:4]:
        m = MERGE_RE.search(gh(["api", f"repos/{REPO}/actions/jobs/{j['id']}/logs"], ok_fail=True) or "")
        if m:
            base = m.group(2)
            break
    if not base:
        return None
    cmp = gh_json(["api", f"repos/{REPO}/compare/{base}...{ref}?per_page=250"], ok_fail=True) or {}
    by_parent = {c["parents"][0]["sha"]: c for c in cmp.get("commits", []) if c.get("parents")}
    dev, cur = base, base
    while cur in by_parent:
        c = by_parent[cur]
        if c["commit"]["committer"]["date"] > started:
            break
        dev = cur = c["sha"]
    return base, dev


FAILED = ("failure", "timed_out", "startup_failure")
UNRELEASED_NOTE = "[UNRELEASED run: re-run without a release; its result does not qualify the head; PR stays held]"


def failed_jobs(d):
    return [j["name"] for j in d.get("jobs", []) if j.get("conclusion") in FAILED]


def cancelled_by(rid, d):
    """Who cancelled the attempt d of run rid: this watcher, when it recorded cancelling that attempt."""
    return "by the watcher" if CANCEL_ATT.get(str(rid)) == d.get("attempt") else "not by me"


def queue_place(st, pr, where):
    """How an entry got into the queue and its place there, as the reports word it."""
    return f"{where}, position {[e['pr'] for e in st['queue']].index(pr) + 1}/{len(st['queue'])}"


def run_head(rid, d):
    return d.get("headSha") or (gh_json(["api", f"repos/{REPO}/actions/runs/{rid}"], ok_fail=True) or {}).get("head_sha")


def base_wording(pr, rid, d):
    """(green, base) of a finished run: how its report words a green result and names the base it ran on."""
    ref = (((pull(pr) or {}).get("base") or {}).get("ref") or BASE_BRANCH) if isinstance(pr, int) else BASE_BRANCH
    bn = base_note(rid, d.get("startedAt") or "", ref) if d.get("conclusion") != "cancelled" else None
    green = "GREEN"
    if not bn:
        base = f"base unread ({ref})"
    elif ref == BASE_BRANCH:
        base = f"base {bn[0][:10]}"
    else:
        base = f"base={ref}@{bn[0][:10]}"
    if bn and bn[0] != bn[1] and pr not in EXEMPT_PRS:
        green = f"GREEN (base {bn[0][:10]}; {ref} now {bn[1][:10]}, freshness supplied by the merge queue)"
    need = CFG.read("need_base.json") or None
    hs = run_head(rid, d)
    if need and hs and (gh_json(["api", f"repos/{REPO}/compare/{need}...{hs}", "--jq", "{s: .status}"], ok_fail=True) or {}).get("s") not in (None, "ahead", "identical"):
        green = f"GREEN on {bn[0][:10] if bn else 'unread base'}, head lacks {need[:10]}; maintainer decides"
        base = f"{base}, head lacks {need[:10]}"
    return green, base


def requeue_current_head(st, pr, rid, d, rep):
    """A run that finished on a stale head: queue the current head's run when this watcher had cancelled it."""
    p0 = pull(pr)
    rh = run_head(rid, d)
    if not (p0 and p0["state"] == "open" and rh and p0["head"]["sha"] != rh):
        return
    rr = gh_json(["api", f"repos/{REPO}/actions/runs?head_sha={p0['head']['sha']}&event=pull_request&per_page=50"], ok_fail=True)
    cur = sorted([r for r in (rr or {}).get("workflow_runs", []) if r["name"] == MAIN_WF], key=lambda r: r["created_at"], reverse=True)
    if cur and cur[0]["status"] == "completed" and cur[0]["conclusion"] == "cancelled" and cur[0]["id"] in st["cancelled"]:
        where = enqueue(st, {"pr": pr, "run": cur[0]["id"], "sha": p0["head"]["sha"], "kind": "rerun"})
        rep(f"#{pr}: finished run {rid} was on stale head {rh[:8]}; current head {p0['head']['sha'][:8]}'s pre-cancelled run {cur[0]['id']} "
            f"queued ({queue_place(st, pr, where)})")


def excused(rid, d, bad):
    """The verdict of a red run whose every failure fail_ok.json authorizes for it, or "" when it is not excused."""
    fo = (CFG.read("fail_ok.json") or {}).get(str(rid))
    if not fo or d.get("conclusion") != "failure" or not bad:
        return ""
    jobs = d.get("jobs", [])
    aggbad = [j["name"] for j in jobs if j["name"] in fo.get("agg_only", []) and j.get("conclusion") == "failure"
              and [x["name"] for x in j.get("steps", []) if x.get("conclusion") == "failure"] != ["Verify required checks"]]
    unlisted = [j for j in jobs if j["name"] not in fo["jobs"] and j.get("conclusion") not in ("success", "skipped")]
    if aggbad or not set(bad) <= set(fo["jobs"]) or unlisted:
        return ""
    if fo.get("label"):
        return f"{fo['label']} ({', '.join(bad)}; {fo['why']}); every other job success/skipped"
    return f"RED only on {', '.join(bad)}; validated under the authorized exception ({fo['why']})"


class Finished:
    """A completed run that held a slot, with the wording its reports share."""

    def __init__(self, st, pr, h, d, rep):
        self.st, self.pr, self.h, self.d, self.rep = st, pr, h, d, rep
        self.rid, self.concl, self.bad = h["run"], d["conclusion"], failed_jobs(d)
        self.green, self.base = base_wording(pr, self.rid, d)
        if isinstance(pr, int):
            requeue_current_head(st, pr, self.rid, d, rep)
        self.excused = excused(self.rid, d, self.bad)

    def parked_dirty(self):
        """A run that was not cancelled on a PR that is now dirty: parked with its verdict. True when that was done."""
        if self.concl == "cancelled" or not isinstance(self.pr, int):
            return False
        p = pull(self.pr)
        if not (p and p["state"] == "open" and p.get("mergeable_state") == "dirty"):
            return False
        park(self.st, {"pr": self.pr, "run": self.rid, "kind": "rerun"}, p["head"]["sha"])
        if self.concl == "success":
            verdict = self.green
        elif self.excused:
            verdict = self.excused
        else:
            flake = dev_flake(self.d, self.bad)
            verdict = f"RED ({self.concl}); failed jobs: {', '.join(self.bad) or 'n/a'}" + (f" ({flake})" if flake else "")
        self.rep(f"#{self.pr}: run {self.rid} ({self.base}) finished {verdict}; PR is dirty at head {p['head']['sha'][:8]}, parked until a new head lands")
        return True

    def green_run(self):
        st, pr, rid = self.st, self.pr, self.rid
        p = pull(pr) if isinstance(pr, int) else None
        if p and p["state"] == "open" and p.get("draft"):
            where = enqueue(st, {"pr": pr, "run": None, "sha": p["head"]["sha"], "kind": "ready"})
            self.rep(f"#{pr}: run {rid} ({self.base}) finished {self.green} (quick tier only: PR is a draft, not validated); "
                     f"re-queued for un-draft ({queue_place(st, pr, where)})")
            return
        self.rep(f"#{pr}: run {rid} ({self.base}) finished {self.green}")
        if isinstance(pr, int) and p and (released(pr, p["head"]["sha"]) or int(rid) in CFG.read(F_EXT_OK, [])):
            st.setdefault("stale_bt", {})[str(pr)] = {"head": p["head"]["sha"], "green": int(rid)}

    def cancelled_run(self):
        st, pr, rid, d, h = self.st, self.pr, self.rid, self.d, self.h
        tmo = timed_out_jobs(d)
        what = f"TIMEOUT ({'; '.join(tmo)})" if tmo else "CANCELLED"
        p = pull(pr) if isinstance(pr, int) else None
        by = cancelled_by(rid, d)
        if p and p["head"]["sha"] != d.get("headSha"):
            self.rep(f"#{pr}: run {rid} {what if tmo else what.lower()} on stale head {str(d.get('headSha'))[:8]} (PR head {p['head']['sha'][:8]}); not re-queued")
        elif p and p.get("mergeable_state") == "dirty":
            park(st, {"pr": pr, "run": rid, "kind": "rerun", "retries": h.get("retries", 0)}, p["head"]["sha"])
            cj = [j["name"] for j in d.get("jobs", []) if j.get("conclusion") == "cancelled"]
            self.rep(f"#{pr}: run {rid} attempt {d.get('attempt')} was {what} ({by}; cancelled jobs: {', '.join(cj[:5]) or 'n/a'}); PR is dirty, parked until its head changes")
        elif pr in PINNED and p and p["state"] == "open" and not tmo:
            gh(["run", "rerun", str(rid), "--repo", REPO], ok_fail=True)
            h.update(since=now(), seen_active=False)
            st["holders"][str(pr)] = h
            self.rep(f"#{pr}: pinned run {rid} attempt {d.get('attempt')} was CANCELLED ({by}); re-ran it immediately on current head {p['head']['sha'][:8]}")
        else:
            self.rep(f"#{pr}: run {rid} attempt {d.get('attempt')} was {what} ({'job timeout-minutes' if tmo else by}) after holding a slot; not re-run (maintainer decides), dropped")

    def red_run(self):
        pr, rid, bad = self.pr, self.rid, self.bad
        if self.excused:
            self.rep(f"#{pr}: run {rid} ({self.base}) finished {self.excused}")
            return
        flake = dev_flake(self.d, bad)
        if flake:
            self.rep(f"#{pr}: run {rid} ({self.base}) finished RED ({self.concl}); failed jobs: {', '.join(bad)}; {flake}")
            return
        infra = " (release-download outage in effect: red: infra outage, awaiting maintainer re-run; not a PR failure, not re-run)" if CFG.exists("outage.json") else ""
        self.rep(f"#{pr}: run {rid} ({self.base}) finished RED ({self.concl}); failed jobs: {', '.join(bad) or 'n/a'}; dropped{infra}")


def unreleased_report(msg):
    report(f"{msg} {UNRELEASED_NOTE}")


def finish_holder(st, pr, h):
    rid = h.get("run")
    rep = unreleased_report if rid in set(CFG.read("unreleased.json", [])) else report
    if not rid:
        rep(f"#{pr}: marked ready but no PR run appeared within 15 min; slot released")
        return
    d = gh_json(["run", "view", str(rid), "--repo", REPO, "--json", "conclusion,status,jobs,headSha,attempt,startedAt"], ok_fail=True)
    if not d:
        rep(f"#{pr}: run {rid} finished (could not read result)")
        return
    if d.get("status") != "completed" or not d.get("conclusion"):
        st["holders"][str(pr)] = h
        log(f"#{pr}: run {rid} still {d.get('status')}; keeps its slot")
        return
    f = Finished(st, pr, h, d, rep)
    if f.parked_dirty():
        return
    if f.concl == "success":
        f.green_run()
    elif f.concl == "cancelled":
        f.cancelled_run()
    else:
        f.red_run()


def timed_out_jobs(d):
    """Cancelled jobs that ran into their timeout-minutes (at least job_timeout_minutes of wall time)."""
    out = []
    for j in d.get("jobs", []):
        if j.get("conclusion") == "cancelled" and j.get("startedAt") and (j.get("completedAt") or "").startswith("2"):
            dt = parse_utc(j["completedAt"]) - parse_utc(j["startedAt"])
            if dt.total_seconds() >= CFG["job_timeout_minutes"] * 60:
                out.append(f"{j['name']}, {int(dt.total_seconds() // 60)} min")
    return out


# What _start_next does with a queue entry it tried: keep it queued or drop it, and go on to the next or stop.
KEEP, DROP, STOP, KEEP_AND_STOP = "keep", "drop", "stop", "keep and stop"


def _start_next(st):
    q = st["queue"]
    resort(st)
    i = 0
    while i < len(q):
        e = q[i]
        pr = e["pr"]
        p = pull(pr)
        if p and p["state"] == "open" and any(p["head"]["sha"].startswith(x) for x in HOLDS.get(str(pr), [])):
            log(f"#{pr}: owner announced a push; skipped until a new head lands")
            i += 1
            continue
        q.pop(i)
        verdict = unstartable(st, e, p)
        if verdict is None:
            verdict = start_draft(st, e) if p["draft"] else start_rerun(st, e, p)
        if verdict in (KEEP, KEEP_AND_STOP):
            q.insert(i, e)
            i += 1
        if verdict in (STOP, KEEP_AND_STOP):
            return


def unstartable(st, e, p):
    """KEEP or DROP for a queue entry whose PR p cannot take a slot this pass, the reason logged or reported; else None."""
    pr = e["pr"]
    if not p:
        report(f"#{pr}: could not fetch PR; dropped from queue")
        return DROP
    if p["state"] != "open":
        report(f"#{pr}: {'merged' if p.get('merged') else 'closed'}; skipped")
        return DROP
    if p.get("mergeable_state") == "dirty":
        park(st, e, p["head"]["sha"])
        report(f"#{pr}: merge conflicts (dirty) at head {p['head']['sha'][:8]}; parked until its head changes")
        return DROP
    hb = has_base(p["head"]["sha"])
    if hb is None:
        log(f"#{pr}: could not compare head with {(strict_base() or '')[:10]}; retrying next pass")
        return KEEP
    if not hb:
        park(st, dict(e, reason=f"needs {BASE_BRANCH} merge"), p["head"]["sha"])
        report(f"#{pr}: head {p['head']['sha'][:10]} lacks {BASE_BRANCH} {(strict_base() or '')[:10]}; parked (needs {BASE_BRANCH} merge)")
        return DROP
    if p["draft"] and p.get("mergeable_state") in (None, "unknown"):
        log(f"#{pr}: mergeable_state still unknown; not marking ready this pass")
        return KEEP
    return None


def mark_ready(pr):
    return gh(["pr", "ready", str(pr), "--repo", REPO], ok_fail=True) is not None


def start_draft(st, e):
    """Give a queued draft the slot by marking it ready for review, when ready_ok.json approves that."""
    pr = e["pr"]
    if pr not in READY_OK:
        if e.get("kind") == "ready":
            report(f"#{pr}: draft (owner's WIP); taken out of the queue, its runs are never cancelled or re-run")
        else:
            report(f"#{pr}: draft (not approved to un-draft); rerun skipped, left for the maintainer")
        return DROP
    ok = admit(st, pr, {"run": None, "seen_active": False, "kind": "ready"}, functools.partial(mark_ready, pr))
    if ok is None:
        return KEEP_AND_STOP
    if not ok:
        report(f"#{pr}: gh pr ready failed; dropped")
        return DROP
    report(f"#{pr}: slot granted, marked ready for review (gh pr ready); full tier starts from the ready event")
    return STOP


def start_rerun(st, e, p):
    """Give a queued PR the slot: adopt the run already active on its head, or close and reopen it for a fresh one."""
    pr, head = e["pr"], p["head"]["sha"]
    d = gh_json(["api", f"repos/{REPO}/actions/runs?branch={p['head']['ref']}&head_sha={head}&event=pull_request&per_page=100"], ok_fail=True)
    runs = sorted((d or {}).get("workflow_runs", []), key=lambda r: r["created_at"], reverse=True)
    prs_runs = [r for r in runs if r["name"] == MAIN_WF]
    if not prs_runs:
        report(f"#{pr}: no PR run on current head {head[:8]} (mergeable_state={p.get('mergeable_state')}); dropped")
        return DROP
    latest = prs_runs[0]
    rid = latest["id"]
    if latest["status"] != "completed":
        if mine({"databaseId": rid, "attempt": latest["run_attempt"]}, set(st["cancelled"])):
            log(f"#{pr}: my cancel of run {rid} still settling; retry next pass")
            return STOP
        if admit(st, pr, {"run": rid, "seen_active": True, "kind": "adopted"}) is None:
            return KEEP_AND_STOP
        report(f"#{pr}: run {rid} already active on current head; holding slot")
        return STOP
    if pr in NOCANCEL:
        report(f"#{pr}: latest PR run {rid} on head {head[:8]} is {latest['conclusion']} (not cancelled by me); not re-run, dropped")
        return DROP
    if latest["conclusion"] != "cancelled" and rid not in restart_ok():
        report(f"#{pr}: latest PR run {rid} on head {head[:8]} is {latest['conclusion']}, not cancelled; dropped")
        return DROP
    bref = (p.get("base") or {}).get("ref") or BASE_BRANCH
    ro = CFG.read("reopen_ok.json") or []
    if pr not in ro:
        log(f"#{pr}: ordinary re-runs are disabled (a re-run replays run {rid}'s old merge commit); waiting for an authorized close/reopen")
        return KEEP
    if not (merge_fresh(pr, bref) or stale_ok(pr, head)) or not pin_ok(pr):
        log(f"#{pr}: refs/pull/{pr}/merge not yet on current {bref} or without the runner pin; close/reopen deferred")
        return KEEP_AND_STOP
    return STOP if admit(st, pr, start=functools.partial(reopen, pr, ro, bref, head)) else KEEP_AND_STOP


def reopen(pr, ro, bref, head):
    """The authorized close and reopen that gives a PR a fresh pull_request event; False when the close was not sent."""
    if CFG.frozen() or CFG.dry_run or gh(["pr", "close", str(pr), "--repo", REPO], ok_fail=True) is None:
        log(f"#{pr}: close for the authorized close/reopen not sent; kept queued with its authorization")
        return False
    CFG.write("reopen_ok.json", [x for x in ro if x != pr])
    ok = False
    for _ in range(3):
        if gh(["pr", "reopen", str(pr), "--repo", REPO], ok_fail=True) is not None:
            ok = True
            break
        time.sleep(5)
    report(f"#{pr}: slot free; merge ref fresh on {bref} with the runner pin, so closed and reopened (authorized) for a fresh pull_request event on head {head[:8]}"
           + ("; new run id follows, check its first job's Image" if ok else "; ATTENTION: reopen FAILED three times, the PR is left CLOSED; reopen it by hand"))
    return True


GATE = {}


CREATED = {}


def run_created(rid):
    if rid not in CREATED:
        d = gh_json(["api", f"repos/{REPO}/actions/runs/{rid}", "--jq", "{c: .created_at}"], ok_fail=True)
        CREATED[rid] = (d or {}).get("c") or ""
    return CREATED[rid]


def deferred(k):
    f = f"{DIR}/defer.json"
    if not os.path.exists(f):
        return False
    return any((pull(b) or {}).get("state") == "open" for b in load_json(f).get(str(k), []))


REVIEW_NOTED = {}
HEADC = {}


def cur_head(pr):
    c = HEADC.get(pr)
    if c and time.time() - c[0] < 60:
        return c[1]
    p = gh_json(["api", f"repos/{REPO}/pulls/{pr}"], ok_fail=True)
    HEADC[pr] = (time.time(), p)
    return p


def released(pr, sha):
    f = f"{DIR}/release.json"
    # pr "*" pre-releases an exact head before its PR exists, so the opening run is never held.
    return os.path.exists(f) and any((x["pr"] == "*" and len(x["sha"]) == 40 or x["pr"] != "*" and int(x["pr"]) == int(pr)) and sha.startswith(x["sha"])
                                     for x in load_json(f))


def review_clear(pr):
    q = "query($o:String!,$n:String!,$p:Int!){repository(owner:$o,name:$n){pullRequest(number:$p){reviewThreads(first:100){totalCount nodes{isResolved}}}}}"
    d = gh_json(["api", "graphql", "-f", f"query={q}", "-f", "o=" + REPO.split("/")[0], "-f", "n=" + REPO.split("/")[1], "-F", f"p={pr}"], ok_fail=True)
    if not d:
        return False
    threads = d["data"]["repository"]["pullRequest"]["reviewThreads"]
    if threads.get("totalCount", 0) > len(threads["nodes"]):
        log(f"#{pr}: {threads['totalCount']} review threads, more than one page; not counted as clear")
        return False
    return not any(not t["isResolved"] for t in threads["nodes"])


def review_gate(pr):
    """Admission gate: start only coordinator-released (PR, head) pairs, head unchanged, zero unresolved review threads."""
    if not CFG.exists(F_REVIEW_GATE) or int(pr) in EXEMPT_PRS:
        return True
    p = cur_head(int(pr))
    if not p:
        HEADC.pop(int(pr), None)
        log(f"#{pr}: could not read the PR (API error); held under the review gate this pass")
        return False
    if p.get("state") != "open":
        return True
    sha = p["head"]["sha"]
    if not released(pr, sha):
        if REVIEW_NOTED.get(str(pr)) != sha + ":rel":
            REVIEW_NOTED[str(pr)] = sha + ":rel"
            log(f"#{pr}: head {sha[:10]} not on the release list; held under the review gate")
        return False
    if not review_clear(pr):
        if REVIEW_NOTED.get(str(pr)) != sha + ":rev":
            REVIEW_NOTED[str(pr)] = sha + ":rev"
            report(f"#{pr}: Review not clear on {sha[:10]}; not started")
        return False
    return True


def runnable(e):
    if deferred(e["pr"]):
        return False
    if GATE and e["pr"] not in GATE["allow"]:
        return False
    if stack_hold(e["pr"]):
        return False
    cut = CFG.read("cutoff.json")["before"] if CFG.exists("cutoff.json") else None
    if cut and e.get("run") and e.get("kind") == "rerun" and run_created(e["run"]) < cut:
        return False
    return review_gate(e["pr"])


def probe_code(url):
    """The status of a ranged GET of an https URL; anything else in outage.json is never handed to curl."""
    try:
        if not isinstance(url, str) or urllib.parse.urlsplit(url).scheme != "https":
            return "err"
    except ValueError:
        return "err"
    return subprocess.run(["curl", "-fsSL", "--proto", "=https", "--proto-redir", "=https", "--range", "0-1023",
                           "-o", "/dev/null", "-m", "60", "-w", "%{http_code}", "--url", url],
                          capture_output=True, text=True).stdout.strip() or "err"


def outage_hold():
    f = f"{DIR}/outage.json"
    if not os.path.exists(f):
        return False
    o = load_json(f)
    codes = [probe_code(u) for u in o["probes"]]
    code = "200" if all(c in ("200", "206") for c in codes) else "/".join(codes)
    t = time.time()
    if code == "200":
        if o.get("ok_at") and t - o["ok_at"] >= 60:
            os.remove(f)
            report("release downloads back: every outage probe (ranged GET) returned 200/206 twice, "
                   ">=1 min apart; outage gate lifted, resuming slot grants")
            return False
        o.setdefault("ok_at", t)
    else:
        o.pop("ok_at", None)
    dump_json(f, o)
    log(f"outage gate: probe {code}; no slot granted")
    return True


def auth_pending():
    f = f"{DIR}/auth_rerun.json"
    return load_json(f) if os.path.exists(f) else []


def drop_authorized(e):
    with CFG.locked(F_AUTH_RERUN):
        CFG.write(F_AUTH_RERUN, [x for x in CFG.read(F_AUTH_RERUN, []) if x != e])


def rerun_authorized(e):
    """Send an authorized re-run, which is spent whether or not GitHub accepts it."""
    mode = [] if e.get("full") else ["--failed"]
    drop_authorized(e)
    if gh(["run", "rerun", str(e["run"]), *mode, "--repo", REPO], ok_fail=True) is None:
        report(f"#{e['pr']}: authorized `gh run rerun {' '.join([str(e['run']), *mode])}` FAILED (see watch.log)")
        return False
    return True


def start_authorized(st):
    for e in sorted(auth_pending(), key=lambda e: tier_rank(e["pr"])):
        if stack_hold(e["pr"]):
            continue
        if CFG.frozen() or CFG.dry_run:
            log(f"{'FROZEN (freeze.json)' if CFG.frozen() else 'DRY RUN'}: authorized re-run of {e['run']} deferred")
            return None
        p = pull(e["pr"])
        if not p:
            log(f"#{e['pr']}: PR unreadable; authorized re-run of {e['run']} kept for the next pass")
            return None
        if p["state"] != "open" or not p["head"]["sha"].startswith(e["head"]):
            drop_authorized(e)
            report(f"#{e['pr']}: authorized re-run of {e['run']} skipped: PR head is now "
                   f"{p['head']['sha'][:10]} (state {p['state']}), not {e['head']}")
            continue
        ok = admit(st, e["pr"], {"run": e["run"], "seen_active": False, "kind": "rerun", "retries": 0}, functools.partial(rerun_authorized, e))
        if ok is None:
            return None
        if not ok:
            continue
        report(f"#{e['pr']}: started authorized re-run ({'all jobs' if e.get('full') else 'failed jobs only'}) of run {e['run']} on head {e['head']}")
        return True
    return None


def merge_queue_entries(fields):
    q = (f"query{{repository(owner:{json.dumps(CFG.owner)},name:{json.dumps(CFG.name)})"
         f"{{mergeQueue(branch:{json.dumps(BASE_BRANCH)}){{entries(first:20){{nodes{{{fields}}}}}}}}}}}")
    d = gh_json(["api", "graphql", "-f", f"query={q}"], ok_fail=True)
    if d is None:
        return None
    return (((d.get("data") or {}).get("repository") or {}).get("mergeQueue") or {}).get("entries", {}).get("nodes") or []


def mq_waiting():
    """Release authority: the merge queue never waits for runners; no ordinary slot is granted while a merge_group run has jobs waiting."""
    seen = False
    for st in ("queued", "in_progress", "waiting", "requested", "pending"):
        out = gh_json(["run", "list", "--repo", REPO, "--status", st, "--event", "merge_group", "-L", "50", "--json", "databaseId,status"], ok_fail=True)
        if out is None:
            return "merge_group runs unreadable"
        seen = seen or bool(out)
        for r in out:
            if r["status"] == "queued":
                return f"merge_group run {r['databaseId']} queued"
            n = gh_json(["api", f"repos/{REPO}/actions/runs/{r['databaseId']}/jobs?per_page=100", "--jq",
                         '[.jobs[] | select(.status == "queued" or .status == "waiting")] | length'], ok_fail=True)
            if n is None:
                return f"merge_group run {r['databaseId']} jobs unreadable"
            if n:
                return f"merge_group run {r['databaseId']} has {n} job(s) waiting for runners"
    if not seen:
        ns = merge_queue_entries("pullRequest{number}")
        if ns is None:
            return "merge queue unreadable"
        if ns:
            return f"merge queue holds {len(ns)} entr{'y' if len(ns) == 1 else 'ies'} (#{ns[0]['pullRequest']['number']} next) between builds"
    return None


MQ_HELD = {}
STACK_HELD = {}


def stack_hold(pr):
    """Why a stacked PR takes no slot yet, or None: its nearest open predecessor in stacks.json (merged and closed ones
    are skipped) has no pull_request run on its current head. An unreadable predecessor or run list holds it too.
    Decided once per pass."""
    pr = int(pr)
    if pr in STACK_HELD:
        return STACK_HELD[pr]
    why, s = None, stack_of(CFG, pr) or [pr]
    for n in reversed(s[:s.index(pr)]):
        p = pull(n)
        if not p:
            why = f"its predecessor #{n} is unreadable"
            break
        if p["state"] != "open":
            continue
        head = p["head"]["sha"]
        d = gh_json(["api", f"repos/{REPO}/actions/runs?head_sha={head}&event=pull_request&per_page=100"], ok_fail=True)
        if d is None:
            why = f"the runs of its predecessor #{n} are unreadable"
        elif not any(r.get("name") in CFG["workflows"] for r in d.get("workflow_runs", [])):
            why = f"its predecessor #{n} has no pull_request run on its head {head[:10]} yet"
        break
    STACK_HELD[pr] = why
    if why:
        log(f"stack: #{pr} takes no slot: {why}")
    return why


def admit(st, pr, holder=None, start=None):
    """The one admission point for an ordinary PR run, by every path (a new run, the queue, a CI-PR auto-admit,
    an authorized re-run): nothing is admitted while the merge queue waits for runners (mq_waiting, failing closed).
    start() sends the mutation that starts the run; holder is the slot record. None when the merge queue holds
    the admission, False when start() fails, True once admitted."""
    mw = MQ_HELD.get("why") or mq_waiting()
    if mw:
        MQ_HELD["why"] = mw
        log(f"merge-queue first: {mw}; no ordinary slot granted (#{pr} not admitted)")
        return None
    if stack_hold(pr):
        return None
    if start is not None and not start():
        return False
    if holder is not None:
        st["holders"][str(pr)] = dict(holder, since=now())
    return True


def slot(g, kind):
    """The slot record of a run group: its main-workflow run holds the slot."""
    return {"run": next((r["databaseId"] for r in g if r["workflowName"] == MAIN_WF), g[0]["databaseId"]), "seen_active": True, "kind": kind}


def start_next(st, *a, **kw):
    if outage_hold():
        return None
    if CFG.exists("release_hold.json"):
        log(f"release hold (release_hold.json): {CFG.read('release_hold.json').get('reason', '')}; no ordinary slot granted")
        return None
    sp = solo_pr()
    if sp is not None and not kw.pop("_solo_done", False):
        rest = [e for e in st["queue"] if str(e["pr"]) != sp]
        if len(rest) == len(st["queue"]):
            log(f"sole-priority hold for #{sp}: no other slot granted")
            return None
        st["queue"][:] = [e for e in st["queue"] if str(e["pr"]) == sp]
        try:
            return start_next(st, _solo_done=True)
        finally:
            st["queue"][:] = st["queue"] + rest
    kw.pop("_solo_done", None)
    ap = [e for e in auth_pending() if not stack_hold(e["pr"])]
    if ap:
        runq = [e for e in st["queue"] if runnable(e) and str(e["pr"]) not in HOLDS]
        if not runq or min(tier_rank(e["pr"]) for e in ap) <= min(tier_rank(e["pr"]) for e in runq):
            return start_authorized(st)
    keep = [e for e in st["queue"] if not runnable(e)]
    st["queue"][:] = [e for e in st["queue"] if runnable(e)]
    try:
        return _start_next(st, *a, **kw) if st["queue"] else None
    finally:
        st["queue"][:] = st["queue"] + keep


def solo_pr():
    f = f"{DIR}/solo.json"
    return str(load_json(f)["pr"]) if os.path.exists(f) else None


def arm_pushed():
    fa = f"{DIR}/solo_arm.json"
    if not os.path.exists(fa):
        return None
    a = load_json(fa)
    p = gh_json(["api", f"repos/{REPO}/pulls/{a['pr']}"], ok_fail=True)
    if not p or p["state"] != "open" or p["head"]["sha"] == a["head"]:
        return None
    return a, p


HELD_RUNS = set()


def priority_hold():
    """No new PR run starts while any named runner-priority run is unfinished (fails closed on an API error); an awaited next-head hold binds only once that head's run appears."""
    held = False
    for f in sorted(glob.glob(f"{DIR}/priority_hold*.json")):
        h = load_json(f)
        lbl = h.get("label", "Nightly run")
        if h.get("run") is None:
            p = pull(int(h["pr"])) or {}
            hd = (p.get("head") or {})
            if not hd.get("sha") or hd["sha"].startswith(h["after"]):
                continue
            rs = gh_json(["run", "list", "--repo", REPO, "--branch", hd["ref"], "--workflow", MAIN_WF, "--limit", "10",
                          "--json", "databaseId,headSha"], ok_fail=True)
            new = [r for r in (rs or []) if r["headSha"] == hd["sha"]]
            if not new:
                continue
            h["run"], h["head"] = new[0]["databaseId"], hd["sha"]
            h["label"] = lbl = f"#{h['pr']} {hd['sha'][:10]} run"
            dump_json(f, h)
            report(f"#{h['pr']}: PUSHED new head {hd['sha'][:10]}; run {h['run']}; runner-priority hold on it (cap 0 for other new starts until it finishes)")
        d = gh_json(["run", "view", str(h["run"]), "--repo", REPO, "--json", "status,conclusion"], ok_fail=True)
        if d is None or d.get("status") != "completed":
            held = True
            HELD_RUNS.add(h["run"])
            continue
        if h.get("pr") and d.get("conclusion") == "cancelled":
            cur = ((pull(int(h["pr"])) or {}).get("head") or {}).get("sha") or ""
            if cur and cur != h.get("head"):
                h.update({"run": None, "after": h["head"][:10], "label": f"#{h['pr']} next-head run"})
                dump_json(f, h)
                report(f"#{h['pr']}: run of {h['after']} cancelled by a newer push; runner-priority hold moves to the next head's run")
                held = True
                continue
        os.remove(f)
        report(f"{lbl} {h['run']} completed: {d.get('conclusion')}; its runner-priority hold lifted")
    return held


def wide_cap(st):
    """Release authority: a raised cap until each listed PR's next admitted run has finished."""
    f = f"{DIR}/wide_cap.json"
    if not os.path.exists(f):
        return BASE_CAP
    w = load_json(f)
    for pr in w["prs"]:
        h = st["holders"].get(str(pr))
        if h and h.get("run"):
            w["seen"][str(pr)] = h["run"]
        elif str(pr) in w["seen"] and pr not in w["done"]:
            w["done"].append(pr)
    dump_json(f, w)
    if set(w["done"]) >= set(w["prs"]):
        os.remove(f)
        report(f"runs of {', '.join('#' + str(p) for p in w['prs'])} finished; cap back to {BASE_CAP}")
        return BASE_CAP
    return w["cap"]


def solo_cap(st):
    if CFG.exists("freeze.json"):
        return 0
    if priority_hold():
        return 0
    f = f"{DIR}/solo.json"
    ap = arm_pushed()
    if ap:
        a, p = ap
        dump_json(f, {"pr": a["pr"], "run": None, "head": p["head"]["sha"]})
        os.remove(f"{DIR}/solo_arm.json")
        report(f"#{a['pr']}: PUSHED new head {p['head']['sha'][:10]} (mergeable_state {p.get('mergeable_state')}); "
               "sole-priority hold on: its run takes the next slot, no other slot granted until it finishes")
    if not os.path.exists(f):
        return wide_cap(st)
    o = load_json(f)
    if str(o["pr"]) in st["holders"]:
        if o.get("run") != st["holders"][str(o["pr"])]["run"]:
            o["run"] = st["holders"][str(o["pr"])]["run"]
            dump_json(f, o)
        return 1
    if o.get("run") is None:
        return 1
    d = gh_json(["run", "view", str(o["run"]), "--repo", REPO, "--json", "status"], ok_fail=True)
    if d is None or d.get("status") != "completed":
        return 1
    os.remove(f)
    report(f"#{o['pr']}: sole-priority hold ended (its run finished); normal cap {BASE_CAP} resumes")
    return BASE_CAP


def attempt(name, step, st):
    """step(st); an exception is logged under name and answered as None, so one failing step does not end the pass."""
    try:
        return step(st)
    except Exception:
        log(f"ERROR {name} " + traceback.format_exc().replace("\n", " | "))
        return None


def main_run(g):
    """The run of a PR's run group that holds its slot: the main workflow's, else the first."""
    return next((r for r in g if r["workflowName"] == MAIN_WF), g[0])


def run_ids(g):
    return [r["databaseId"] for r in g]


def is_dirty(p):
    return bool(p) and p.get("mergeable_state") == "dirty"


def unqueue(st, k):
    st["queue"][:] = [e for e in st["queue"] if str(e["pr"]) != k]


def lacks_base():
    return f"{BASE_BRANCH} {(strict_base() or '')[:10]}"


def iterate(st):
    global CAP, CANCEL_ATT
    MQ_HELD.clear()
    STACK_HELD.clear()
    attempt("gate_sweep", gate_sweep, st)
    CAP = solo_cap(st)
    mq = attempt("mq_track", mq_track, st)
    # Cap 1 only while a merge-queue job waits for runners (or the merge queue is unreadable); an active build
    # with nothing waiting leaves the full cap, the one the sweeper enforces.
    if mq is None or (mq > 0 and mq_waiting()):
        CAP = min(CAP, 1)
    attempt("stale_bt", stale_bt, st)
    CANCEL_ATT = st.setdefault("cancelled_att", {})
    open_prs = gh_json(["pr", "list", "--repo", REPO, "--state", "open", "-L", "200", "--json", "number,title,headRefName,headRefOid,author,body,createdAt,isDraft,baseRefName,mergeable"])
    heads = {str(p["number"]): p["headRefOid"] for p in open_prs}
    note_base_head(st)
    note_mergeable(st, open_prs)
    note_base_changes(st, open_prs)
    unpark_moved(st, heads)
    bmap = {p["headRefName"]: p["number"] for p in open_prs}
    backmerge = {bmap[b] for b in CFG.read("backmerge.json", []) if b in bmap}
    queue_ready_drafts(st, open_prs)
    park_baseless_queue(st, heads)
    reload_lists(heads, bmap)
    runs = active_runs()
    drop_ghost_holders(st)
    LAST_SEEN.clear()
    LAST_SEEN.update(r["databaseId"] for r in runs)
    groups = group_runs(runs, bmap)
    holders = st["holders"]
    blk = CFG.read("blocked.json") or {"prs": []}
    why = blk.get("why") or f"blocked until #{blk.get('until')}'s run is executing"
    cancel_blocked_runs(groups, blk, why)
    release_blocked_holders(st, groups, blk, why)
    apply_gate(st, groups)
    if blk.get("until") and str(blk["until"]) in groups and str(blk["until"]) in holders \
            and any(jobs_started(r) for r in groups[str(blk["until"])]):
        os.remove(f"{DIR}/blocked.json")
        report(f"#{blk['until']}: run {holders[str(blk['until'])]['run']} is executing; block on {blk['prs']} lifted")
    free_gated_preflights(st, groups)
    for key, h in list(holders.items()):
        track_holder(st, groups, key, h)
    for k, h in list(holders.items()):
        if k.isdigit() and int(k) not in backmerge:
            park_dirty_holder(st, groups, k, h)
    for k, g in list(groups.items()):
        if k.isdigit() and k not in holders and int(k) not in PINNED and int(k) not in backmerge and g:
            park_baseless_run(st, groups, k, g)
    res = CFG.read("reserve.json").get("pr") if CFG.exists("reserve.json") else None
    res_open = bool(res) and str(res) not in holders
    place_new_runs(st, groups, res, res_open)
    trim_over_cap(st, groups)
    fill_slots(st, res, res_open)
    outage_hold()
    announce_free_slot(st)
    if not st["queue"] and len(holders) <= CAP and not st.get("done_reported"):
        st["done_reported"] = True
        report(f"DONE: hold queue empty, <={CAP} PR runs active; still watching and enforcing the cap")
    if st["queue"]:
        st["done_reported"] = False
    log(f"holders={ {k: v.get('run') for k, v in holders.items()} } queue={[e['pr'] for e in st['queue']]}")


def note_base_head(st):
    dh = gh_json(["api", f"repos/{REPO}/commits/{BASE_BRANCH}", "--jq",
                 '{s: .sha, m: (.commit.message | split("\\n")[0])}'], ok_fail=True)
    if dh and st.get("develop_head") != dh["s"]:
        if st.get("develop_head"):
            report(f"{BASE_BRANCH} moved {st['develop_head'][:10]} -> {dh['s'][:10]}: {dh['m']}")
        st["develop_head"] = dh["s"]


def note_mergeable(st, open_prs):
    mg = st.setdefault("mergeable", {})
    for p in open_prs:
        if p.get("baseRefName") == "main":
            continue
        n, m = str(p["number"]), p.get("mergeable")
        if m == "CONFLICTING" and mg.get(n) == "MERGEABLE":
            report(f"#{n} turned DIRTY (CONFLICTING) at head {p['headRefOid'][:10]} against {p['baseRefName']}" + (f" {(st.get('develop_head') or '?')[:10]}" if p['baseRefName'] == BASE_BRANCH else ""))
        if m in ("MERGEABLE", "CONFLICTING"):
            mg[n] = m


def note_base_changes(st, open_prs):
    br = st.setdefault("baseref", {})
    by_head = {p["headRefName"]: p["number"] for p in open_prs}
    for p in open_prs:
        n, b = str(p["number"]), p.get("baseRefName")
        if br.get(n) and br[n] != b:
            report(f"#{n}: base branch changed {br[n]} -> {b}" + (f" (#{by_head[b]}'s branch; stacked behind it)" if b in by_head else ""))
        br[n] = b


def unpark_moved(st, heads):
    """Drop a parked PR that is no longer open; requeue one whose new head is clean and contains the strict base."""
    for k, e in list(st.setdefault("parked", {}).items()):
        if k not in heads:
            del st["parked"][k]
            report(f"#{k}: parked PR no longer open; dropped")
        elif heads[k] != e["parked_sha"]:
            p = pull(int(k))
            nb = has_base(heads[k]) is False
            if p and p.get("mergeable_state") != "dirty" and not nb:
                where = enqueue(st, {"pr": int(k), "run": e.get("run"), "kind": e.get("kind", "rerun"), "retries": e.get("retries", 0)})
                report(f"#{k}: head moved to {heads[k][:8]}, no longer dirty; back in queue ({queue_place(st, int(k), where)})")
            else:
                e["parked_sha"] = heads[k]
                report(f"#{k}: head moved to {heads[k][:8]} but {f'still lacks {lacks_base()}' if nb else 'still dirty'}; stays parked")


def queue_ready_drafts(st, open_prs):
    """Queue each draft that ready_ok.json approves for un-drafting, or park it while it conflicts."""
    excl = set(CFG.read("draft_excl.json", []))
    for p in open_prs:
        n = p["number"]
        if not p.get("isDraft") or n not in READY_OK or n in excl or n in IGNORED_PRS:
            continue
        for e in st["queue"]:
            if e["pr"] == n and e.get("kind") != "ready":
                e["kind"] = "ready"
        placed = str(n) in st["holders"] or str(n) in st.get("parked", {})
        if p.get("mergeable") == "CONFLICTING" and not placed:
            e = next((e for e in st["queue"] if e["pr"] == n), {"pr": n, "run": None, "kind": "ready"})
            park(st, e, p["headRefOid"])
            report(f"#{n}: draft is CONFLICTING at {p['headRefOid'][:8]}; parked until a new non-dirty head lands")
        elif not placed and not any(e["pr"] == n for e in st["queue"]):
            where = enqueue(st, {"pr": n, "run": None, "sha": p["headRefOid"], "kind": "ready"})
            report(f"#{n}: draft on {BASE_BRANCH} queued for un-draft ({queue_place(st, n, where)})")


def park_baseless_queue(st, heads):
    nb = []
    for e in list(st["queue"]):
        sha = heads.get(str(e["pr"]))
        if sha and has_base(sha) is False:
            park(st, dict(e, reason=f"needs {BASE_BRANCH} merge"), sha)
            nb.append(f"#{e['pr']} ({sha[:10]})")
    if nb:
        report(f"needs {BASE_BRANCH} merge (head lacks {(strict_base() or '')[:10]}); parked until a new head contains it: " + ", ".join(nb))


def reload_lists(heads, bmap):
    """Re-read the per-PR lists the release authority edits between passes."""
    IGNORED_PRS.clear()
    IGNORED_PRS.update(CFG.read("ignore.json", []))
    EXEMPT_PRS.clear()
    EXEMPT_PRS.update(CFG.read("exempt.json", []))
    named = tier_members(0, 1)
    EXEMPT_PRS.update(n for n in CI_PRS if n not in CI_FLAGGED and (named is None or n in named))
    READY_OK.clear()
    READY_OK.update(CFG.read("ready_ok.json", []))
    HOLDS.clear()
    HOLDS.update({k: v for k, v in CFG.read(F_HOLD, {}).items()
                  if k in heads and any(heads[k].startswith(x) for x in v)})
    if CFG.exists(F_HOLD):
        CFG.write(F_HOLD, HOLDS)
    PINNED.clear()
    PINNED.update(bmap.get(k, -1) if isinstance(k, str) else k for k in CFG.read("pinned.json", []))
    NOCANCEL.clear()
    NOCANCEL.update(CFG.read("nocancel.json") or [])


def drop_ghost_holders(st):
    for k, h in list(st["holders"].items()):
        if h.get("run") in GHOSTS:
            st["holders"].pop(k)
            report(f"#{k}: run {h['run']} is a jobless ghost (0 jobs); slot no longer counted")


def group_runs(runs, bmap):
    """The active runs by PR number, or by "branch:<name>" for a branch with no open PR."""
    groups = {}
    for r in runs:
        n = run_pr(r, bmap)
        if n in IGNORED_PRS:
            continue
        key = str(n) if n else "branch:" + r["headBranch"]
        groups.setdefault(key, []).append(r)
    return groups


def cancel_blocked_runs(groups, blk, why):
    for k in list(groups):
        if not k.isdigit() or int(k) not in blk["prs"] or any(jobs_started(r) for r in groups[k]):
            continue
        ids = run_ids(groups.pop(k))
        if cancel_all(ids):
            report(f"#{k}: {why}; cancelled not-started run(s) {ids}, not queued")


def release_blocked_holders(st, groups, blk, why):
    holders = st["holders"]
    for k in list(holders):
        if k.isdigit() and int(k) in blk["prs"] and k not in groups:
            rid = holders.pop(k).get("run")
            d = (gh_json(["run", "view", str(rid), "--repo", REPO, "--json", "conclusion,jobs"], ok_fail=True) if rid else None) or {}
            bad = failed_jobs(d)
            res = {"success": "GREEN"}.get(d.get("conclusion"), f"{d.get('conclusion')}" + (f"; failed jobs: {', '.join(bad)}" if bad else ""))
            report(f"#{k}: run {rid} finished {res}; slot released ({why}; not re-queued)")
    st["queue"] = [e for e in st["queue"] if e["pr"] not in blk["prs"]]
    for k in list(st.get("parked", {})):
        if k.isdigit() and int(k) in blk["prs"]:
            del st["parked"][k]


def apply_gate(st, groups):
    """gate.json holds every PR outside its allow list until its PR merges: their unstarted runs are cancelled and queued."""
    GATE.clear()
    if not CFG.exists("gate.json"):
        return
    gate = CFG.read("gate.json")
    if (gh_json(["api", f"repos/{REPO}/pulls/{gate['pr']}"], ok_fail=True) or {}).get("merged"):
        os.remove(f"{DIR}/gate.json")
        report(f"#{gate['pr']} merged; hold released, filling free slots in normal order")
        return
    GATE.update(gate)
    mineset = set(st["cancelled"])
    for k in list(groups):
        g = groups[k]
        if not k.isdigit() or int(k) in gate["allow"] or k in st["holders"] \
                or all(r["databaseId"] in mineset and mine(r, mineset) for r in g):
            continue
        if not cancel_all(run_ids(g), st):
            continue
        groups.pop(k)
        pr_run = main_run(g)
        p = pull(int(k))
        if k in st["parked"] or is_dirty(p):
            if k not in st["parked"]:
                park(st, {"pr": int(k), "run": pr_run["databaseId"], "kind": "rerun"}, pr_run["headSha"])
            report(f"#{k}: held until #{gate['pr']} merges; cancelled run(s) {run_ids(g)}; PR is dirty, parked")
            continue
        where = enqueue(st, {"pr": int(k), "run": pr_run["databaseId"], "sha": pr_run["headSha"], "kind": "rerun"})
        report(f"#{k}: held until #{gate['pr']} merges; cancelled run(s) {run_ids(g)}; queued ({where})")


def free_gated_preflights(st, groups):
    """A slot holder whose head is not released and whose runs have only the preflight done gives its slot back."""
    holders = st["holders"]
    for key in [k for k in holders if k.isdigit() and int(k) not in PINNED and groups.get(k)]:
        g = groups[key]
        if not gated(key) or not all(preflight_only(r) for r in g):
            continue
        if cancel_all(run_ids(g), st, allow_preflight=True):
            holders.pop(key)
            groups.pop(key, None)
            pr_run = main_run(g)
            where = enqueue(st, {"pr": int(key), "run": pr_run["databaseId"], "sha": pr_run["headSha"], "kind": "rerun"})
            report(f"#{key}: head {pr_run['headSha'][:10]} not released; run(s) {run_ids(g)} had only the preflight done, "
                   f"no job executing; cancelled, slot freed, held under the review gate ({where})")


def track_holder(st, groups, key, h):
    """Follow one slot holder: a newer run on its PR takes the slot over, and a finished one is closed out."""
    g = groups.get(key)
    if g:
        h["seen_active"] = True
        pr_runs = sorted([r for r in g if r["workflowName"] == MAIN_WF], key=lambda r: r["createdAt"], reverse=True)
        if pr_runs and pr_runs[0]["databaseId"] != h.get("run"):
            take_over(st, groups, key, h, pr_runs[0])
        return
    if h.get("run"):
        cur = gh_json(["run", "view", str(h["run"]), "--repo", REPO, "--json", "status,conclusion"], ok_fail=True)
        if not cur or cur.get("status") != "completed" or not cur.get("conclusion"):
            return
    limit = 900 if h.get("run") is None else 600
    if h.get("seen_active") or now() - h["since"] > limit:
        del st["holders"][key]
        finish_holder(st, int(key) if key.isdigit() else key, h)


def take_over(st, groups, key, h, nr):
    """The newest run nr of a slot holder's PR takes its slot, unless its head is unreleased and it can still be cancelled."""
    if key.isdigit() and not jobs_started(nr) and not review_gate(int(key)):
        if cancel_all([nr["databaseId"]], st):
            st["holders"].pop(key)
            groups.pop(key, None)
            where = enqueue(st, {"pr": int(key), "run": nr["databaseId"], "sha": nr["headSha"], "kind": "rerun"})
            report(f"#{key}: new head {nr['headSha'][:10]} not released; cancelled queued run {nr['databaseId']}, slot freed; held under the review gate ({where})")
        return
    if h.get("run"):
        report(f"#{key}: new head {nr['headSha'][:8]}; run {nr['databaseId']} takes over the slot from {h.get('run')}")
    h["run"] = nr["databaseId"]
    if h.get("kind") == "ready" and not h.get("announced"):
        h["announced"] = True
        report(f"#{key}: ready event started run {h['run']}")


def park_dirty_holder(st, groups, k, h):
    """A slot holder whose PR turned dirty: its unstarted runs are cancelled and the PR parked."""
    p = pull(int(k))
    if not (p and p["state"] == "open" and p.get("mergeable_state") == "dirty"):
        return
    sha = p["head"]["sha"]
    ids = run_ids(groups.get(k, []))
    kept = None
    if any(jobs_started(r) for r in groups.get(k, [])):
        kept = f"#{k}: head {sha[:8]} is DIRTY but run(s) {ids} already started; not cancelled, still holding a slot"
    elif int(k) in NOCANCEL:
        kept = f"#{k}: head {sha[:8]} is DIRTY; run(s) {ids} not cancelled (never cancelled before start), still holding a slot"
    if kept:
        if h.get("dirty_seen") != sha:
            h["dirty_seen"] = sha
            report(kept)
        return
    if not cancel_all(ids, st):
        return
    del st["holders"][k]
    park(st, {"pr": int(k), "run": h.get("run"), "kind": "rerun"}, sha)
    report(f"#{k}: head {sha[:8]} is DIRTY; cancelled its run(s) {ids} and parked until a new non-dirty head lands")


def park_baseless_run(st, groups, k, g):
    """A PR without a slot whose head lacks the strict base: its unstarted runs are cancelled and the PR parked."""
    pr_run = main_run(g)
    sha = pr_run["headSha"]
    if has_base(sha) is not False:
        return
    ids = run_ids(g)
    if any(jobs_started(r) for r in g):
        return
    if int(k) in NOCANCEL:
        if st.setdefault("nc_seen", {}).get(k + ":nobase") != sha:
            st["nc_seen"][k + ":nobase"] = sha
            report(f"#{k}: run(s) {ids} on {sha[:10]} lack {lacks_base()}; not cancelled (never cancelled before start)")
        return
    if not cancel_all(ids, st):
        return
    park(st, {"pr": int(k), "run": pr_run["databaseId"], "kind": "rerun", "reason": f"needs {BASE_BRANCH} merge"}, sha)
    del groups[k]
    report(f"#{k}: head {sha[:10]} lacks {lacks_base()}; cancelled not-yet-started run(s) {ids}; parked (needs {BASE_BRANCH} merge)")


def place_new_runs(st, groups, res, res_open):
    """Every run group without a slot: a started one is adopted, the others take a free slot or are cancelled and queued."""
    cancelled = set(st.setdefault("cancelled", []))
    others = {k: g for k, g in groups.items() if k not in st["holders"]}
    admit_pinned(st, others)
    info = [(k, g, run_started(k, g, cancelled), min((r.get("startedAt") or r["createdAt"]) for r in g)) for k, g in others.items()]
    for k, g, started, _ in info:
        if started and not hold_cancel(st, k, g):
            adopt_started(st, k, g)
    rest = sorted([x for x in info if not x[2]],
                  key=lambda x: (qkey({"pr": int(x[0]), "run": x[1][0]["databaseId"]}) if x[0].isdigit() else (float("inf"),), x[3]))
    for k, g, _, _ in rest:
        place_waiting(st, k, g, cancelled, res, res_open)


def admit_pinned(st, others):
    holders = st["holders"]
    for k in [k for k in others if k.isdigit() and int(k) in PINNED]:
        g = others.pop(k)
        if any(jobs_started(r) for r in g):
            holders[k] = dict(slot(g, "pinned"), since=now())
        elif not admit(st, k, slot(g, "pinned")):
            continue
        unqueue(st, k)
        st.setdefault("parked", {}).pop(k, None)
        report(f"#{k}: pinned PR; run {holders[k]['run']} holds a slot (never cancelled for the cap)")


def run_started(k, g, cancelled):
    """Whether a run group counts as started: a heavy job began, it is not the preflight of a gated head, and its PR is not dirty."""
    started = not all(mine(r, cancelled) for r in g) and any(jobs_started(r) for r in g)
    if started and gated(k) and all(preflight_only(r) for r in g):
        started = False
    if started and k.isdigit():
        pp = pull(int(k))
        started = not (pp and pp["state"] == "open" and pp.get("mergeable_state") == "dirty")
    return started


def adopt_started(st, k, g):
    holders = st["holders"]
    holders[k] = dict(slot(g, "adopted"), since=now())
    unqueue(st, k)
    jd = gh_json(["run", "view", str(holders[k]["run"]), "--repo", REPO, "--json", "jobs"], ok_fail=True) or {}
    js = [j.get("status") for j in jd.get("jobs", [])]
    cnt = f"{js.count('completed')} done / {js.count('in_progress')} in progress / {js.count('queued')} queued"
    if js.count("in_progress"):
        what = "already executing jobs"
    elif js:
        what = "finishing (no job executing)"
    else:
        what = "status unread, treated as started"
    report(f"#{k}: run {holders[k]['run']} {what} ({cnt}); never cancelled once started; holding a slot")


def place_waiting(st, k, g, cancelled, res, res_open):
    """One run group with no job started: it takes a slot, stays queued uncancelled, or is cancelled and queued."""
    holders = st["holders"]
    if all(mine(r, cancelled) for r in g):
        for r in g:
            gh(["api", "-X", "POST", f"repos/{REPO}/actions/runs/{r['databaseId']}/force-cancel"], ok_fail=True)
        log(f"#{k}: runs {run_ids(g)} still active after my cancel; force-cancel sent")
        return
    hr = [r["databaseId"] for r in g if r["databaseId"] in HELD_RUNS]
    if hr:
        if admit(st, k, {"run": hr[0], "seen_active": True, "kind": "priority"}):
            unqueue(st, k)
            report(f"#{k}: runner-priority run {hr[0]} holds a slot (exempt from its own hold)")
        return
    dirty = is_dirty(pull(int(k)) if k.isdigit() else None)
    if take_named_slot(st, k, g, dirty, res, res_open):
        return
    if k.isdigit():
        HEADC.pop(int(k), None)
    cap_k = CAP - 1 if res_open and k != str(res) else CAP
    mq_held = False
    if len(holders) < cap_k and k.isdigit() and not dirty and may_take_slot(st, k, g):
        if admit(st, k, slot(g, "adopted")):
            unqueue(st, k)
            report(f"#{k}: new run {holders[k]['run']} took a free slot")
            return
        mq_held = bool(MQ_HELD.get("why"))
    if k.isdigit() and int(k) in NOCANCEL:
        queue_uncancelled(st, k, g)
    else:
        cancel_and_queue(st, k, g, dirty, mq_held)


def take_named_slot(st, k, g, dirty, res, res_open):
    """The reserved PR's run over a full cap, or the sole-priority PR's run, is admitted by name. True when k is one of them."""
    holders = st["holders"]
    if res_open and k == str(res) and len(holders) >= CAP and not dirty:
        if admit(st, k, slot(g, "reserved")):
            report(f"ATTENTION #{k}: reserved PR pushed while {len(holders) - 1} runs were active; run {holders[k]['run']} NOT cancelled, now {len(holders)} active")
        return True
    if k == solo_pr() and not dirty:
        if admit(st, k, slot(g, "solo")):
            unqueue(st, k)
            report(f"#{k}: sole-priority run {holders[k]['run']} took the slot")
        return True
    return False


def may_take_slot(st, k, g):
    """A new run takes a free slot only when nothing runnable outranks it or its head is released, and the review gate lets it."""
    pr_run = main_run(g)
    return not deferred(k) and (outranks(st, {"pr": int(k), "run": pr_run["databaseId"]}) or released(int(k), pr_run["headSha"])) \
        and review_gate(int(k))


def queue_uncancelled(st, k, g):
    """A nocancel.json PR's run waits in the queue for the first free slot instead of being cancelled."""
    pr_run = main_run(g)
    enqueue(st, {"pr": int(k), "run": pr_run["databaseId"], "sha": pr_run["headSha"], "kind": "rerun"})
    resort(st)
    if st.setdefault("nc_seen", {}).get(k) != pr_run["databaseId"]:
        st["nc_seen"][k] = pr_run["databaseId"]
        pos = [e["pr"] for e in st["queue"]].index(int(k)) + 1
        slots = ", ".join(f"#{x} {v.get('run')}" for x, v in st["holders"].items())
        report(f"#{k}: run {pr_run['databaseId']} on {pr_run['headSha'][:10]} queued at position {pos}/{len(st['queue'])}, NOT cancelled; "
               f"takes the first free slot (slots: {slots})")


def cancel_and_queue(st, k, g, dirty, mq_held):
    if not cancel_all(run_ids(g), st, allow_preflight=gated(k)):
        return
    if not k.isdigit():
        report(f"cancelled run(s) {run_ids(g)} on {k} (no open PR); not queued")
        return
    pr_run = main_run(g)
    if k in st["parked"] or dirty:
        if k not in st["parked"]:
            park(st, {"pr": int(k), "run": pr_run["databaseId"], "kind": "rerun"}, pr_run["headSha"])
        report(f"#{k}: cancelled run(s) {run_ids(g)}; PR is dirty, stays parked")
        return
    where = enqueue(st, {"pr": int(k), "run": pr_run["databaseId"], "sha": pr_run["headSha"], "kind": "rerun"})
    if mq_held:
        why = "while the merge queue waits for runners"
    elif STACK_HELD.get(int(k)):
        why = "until its stack predecessor has a run"
    elif len(st["holders"]) >= CAP:
        why = "over the cap"
    else:
        why = "under the review gate"
    report(f"#{k}: cancelled queued run(s) {run_ids(g)} {why}; queued ({queue_place(st, int(k), where)})")


def trim_over_cap(st, groups):
    """Over the cap, the lowest-ranked holders whose runs have not started are cancelled and queued."""
    holders = st["holders"]
    over = len(holders) - CAP
    if over <= 0:
        return
    cands = [k for k in holders if k.isdigit() and int(k) not in PINNED and holders[k].get("kind") != "reserved" and k != solo_pr() and int(k) not in NOCANCEL]
    if not any(k.isdigit() and int(k) in PINNED for k in holders):
        cands = [k for k in cands if groups.get(k)
                 and not any(jobs_started(r) for r in groups[k])]
    cands.sort(key=lambda k: qkey({"pr": int(k), "run": holders[k]["run"]}), reverse=True)
    cands.sort(key=lambda k: any(jobs_started(r) for r in groups.get(k, [])))
    for k in cands[:over]:
        h = holders.pop(k)
        g = groups.get(k, [])
        ids = run_ids(g) or [h["run"]]
        if not cancel_all(ids, st):
            holders[k] = h
            continue
        pr_run = next((r for r in g if r["workflowName"] == MAIN_WF), None)
        where = enqueue(st, {"pr": int(k), "run": h["run"], "sha": pr_run["headSha"] if pr_run else None, "kind": "rerun"})
        report(f"#{k}: cancelled not-yet-started run(s) {ids} over the cap; queued ({where})")


def fill_slots(st, res, res_open):
    """Start queued entries into the free slots, the reserved PR first while its reservation is open."""
    holders = st["holders"]
    if res and str(res) in holders:
        os.remove(f"{DIR}/reserve.json")
        report(f"#{res}: reserved slot filled by run {holders[str(res)]['run']}; reservation cleared")
    elif res_open and len(holders) < CAP and any(e["pr"] == res for e in st["queue"]):
        rest_q = [e for e in st["queue"] if e["pr"] != res]
        st["queue"] = [e for e in st["queue"] if e["pr"] == res]
        start_next(st)
        st["queue"] += rest_q
    elif res_open and len(holders) >= CAP - 1:
        if st["queue"]:
            log(f"slot reserved for #{res}; not starting {[e['pr'] for e in st['queue']]}")
    elif len(holders) < CAP and (st["queue"] or auth_pending()):
        for _ in range(CAP):
            n0 = len(holders)
            start_next(st)
            if len(holders) <= n0 or len(holders) >= CAP or not (st["queue"] or auth_pending()):
                break


def announce_free_slot(st):
    """With a slot free and nothing able to take it, say once which held PR is first in line."""
    nh = len(st["holders"])
    if nh >= CAP or solo_pr():
        st.pop("slot_free_for", None)
        return
    cands = list(st["queue"]) + list(st.get("parked", {}).values())
    if CFG.exists("outage.json") or not cands:
        return
    top = min(cands, key=qkey)
    if st.get("slot_free_for") == top["pr"]:
        return
    st["slot_free_for"] = top["pr"]
    why = top.get("reason") or ("parked, dirty" if str(top["pr"]) in st.get("parked", {}) else "queued, no runnable head")
    if CFG.exists(F_REVIEW_GATE):
        log(f"{CAP - nh} slot(s) free; no released head waiting (top held: #{top['pr']}, {why}); no invitation sent")
    else:
        report(f"slot free for #{top['pr']} ({CAP - nh} free; #{top['pr']} is {why}): it may push its prepared head now")


FAST = 10
LAST_SEEN = set()


CI_AFTER = None
CI_PRS = set()
CI_SEEN = {}
CI_FLAGGED = set()


def ci_files(n):
    fs = gh_json(["pr", "view", str(n), "--repo", REPO, "--json", "files,state"], ok_fail=True)
    if fs is None:
        return None
    return fs.get("state"), {f["path"] for f in fs.get("files", [])}


def ci_overlap(n):
    """Release authority: a PRIORITY CI auto-admit that changes files of an already-admitted priority PR is flagged before its run is released."""
    mine = ci_files(n)
    if mine is None:
        return "files unreadable"
    for o in sorted(CI_PRS - {n}):
        other = ci_files(o)
        if other is None:
            return f"files of #{o} unreadable"
        if other[0] == "OPEN" and mine[1] & other[1]:
            return f"changes {len(mine[1] & other[1])} file(s) of already-admitted priority PR #{o}"
    return None


def ci_mark(p):
    """With ci_priority configured, a new CI-optimization PR into the base branch (with tiers.json, in tier 1 or 2) is exempt: runs admitted automatically, never cancelled."""
    n = p["number"]
    # ci_prio.json is re-read so the release authority can add a priority PR without a restart.
    if CFG.exists(F_CI_PRIO):
        CI_PRS.update(CFG.read(F_CI_PRIO))
    if n in CI_FLAGGED:
        return False
    named = tier_members(0, 1)
    if named is not None and n not in named:
        return False
    if n in CI_PRS:
        EXEMPT_PRS.add(n)
        return True
    cp = CFG["ci_priority"]
    if CI_AFTER is None or n <= CI_AFTER or p.get("baseRefName") != BASE_BRANCH or CI_SEEN.get(n) == p.get("headRefOid"):
        return False
    why = "CI title" if cp.get("title_re") and re.search(cp["title_re"], p.get("title") or "", re.I) else None
    if not why:
        fs = gh_json(["pr", "view", str(n), "--repo", REPO, "--json", "files"], ok_fail=True)
        if fs is None:
            return False
        hits = [f["path"] for f in fs.get("files", []) if cp.get("path_re") and re.search(cp["path_re"], f["path"])]
        if hits:
            why = "touches " + ", ".join(hits[:4])
    CI_SEEN[n] = p.get("headRefOid")
    if not why:
        return False
    ov = ci_overlap(n)
    if ov:
        CI_FLAGGED.add(n)
        CFG.write("ci_flagged.json", sorted(CI_FLAGGED))
        report(f"FLAG for release authority: CI PR #{n} (\"{p.get('title')}\") head {(p.get('headRefOid') or '')[:10]}: {why}, but {ov}; "
               f"NOT auto-admitted as priority, its run stays held until the release authority decides")
        return False
    CI_PRS.add(n)
    EXEMPT_PRS.add(n)
    CFG.write(F_CI_PRIO, sorted(CI_PRS))
    report(f"PRIORITY CI PR #{n} (\"{p.get('title')}\") head {(p.get('headRefOid') or '')[:10]}: {why}; runs admitted automatically, never cancelled; "
           f"ahead of everything not already in the merge queue")
    return True


def refresh_ci_priority():
    ps = gh_json(["pr", "list", "--repo", REPO, "--state", "open", "--limit", "300", "--json", "number,title,baseRefName,headRefOid"], ok_fail=True)
    for p in ps or []:
        ci_mark(p)


def stale_bt(st):
    """Release authority: after a released run goes green, a failing/cancelled latest required check from another run on that head blocks the queue; rerun it --failed (protected, priority hold) once no other run is live on the head."""
    for k, e in list(st.get("stale_bt", {}).items()):
        p = gh_json(["api", f"repos/{REPO}/pulls/{k}"], ok_fail=True)
        if p is None:
            continue
        if p["state"] != "open" or p["head"]["sha"] != e["head"]:
            st["stale_bt"].pop(k)
            continue
        if any(x.split("@")[0] == str(k) for x in st.get("mq", {}).get("entries", [])):
            report(f"#{k}: already in the merge queue; stale '{REQUIRED}' on {e['head'][:10]} is not blocking it, no rerun")
            st["stale_bt"].pop(k)
            continue
        if p.get("auto_merge") is None:
            report(f"#{k}: auto-merge not armed, so no queue entry is waiting on the stale '{REQUIRED}' on {e['head'][:10]}; no rerun")
            st["stale_bt"].pop(k)
            continue
        cr = gh_json(["api", f"repos/{REPO}/commits/{e['head']}/check-runs?check_name={urllib.parse.quote(REQUIRED)}&filter=latest&per_page=50"], ok_fail=True)
        if cr is None:
            continue
        bad = sorted({int(re.search(r"runs/(\d+)", c["details_url"]).group(1)) for c in cr.get("check_runs", [])
                      if c.get("status") == "completed" and c.get("conclusion") in ("failure", "cancelled", "timed_out")})
        bad = [r for r in bad if r != e["green"]]
        if not bad:
            report(f"#{k}: no stale failing '{REQUIRED}' on head {e['head'][:10]}; required check clear")
            st["stale_bt"].pop(k)
            continue
        live = gh_json(["api", f"repos/{REPO}/actions/runs?head_sha={e['head']}&per_page=50", "--jq",
                        f'[.workflow_runs[]|select(.name=={json.dumps(MAIN_WF)} and .status!="completed")|.id]'], ok_fail=True)
        if live is None or live:
            if not e.get("deferred"):
                e["deferred"] = True
                report(f"#{k}: stale failing '{REQUIRED}' from run(s) {bad} on head {e['head'][:10]}; rerun deferred while run(s) {live} are live on the head (concurrency)")
            continue
        gc = gh_json(["run", "view", str(e["green"]), "--repo", REPO, "--json", "conclusion"], ok_fail=True)
        if gc is None:
            continue
        if gc.get("conclusion") != "success":
            report(f"#{k}: run {e['green']} did not finish green ({gc.get('conclusion')}); stale '{REQUIRED}' rerun dropped")
            st["stale_bt"].pop(k)
            continue
        r = bad[0]
        ex = CFG.read(F_EXT_OK, [])
        if r not in ex:
            ex.append(r)
            CFG.write(F_EXT_OK, ex)
        CFG.write(f"priority_hold_{k}.json", {"pr": int(k), "after": "0000000000", "run": r, "head": e["head"],
                                             "label": f"#{k} {e['head'][:10]} stale required-check rerun"})
        out = gh(["run", "rerun", str(r), "--failed", "--repo", REPO], ok_fail=True)
        if out is None:
            report(f"#{k}: rerun --failed of stale '{REQUIRED}' run {r} FAILED to start; will retry")
            continue
        report(f"#{k}: stale failing '{REQUIRED}' from run {r} on head {e['head'][:10]} blocks the queue; rerun --failed started (cancel-protected, runner-priority hold)")
        st["stale_bt"].pop(k)


def mq_track(st):
    """Release authority: merge_group runs are never cancelled or held. While one is active with no job waiting the full cap
    (3 by default) applies; the cap is 1 only while a merge-queue job waits. Returns the active count, None on API failure."""
    m = st.setdefault("mq", {"runs": {}, "entries": []})
    d = gh_json(["api", f"repos/{REPO}/actions/runs?event=merge_group&per_page=30"], ok_fail=True)
    if d is None:
        return None
    act = 0
    for r in d.get("workflow_runs", []):
        if r.get("name") not in WORKFLOWS:
            continue
        rid, br, stt = str(r["id"]), r.get("head_branch") or "", r.get("status")
        mm = re.search(r"pr-(\d+)-", br)
        pr = mm.group(1) if mm else "?"
        old = m["runs"].get(rid)
        if stt != "completed":
            act += 1
            if old is None:
                m["runs"][rid] = "active"
                report(f"MERGE QUEUE: #{pr} queue build run {rid} started on {br} (merge commit {r['head_sha'][:10]}); never cancelled")
        elif old != "done":
            m["runs"][rid] = "done"
            report_queue_build(pr, rid, r)
    track_queue_entries(m)
    return act


def report_queue_build(pr, rid, r):
    c = r.get("conclusion")
    if c == "success":
        report(f"MERGE QUEUE: #{pr} queue build run {rid} GREEN (merge commit {r['head_sha'][:10]})")
        return
    j = gh_json(["run", "view", rid, "--repo", REPO, "--json", "jobs"], ok_fail=True) or {}
    fj = [f"{x['name']} [{','.join(s_['name'] for s_ in x.get('steps') or [] if s_.get('conclusion') == 'failure')}]"
          for x in j.get("jobs", []) if x.get("conclusion") == "failure"]
    report(f"MERGE QUEUE: #{pr} queue build run {rid} {'RED' if c == 'failure' else c}: {'; '.join(fj) or 'no failing job listed'}")


def track_queue_entries(m):
    """Report each PR head that entered or left the merge queue since the last pass."""
    ns = merge_queue_entries("position state pullRequest{number headRefOid}")
    if ns is None:
        return
    cur = [f"{n['pullRequest']['number']}@{n['pullRequest']['headRefOid'][:10]}" for n in ns]
    for n, k in zip(ns, cur):
        if k not in m["entries"]:
            report(f"MERGE QUEUE: #{n['pullRequest']['number']} {k.split('@')[1]} entered the queue at position {n['position']} ({n['state']})")
    for k in m["entries"]:
        if k not in cur:
            report(f"MERGE QUEUE: #{k.split('@')[0]} {k.split('@')[1]} left the queue")
    m["entries"] = cur


def gate_sweep(st):
    """Between passes: cancel a just-started run of an unreleased head while only its preflight has run."""
    if not CFG.exists(F_REVIEW_GATE):
        return False
    hit = False
    for status in ("queued", "in_progress"):
        rr = gh_json(["api", f"repos/{REPO}/actions/runs?event=pull_request&status={status}&per_page=50"], ok_fail=True) or {}
        for r in rr.get("workflow_runs", []):
            if r.get("name") in WORKFLOWS and r.get("pull_requests") and sweep_run(st, r):
                hit = True
    if hit:
        save(st)
    return hit


def sweep_run(st, r):
    """Cancel one active run when its head is unreleased and only its preflight has run; True when it was cancelled."""
    pr, rid, sha = r["pull_requests"][0]["number"], r["id"], r["head_sha"]
    if pr in PINNED or pr in NOCANCEL or pr in EXEMPT_PRS or released(pr, sha) or rid in st.get("cancelled", []):
        return False
    p = gh_json(["api", f"repos/{REPO}/pulls/{pr}"], ok_fail=True)
    if not p or ci_mark({"number": pr, "title": p.get("title"), "baseRefName": p["base"]["ref"], "headRefOid": p["head"]["sha"]}):
        return False
    if p.get("state") != "open" or p["head"]["sha"] != sha or p["base"]["ref"] != r["pull_requests"][0]["base"]["ref"]:
        return False
    if not preflight_only({"databaseId": rid}) or not cancel_all([rid], st, allow_preflight=True):
        return False
    if (st["holders"].get(str(pr)) or {}).get("run") == rid:
        st["holders"].pop(str(pr))
    if p.get("draft"):
        report(f"#{pr}: draft head {sha[:10]} not released; run {rid} caught with only the preflight running; cancelled, held until released")
        return True
    where = enqueue(st, {"pr": pr, "run": rid, "sha": sha, "kind": "rerun"})
    report(f"#{pr}: head {sha[:10]} not released; run {rid} caught with only the preflight running, no heavy job started; "
           f"cancelled, held under the review gate ({where})")
    return True


def queued_pr_runs():
    d = gh_json(["run", "list", "--repo", REPO, "--status", "queued", "-L", "50",
                 "--json", "databaseId,workflowName,event"], ok_fail=True) or []
    return {r["databaseId"] for r in d if r.get("event") == "pull_request" and r.get("workflowName") in WORKFLOWS}


def wait_for_new(st):
    seen = set(LAST_SEEN)
    end = time.time() + INTERVAL
    while time.time() < end:
        time.sleep(min(FAST, max(0, end - time.time())))
        why = pass_due(st, seen)
        if why:
            log(f"fast poll: {why}; running a pass now")
            return


def run_completed(rid):
    d = rid and gh_json(["run", "view", str(rid), "--repo", REPO, "--json", "status"], ok_fail=True)
    return bool(d) and d.get("status") == "completed"


def pass_due(st, seen):
    """Why the next pass should run before the interval ends, or None."""
    if attempt("gate_sweep", gate_sweep, st):
        return "cancelled unreleased run(s) during preflight"
    if arm_pushed():
        return "armed sole-priority PR pushed"
    if CFG.exists("solo.json"):
        o = CFG.read("solo.json")
        if run_completed(o.get("run")):
            return f"#{o['pr']}'s solo run {o['run']} completed"
    try:
        hs = CFG.read(F_STATE).get("holders", {})
    except Exception:
        hs = {}
    for pr in NOCANCEL:
        rid = (hs.get(str(pr)) or {}).get("run")
        if run_completed(rid):
            return f"#{pr}'s run {rid} completed"
    new = queued_pr_runs() - seen - GHOSTS
    if not new:
        return None
    for _ in range(3):
        if attempt("gate_sweep", gate_sweep, st):
            break
        time.sleep(5)
    return f"new queued run(s) {sorted(new)}"


def main(argv=None):
    ap = argparse.ArgumentParser(description="Throttle a repository's pull-request CI runs; see ci/README.md.")
    ap.add_argument("--config", help="configuration file (default $CI_THROTTLE_CONFIG or ~/ci-throttle/config.json)")
    ap.add_argument("--dry-run", action="store_true", help="read GitHub and log every mutation instead of running it")
    ap.add_argument("--once", action="store_true", help="run one pass and exit")
    args = ap.parse_args(argv)
    configure(Config(args.config, dry_run=args.dry_run))
    os.makedirs(DIR, exist_ok=True)
    while True:
        st = load()
        if st.get("stop") or CFG.exists("stop.json"):
            log("stop flag set; exiting")
            break
        try:
            refresh_ci_priority()
            iterate(st)
        except Exception:
            log("ERROR " + traceback.format_exc().replace("\n", " | "))
        save(st)
        if args.once:
            break
        wait_for_new(st)
    return 0


if __name__ == "__main__":
    sys.exit(main())
