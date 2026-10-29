#!/usr/bin/env python3
"""Print the download counts of every GitHub release of a repository.

One line per release with its publication date, asset count and downloads,
then the grand total; the per-asset figures on request. Nothing but the GitHub
REST API is read, so the script runs anywhere Python does.

    python3 scripts/release-downloads.py                 # every release, newest first
    python3 scripts/release-downloads.py --stable        # skip pre-releases (the nightlies)
    python3 scripts/release-downloads.py --assets        # the per-asset counts under each release
    python3 scripts/release-downloads.py --by-asset      # totals per asset name across releases
    python3 scripts/release-downloads.py --json          # the raw figures, for another tool
    python3 scripts/release-downloads.py --traffic       # clones and views of the last 14 days
    python3 scripts/release-downloads.py --repo Open-MBEE/sysml-toolkit

Clone and view counts come from the traffic API, which keeps the last 14 days
only (per day, total and unique) and answers only a token with push access to
the repository; there is no all-time clone count to be had from GitHub.

Environment:
    GITHUB_TOKEN/GH_TOKEN   optional for the release figures (unauthenticated
                            requests are limited to 60 an hour per address,
                            which lists several hundred releases); required,
                            with push access, for --traffic
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
import urllib.request
from collections import OrderedDict
from typing import Any

DEFAULT_REPO = "Open-MBEE/OpenSysML"
PAGE_SIZE = 100
NEXT_LINK = re.compile(r'<(?P<url>[^>]+)>;\s*rel="next"')


def _token() -> str:
    for name in ("GITHUB_TOKEN", "GH_TOKEN"):
        value = os.environ.get(name, "").strip()
        if value:
            return value
    return ""


def _get(url: str, token: str) -> tuple[Any, str]:
    """One API request; returns the decoded JSON body and the Link header."""
    request = urllib.request.Request(url)
    request.add_header("Accept", "application/vnd.github+json")
    request.add_header("X-GitHub-Api-Version", "2022-11-28")
    if token:
        request.add_header("Authorization", f"Bearer {token}")
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.loads(response.read().decode("utf-8")), response.headers.get("Link", "")


def fetch_releases(repo: str, token: str = "") -> list[dict[str, Any]]:
    """Every release of the repository, following the API's pagination."""
    url: str | None = f"https://api.github.com/repos/{repo}/releases?per_page={PAGE_SIZE}"
    releases: list[dict[str, Any]] = []
    while url:
        page, link = _get(url, token)
        releases.extend(page)
        match = NEXT_LINK.search(link)
        url = match.group("url") if match else None
    return releases


def fetch_traffic(repo: str, token: str) -> dict[str, Any]:
    """The clones and views of the last 14 days, as the traffic API reports them."""
    base = f"https://api.github.com/repos/{repo}/traffic"
    clones, _ = _get(f"{base}/clones", token)
    views, _ = _get(f"{base}/views", token)
    return {"clones": clones, "views": views}


def summarize(releases: list[dict[str, Any]], stable_only: bool = False) -> list[dict[str, Any]]:
    """The figures of each release, newest first, as plain dictionaries."""
    rows = []
    for release in releases:
        if stable_only and (release.get("prerelease") or release.get("draft")):
            continue
        assets = [
            {"name": a["name"], "downloads": int(a.get("download_count", 0)), "size": int(a.get("size", 0))}
            for a in release.get("assets", [])
        ]
        rows.append(
            {
                "tag": release.get("tag_name", ""),
                "name": release.get("name") or "",
                "published": (release.get("published_at") or "")[:10],
                "prerelease": bool(release.get("prerelease")),
                "draft": bool(release.get("draft")),
                "assets": assets,
                "downloads": sum(a["downloads"] for a in assets),
            }
        )
    rows.sort(key=lambda r: (r["published"], r["tag"]), reverse=True)
    return rows


def totals_by_asset(rows: list[dict[str, Any]]) -> "OrderedDict[str, dict[str, int]]":
    """Downloads per asset name summed over the releases, most downloaded first."""
    totals: dict[str, dict[str, int]] = {}
    for row in rows:
        for asset in row["assets"]:
            entry = totals.setdefault(asset["name"], {"downloads": 0, "releases": 0})
            entry["downloads"] += asset["downloads"]
            entry["releases"] += 1
    ordered = sorted(totals.items(), key=lambda item: (-item[1]["downloads"], item[0]))
    return OrderedDict(ordered)


def _flag(row: dict[str, Any]) -> str:
    if row["draft"]:
        return "draft"
    if row["prerelease"]:
        return "pre-release"
    return ""


