"""Publish the security policy where users look: the Security page and security.txt.

SECURITY.md at the repository root is the one statement of how to report a vulnerability,
what to expect and which versions are supported. docs/guide/security.md carries a
`<!-- security-policy:begin -->` … `<!-- security-policy:end -->` block that, read on
GitHub, points at that file; when the site is built the block is replaced with the file's
sections, so the policy is never hand-copied into the page.

The same build writes RFC 9116's `security.txt` at `/.well-known/security.txt` and
`/security.txt`, with the contacts and the policy URL read from SECURITY.md. `Expires`
is a year after the commit the site is built from (`SOURCE_DATE_EPOCH` when set), so a
site republished from a moving branch never serves a lapsed file, and the same commit
builds the same file.
"""

import datetime as _dt
import logging
import os
import posixpath
import re
import subprocess
from pathlib import Path

from mkdocs.structure.files import File, InclusionLevel

log = logging.getLogger("mkdocs.hooks.security")

POLICY = "SECURITY.md"
PAGE = "guide/security.md"
BEGIN = "<!-- security-policy:begin -->\n"
END = "<!-- security-policy:end -->\n"
SECURITY_TXT = ("security.txt", ".well-known/security.txt")
# RFC 9116 § 2.5.5: the field is RECOMMENDED to be less than a year into the future.
VALIDITY = _dt.timedelta(days=365)

TITLE = re.compile(r"^# ")
# Inline links and images; the policy uses no reference-style links.
LINK = re.compile(r"(!?\[[^\]]*\]\()([^)\s]+)((?:\s+\"[^\"]*\")?\))")
SCHEME = re.compile(r"^[A-Za-z][A-Za-z0-9+.-]*:")
REPORT_URL = re.compile(r"\((https://github\.com/[^)\s]+/security/advisories/new)\)")
MAILTO = re.compile(r"<([^<>@\s]+@[^<>@\s]+)>")


def policy_sections(text: str, page_dir: str = "docs/guide") -> str:
    """SECURITY.md without its title, its links re-rooted to the page that embeds it.

    The page has a title of its own, so the file's `# Security policy` is dropped and its
    `##` sections become sections of the page. A link SECURITY.md writes from the repository
    root (`docs/project/releasing.md`) is rewritten relative to `page_dir`, where the
    repo-links hook then decides whether it is a page of the site or a file on GitHub.
    """
    lines = [line for line in text.splitlines(keepends=True) if not TITLE.match(line)]

    def reroot(match: re.Match) -> str:
        target = match.group(2)
        if target.startswith(("#", "/")) or SCHEME.match(target):
            return match.group(0)
        path, _, fragment = target.partition("#")
        rel = posixpath.relpath(path, page_dir)
        return f"{match.group(1)}{rel}{'#' + fragment if fragment else ''}{match.group(3)}"

    return LINK.sub(reroot, "".join(lines).lstrip("\n"))


def contacts(text: str) -> list[str]:
    """The reporting channels SECURITY.md names, the private report form first."""
    found = [m.group(1) for m in REPORT_URL.finditer(text)]
    found += [f"mailto:{m.group(1)}" for m in MAILTO.finditer(text)]
    if len(found) < 2:
        raise ValueError(f"{POLICY} must name the private report form and an e-mail address")
    return list(dict.fromkeys(found))


def build_time(root: Path) -> _dt.datetime:
    """When the tree being built was committed: SOURCE_DATE_EPOCH, else HEAD's commit time,
    else now (a tree that is not a checkout is dated by the clock)."""
    epoch = os.environ.get("SOURCE_DATE_EPOCH")
    if epoch:
        return _dt.datetime.fromtimestamp(int(epoch), _dt.timezone.utc)
    try:
        out = subprocess.run(
            ["git", "-C", str(root), "log", "-1", "--format=%ct"],
            check=True,
            capture_output=True,
            text=True,
        ).stdout.strip()
        return _dt.datetime.fromtimestamp(int(out), _dt.timezone.utc)
    except (subprocess.CalledProcessError, FileNotFoundError, ValueError):
        return _dt.datetime.now(_dt.timezone.utc)


def security_txt(policy: str, site_url: str, repo_url: str, built: _dt.datetime) -> str:
    """The RFC 9116 file: every field once, `Expires` a year after the build."""
    expires = (built + VALIDITY).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    lines = [f"Contact: {contact}" for contact in contacts(policy)]
    lines += [
        f"Expires: {expires}",
        f"Policy: {repo_url}/security/policy",
        f"Canonical: {site_url}.well-known/security.txt",
        "Preferred-Languages: en",
        f"Acknowledgments: {repo_url}/security/advisories",
    ]
    return "\n".join(lines) + "\n"


def on_files(files, config):
    root = Path(config.config_file_path).resolve().parent
    policy = (root / POLICY).read_text(encoding="utf-8")
    site_url = (config.get("site_url") or "").rstrip("/") + "/"
    repo_url = (config.get("repo_url") or "").rstrip("/")
    if not config.get("site_url") or not repo_url:
        raise ValueError("security.txt needs site_url and repo_url in mkdocs.yml")
    content = security_txt(policy, site_url, repo_url, build_time(root))
    for uri in SECURITY_TXT:
        # Generated, not read from docs/: MkDocs skips dot-directories there.
        files.append(File.generated(config, uri, content=content, inclusion=InclusionLevel.INCLUDED))
    return files


def on_page_markdown(markdown: str, page, config, files) -> str:
    if page.file.src_uri != PAGE:
        return markdown
    begin, end = markdown.find(BEGIN), markdown.find(END)
    if begin < 0 or end < begin:
        log.warning("%s has no %s … %s block to embed %s into", PAGE, BEGIN.strip(), END.strip(), POLICY)
        return markdown
    root = Path(config.config_file_path).resolve().parent
    policy = (root / POLICY).read_text(encoding="utf-8")
    return markdown[:begin] + policy_sections(policy) + markdown[end + len(END):]
