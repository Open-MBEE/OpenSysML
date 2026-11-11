#!/usr/bin/env python3
"""Report whether a commit already passed the full test suite elsewhere.

CircleCI's build-test workflow runs on every push to develop and main, and the
release workflow on every v* tag, over trees the pull-request workflow (or the
merge queue) has usually just tested. The verify-tree job runs this script first
and persists its verdict as verified-tree.txt; the suite jobs read the marker
and, when the first line is `verified: ...`, halt before doing their work again.

The verdict is one line on stdout, `verified: <evidence>` or `unverified:
<reason>`; the evidence considered goes to stderr. Two sources count:

  - GitHub Actions: a completed, successful check run named `Build and test`
    from the github-actions app on the commit, whose check suite also holds at
    least one successful `Go race tests (…)` job. Only the full tier runs the
    race shards (the quick tier of a draft pull request runs `Go tests (…)`),
    so the second condition is what keeps a draft-tier run from counting.
  - CircleCI, for a tag only: a successful `build-test` workflow of a pipeline
    on the branch (main) whose commit is the tag's. Reading workflows needs
    CIRCLECI_API_TOKEN; without it this source reports unverified.

Any failure to read the evidence — a rate limit, a network error, a malformed
answer — is reported as unverified, so an error always runs the suite. The
exit status is 0 for either verdict and 2 for a usage error.

    python3 scripts/ci-verified-tree.py --sha <commit>
    python3 scripts/ci-verified-tree.py --sha <commit> --tag v1.2.3
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
FULL_TIER_PREFIX = "Go race tests ("
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


def github_verdict(repo: str, sha: str) -> str | None:
    """The GitHub evidence for a full-tier pass of the commit, or None."""
    aggregates = [run for run in github_check_runs(repo, sha, AGGREGATE_CHECK) if run.get("name") == AGGREGATE_CHECK]
    passing = [run for run in aggregates if green(run, sha)]
    log(f"GitHub: {len(aggregates)} '{AGGREGATE_CHECK}' check run(s) on {sha}, {len(passing)} completed successfully")
    if not passing:
        return None
    suites = {run["check_suite"]["id"] for run in passing}
    shards_by_suite: dict[int, int] = {}
    for run in github_check_runs(repo, sha, None):
        suite = (run.get("check_suite") or {}).get("id")
        if suite in suites and str(run.get("name", "")).startswith(FULL_TIER_PREFIX) and green(run, sha):
            shards_by_suite[suite] = shards_by_suite.get(suite, 0) + 1
    for suite in sorted(suites):
        shards = shards_by_suite.get(suite, 0)
        log(f"GitHub: check suite {suite} holds {shards} successful '{FULL_TIER_PREFIX}…)' job(s)")
        if shards > 0:
            return f"GitHub Actions check suite {suite} passed '{AGGREGATE_CHECK}' on {sha} with {shards} '{FULL_TIER_PREFIX}…)' shards (full tier)"
    log("GitHub: no passing suite ran the race shards, so every pass was a quick-tier run")
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
    parser.add_argument("--tag", help="the tag being released; also accepts a green CircleCI build-test on the commit")
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
    try:
        evidence = github_verdict(args.repo, args.sha)
        if evidence is None and args.tag:
            evidence = circleci_verdict(args.circleci_project, args.circleci_branch, args.sha)
    except Exception as error:  # noqa: BLE001 - any failure to read the evidence runs the suite
        print(f"unverified: could not read the evidence for {args.sha} ({type(error).__name__}: {error}); running the suite")
        return 0
    if evidence:
        print(f"verified: {evidence}")
    else:
        print(f"unverified: no green full-tier run of {args.sha} was found; running the suite")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
