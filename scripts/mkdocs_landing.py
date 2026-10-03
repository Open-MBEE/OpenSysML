"""Render docs/README.md with the landing page template, and check the templates' links.

Selecting the template here rather than in front matter keeps the page plain Markdown
on GitHub; the site-relative links every template in overrides/ holds, which MkDocs
cannot validate, are checked against the built file set so a renamed page fails
`--strict` like any link.

A template also links documentation the site does not publish, the engineering records
under docs/project/ among them. `record('project/x.md', base_url)` resolves such a path
the way scripts/mkdocs_repo_links.py resolves it in Markdown: to the page when the site
publishes it, and to the file on GitHub when it does not.
"""

import logging
import re
from pathlib import Path

from mkdocs.utils import normalize_url

log = logging.getLogger("mkdocs.hooks.landing")

LANDING = "README.md"
TEMPLATE = "home.html"

# `{{ 'guide/01-install/'|url }}` — the site-relative targets the template links to.
URL_FILTER = re.compile(r"""\{\{\s*['"]([^'"]+)['"]\s*\|\s*url\s*\}\}""")

# `{{ record('project/pilot-xpect.md', base_url) }}` — a docs/ path, published or not.
RECORD = re.compile(r"""\{\{\s*record\(\s*['"]([^'"]+)['"]""")


def _source_candidates(target: str) -> list[str]:
    """The docs/ paths that would publish at `target`: a page under use_directory_urls,
    or a file such as an asset that publishes as itself."""
    path = target.strip("/")
    if not path:
        return ["index.md", LANDING]
    return [path, f"{path}.md", f"{path}/index.md", f"{path}/{LANDING}"]


def on_files(files, config):
    overrides = Path(config["theme"].custom_dir or "")
    if not (overrides / TEMPLATE).is_file():
        log.warning("the landing page template %s is missing", overrides / TEMPLATE)
        return files

    # A page excluded from the site stays in `files`, so publication is what is asked:
    # a template link to an excluded page would 404 exactly like a renamed one.
    published = {file.src_uri for file in files if file.inclusion.is_included()}
    docs_dir = Path(config["docs_dir"])
    for template in sorted(overrides.glob("**/*.html")):
        text = template.read_text()
        name = template.relative_to(overrides)
        for target in dict.fromkeys(URL_FILTER.findall(text)):
            if not any(candidate in published for candidate in _source_candidates(target)):
                log.warning("%s links to %r, which no page publishes", name, target)
        for target in dict.fromkeys(RECORD.findall(text)):
            if not (docs_dir / target).is_file():
                log.warning("%s links to the record %r, which does not exist", name, target)
    return files


def on_env(env, config, files):
    repo_url = (config.get("repo_url") or "").rstrip("/")

    def record(path: str, base_url: str = "") -> str:
        file = files.get_file_from_path(path)
        if file and file.inclusion.is_included():
            return normalize_url(file.url, base=base_url)
        return f"{repo_url}/blob/main/{Path(config['docs_dir']).name}/{path}"

    env.globals["record"] = record
    return env


def on_page_markdown(markdown: str, page, **_kwargs) -> str:
    if page.file.src_uri == LANDING:
        # The hero wants the full width, and the map below it is its own contents list.
        page.meta["template"] = TEMPLATE
        page.meta["hide"] = ["navigation", "toc"]
    return markdown
