"""Stamp versioned release assets in the documentation site."""

import logging
import re
import subprocess
from pathlib import Path

log = logging.getLogger("mkdocs.hooks.release_version")

ROOT = Path(__file__).resolve().parents[1]
VERSION_FILE = ROOT / "client" / "python" / "opensysml" / "_version.py"
TOKEN = "{{release_version}}"
STABLE_TAG = re.compile(r"^v(\d+)\.(\d+)\.(\d+)$")
PYTHON_VERSION = re.compile(r'^VERSION\s*=\s*["\']([^"\']+)["\']\s*$', re.MULTILINE)


def _latest_stable_version(tags: list[str]) -> str | None:
    versions = []
    for tag in tags:
        match = STABLE_TAG.fullmatch(tag.strip())
        if match:
            parts = tuple(map(int, match.groups()))
            versions.append((parts, ".".join(match.groups())))
    return max(versions, default=(None, None))[1]


def _fallback_version() -> str:
    match = PYTHON_VERSION.search(VERSION_FILE.read_text(encoding="utf-8"))
    if not match:
        raise RuntimeError(f"VERSION was not found in {VERSION_FILE}")
    return match.group(1)


def on_config(config):
    try:
        result = subprocess.run(
            ["git", "tag", "--list", "v*"],
            cwd=ROOT,
            check=True,
            capture_output=True,
            text=True,
        )
        version = _latest_stable_version(result.stdout.splitlines())
    except (OSError, subprocess.CalledProcessError):
        version = None
        reason = "Git release tags are unavailable"
    else:
        reason = "No stable release tags were found"

    if version is None:
        version = _fallback_version()
        log.warning("%s; using Python package VERSION %s", reason, version)

    config["release_version"] = version
    return config


def on_page_markdown(markdown: str, page, config, **_kwargs) -> str:
    markdown = markdown.replace(TOKEN, config["release_version"])
    if TOKEN in markdown:
        log.warning("Unreplaced %s token on %s", TOKEN, page.file.src_uri)
    return markdown
