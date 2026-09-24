#!/usr/bin/env python3
"""Keep CHANGELOG.md out of every pull request.

A change describes itself in a fragment under changes/unreleased/ instead of
editing the shared "## Unreleased" section, so two branches that both add an
entry never touch the same lines. The fragments are folded into CHANGELOG.md
when a release is prepared.

    changes/unreleased/<slug>.<section>.md

<slug> is free-form (the branch or topic name); <section> is one of the
Keep a Changelog headings this file uses, lower-cased: added, changed,
deprecated, removed, fixed, security, performance. The body is the entry as it
will appear — one or more list items in the changelog's own style.

Run from the repository root:

    python3 scripts/changelog.py check              # every fragment is well-formed (CI)
    python3 scripts/changelog.py render             # fold fragments into "## Unreleased", delete them
    python3 scripts/changelog.py summary            # one line per unreleased entry, for a snapshot's notes
    python3 scripts/changelog.py release 0.5.0      # render, then date the section as a release
    python3 scripts/changelog.py release 0.5.0 --date 2026-09-10
"""

from __future__ import annotations

import argparse
import datetime as _dt
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
CHANGELOG = ROOT / "CHANGELOG.md"
FRAGMENTS = ROOT / "changes" / "unreleased"

SECTIONS = ["Added", "Changed", "Deprecated", "Removed", "Fixed", "Security", "Performance"]
SECTION_BY_KEY = {s.lower(): s for s in SECTIONS}

FRAGMENT_NAME = re.compile(r"^(?P<slug>[A-Za-z0-9][A-Za-z0-9._-]*)\.(?P<section>[a-z]+)\.md$")
UNRELEASED = re.compile(r"^## Unreleased[ \t]*\n", re.MULTILINE)
VERSION_HEADING = re.compile(r"^## (?!Unreleased)", re.MULTILINE)
SECTION_HEADING = re.compile(r"^### (?P<name>.*)$", re.MULTILINE)
LIST_ITEM = re.compile(r"^- ", re.MULTILINE)
BOLD_LEAD = re.compile(r"\A\*\*(?P<lead>.+?)\*\*", re.DOTALL)
FIRST_SENTENCE = re.compile(r".+?[.!?](?:\s|$)")
# Semantic Versioning 2.0.0, as published at semver.org.
_NUM = r"(?:0|[1-9][0-9]*)"
_PRE_ID = rf"(?:{_NUM}|[0-9]*[A-Za-z-][0-9A-Za-z-]*)"
VERSION = re.compile(
    rf"^{_NUM}\.{_NUM}\.{_NUM}"
    rf"(?:-{_PRE_ID}(?:\.{_PRE_ID})*)?"
    r"(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$"
)


class FragmentError(Exception):
    pass


def _rel(path: pathlib.Path) -> str:
    try:
        return str(path.relative_to(ROOT))
    except ValueError:
        return str(path)


def fragments() -> list[pathlib.Path]:
    if not FRAGMENTS.is_dir():
        return []
    return sorted(p for p in FRAGMENTS.iterdir() if p.is_file() and p.name != "README.md" and not p.name.startswith("."))


def parse_fragment(path: pathlib.Path) -> tuple[str, str]:
    """Return (section, body) for one fragment, or raise FragmentError."""
    m = FRAGMENT_NAME.match(path.name)
    if not m:
        raise FragmentError(f"{_rel(path)}: name must be <slug>.<section>.md")
    key = m.group("section")
    if key not in SECTION_BY_KEY:
        raise FragmentError(
            f"{_rel(path)}: section {key!r} is not one of {', '.join(SECTION_BY_KEY)}"
        )
    body = path.read_text(encoding="utf-8").strip("\n")
    if not body.strip():
        raise FragmentError(f"{_rel(path)}: fragment is empty")
    for line in body.splitlines():
        if line.startswith("#"):
            raise FragmentError(
                f"{_rel(path)}: fragments hold list items only; the heading comes from the file name"
            )
    if not body.lstrip().startswith("- "):
        raise FragmentError(f"{_rel(path)}: body must start with a list item ('- ')")
    return SECTION_BY_KEY[key], body


def check() -> int:
    errors: list[str] = []
    for p in fragments():
        try:
            parse_fragment(p)
        except FragmentError as e:
            errors.append(str(e))
    for e in errors:
        print(e, file=sys.stderr)
    return 1 if errors else 0


def _unreleased_bounds(text: str) -> tuple[int, int]:
    """Offsets of the body of "## Unreleased": after its heading line, up to the next version heading."""
    m = UNRELEASED.search(text)
    if not m:
        raise SystemExit("CHANGELOG.md has no '## Unreleased' heading")
    start = m.end()
    nxt = VERSION_HEADING.search(text, start)
    end = nxt.start() if nxt else len(text)
    return start, end


def _split_sections(body: str) -> tuple[str, list[tuple[str, str]]]:
    """Split an Unreleased body into its preamble and (heading, content) pairs."""
    heads = list(SECTION_HEADING.finditer(body))
    preamble = body[: heads[0].start()] if heads else body
    sections: list[tuple[str, str]] = []
    for i, h in enumerate(heads):
        content_end = heads[i + 1].start() if i + 1 < len(heads) else len(body)
        sections.append((h.group("name").strip(), body[h.end() : content_end]))
    return preamble, sections


