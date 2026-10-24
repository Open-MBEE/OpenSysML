#!/usr/bin/env python3
"""Post the Slack announcement of a released tag.

Usage: scripts/release-announce.py --tag v1.2.3 [--snippet FILE] [--dry-run]

The release workflow's announce-release-slack job runs this after the GitHub
release exists. It reads the release's title and URL from the GitHub API (the
release body stays human-written and can be edited up to the moment of the
post), wraps them in a short Slack message and POSTs it to the incoming webhook
in OPENSYSML_WEBHOOK_URL. The optional --snippet file carries extra body text
("Get it: pip install opensysml …"), appended verbatim so the announcement a
release deserves can be hand-written without touching this script.
The webhook posts to the channel chosen when it was added, under the Slack
app's own name and icon.

Environment:
    OPENSYSML_WEBHOOK_URL   incoming-webhook URL for the announcement channel
    GITHUB_TOKEN/GH_TOKEN/CIRCLE_TOKEN   token for the release API read
    GH_REPO             owner/name, default from CIRCLE_PROJECT_USERNAME and
                        CIRCLE_PROJECT_REPONAME, else Open-MBEE/OpenSysML

--dry-run prints the payload instead of posting; the release pipeline's
rehearsal uses it so a rehearsal exercises everything but the POST.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.request
from typing import Any


def _env(*names: str) -> str:
    for name in names:
        value = os.environ.get(name, "").strip()
        if value:
            return value
    return ""


def _slack_escape(text: str) -> str:
    return text.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def _github_release(repo: str, tag: str, token: str) -> dict[str, Any]:
    url = f"https://api.github.com/repos/{repo}/releases/tags/{tag}"
    request = urllib.request.Request(url)
    request.add_header("Accept", "application/vnd.github+json")
    if token:
        request.add_header("Authorization", f"Bearer {token}")
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            return json.loads(response.read().decode("utf-8"))
    except Exception as error:  # noqa: BLE001 - report the HTTP status, not a stack
        sys.exit(f"Error: reading release {tag} of {repo} failed: {error}")


def build_payload(release_name: str, release_url: str, tag: str,
                  snippet: str) -> dict[str, Any]:
    """The Slack message for one release."""
    title = _slack_escape(release_name.strip() or tag)
    lines = [
        f":tada: *{title}* is out!",
        f"Release notes and downloads: {release_url}",
    ]
    if snippet.strip():
        lines.append("")
        lines.append(snippet.rstrip())
    return {"text": "\n".join(lines)}


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--tag", required=True, help="release tag, e.g. v1.2.3")
    parser.add_argument("--snippet", help="file with extra body lines to append verbatim")
    parser.add_argument("--release-name", help="use this title instead of a GitHub API read")
    parser.add_argument("--release-url", help="use this URL instead of a GitHub API read")
    parser.add_argument("--dry-run", action="store_true",
                        help="print the payload instead of posting it")
    args = parser.parse_args(argv)
    if "-" in args.tag:
        print(f"Not announcing pre-release {args.tag}; only stable releases are announced.")
        return 0

    repo = _env("GH_REPO")
    if not repo:
        owner, name = _env("CIRCLE_PROJECT_USERNAME"), _env("CIRCLE_PROJECT_REPONAME")
        repo = f"{owner}/{name}" if owner and name else "Open-MBEE/OpenSysML"

    snippet = ""
    if args.snippet:
        with open(args.snippet, encoding="utf-8") as handle:
            snippet = handle.read()

    if args.release_name or args.release_url:
        release_name = args.release_name or args.tag
        release_url = args.release_url or f"https://github.com/{repo}/releases/tag/{args.tag}"
    else:
        token = _env("GITHUB_TOKEN", "GH_TOKEN", "CIRCLE_TOKEN")
        release = _github_release(repo, args.tag, token)
        release_name = release.get("name") or args.tag
        release_url = release.get("html_url") or f"https://github.com/{repo}/releases/tag/{args.tag}"

    payload = build_payload(
        release_name=release_name,
        release_url=release_url,
        tag=args.tag,
        snippet=snippet,
    )

    if args.dry_run:
        print(json.dumps(payload, indent=2))
        return 0

    webhook = _env("OPENSYSML_WEBHOOK_URL")
    if not webhook:
        sys.exit("Error: OPENSYSML_WEBHOOK_URL is not set; add the incoming-webhook "
                 "URL to the 'Slack' context (Organization Settings -> Contexts).")
    request = urllib.request.Request(
        webhook,
        data=json.dumps(payload).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            body = response.read().decode("utf-8", errors="replace")
            if body.strip() != "ok":
                sys.exit(f"Error: the Slack webhook answered {response.status} {body!r}")
    except Exception as error:  # noqa: BLE001 - report the HTTP status, not a stack
        sys.exit(f"Error: posting to the Slack webhook failed: {error}")
    print(f"Announced {args.tag} of {repo} to Slack.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
