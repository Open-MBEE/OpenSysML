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

The table lives in client/release-digests.json, and `--write` syncs it into
every client that ships a copy (scripts/sync-release-digests.py).

With `--from-manifest` or `--from-binaries`, stamping the shared table also
syncs client copies. An explicit `--table` for a package-local table stays
isolated.

Two families of binaries are pinned: the `sysml-grpc-*` service the clients
start, and the `sysml-jupyter-kernel-*` kernel jupyter-opensysml-kernel
installs. A published release carries the service family and, from the release
that introduced the kernel, the kernel family too; a family is pinned whole or
not at all, so a release missing one platform of either is refused. A directory
of binaries is stamped by the families it holds: the service job is handed
dist/grpc, the kernel job dist/jupyter.

A release job can stamp the table a package ships without a GitHub token or
network access, from what the release has already produced. The Rust job
stamps its crate from the checksum manifest:

    python scripts/pin_release_checksums.py --version v0.9.1 \\
        --from-manifest dist/SHA256SUMS.txt \\
        --table client/rust/opensysml/release-digests.json

The Python jobs build their wheels before that manifest exists (the manifest
lists the wheels), so each stamps from the binaries themselves, hashed here:

    python scripts/pin_release_checksums.py --version v0.9.1 \\
        --from-binaries dist/grpc \\
        --table client/python/opensysml/release-digests.json
    python scripts/pin_release_checksums.py --version v0.9.1 \\
        --from-binaries dist/jupyter \\
        --table client/jupyter-kernel/jupyter_opensysml_kernel/release-digests.json

