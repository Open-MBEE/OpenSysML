#!/usr/bin/env python3
"""Configuration, control files and the guarded `gh` wrapper shared by the CI throttle tools."""
import contextlib
import datetime
import fcntl
import json
import os
import subprocess
import sys
import time

DEFAULT_CONFIG = "~/ci-throttle/config.json"
MERGE_QUEUE_PREFIX = "gh-readonly-queue/"
DEFAULTS = {
    "base_branch": "develop",
    "workflows": ["PR"],
    "preflight_jobs": ["Changed areas"],
    "required_check": "Build and test",
    "cap": 3,
    "interval_seconds": 120,
    "fast_poll_seconds": 10,
    "exempt_branches": ["develop", "main"],
    "exempt_prefixes": ["release/", "hotfix/"],
    "runner_pin": None,
    "ci_priority": None,
    "known_flakes": [],
    "job_timeout_minutes": 59,
    "starve_grace_seconds": 180,
    "gh_token_file": None,
}
# Commands that change GitHub state; a freeze file or a dry run blocks every one of them.
AUTH_ERRORS = ("gh auth login", "Bad credentials", "HTTP 401")
AUTH_RETRY_SECONDS = 5
MUTATING = (["run", "rerun"], ["run", "cancel"], ["pr", "close"], ["pr", "reopen"], ["pr", "edit"], ["pr", "ready"],
            ["pr", "merge"], ["workflow", "run"])


class Config:
    def __init__(self, path=None, dry_run=False):
        path = os.path.expanduser(path or os.environ.get("CI_THROTTLE_CONFIG") or DEFAULT_CONFIG)
        with open(path) as f:
            raw = json.load(f)
        if not raw.get("repo") or "/" not in raw["repo"]:
            raise SystemExit(f"{path}: \"repo\" must be OWNER/NAME")
        if "exempt_body_tag" in raw:
            raise SystemExit(f"{path}: \"exempt_body_tag\" was removed, since any PR author can edit a PR body; list exempt PRs in exempt.json")
        self.path = path
        self.values = dict(DEFAULTS, **raw)
        self.repo = raw["repo"]
        self.owner, self.name = self.repo.split("/", 1)
        self.state_dir = os.path.expanduser(raw.get("state_dir") or os.path.dirname(os.path.abspath(path)))
        self.checkout = os.path.expanduser(raw["checkout"]) if raw.get("checkout") else None
        self.dry_run = dry_run

    def __getitem__(self, key):
        return self.values[key]

    def file(self, name):
        return os.path.join(self.state_dir, name)

    def exists(self, name):
        return os.path.exists(self.file(name))

    def read(self, name, default=None):
        """A control file's JSON, or default when it is absent."""
        try:
            with open(self.file(name)) as f:
                return json.load(f)
        except FileNotFoundError:
            return default

    def write(self, name, value):
        tmp = self.file(name) + ".tmp"
        with open(tmp, "w") as f:
            json.dump(value, f, indent=1)
        os.replace(tmp, self.file(name))

    def remove(self, name):
        try:
            os.remove(self.file(name))
        except FileNotFoundError:
            pass

    def frozen(self):
        return self.exists("freeze.json")

    @contextlib.contextmanager
    def locked(self, name):
        """Hold an exclusive lock on a control file across a read-modify-write shared between tools."""
        with open(self.file(name) + ".lock", "a") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            try:
                yield
            finally:
                fcntl.flock(lock, fcntl.LOCK_UN)


def utc_stamp():
    return datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def mutating(args):
    return list(args[:2]) in [list(m) for m in MUTATING] or (args[:1] == ["api"] and any(a in ("-X", "--method") for a in args))


def heavy_job_started(cfg, jobs):
    """A run has started once a job other than the preflight is executing or ran to a real conclusion."""
    return any(j.get("name") not in cfg["preflight_jobs"] and (j.get("status") == "in_progress" or (
        j.get("status") == "completed" and j.get("conclusion") not in ("cancelled", "skipped"))) for j in jobs)


def protected_branch(cfg, branch, event=None):
    """Runs no tool may cancel: the merge queue, the long-lived and release branches, and tags."""
    branch = branch or ""
    if event == "merge_group" or branch.startswith(MERGE_QUEUE_PREFIX):
        return True
    if branch in cfg["exempt_branches"] or branch.startswith(tuple(cfg["exempt_prefixes"])):
        return True
    return branch.startswith("v") and branch[1:2].isdigit()


class GH:
    """Runs `gh`. Mutations are skipped under a freeze file or a dry run; a failure raises unless ok_fail."""

    def __init__(self, cfg, log):
        self.cfg = cfg
        self.log = log

    def __call__(self, args, ok_fail=False):
        if mutating(args):
            if self.cfg.frozen():
                self.log(f"FROZEN (freeze.json): skipped gh {' '.join(args)}")
                return None
            if self.cfg.dry_run:
                self.log(f"DRY RUN: would run gh {' '.join(args)}")
                return None
        p = self.run(args)
        if p.returncode != 0 and any(e in p.stderr for e in AUTH_ERRORS):
            # The credentials are rewritten on token refresh; one retry picks up the new token.
            self.log(f"gh {' '.join(args)}: authentication failed; retrying once in {AUTH_RETRY_SECONDS} s")
            time.sleep(AUTH_RETRY_SECONDS)
            p = self.run(args)
        if p.returncode != 0:
            if ok_fail:
                self.log(f"gh {' '.join(args)} failed: {p.stderr.strip()[:300]}")
                return None
            raise RuntimeError(f"gh {' '.join(args)}: {p.stderr.strip()[:300]}")
        return p.stdout

    def run(self, args):
        env = None
        if self.cfg["gh_token_file"]:
            with open(os.path.expanduser(self.cfg["gh_token_file"])) as f:
                env = dict(os.environ, GH_TOKEN=f.read().strip())
        return subprocess.run(["gh"] + list(args), capture_output=True, text=True, timeout=120, env=env)

    def json(self, args, ok_fail=False):
        out = self(args, ok_fail)
        return None if out is None else json.loads(out or "null")


def main(argv):
    """`throttle_common.py [--config PATH] get KEY` prints one configuration value for the shell tools."""
    path = None
    if argv[:1] == ["--config"]:
        path, argv = argv[1], argv[2:]
    if len(argv) != 2 or argv[0] != "get":
        print("usage: throttle_common.py [--config PATH] get KEY", file=sys.stderr)
        return 2
    cfg = Config(path)
    key = argv[1]
    value = {"repo": cfg.repo, "state_dir": cfg.state_dir, "checkout": cfg.checkout}.get(key, cfg.values.get(key))
    if value is None:
        print(f"{cfg.path}: no value for {key}", file=sys.stderr)
        return 1
    print(value if isinstance(value, str) else json.dumps(value))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
