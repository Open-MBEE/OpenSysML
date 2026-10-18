#!/usr/bin/env python3
"""Fail when a page quotes an oracle figure without saying which round it measured.

The oracle totals move whenever a rule, a fixture or the pin does, and the only
statement of the current ones is the ``doc-counts`` generated block, regenerated
from the committed baselines. Every other mention is a snapshot of the round that
wrote it, so a page carrying one has to say so — otherwise a reader takes a figure
from an engineering record as current, which is the drift this check exists to
prevent.

A page passes when either no figure outside a generated block is quoted, or the
disclaimer phrase is present. Run via ``make docs-check``.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

# A quoted oracle total: one of the three oracles' buckets beside a number. The
# differential names "only ours"/"only the pilot's"/"fully agreeing", the Xpect
# suite "agree"/"wording-only"/"disagree", the rejection corpus "both reject" —
# each written either "27 only ours" or, in a table cell, "only ours | 27".
BUCKETS = (
    r"only\s+(?:the\s+)?pilot(?:'s)?|only\s+ours|only\s+us|only\s+we"
    r"|fully\s+agreeing|both\s+reject|both\s+accept"
    r"|wording-only|location-only|disagree(?:ing)?|agree(?:ing)?"
)
NUMBER = r"\*{0,2}\d+\*{0,2}"
FIGURE = re.compile(
    rf"(?:{NUMBER}\s+(?:{BUCKETS})|(?:{BUCKETS})\s*[|:]?\s*{NUMBER})",
    re.IGNORECASE,
)

DISCLAIMER = "not the current baseline"

BLOCK_BEGIN = "doc-counts:begin"
BLOCK_END = "doc-counts:end"

# Both changelogs date every figure by the release heading it sits under, and the
# agent skills tell their reader to read the number out of the baseline JSON.
EXEMPT = {"CHANGELOG.md", "docs/project/archive/CHANGELOG-0.0-0.8.md"}
EXEMPT_DIRS = {".agents"}


def scanned_files() -> list[Path]:
    files = [ROOT / "README.md"]
    files += sorted((ROOT / "docs").rglob("*.md"))
    return [f for f in files if f.is_file()]


def normalized(text: str) -> str:
    """Drop blockquote markers, emphasis and line breaks so a wrapped disclaimer still matches."""
    lines = (re.sub(r"^\s*>+\s*", "", line) for line in text.splitlines())
    return " ".join(" ".join(lines).replace("*", "").replace("_", "").split())


def offending_lines(text: str) -> list[tuple[int, str]]:
    out: list[tuple[int, str]] = []
    generated = False
    for number, line in enumerate(text.splitlines(), start=1):
        if BLOCK_BEGIN in line:
            generated = True
            continue
        if BLOCK_END in line:
            generated = False
            continue
        if generated:
            continue
        if FIGURE.search(line):
            out.append((number, line.strip()))
    return out


def main() -> int:
    failures: list[str] = []
    for path in scanned_files():
        relative = path.relative_to(ROOT)
        if str(relative) in EXEMPT or relative.parts[0] in EXEMPT_DIRS:
            continue
        text = path.read_text(encoding="utf-8")
        quoted = offending_lines(text)
        if not quoted:
            continue
        if DISCLAIMER in normalized(text):
            continue
        shown = "\n".join(f"    line {n}: {line[:120]}" for n, line in quoted[:5])
        more = "" if len(quoted) <= 5 else f"\n    … and {len(quoted) - 5} more"
        failures.append(
            f"{relative}: quotes an oracle figure but never says which round it measured.\n"
            f"{shown}{more}\n"
            f'    Add the phrase "{DISCLAIMER}" where the page states its figures, or move '
            f"them into a doc-counts generated block."
        )

    if failures:
        print("check-doc-figures: undated oracle figures\n", file=sys.stderr)
        for failure in failures:
            print(failure + "\n", file=sys.stderr)
        return 1

    print("check-doc-figures: every quoted oracle figure names the round it measured")
    return 0


if __name__ == "__main__":
    sys.exit(main())