Both require every platform of each family they find, and a `.sha256` sidecar
beside a binary must agree with the digest hashed from it.
"""

import argparse
import hashlib
import importlib.util
import json
import os
import re
import sys
import urllib.error
import urllib.request

REPO_ROOT = os.path.dirname(
    os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
)
DIGESTS_FILE = os.path.join(REPO_ROOT, "client", "release-digests.json")
SYNC_SCRIPT = os.path.join(REPO_ROOT, "scripts", "sync-release-digests.py")
DEFAULT_REPO = "Open-MBEE/OpenSysML"
#: The platforms every binary family is released for.
PLATFORMS = ("darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64.exe")
#: The service binaries every release publishes; a package pin needs all of them.
SERVICE_PREFIX = "sysml-grpc-"
#: The Jupyter kernel binaries, published from the release that introduced them.
KERNEL_PREFIX = "sysml-jupyter-kernel-"
SIDECAR_SUFFIX = ".sha256"
SERVICE_ASSETS = frozenset(SERVICE_PREFIX + platform for platform in PLATFORMS)
KERNEL_ASSETS = frozenset(KERNEL_PREFIX + platform for platform in PLATFORMS)
#: Every pinned family, by the prefix its assets share; each is pinned whole.
ASSET_FAMILIES = {SERVICE_PREFIX: SERVICE_ASSETS, KERNEL_PREFIX: KERNEL_ASSETS}
ASSET_PREFIXES = tuple(ASSET_FAMILIES)
SHA256_PATTERN = re.compile(r"[0-9a-f]{64}\Z")
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
        digests_file (str, optional): Path to client/release-digests.json

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
        dict: asset name -> browser download URL, pinned binaries only

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
        if asset["name"].startswith(ASSET_PREFIXES) and not asset["name"].endswith(SIDECAR_SUFFIX)
    }
    if not assets:
        raise PinError(f"release {version} of {repo} publishes no {SERVICE_PREFIX}* assets")
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
        with urllib.request.urlopen(url + SIDECAR_SUFFIX, timeout=NETWORK_TIMEOUT) as response:
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


def manifest_service_digests(manifest_path):
    """The binary digests a release's checksum manifest lists.

    Args:
        manifest_path (str): Path to the release's SHA256SUMS.txt

    Returns:
        dict: asset name -> SHA-256 hex digest, every platform of each family listed

    Raises:
        PinError: If the manifest cannot be read, lists a malformed or duplicate
            entry, lists no binary, or lacks a platform of a family it lists
    """
    digests = {}
    try:
        with open(manifest_path, encoding="utf-8") as manifest:
            lines = manifest.readlines()
    except (OSError, UnicodeError) as e:
        raise PinError(f"cannot read checksum manifest {manifest_path}: {e}")

    for line_number, line in enumerate(lines, start=1):
        fields = line.split()
        if not fields:
            continue
        asset = fields[1] if len(fields) > 1 else fields[0]
        if asset.endswith(SIDECAR_SUFFIX) or not asset.startswith(ASSET_PREFIXES):
            continue
        if len(fields) != 2:
            raise PinError(
                f"malformed checksum entry for {asset} on line {line_number} "
                f"of {manifest_path}"
            )
        digest, asset = fields
        if SHA256_PATTERN.fullmatch(digest) is None:
            raise PinError(
                f"malformed SHA-256 digest for {asset} on line {line_number} "
                f"of {manifest_path}"
            )
        if asset in digests:
            raise PinError(f"duplicate asset {asset} in {manifest_path}")
        digests[asset] = digest

    _require_whole_families(digests, f"checksum manifest {manifest_path}", required=(SERVICE_PREFIX,))
    return digests


def binary_service_digests(binaries_dir):
    """Hash the binaries a release job has built, before any manifest exists.

    The manifest that lists them is written after the Python distributions they
    are stamped into, so a Python job hashes the binaries it was handed. A
    `.sha256` sidecar beside a binary is compared with the digest, never used
    as one, exactly as with a published release.

    Args:
        binaries_dir (str): Directory holding the sysml-grpc-* or
            sysml-jupyter-kernel-* binaries

    The service job is handed dist/grpc and the kernel job dist/jupyter, so the
    directory is stamped by the families it holds, each whole.

    Returns:
        dict: asset name -> SHA-256 hex digest, every platform of each family found

    Raises:
        PinError: If the directory cannot be read, a binary cannot be hashed, a
            sidecar disagrees with its binary, no binary is found, or a platform
            of a family found is absent
    """
    try:
        names = sorted(os.listdir(binaries_dir))
    except OSError as e:
        raise PinError(f"cannot read the binaries in {binaries_dir}: {e}")

    digests = {}
    for asset in names:
        if asset.endswith(SIDECAR_SUFFIX) or not asset.startswith(ASSET_PREFIXES):
            continue
        path = os.path.join(binaries_dir, asset)
        if not os.path.isfile(path):
            continue
        digests[asset] = _file_digest(path)
        sidecar = _sidecar_digest(path + SIDECAR_SUFFIX)
        if sidecar is not None and sidecar != digests[asset]:
            raise PinError(
                f"{asset} in {binaries_dir} hashes to {digests[asset]}, but its "
                f".sha256 says {sidecar}; the build is inconsistent and was not pinned"
            )
    if not digests:
        raise PinError(
            f"no {' or '.join(prefix + '*' for prefix in ASSET_PREFIXES)} binaries in {binaries_dir}"
        )
    _require_whole_families(digests, f"binaries in {binaries_dir}")
    return digests


def _file_digest(path):
    """The SHA-256 of a file, hashed as it is read.

    Args:
        path (str): File to hash

    Returns:
        str: SHA-256 hex digest

    Raises:
        PinError: If the file cannot be read
    """
    digest = hashlib.sha256()
    try:
        with open(path, "rb") as f:
            for chunk in iter(lambda: f.read(1024 * 256), b""):
                digest.update(chunk)
    except OSError as e:
        raise PinError(f"cannot hash {path}: {e}")
    return digest.hexdigest()


def _sidecar_digest(path):
    """The digest a `.sha256` sidecar file records, if there is one.

    Args:
        path (str): Path of the sidecar

    Returns:
        str or None: The digest, or None when there is no sidecar

    Raises:
        PinError: If there is a sidecar and it does not hold a SHA-256 digest
    """
    try:
        with open(path, encoding="utf-8") as f:
            fields = f.read().split()
    except FileNotFoundError:
        return None
    except (OSError, UnicodeError) as e:
        raise PinError(f"cannot read the checksum sidecar {path}: {e}")
    if not fields or SHA256_PATTERN.fullmatch(fields[0]) is None:
        raise PinError(f"malformed SHA-256 digest in the checksum sidecar {path}")
    return fields[0]


def _require_whole_families(digests, source, required=()):
    """Fail unless every family found is complete, and every family required is found.

    A package verifies the one platform it runs on, so a family pinned for
    some platforms would install on those and refuse the rest for the same
    release; a family is pinned whole or not at all.

    Args:
        digests (dict): asset name -> digest
        source (str): Where the digests came from, for the message
        required (tuple): Asset prefixes of the families the source must hold

    Raises:
        PinError: Naming the assets that are absent
    """
    missing = []
    for prefix, family in ASSET_FAMILIES.items():
        found = {asset for asset in digests if asset.startswith(prefix)}
        if found or prefix in required:
            missing.extend(sorted(family - found))
    if missing:
        raise PinError(f"{source} is missing assets: {', '.join(missing)}")


def stamp_from_manifest(manifest_path, version, repo=DEFAULT_REPO, table_path=None):
    """Add one release's service digests from its already-produced checksum manifest.

    Args:
        manifest_path (str): Path to the release's SHA256SUMS.txt
        version (str): Release tag
        repo (str): GitHub repository (owner/repo)
        table_path (str, optional): Digest table to update; defaults to DIGESTS_FILE

    Returns:
        bool: Whether a new pin was written

    Raises:
        PinError: If the manifest is invalid or an existing pin conflicts
    """
    return stamp(manifest_service_digests(manifest_path), version, repo, table_path)


def stamp_from_binaries(binaries_dir, version, repo=DEFAULT_REPO, table_path=None):
    """Add one release's service digests, hashed from the binaries it built.

    Args:
        binaries_dir (str): Directory holding the sysml-grpc-* binaries
        version (str): Release tag
        repo (str): GitHub repository (owner/repo)
        table_path (str, optional): Digest table to update; defaults to DIGESTS_FILE

    Returns:
        bool: Whether a new pin was written

    Raises:
        PinError: If the binaries are incomplete or inconsistent, or an existing
            pin conflicts
    """
    return stamp(binary_service_digests(binaries_dir), version, repo, table_path)


def stamp(digests, version, repo=DEFAULT_REPO, table_path=None):
    """Add one release's service digests to a table, refusing to change a pin.

    Stamping DIGESTS_FILE syncs the client copies; an explicit alternate table
    (the one a package ships, in a release job) is updated alone.

    Args:
        digests (dict): asset name -> SHA-256 hex digest, every service asset
        version (str): Release tag
        repo (str): GitHub repository (owner/repo)
        table_path (str, optional): Digest table to update; defaults to DIGESTS_FILE

    Returns:
        bool: Whether a new pin was written; False when the same pin is there

    Raises:
        PinError: If the table cannot be read or written, or holds a different
            pin for the release already
    """
    table_path = table_path or DIGESTS_FILE
    sync_shared_table = os.path.realpath(table_path) == os.path.realpath(DIGESTS_FILE)
    if sync_shared_table:
        table_path = DIGESTS_FILE
    table = pinned_table(table_path)
    versions = table.get(repo, {})
    if version in versions:
        if versions[version] == digests:
            return False
        raise PinError(
            f"a different digest pin already exists for {version} of {repo}; "
            "refusing to replace it"
        )

    table.setdefault(repo, {})[version] = digests
    try:
        with open(table_path, "w", encoding="utf-8") as output:
            output.write(render_table(table))
    except OSError as e:
        raise PinError(f"cannot write digest table {table_path}: {e}")
    if sync_shared_table:
        sync_clients(table_path)
    return True


def write_table(table, digests_file=None):
    """Store a table, and sync it into every client that ships a copy.

    Args:
        table (dict): repo -> version -> asset -> digest
        digests_file (str, optional): Path to client/release-digests.json

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
        "--from-manifest",
        metavar="PATH",
        help="stamp the binary digests an existing checksum manifest lists",
    )
    parser.add_argument(
        "--from-binaries",
        metavar="DIR",
        help="stamp the digests hashed from the built binaries in DIR",
    )
    parser.add_argument(
        "--table",
        default=DIGESTS_FILE,
        help=(
            "table to stamp; the shared source syncs client copies, "
            "other tables stay isolated"
        ),
    )
    parser.add_argument(
        "--write",
        action="store_true",
        help="rewrite client/release-digests.json instead of printing the table",
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="re-hash the assets of the pinned releases and fail on a mismatch",
    )
    args = parser.parse_args(argv)

    stamping = args.from_manifest or args.from_binaries
    if args.from_manifest and args.from_binaries:
        parser.error("--from-manifest and --from-binaries are alternatives")
    if stamping:
        if args.check or args.write:
            parser.error("--from-manifest/--from-binaries cannot be combined with --check or --write")
        if not args.version:
            parser.error("--version is required with --from-manifest/--from-binaries")
    elif args.table != DIGESTS_FILE:
        parser.error("--table can only be used with --from-manifest or --from-binaries")

    try:
        if stamping:
            if args.from_manifest:
                digests = manifest_service_digests(args.from_manifest)
            else:
                digests = binary_service_digests(args.from_binaries)
            for asset, digest in sorted(digests.items()):
                print(f"{asset} {digest}", file=sys.stderr)
            changed = stamp(digests, args.version, args.repo, args.table)
            action = "stamped" if changed else "already has"
            print(f"{action} {args.version} of {args.repo} in {args.table or DIGESTS_FILE}")
            return 0

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
