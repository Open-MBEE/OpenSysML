"""Count the compliance map's rule rows when the site is built, not in git.

docs/project/spec-compliance.md carries a `<!-- doc-counts:begin census -->` …
`<!-- doc-counts:end census -->` block instead of literal counts, so a pull request
that adds a row never rewrites a shared line. This hook replaces the block with the
census it counts from the page's own rows; `tools/census/doccounts` counts rows the same way.
"""

import logging

log = logging.getLogger("mkdocs.hooks.census")

PAGE = "project/spec-compliance.md"
BEGIN = "<!-- doc-counts:begin census -->\n"
END = "<!-- doc-counts:end census -->\n"
# '⚠' without its variation selector: the map writes both spellings.
MARKERS = ("✅", "⚠", "❌", "⛔", "🚧")
UNREFEREED = "**No external referee:**"


def is_rule_row(line: str) -> bool:
    """A table row carrying exactly one status marker; notes naming several are prose."""
    text = line.strip()
    if not text.startswith("|"):
        return False
    return sum(cell.count(m) for cell in text.strip("|").split("|") for m in MARKERS) == 1


def count_rules(markdown: str) -> dict[str, int]:
    counts = dict.fromkeys(MARKERS, 0)
    counts["self-assessed"] = 0
    unrefereed = False
    for line in markdown.splitlines():
        text = line.strip()
        if text.startswith("#"):
            unrefereed = False
        elif text.startswith(UNREFEREED):
            unrefereed = True
        elif is_rule_row(text):
            counts[next(m for m in MARKERS if m in text)] += 1
            if unrefereed:
                counts["self-assessed"] += 1
    counts["total"] = sum(counts[m] for m in MARKERS)
    return counts


def census(markdown: str) -> str:
    """Return the page with its census block replaced by the counted sentence."""
    lines = markdown.splitlines(keepends=True)
    begins = [i for i, line in enumerate(lines) if line == BEGIN]
    ends = [i for i, line in enumerate(lines) if line == END]
    if len(begins) != 1 or len(ends) != 1:
        raise ValueError(f"{PAGE}: exactly one census begin and one end marker line required")
    start, end = begins[0], ends[0]
    if end < start:
        raise ValueError(f"{PAGE}: census end marker precedes its begin marker")
    c = count_rules(markdown)
    if c["total"] == 0:
        raise ValueError(f"{PAGE}: no rule rows to count")
    if c["🚧"]:
        raise ValueError(f"{PAGE}: {c['🚧']} 🚧 rows; give them a status the census states")
    sentence = (
        f"The map below tracks {c['total']} semantic rules: **{c['✅']} ✅ faithful, "
        f"{c['⚠']} ⚠️ approximate, {c['❌']} ❌ not implemented, {c['⛔']} ⛔ deliberate divergence**; "
        f"{c['self-assessed']} of them have no external referee.\n"
    )
    return "".join(lines[:start]) + sentence + "".join(lines[end + 1 :])


def on_page_markdown(markdown: str, page, **_kwargs) -> str:
    if page.file.src_uri != PAGE:
        return markdown
    try:
        return census(markdown)
    except ValueError as e:
        log.warning("%s", e)
        return markdown
