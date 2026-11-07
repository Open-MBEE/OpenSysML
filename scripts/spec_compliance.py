#!/usr/bin/env python3
"""Keep the combined compliance page out of every pull request.

docs/project/spec-compliance.md is a 2 MB page that almost every feature
branch edits by one line, so it is kept as small per-area sources under
docs/project/spec-compliance/ and combined into the page when the site is
built (scripts/mkdocs_spec_compliance.py) and when a release is prepared:

    docs/project/spec-compliance/<NN>-<area>.md
    docs/project/spec-compliance/<NN>-<section>/<NN>-<area>.md

A source is a file named `<two digits>-<slug>.md`, at the top of the directory
or one level down; the digits fix the order of the combined page, a nested
directory holds one `##` section split by `###` area. Each source starts with a
heading and ends with one newline; the page is the sources in path order with a
blank line between them, so a source holds exactly the text the page shows at
that place. A `README.md` in the directory is not a source.

Run from the repository root:

    python3 scripts/spec_compliance.py check                 # sources well-formed; a present page is current (CI)
    python3 scripts/spec_compliance.py check --forbid-page   # the same, and the page is not tracked (develop)
    python3 scripts/spec_compliance.py check --require-page  # the same, and the page is present (a release)
    python3 scripts/spec_compliance.py render                # write docs/project/spec-compliance.md
    python3 scripts/spec_compliance.py print                 # the combined page on stdout
    python3 scripts/spec_compliance.py sources               # the sources, in page order
    python3 scripts/spec_compliance.py locate <heading>      # the source holding a heading
"""

from __future__ import annotations

import argparse
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
PAGE = ROOT / "docs" / "project" / "spec-compliance.md"
SOURCES = ROOT / "docs" / "project" / "spec-compliance"
SOURCE_NAME = re.compile(r"^[0-9]{2}-[a-z0-9][a-z0-9-]*\.md$")
SECTION_NAME = re.compile(r"^[0-9]{2}-[a-z0-9][a-z0-9-]*$")
HEADING = re.compile(r"^#{1,6} \S")


class SourceError(Exception):
    pass


def _rel(path: pathlib.Path) -> str:
    try:
        return path.relative_to(ROOT).as_posix()
    except ValueError:
        return path.as_posix()


def sources(root: pathlib.Path = SOURCES) -> list[pathlib.Path]:
    """The source files in page order; a stray file in the directory is an error."""
    if not root.is_dir():
        raise SourceError(f"{_rel(root)}: no such directory")
    found: list[pathlib.Path] = []
    for entry in sorted(root.iterdir()):
        if entry.is_dir():
            if not SECTION_NAME.match(entry.name):
                raise SourceError(f"{_rel(entry)}: a section directory is named <NN>-<slug>")
            nested = sorted(entry.iterdir())
            if not nested:
                raise SourceError(f"{_rel(entry)}: an empty section directory")
            for sub in nested:
                if sub.is_dir() or not SOURCE_NAME.match(sub.name):
                    raise SourceError(f"{_rel(sub)}: a source is a file named <NN>-<slug>.md")
                found.append(sub)
        elif entry.name == "README.md":
            continue
        elif not SOURCE_NAME.match(entry.name):
            raise SourceError(f"{_rel(entry)}: a source is a file named <NN>-<slug>.md")
        else:
            found.append(entry)
    if not found:
        raise SourceError(f"{_rel(root)}: no sources")
    return found


def read_source(path: pathlib.Path) -> str:
    """A source's text, checked to hold exactly what the page shows at its place."""
    text = path.read_text(encoding="utf-8")
    if not text.endswith("\n") or text.endswith("\n\n"):
        raise SourceError(f"{_rel(path)}: a source ends with exactly one newline")
    if not HEADING.match(text):
        raise SourceError(f"{_rel(path)}: a source starts with a heading")
    return text


def render(root: pathlib.Path = SOURCES) -> str:
    """The combined page: the sources in order, a blank line between them."""
    return "\n".join(read_source(path) for path in sources(root))


def locate(heading: str, root: pathlib.Path = SOURCES) -> list[pathlib.Path]:
    """The sources whose heading lines contain the text, in page order."""
    pattern = re.compile(r"^#{1,6} .*" + re.escape(heading), re.MULTILINE | re.IGNORECASE)
    return [path for path in sources(root) if pattern.search(read_source(path))]


def tracked(path: pathlib.Path) -> bool:
    out = subprocess.run(
        ["git", "ls-files", "--error-unmatch", "--", str(path)],
        cwd=ROOT, capture_output=True, text=True, check=False,
    )
    return out.returncode == 0


def check(require_page: bool, forbid_page: bool) -> list[str]:
    """What is wrong with the sources and the page, as messages; empty when nothing is."""
    try:
        page = render()
    except SourceError as e:
        return [str(e)]
    problems: list[str] = []
    if forbid_page and tracked(PAGE):
        problems.append(
            f"{_rel(PAGE)} is committed; it is rendered when the site is built and only a release "
            f"commits it — edit the sources under {_rel(SOURCES)}/ and `git rm` the page"
        )
    if PAGE.exists():
        if PAGE.read_text(encoding="utf-8") != page:
            problems.append(f"{_rel(PAGE)} is stale: run `make spec-compliance-render`")
    elif require_page:
        problems.append(f"{_rel(PAGE)} is missing: a release renders it with `make spec-compliance-render`")
    return problems


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="command", required=True)
    checker = sub.add_parser("check", help="verify the sources, and the page where one is present or required")
    policy = checker.add_mutually_exclusive_group()
    policy.add_argument("--require-page", action="store_true", help="the combined page must be present (a release)")
    policy.add_argument("--forbid-page", action="store_true", help="the combined page must not be tracked (develop)")
    sub.add_parser("render", help="write the combined page")
    sub.add_parser("print", help="print the combined page")
    sub.add_parser("sources", help="list the sources in page order")
    locator = sub.add_parser("locate", help="name the source holding a heading")
    locator.add_argument("heading")
    args = parser.parse_args(argv)

    try:
        if args.command == "check":
            problems = check(args.require_page, args.forbid_page)
            for problem in problems:
                print(problem, file=sys.stderr)
            if not problems:
                print(f"spec-compliance: {len(sources())} sources are well-formed")
            return 1 if problems else 0
        if args.command == "render":
            text = render()
            if PAGE.exists() and PAGE.read_text(encoding="utf-8") == text:
                print(f"spec-compliance: {_rel(PAGE)} already current")
            else:
                PAGE.write_text(text, encoding="utf-8")
                print(f"spec-compliance: wrote {_rel(PAGE)}")
            return 0
        if args.command == "print":
            sys.stdout.write(render())
            return 0
        if args.command == "sources":
            for path in sources():
                print(_rel(path))
            return 0
        found = locate(args.heading)
        for path in found:
            print(_rel(path))
        if not found:
            print(f"spec-compliance: no heading matches {args.heading!r}", file=sys.stderr)
        return 0 if found else 1
    except SourceError as e:
        print(str(e), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
