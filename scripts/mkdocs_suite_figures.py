"""Render the generated figures when the site is built, not in git.

The test inventory of the spec-compliance map (docs/project/spec-compliance/) carries inline
`<!-- doc-counts:begin <name> -->` … `<!-- doc-counts:end <name> -->` blocks that
name what is counted and state no figure, so a pull request adding a test or a
fixture never rewrites a shared line; the api-json differential record carries the
same blocks for the figures its baseline states, so a re-recording never rewrites
the page. This hook runs each generator in COMMANDS (`go run -C tools ./cmd/doc-counts
-site-blocks`, which counts the tree the way the gates enumerate it, and
`go run -C tools ./cmd/pilot-apijson-diff -site-blocks`, which reads the committed
baseline) and splices the rendered text into the blocks; `go run -C tools ./cmd/doc-counts
-check` and the referee's tests refuse a figure typed into one.
"""

import json
import logging
import pathlib
import subprocess

log = logging.getLogger("mkdocs.hooks.suite_figures")

ROOT = pathlib.Path(__file__).resolve().parent.parent
COMMANDS = (
    ("go", "run", "-C", "tools", "./cmd/doc-counts", "-site-blocks"),
    ("go", "run", "-C", "tools", "./cmd/pilot-apijson-diff", "-site-blocks"),
)
DOCS_DIR = "docs/"

_rendered: dict[str, dict[str, str]] | None = None


def render_site_blocks(commands=COMMANDS, root=ROOT) -> dict[str, dict[str, str]]:
    """Run every generator and return the rendered blocks by repository-relative page, then name."""
    rendered: dict[str, dict[str, str]] = {}
    for command in commands:
        rendered = merge_rendered(rendered, render_command(command, root), command)
    return rendered


def merge_rendered(rendered, more, command) -> dict[str, dict[str, str]]:
    """Add one generator's blocks to rendered; a block rendered twice is an error."""
    for page, blocks in more.items():
        for name in blocks:
            if name in rendered.get(page, {}):
                raise ValueError(f"{' '.join(command)}: {page}: block {name!r} rendered twice")
        rendered.setdefault(page, {}).update(blocks)
    return rendered


def render_command(command, root) -> dict[str, dict[str, str]]:
    """Run one generator and return the rendered blocks by repository-relative page, then name."""
    try:
        completed = subprocess.run(command, cwd=root, capture_output=True, text=True, check=True)
    except FileNotFoundError as e:
        raise ValueError(f"{command[0]}: not found; the test-suite figures need the Go toolchain") from e
    except subprocess.CalledProcessError as e:
        raise ValueError(f"{' '.join(command)}: {e.stderr.strip() or f'exit status {e.returncode}'}") from e
    try:
        rendered = json.loads(completed.stdout)
    except json.JSONDecodeError as e:
        raise ValueError(f"{' '.join(command)}: not JSON: {e}") from e
    if not isinstance(rendered, dict) or not rendered:
        raise ValueError(f"{' '.join(command)}: no pages rendered")
    for page, blocks in rendered.items():
        if not isinstance(blocks, dict) or not blocks:
            raise ValueError(f"{' '.join(command)}: {page}: no blocks rendered")
        for name, text in blocks.items():
            if not isinstance(text, str) or not text.strip():
                raise ValueError(f"{' '.join(command)}: {page}: block {name!r} rendered empty")
    return rendered


def splice(markdown: str, page: str, blocks: dict[str, str]) -> str:
    """Return the page with each named block replaced by its rendered text, markers dropped."""
    for name, text in blocks.items():
        begin = f"<!-- doc-counts:begin {name} -->"
        end = f"<!-- doc-counts:end {name} -->"
        if markdown.count(begin) != 1 or markdown.count(end) != 1:
            raise ValueError(f"{page}: exactly one block named {name!r} required")
        start, stop = markdown.index(begin), markdown.index(end)
        if stop < start:
            raise ValueError(f"{page}: the block named {name!r} ends before it begins")
        markdown = markdown[:start] + text + markdown[stop + len(end) :]
    return markdown


def on_pre_build(**_kwargs) -> None:
    """Render every generator that succeeds; a failing one loses only its own blocks."""
    global _rendered
    rendered: dict[str, dict[str, str]] = {}
    for command in COMMANDS:
        try:
            rendered = merge_rendered(rendered, render_command(command, ROOT), command)
        except ValueError as e:
            log.warning("%s", e)
    _rendered = rendered or None


def on_page_markdown(markdown: str, page, **_kwargs) -> str:
    if _rendered is None:
        return markdown
    blocks = _rendered.get(DOCS_DIR + page.file.src_uri)
    if not blocks:
        return markdown
    try:
        return splice(markdown, page.file.src_uri, blocks)
    except ValueError as e:
        log.warning("%s", e)
        return markdown
