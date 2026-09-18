#!/usr/bin/env python3
"""Pin the SHA-256 digest of every asset of a release into the clients.

The download check is otherwise same-origin: the .sha256 served beside a binary
comes from whoever served the binary, so a republished release would be trusted.
A digest committed here is independent of the serving origin.

Run once per release of the service binaries, after the release is published and
its assets are final:

    export GITHUB_TOKEN=...   # must be able to read the repository's releases
    python scripts/pin_release_checksums.py --version v0.0.8 --write
    git commit -am 'chore(clients): pin release digests for v0.0.8'

Each asset is downloaded and hashed here; the sidecar is only compared against
that digest, never used as one. `--check` re-hashes the assets of the versions
already pinned and fails on any disagreement, so a republished release is caught
without changing the table.

The table lives in clients/release-digests.json, and `--write` syncs it into
every client that ships a copy (scripts/sync-release-digests.py).
"""

import argparse
import hashlib
import importlib.util
import json
import os
import sys
import urllib.error
import urllib.request

REPO_ROOT = os.path.dirname(
    os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
)
DIGESTS_FILE = os.path.join(REPO_ROOT, "clients", "release-digests.json")
SYNC_SCRIPT = os.path.join(REPO_ROOT, "scripts", "sync-release-digests.py")
DEFAULT_REPO = "Open-MBEE/OpenSysML"
ASSET_PREFIX = "sysml-grpc-"
NETWORK_TIMEOUT = 60


