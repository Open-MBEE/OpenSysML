#!/usr/bin/env python3
"""Fail when reader-facing documentation cites an internal work-item label.

Waves, follow-up rows (F<n>), adjudication probes (P<n>) and diagnostic classes
(K<n>/S<n>) are this project's own bookkeeping: they have no public referent, so
a reader cannot resolve them. The deep conformance records under docs/project/
may use them — they define them and cross-reference each other by them — but the
handbook, the reference, the architecture pages and the changelog must say what
the behaviour or the decision was instead.

Run from the repository root:

    python3 scripts/check-doc-ids.py
"""

from __future__ import annotations

import pathlib
import re
import sys

# Documentation a user reads. docs/project/ is deliberately absent: those pages
# are engineering records that define the labels they use. So are the pages that
# state this rule, CONTRIBUTING.md and AGENTS.md, which have to quote examples.
# The archived changelog releases are the one page under docs/project/ a user reads.
READER_FACING = [
    "CHANGELOG.md",
    "docs/project/archive/CHANGELOG-0.0-0.8.md",
    "changes/unreleased",
    "README.md",
    "docs/index.md",
    "docs/guide",
    "docs/reference",
    "docs/internals",
    "docs/tutorials",
]

# Each pattern is (regex, what to write instead).
PATTERNS = [
    (re.compile(r"\bwaves?[\s-]+\d", re.IGNORECASE), "name the change, not the development round"),
    # A slice label is W<round><letter>, the shape a standards body's name also has:
    # the reference pages say W3C for RDF and OSLC, and mean the consortium.
    (re.compile(r"\bW(?!3C\b)\d{1,2}[A-Z]\b"), "name the change, not the development round"),
    (re.compile(r"\bF\d{2,3}\b"), "describe the follow-up, do not cite its row number"),
    (re.compile(r"\bP\d\b(?!\d)"), "describe the adjudication, do not cite its probe number"),
    (re.compile(r"\b[KS]\d{1,2}\b(?!\d)"), "describe the diagnostic, do not cite its class"),
]

# Keyboard shortcuts are genuine user-facing names and are spelled with <kbd>.
KBD = re.compile(r"<kbd>[^<]*</kbd>")


def offences(path: pathlib.Path) -> list[tuple[int, str, str]]:
    found: list[tuple[int, str, str]] = []
    for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        stripped = KBD.sub("", line)
        for pattern, remedy in PATTERNS:
            match = pattern.search(stripped)
            if match:
                found.append((number, match.group(0), remedy))
    return found


def targets() -> list[pathlib.Path]:
    paths: list[pathlib.Path] = []
    for entry in READER_FACING:
        target = pathlib.Path(entry)
        if target.is_dir():
            paths.extend(sorted(target.rglob("*.md")))
        elif target.is_file():
            paths.append(target)
    return paths


def main() -> int:
    total = 0
    for path in targets():
        for number, label, remedy in offences(path):
            print(f"{path}:{number}: internal label {label!r} in reader-facing docs — {remedy}")
            total += 1
    if total:
        print(f"\n{total} internal label(s) in reader-facing documentation")
        print("PR bodies and release notes carry the same rule; see AGENTS.md §7.")
        return 1
    print("No internal work-item labels in reader-facing documentation")
    return 0


if __name__ == "__main__":
    sys.exit(main())