def render_table(rows: list[dict[str, Any]], with_assets: bool = False) -> str:
    """The human-readable report."""
    if not rows:
        return "No releases.\n"
    tag_width = max(len("Release"), *(len(r["tag"]) for r in rows))
    lines = [f"{'Release':<{tag_width}}  {'Published':<10}  {'Assets':>6}  {'Downloads':>9}  "]
    for row in rows:
        lines.append(
            f"{row['tag']:<{tag_width}}  {row['published']:<10}  {len(row['assets']):>6}  "
            f"{row['downloads']:>9,}  {_flag(row)}"
        )
        if with_assets:
            for asset in sorted(row["assets"], key=lambda a: (-a["downloads"], a["name"])):
                lines.append(f"    {asset['downloads']:>9,}  {asset['name']}")
    total = sum(r["downloads"] for r in rows)
    lines.append("")
    lines.append(f"{len(rows)} releases, {total:,} downloads")
    return "\n".join(line.rstrip() for line in lines) + "\n"


def render_by_asset(rows: list[dict[str, Any]]) -> str:
    totals = totals_by_asset(rows)
    if not totals:
        return "No assets.\n"
    lines = [f"{'Downloads':>9}  {'Releases':>8}  Asset"]
    for name, entry in totals.items():
        lines.append(f"{entry['downloads']:>9,}  {entry['releases']:>8}  {name}")
    total = sum(e["downloads"] for e in totals.values())
    lines.append("")
    lines.append(f"{len(totals)} asset names, {total:,} downloads")
    return "\n".join(lines) + "\n"


def render_traffic(traffic: dict[str, Any]) -> str:
    """Clones and views side by side, one line per day, then the window's totals."""
    by_day: dict[str, dict[str, int]] = {}
    for kind in ("clones", "views"):
        for day in traffic[kind].get(kind, []):
            entry = by_day.setdefault(day["timestamp"][:10], {})
            entry[kind] = int(day.get("count", 0))
            entry[f"unique_{kind}"] = int(day.get("uniques", 0))
    lines = [f"{'Day':<10}  {'Clones':>6}  {'Unique':>6}  {'Views':>6}  {'Unique':>6}"]
    for day in sorted(by_day):
        e = by_day[day]
        lines.append(
            f"{day:<10}  {e.get('clones', 0):>6,}  {e.get('unique_clones', 0):>6,}  "
            f"{e.get('views', 0):>6,}  {e.get('unique_views', 0):>6,}"
        )
    lines.append("")
    lines.append(
        f"Last 14 days: {traffic['clones'].get('count', 0):,} clones "
        f"({traffic['clones'].get('uniques', 0):,} unique cloners), "
        f"{traffic['views'].get('count', 0):,} views "
        f"({traffic['views'].get('uniques', 0):,} unique visitors)"
    )
    return "\n".join(lines) + "\n"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--repo", default=DEFAULT_REPO, help=f"owner/name (default {DEFAULT_REPO})")
    parser.add_argument("--stable", action="store_true", help="skip pre-releases and drafts")
    view = parser.add_mutually_exclusive_group()
    view.add_argument("--assets", action="store_true", help="list each release's assets under it")
    view.add_argument("--by-asset", action="store_true", help="total each asset name across releases")
    view.add_argument("--json", action="store_true", help="print the figures as JSON")
    view.add_argument("--traffic", action="store_true",
                      help="clones and views of the last 14 days (needs a token with push access)")
    args = parser.parse_args(argv)
    token = _token()

    if args.traffic:
        if not token:
            sys.exit("Error: --traffic needs GITHUB_TOKEN or GH_TOKEN with push access to the repository")
        try:
            traffic = fetch_traffic(args.repo, token)
        except Exception as error:  # report the HTTP status, not a stack
            sys.exit(f"Error: reading the traffic of {args.repo} failed: {error}\n"
                     "The traffic API answers only a token with push access to the repository.")
        sys.stdout.write(render_traffic(traffic))
        return 0

    try:
        releases = fetch_releases(args.repo, token)
    except Exception as error:  # report the HTTP status, not a stack
        sys.exit(f"Error: listing releases of {args.repo} failed: {error}")
    rows = summarize(releases, stable_only=args.stable)

    if args.json:
        json.dump({"repo": args.repo, "releases": rows}, sys.stdout, indent=2)
        sys.stdout.write("\n")
    elif args.by_asset:
        sys.stdout.write(render_by_asset(rows))
    else:
        sys.stdout.write(render_table(rows, with_assets=args.assets))
    return 0


if __name__ == "__main__":
    sys.exit(main())
