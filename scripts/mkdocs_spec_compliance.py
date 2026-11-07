"""MkDocs hook: serve docs/project/spec-compliance.md combined from its sources.

The compliance map is kept as per-area sources under docs/project/spec-compliance/
(see scripts/spec_compliance.py) and the combined page is not committed on develop,
so the site builds it here as a generated page at the path the page has always had.
A committed copy, which a release carries, is replaced by the same text so the
site never shows a stale page.
"""

from __future__ import annotations

import importlib.util
import logging
import pathlib

from mkdocs.structure.files import File

PAGE = "project/spec-compliance.md"

HERE = pathlib.Path(__file__).resolve().parent
_spec = importlib.util.spec_from_file_location("spec_compliance", HERE / "spec_compliance.py")
spec_compliance = importlib.util.module_from_spec(_spec)
assert _spec.loader is not None
_spec.loader.exec_module(spec_compliance)

log = logging.getLogger("mkdocs.hooks.spec_compliance")


def on_files(files, config, **_kwargs):
    try:
        content = spec_compliance.render()
    except spec_compliance.SourceError as e:
        log.warning("%s", e)
        return files
    existing = files.get_file_from_path(PAGE)
    if existing is not None:
        files.remove(existing)
    files.append(File.generated(config, PAGE, content=content))
    return files