def _load_sync_script():
    """Load scripts/sync-release-digests.py, which is tooling, not a module.

    Returns:
        module: The loaded script
    """
    spec = importlib.util.spec_from_file_location("sync_release_digests", SYNC_SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


sync_script = _load_sync_script()


class PinError(Exception):
    """A release whose digests cannot be pinned as published."""


class MissingTokenError(PinError):
    """No GitHub token in the environment, so no release can be read."""


#: Env vars a token is read from, in order; the release workflow sets GITHUB_TOKEN.
TOKEN_ENV_VARS = ("GITHUB_TOKEN", "GH_TOKEN")


def github_token():
    """The token release API calls are authenticated with.

    Returns:
        str: The token

    Raises:
        MissingTokenError: If no token is set in the environment
    """
    for name in TOKEN_ENV_VARS:
        token = os.environ.get(name)
        if token:
            return token
    raise MissingTokenError(
        f"no GitHub token: set ${TOKEN_ENV_VARS[0]} (or ${TOKEN_ENV_VARS[1]}) to a "
        f"token that can read this repository's releases \u2014 the 'public_repo' "
        f"scope for a classic token, or 'Contents: read' for a fine-grained one. "
        f"Unauthenticated calls to the releases API are rate-limited per address "
        f"and fail as an opaque HTTP 403; see 'Pinned release digests' in "
        f"docs/project/releasing.md."
    )


def pinned_table(digests_file=None):
    """The digests the clients currently pin.

    Args:
        digests_file (str, optional): Path to clients/release-digests.json

    Returns:
        dict: repo -> version -> asset -> digest

    Raises:
        PinError: If the table cannot be read
    """
    digests_file = digests_file or DIGESTS_FILE
    try:
        with open(digests_file, encoding="utf-8") as f:
            return json.load(f)
    except (OSError, json.JSONDecodeError) as e:
        raise PinError(f"cannot read the pinned digests from {digests_file}: {e}")


def release_assets(repo, version):
    """The downloadable assets of a release, by asset name.

    Args:
        repo (str): GitHub repository (owner/repo)
        version (str): Release tag, resolved (never 'latest')

    Returns:
        dict: asset name -> browser download URL, service binaries only

    Raises:
        MissingTokenError: If no GitHub token is set in the environment
        PinError: If the release cannot be read or publishes no binaries
    """
    url = f"https://api.github.com/repos/{repo}/releases/tags/{version}"
    request = urllib.request.Request(url, headers={"Accept": "application/vnd.github+json"})
    request.add_header("Authorization", f"Bearer {github_token()}")
    try:
        with urllib.request.urlopen(request, timeout=NETWORK_TIMEOUT) as response:
            release = json.load(response)
    except (urllib.error.URLError, json.JSONDecodeError) as e:
        raise PinError(f"cannot read release {version} of {repo}: {e}")

    assets = {
        asset["name"]: asset["browser_download_url"]
        for asset in release.get("assets", [])
        if asset["name"].startswith(ASSET_PREFIX) and not asset["name"].endswith(".sha256")
    }
    if not assets:
        raise PinError(f"release {version} of {repo} publishes no {ASSET_PREFIX}* assets")
    return assets


def download_digest(url):
    """The SHA-256 of what a URL serves, hashed as it is streamed.

    Args:
        url (str): Asset URL

    Returns:
        str: SHA-256 hex digest

    Raises:
        PinError: If the asset cannot be downloaded
    """
    digest = hashlib.sha256()
    try:
        with urllib.request.urlopen(url, timeout=NETWORK_TIMEOUT) as response:
            for chunk in iter(lambda: response.read(1024 * 256), b""):
                digest.update(chunk)
    except urllib.error.URLError as e:
        raise PinError(f"cannot download {url}: {e}")
    return digest.hexdigest()


def served_digest(url):
    """The digest the .sha256 sidecar serves for an asset, if it serves one.

    Args:
        url (str): Asset URL

    Returns:
        str or None: The digest read from the sidecar, or None when absent
    """
    try:
        with urllib.request.urlopen(url + ".sha256", timeout=NETWORK_TIMEOUT) as response:
            return response.read().decode().split()[0].strip()
    except (urllib.error.URLError, IndexError, UnicodeDecodeError):
        return None


def digests_of(repo, version):
    """Hash every asset of a release, reporting a sidecar that disagrees.

    Args:
        repo (str): GitHub repository (owner/repo)
        version (str): Release tag

    Returns:
        dict: asset name -> SHA-256 hex digest

    Raises:
        PinError: If a sidecar contradicts the asset it describes, which means
            the release is inconsistent and must not be pinned as published
    """
    digests = {}
    for asset, url in sorted(release_assets(repo, version).items()):
        digest = download_digest(url)
        sidecar = served_digest(url)
        if sidecar is not None and sidecar != digest:
            raise PinError(
                f"{asset} of {version} hashes to {digest}, but its .sha256 serves "
                f"{sidecar}; the release is inconsistent and was not pinned"
            )
        print(f"{asset} {digest}", file=sys.stderr)
        digests[asset] = digest
    return digests


def render_table(table):
    """The table as it is stored, sorted so diffs stay readable.

    Args:
        table (dict): repo -> version -> asset -> digest

    Returns:
        str: The JSON document, ending in a newline
    """
    return json.dumps(table, indent=2, sort_keys=True) + "\n"


def write_table(table, digests_file=None):
    """Store a table, and sync it into every client that ships a copy.

    Args:
        table (dict): repo -> version -> asset -> digest
        digests_file (str, optional): Path to clients/release-digests.json

    Raises:
        PinError: If the clients' copies cannot be rewritten
    """
    digests_file = digests_file or DIGESTS_FILE
    with open(digests_file, "w", encoding="utf-8") as f:
        f.write(render_table(table))
    sync_clients(digests_file)


def sync_clients(digests_file=None):
    """Rewrite each client's shipped copy of the table.

    A client verifies a download against the copy it publishes, so a table
    written without this leaves every client pinning what it pinned before.

    Args:
        digests_file (str, optional): Path to the table to sync from
    """
    sync_script.sync(source=digests_file or DIGESTS_FILE)


def check(table):
    """Re-hash the assets of every pinned release and report disagreements.

    Args:
        table (dict): repo -> version -> asset -> digest

    Returns:
        list[str]: One message per asset that no longer hashes to its pin
    """
    problems = []
    for repo in sorted(table):
        for version in sorted(table[repo]):
            published = digests_of(repo, version)
            for asset, pinned in sorted(table[repo][version].items()):
                actual = published.get(asset)
                if actual is None:
                    problems.append(f"{asset} of {version} of {repo} is no longer published")
                elif actual != pinned:
                    problems.append(
                        f"{asset} of {version} of {repo} now hashes to {actual}, "
                        f"but {pinned} is pinned: the release was republished"
                    )
            for asset in sorted(set(published) - set(table[repo][version])):
                problems.append(f"{asset} of {version} of {repo} is published but unpinned")
    return problems


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", help="release tag to pin, e.g. v0.0.8")
    parser.add_argument("--repo", default=DEFAULT_REPO, help="GitHub repository (owner/repo)")
    parser.add_argument(
        "--write",
        action="store_true",
        help="rewrite clients/release-digests.json instead of printing the table",
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="re-hash the assets of the pinned releases and fail on a mismatch",
    )
    args = parser.parse_args(argv)

    try:
        table = pinned_table()
        if args.check:
            problems = check(table)
            for problem in problems:
                print(f"error: {problem}", file=sys.stderr)
            return 1 if problems else 0
        if not args.version:
            parser.error("--version is required unless --check is given")
        table.setdefault(args.repo, {})[args.version] = digests_of(args.repo, args.version)
        if args.write:
            write_table(table)
            print(f"pinned {args.version} of {args.repo} in {DIGESTS_FILE}")
        else:
            print(render_table(table), end="")
    except PinError as e:
        print(f"error: {e}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
