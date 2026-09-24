#!/usr/bin/env python3
"""Check that every relative Markdown link in the repository resolves.

A link to a file must name a file that exists; a link with a `#fragment` must name a
heading that exists in that file (or an explicit anchor). External links, mailto: and
in-page-only fragments pointing at a heading of the same file are checked too. A page
named in prose as `docs/….md` must exist as well, outside the historical plan notes. What a
fenced code block shows is a sample rather than structure, so none of it is read either way.

Run from the repository root:

    python3 scripts/check-doc-links.py
"""

import re
import subprocess
import sys
from pathlib import Path

FENCE = re.compile(r"^ {0,3}(`{3,}|~{3,})")
# The destination is either angle-bracketed (CommonMark, may contain spaces and
# backslash-escaped characters) or bare.
LINK = re.compile(r"\[[^\]]*\]\((<(?:\\.|[^<>\\])*>|[^)\s]+)(?:\s+\"[^\"]*\")?\)")
HEADING = re.compile(r"^#{1,6}\s(.*)$", re.MULTILINE)
ANCHOR = re.compile(r"<a\s+(?:id|name)=[\"']([^\"']+)[\"']", re.IGNORECASE)
# A link naming any scheme points outside the tree, so this checker leaves it alone.
SKIP_PREFIX = tuple(f"{scheme}://" for scheme in ("http", "https", "ftp")) + ("mailto:", "tel:")
# A page named in prose as `docs/…md` rather than linked, which a move breaks just as silently.
CITED = re.compile(r"`(docs/[A-Za-z0-9_./-]+\.md)`")
# Records of what a plan created at the time; their paths are history, not pointers.
HISTORICAL = ("docs/internals/notes/", "docs/internals/design/")
# A changelog fragment is folded into CHANGELOG.md, so its links are written from the root.
FRAGMENTS = "changes/unreleased/"


def prose(text: str) -> str:
    """The text outside fenced code blocks, whose links and `#` lines are samples, not structure."""
    kept: list[str] = []
    fence: str | None = None
    for line in text.splitlines():
        opening = FENCE.match(line)
        if fence is None:
            if opening:
                fence = opening.group(1)
            else:
                kept.append(line)
            continue
        if opening and opening.group(1)[0] == fence[0] and len(opening.group(1)) >= len(fence):
            fence = None
    return "\n".join(kept)


def slugs(heading: str) -> set[str]:
    """GitHub's heading slug: lowercased, punctuation dropped, a dash per space.

    Dropped punctuation leaves a run of dashes; the collapsed spelling is accepted too.
    """
    text = re.sub(r"`|\*|_|<[^>]+>", "", heading)
    text = re.sub(r"\[([^\]]*)\]\([^)]*\)", r"\1", text)
    text = text.strip().lower()
    text = re.sub(r"[^\w\s-]", "", text, flags=re.UNICODE)
    each = re.sub(r"\s", "-", text)
    return {s for s in (each, re.sub(r"-+", "-", each)) if s}


def anchors_of(path: Path) -> set[str]:
    text = prose(path.read_text(encoding="utf-8", errors="replace"))
    found: set[str] = set()
    seen: dict[str, int] = {}
    for line in HEADING.findall(text):
        # A heading may be closed with trailing #s, which are not part of its text.
        heading = line.strip().rstrip("#").strip()
        for slug in slugs(heading):
            # GitHub disambiguates a repeated heading with -1, -2, … in document order.
            found.add(slug if slug not in seen else f"{slug}-{seen[slug]}")
            seen[slug] = seen.get(slug, 0) + 1
    # Fragments are compared lowercased, so an explicit anchor's case must not matter either.
    found.update(anchor.lower() for anchor in ANCHOR.findall(text))
    return found


def tracked_markdown() -> list[Path]:
    out = subprocess.run(
        ["git", "ls-files", "*.md"], capture_output=True, text=True, check=True
    ).stdout
    return [Path(line) for line in out.splitlines() if line]


def link_failure(md: Path, link: str, root: Path, anchors: dict[Path, set[str]]) -> str | None:
    """What is wrong with one link, or None if it resolves."""
    target, _, fragment = link.partition("#")
    if target:
        base = root if md.as_posix().startswith(FRAGMENTS) else md.parent
        dest = (base / target).resolve()
        if not dest.is_relative_to(root):
            return f"{md}: escapes the repository: {link}"
        if not dest.exists():
            return f"{md}: no such file: {link}"
    else:
        dest = md.resolve()
    if not fragment:
        return None
    if dest.is_dir():
        dest = dest / "README.md"
        if not dest.exists():
            return f"{md}: no README.md for the fragment in {link}"
    if dest.suffix != ".md":
        return None
    if dest not in anchors:
        anchors[dest] = anchors_of(dest)
    if fragment.lower() not in anchors[dest]:
        return f"{md}: no heading '#{fragment}' in {target or md.name}"
    return None


def page_failures(md: Path, root: Path, anchors: dict[Path, set[str]]) -> list[str]:
    """Everything one page points at that is not there."""
    text = prose(md.read_text(encoding="utf-8", errors="replace"))
    failures: list[str] = []
    for raw in LINK.findall(text):
        link = raw.strip()
        # CommonMark allows the destination in angle brackets: [text](<dest>).
        if link.startswith("<") and link.endswith(">"):
            # A backslash escapes only ASCII punctuation; elsewhere it is literal.
            link = re.sub(r"\\([!-/:-@\[-`{-~])", r"\1", link[1:-1])
        if not link or link.startswith(SKIP_PREFIX):
            continue
        failure = link_failure(md, link, root, anchors)
        if failure:
            failures.append(failure)
    if md.as_posix().startswith(HISTORICAL):
        return failures
    failures.extend(
        f"{md}: no such file, cited in prose: {cited}"
        for cited in sorted(set(CITED.findall(text)))
        if not (root / cited).exists()
    )
    return failures


def main() -> int:
    root = Path(".").resolve()
    anchors: dict[Path, set[str]] = {}
    failures: list[str] = []

    for md in tracked_markdown():
        failures.extend(page_failures(md, root, anchors))

    for failure in sorted(failures):
        print(failure)
    print(f"{len(failures)} broken link(s)")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