def _already_folded(body: str, section: str, entry: str) -> bool:
    """True if `entry` sits whole, on item boundaries, under `section` (a previous render wrote it)."""
    at_boundaries = re.compile(rf"(?:\A|\n\n){re.escape(entry)}(?:\Z|\n\n)")
    for name, content in _split_sections(body)[1]:
        if name == section and at_boundaries.search(content.strip("\n")):
            return True
    return False


def fold(text: str, entries: dict[str, list[str]]) -> str:
    """Return CHANGELOG text with entries appended under their sections in "## Unreleased"."""
    start, end = _unreleased_bounds(text)
    body = text[start:end]
    preamble, sections = _split_sections(body)

    for section in SECTIONS:
        items = entries.get(section)
        if not items:
            continue
        block = "\n\n".join(items)
        names = [n for n, _ in sections]
        if section in names:
            i = names.index(section)
            content = sections[i][1].strip("\n")
            sections[i] = (section, (content + "\n\n" if content else "") + block)
        else:
            # Insert after the last existing section that precedes it canonically.
            rank = SECTIONS.index(section)
            at = 0
            for i, n in enumerate(names):
                if n in SECTIONS and SECTIONS.index(n) < rank:
                    at = i + 1
            sections.insert(at, (section, block))

    out = preamble.rstrip("\n") + "\n\n" if preamble.strip() else "\n"
    for name, content in sections:
        out += f"### {name}\n\n" + content.strip("\n") + "\n\n"
    return text[:start] + out + text[end:]


def _folded(paths: list[pathlib.Path]) -> str:
    """CHANGELOG text with every fragment folded into "## Unreleased", written nowhere."""
    text = CHANGELOG.read_text(encoding="utf-8")
    start, end = _unreleased_bounds(text)
    entries: dict[str, list[str]] = {}
    for p in paths:
        section, body = parse_fragment(p)
        # Already folded by an interrupted run: only the deletion is outstanding.
        if _already_folded(text[start:end], section, body):
            continue
        entries.setdefault(section, []).append(body)
    return fold(text, entries) if entries else text


def render(dry_run: bool = False) -> None:
    paths = fragments()
    if not paths:
        print("no fragments under changes/unreleased/")
        return
    new = _folded(paths)
    if dry_run:
        start, end = _unreleased_bounds(new)
        sys.stdout.write("## Unreleased\n" + new[start:end])
        return
    CHANGELOG.write_text(new, encoding="utf-8")
    for p in paths:
        p.unlink()
    print(f"folded {len(paths)} fragment(s) into CHANGELOG.md")


def _lead(item: str) -> str:
    """The bold sentence an entry opens with, or its first sentence when it has none."""
    m = BOLD_LEAD.match(item)
    if m:
        return " ".join(m.group("lead").split())
    text = " ".join(item.split())
    m = FIRST_SENTENCE.match(text)
    return m.group(0).rstrip() if m else text


def summary() -> None:
    """Print each unreleased entry as one line under its section, fragments included."""
    new = _folded(fragments())
    start, end = _unreleased_bounds(new)
    out = []
    for name, content in _split_sections(new[start:end])[1]:
        items = [i.strip() for i in LIST_ITEM.split(content.strip("\n")) if i.strip()]
        if not items:
            continue
        out.append(f"### {name}\n")
        out.extend(f"- {_lead(item)}" for item in items)
        out.append("")
    sys.stdout.write("\n".join(out))


def release(version: str, date: str | None) -> int:
    version = version.lstrip("v")
    if not VERSION.match(version):
        raise SystemExit(f"version {version!r} is not a semantic version")
    if date is None:
        date = _dt.date.today().isoformat()
    else:
        try:
            date = _dt.date.fromisoformat(date).isoformat()
        except ValueError:
            raise SystemExit(f"date {date!r} is not YYYY-MM-DD") from None
    render()
    text = CHANGELOG.read_text(encoding="utf-8")
    m = UNRELEASED.search(text)
    if not m:
        raise SystemExit("CHANGELOG.md has no '## Unreleased' heading")
    heading = f"## Unreleased\n\n## {version} — {date}\n"
    text = text[: m.start()] + heading + text[m.end() :]
    CHANGELOG.write_text(text, encoding="utf-8")
    print(f"CHANGELOG.md: Unreleased is now {version} — {date}")
    return 0


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    sub.add_parser("check", help="validate every fragment")
    r = sub.add_parser("render", help="fold fragments into CHANGELOG.md and delete them")
    r.add_argument("--dry-run", action="store_true", help="print the resulting Unreleased section, change nothing")
    sub.add_parser("summary", help="print one line per unreleased entry, fragments included; change nothing")
    rel = sub.add_parser("release", help="render, then turn Unreleased into a dated version section")
    rel.add_argument("version")
    rel.add_argument("--date", help="YYYY-MM-DD (default: today)")
    a = ap.parse_args(argv)
    if a.cmd == "check":
        return check()
    if a.cmd == "render":
        render(dry_run=a.dry_run)
        return 0
    if a.cmd == "summary":
        summary()
        return 0
    return release(a.version, a.date)


if __name__ == "__main__":
    sys.exit(main())
