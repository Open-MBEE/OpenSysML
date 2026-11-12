#!/usr/bin/env python3
"""Report whether a commit already passed the test suite elsewhere.

CircleCI's build-test workflow runs on every push to develop and main, and the
release workflow on every v* tag, over trees the pull-request workflow (or the
merge queue) has usually just tested. The verify-tree job runs this script first
and persists its verdict as verified-tree.txt; the suite jobs read the marker
and, when the first line is `verified: ...`, halt before doing their work again.
The nightly full scan (.github/workflows/full-scan.yml) asks it too, before
dispatching the full tier on develop's head.

The verdict is one line on stdout, `verified: <evidence>` or `unverified:
<reason>`; the evidence considered goes to stderr. Two sources count:

  - GitHub Actions: a completed, successful check run named `Build and test`
    from the github-actions app on the commit, whose check suite also holds at
    least one successful Go test job of an accepted tier. The job's name carries
    the tier (.github/workflows/pr.yml): `Go race tests (…)` is the full tier
    (-race over the whole corpus), `Go queue tests (…)` the merge queue's
    (-race everywhere but the examples/ sweeps, which run race-free over the
    construct-cover subset), and `Go tests (…)` the quick tier of a
    pull-request push, which never counts.
  - CircleCI, for a tag only: a successful `build-test` workflow of a pipeline
    on the branch (main) whose commit is the tag's. Reading workflows needs
    CIRCLECI_API_TOKEN; without it this source reports unverified.

Which tiers each caller accepts, and why:

  - A push to develop (no flag) accepts the queue tier as well as the full one:
    every develop commit arrives through the merge queue, and the nightly full
    scan gives develop its -race, whole-corpus run once a night, so build-test
    need not repeat the suite on every landing.
  - A tag (--tag) accepts only the full tier, or a green CircleCI build-test of
    main on the same commit: a release ships nothing the race detector has not
    seen over the whole corpus, and main's own build-test is that run.
  - The nightly full scan (--full-tier) accepts only the full tier, from GitHub
    alone: it exists to give develop's head that run, and skips when the head
    already has one (a ci:full run, an earlier dispatch).

Any failure to read the evidence — a rate limit, a network error, a malformed
answer — is reported as unverified, so an error always runs the suite. The
exit status is 0 for either verdict and 2 for a usage error.

    python3 scripts/ci-verified-tree.py --sha <commit>
    python3 scripts/ci-verified-tree.py --sha <commit> --tag v1.2.3
    python3 scripts/ci-verified-tree.py --sha <commit> --full-tier
    python3 scripts/ci-verified-tree.py --sha <commit> --rehearsal   # always unverified

Environment:
    GH_TOKEN/GITHUB_TOKEN   optional; unauthenticated GitHub requests are
                            limited to 60 an hour per address.
    CIRCLECI_API_TOKEN      optional; needed for the CircleCI source.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request

GITHUB_API = "https://api.github.com"
CIRCLECI_API = "https://circleci.com/api/v2"
AGGREGATE_CHECK = "Build and test"
# The Go test job's name prefix per tier (.github/workflows/pr.yml, go-tests).
TIER_PREFIXES = {"full": "Go race tests (", "queue": "Go queue tests ("}
CIRCLECI_WORKFLOW = "build-test"
PIPELINE_PAGES = 10


def log(message: str) -> None:
    print(message, file=sys.stderr, flush=True)


def get_json(url: str, headers: dict[str, str]):
    request = urllib.request.Request(url, headers=headers)
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.load(response)


def github_headers() -> dict[str, str]:
    headers = {"Accept": "application/vnd.github+json", "User-Agent": "ci-verified-tree"}
    token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
    if token:
        headers["Authorization"] = f"Bearer {token}"
    return headers


def github_check_runs(repo: str, sha: str, check_name: str | None) -> list[dict]:
    runs: list[dict] = []
    for page in range(1, 11):
        query = {"per_page": "100", "page": str(page)}
        if check_name:
            query["check_name"] = check_name
        url = f"{GITHUB_API}/repos/{repo}/commits/{sha}/check-runs?{urllib.parse.urlencode(query)}"
        answer = get_json(url, github_headers())
        batch = answer["check_runs"]
        runs.extend(batch)
        if len(runs) >= answer["total_count"] or not batch:
            break
    return runs


def green(run: dict, sha: str) -> bool:
    return (
        run.get("status") == "completed"
        and run.get("conclusion") == "success"
        and run.get("head_sha") == sha
        and (run.get("app") or {}).get("slug") == "github-actions"
    )


def github_verdict(repo: str, sha: str, tiers: tuple[str, ...]) -> str | None:
    """The GitHub evidence for a pass of the commit at one of the tiers, or None."""
    aggregates = [run for run in github_check_runs(repo, sha, AGGREGATE_CHECK) if run.get("name") == AGGREGATE_CHECK]
    passing = [run for run in aggregates if green(run, sha)]
    log(f"GitHub: {len(aggregates)} '{AGGREGATE_CHECK}' check run(s) on {sha}, {len(passing)} completed successfully")
    if not passing:
        return None
    suites = {run["check_suite"]["id"] for run in passing}
    shards_by_suite: dict[tuple[int, str], int] = {}
    for run in github_check_runs(repo, sha, None):
        suite = (run.get("check_suite") or {}).get("id")
        if suite not in suites or not green(run, sha):
            continue
        for tier, prefix in TIER_PREFIXES.items():
            if str(run.get("name", "")).startswith(prefix):
                shards_by_suite[(suite, tier)] = shards_by_suite.get((suite, tier), 0) + 1
    for suite in sorted(suites):
        for tier, prefix in TIER_PREFIXES.items():
            shards = shards_by_suite.get((suite, tier), 0)
            log(f"GitHub: check suite {suite} holds {shards} successful '{prefix}…)' job(s)")
            if shards > 0 and tier in tiers:
                return f"GitHub Actions check suite {suite} passed '{AGGREGATE_CHECK}' on {sha} with {shards} '{prefix}…)' shards ({tier} tier)"
    log(f"GitHub: no passing suite ran the Go shards of an accepted tier ({', '.join(tiers)})")
    return None


def circleci_verdict(project: str, branch: str, sha: str) -> str | None:
    """The CircleCI evidence for a green build-test workflow of the commit, or None."""
    token = os.environ.get("CIRCLECI_API_TOKEN")
    if not token:
        log("CircleCI: CIRCLECI_API_TOKEN is not set, so its workflows cannot be read")
        return None
    headers = {"Circle-Token": token, "Accept": "application/json", "User-Agent": "ci-verified-tree"}
    page_token = None
    matched = 0
    for _ in range(PIPELINE_PAGES):
        query = {"branch": branch}
        if page_token:
            query["page-token"] = page_token
        answer = get_json(f"{CIRCLECI_API}/project/{project}/pipeline?{urllib.parse.urlencode(query)}", headers)
        for pipeline in answer.get("items", []):
            revision = (pipeline.get("vcs") or {}).get("revision") or (
                ((pipeline.get("trigger_parameters") or {}).get("github_app") or {}).get("commit_sha")
            )
            if revision != sha:
                continue
            matched += 1
            workflows = get_json(f"{CIRCLECI_API}/pipeline/{pipeline['id']}/workflow", headers).get("items", [])
            for workflow in workflows:
                log(f"CircleCI: pipeline {pipeline.get('number')} workflow {workflow.get('name')} is {workflow.get('status')}")
                if workflow.get("name") == CIRCLECI_WORKFLOW and workflow.get("status") == "success":
                    return f"CircleCI pipeline {pipeline.get('number')} on {branch} passed its {CIRCLECI_WORKFLOW} workflow on {sha}"
        page_token = answer.get("next_page_token")
        if not page_token:
            break
    log(f"CircleCI: {matched} pipeline(s) on {branch} built {sha}; none passed {CIRCLECI_WORKFLOW}")
    return None


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument("--sha", required=True, help="the commit CircleCI is building (CIRCLE_SHA1)")
    parser.add_argument("--tag", help="the tag being released; accepts only the full tier, or a green CircleCI build-test on the commit")
    parser.add_argument("--full-tier", action="store_true", help="accept only a full-tier GitHub run (the nightly full scan's question)")
    parser.add_argument("--rehearsal", action="store_true", help="a release rehearsal, which always runs the suite")
    parser.add_argument("--repo", default="Open-MBEE/OpenSysML", help="GitHub repository (owner/name)")
    parser.add_argument("--circleci-project", default="gh/Open-MBEE/OpenSysML", help="CircleCI project slug")
    parser.add_argument("--circleci-branch", default="main", help="the branch whose build-test workflows count for a tag")
    args = parser.parse_args(argv)
    sys.stdout.reconfigure(line_buffering=True)
    if len(args.sha) != 40 or any(c not in "0123456789abcdef" for c in args.sha):
        parser.error("--sha must be a full 40-hexdigit commit id")

    if args.rehearsal:
        print(f"unverified: release rehearsal of {args.sha}; the rehearsal exists to run the suite under the tag")
        return 0
    tiers = ("full",) if args.tag or args.full_tier else ("full", "queue")
    try:
        evidence = github_verdict(args.repo, args.sha, tiers)
        if evidence is None and args.tag:
            evidence = circleci_verdict(args.circleci_project, args.circleci_branch, args.sha)
    except Exception as error:  # noqa: BLE001 - any failure to read the evidence runs the suite
        print(f"unverified: could not read the evidence for {args.sha} ({type(error).__name__}: {error}); running the suite")
        return 0
    if evidence:
        print(f"verified: {evidence}")
    else:
        print(f"unverified: no green {' or '.join(f'{tier}-tier' for tier in tiers)} run of {args.sha} was found; running the suite")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
