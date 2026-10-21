"""Count the compliance map's rule rows when the site is built, not in git.

docs/project/spec-compliance.md carries a `<!-- doc-counts:begin census -->` …
`<!-- doc-counts:end census -->` block instead of literal counts, so a pull request
that adds a row never rewrites a shared line. This hook replaces the block with the
census it counts from the page's own rows; `tools/census/doccounts` counts rows the same way.

A rule row's status is the one marker in its table's `Status` column; markers the other
columns mention are prose. A table with no `Status` column holds no rule rows.
"""

import logging
import re
from typing import Iterator, Optional

log = logging.getLogger("mkdocs.hooks.census")

PAGE = "project/spec-compliance.md"
BEGIN = "<!-- doc-counts:begin census -->\n"
END = "<!-- doc-counts:end census -->\n"
# '⚠' without its variation selector: the map writes both spellings.
MARKERS = ("✅", "🧩", "⚠", "❌", "⛔", "🚧")
UNREFEREED = "**No external referee:**"
STATUS_HEADING = "Status"

# Python-Markdown's tables extension: pipes inside a code span or escaped do not split.
_CODE_PIPES = re.compile(r"(?:(\\\\)|(\\`+)|(`+)|(\\\|)|(\|))")
_SEPARATOR_CHARS = set("|:- ")


def _strip_end_border(row: str) -> str:
    """Drop a closing pipe with the escaped backslashes before it, as Python-Markdown's RE_END_BORDER does."""
    if not row.endswith("|"):
        return row
    body = row[:-1]
    run = len(body) - len(body.rstrip("\\"))
    return body[: len(body) - run] if run % 2 == 0 else row


def split_cells(row: str) -> list[str]:
    """Split a table row into cells as the site's Markdown renderer does."""
    row = row.strip(" ")
    if row.startswith("|"):
        row = row[1:]
    row = _strip_end_border(row)
    pipes: list[int] = []
    tics: list[int] = []
    tic_points: list[tuple[int, int, int]] = []
    for m in _CODE_PIPES.finditer(row):
        if m.group(2):
            tics.append(len(m.group(2)) - 1)
            tic_points.append((m.start(2), m.end(2) - 1, 1))
        elif m.group(3):
            tics.append(len(m.group(3)))
            tic_points.append((m.start(3), m.end(3) - 1, 0))
        elif m.group(5):
            pipes.append(m.start(5))
    regions: list[tuple[int, int]] = []
    pos = 0
    while pos < len(tics):
        size = tics[pos] - tic_points[pos][2]
        try:
            if size == 0:
                raise ValueError
            index = tics[pos + 1 :].index(size) + 1
        except ValueError:
            pos += 1
            continue
        regions.append((tic_points[pos][0], tic_points[pos + index][1]))
        pos += index + 1
    cells: list[str] = []
    start = 0
    for pipe in pipes:
        if any(begin <= pipe <= end for begin, end in regions):
            continue
        cells.append(row[start:pipe])
        start = pipe + 1
    cells.append(row[start:])
    return [cell.strip() for cell in cells]


def status_column(block: list[str]) -> Optional[int]:
    """The index of a table's `Status` column; -1 for a table without one, None for a non-table."""
    if len(block) < 2 or not block[0].strip(" ").startswith("|"):
        return None
    header, separator = split_cells(block[0]), split_cells(block[1])
    if len(header) < 2 or len(separator) != len(header) or not set("".join(separator)) <= _SEPARATOR_CHARS:
        return None
    columns = [i for i, cell in enumerate(header) if cell.strip("* ") == STATUS_HEADING]
    return columns[0] if columns else -1


def row_status(number: int, line: str, column: int) -> str:
    cells = split_cells(line)
    cell = cells[column] if column < len(cells) else ""
    found = [m for m in MARKERS for _ in range(cell.count(m))]
    if not found:
        raise ValueError(f"{PAGE}:{number}: no status marker in the {STATUS_HEADING} column")
    if len(found) > 1:
        raise ValueError(f"{PAGE}:{number}: {len(found)} status markers in the {STATUS_HEADING} column ({', '.join(found)}); state one")
    return found[0]


def rule_rows(markdown: str) -> Iterator[tuple[int, str, bool]]:
    """Yield (line number, status marker, self-assessed) per rule row; a malformed row is a ValueError."""
    lines = markdown.splitlines()
    unrefereed = False
    start = 0
    while start < len(lines):
        end = start
        while end < len(lines) and lines[end].strip():
            end += 1
        block = lines[start:end]
        column = status_column(block)
        for offset, line in enumerate(block):
            number, text = start + offset + 1, line.strip()
            if column is None:
                if text.startswith("#"):
                    unrefereed = False
                elif text.startswith(UNREFEREED):
                    unrefereed = True
                elif text.startswith("|") and any(m in text for m in MARKERS):
                    raise ValueError(f"{PAGE}:{number}: a row with a status marker outside a table")
            elif column < 0:
                if offset >= 2 and any(m in text for m in MARKERS):
                    raise ValueError(f"{PAGE}:{number}: a row with a status marker in a table without a {STATUS_HEADING} column")
            elif offset >= 2:
                yield number, row_status(number, line, column), unrefereed
        start = max(end, start + 1)


def count_rules(markdown: str) -> dict[str, int]:
    counts = dict.fromkeys(MARKERS, 0)
    counts["self-assessed"] = 0
    for _, marker, unrefereed in rule_rows(markdown):
        counts[marker] += 1
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
        f"{c['🧩']} 🧩 extension (UML-derived or OpenSysML, not a SysML v2 claim), "
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
