"""Fetching the kernel binary of a release, verified against a pinned digest.

A release publishes `sysml-jupyter-kernel-<os>-<arch>[.exe]` for each supported
platform beside a `.sha256` sidecar. The sidecar comes from whoever served the
binary, so it is only a consistency check; what the download is held to is the
digest this package shipped with for the release it was built against, which
is independent of the serving origin. Nothing unpinned is ever installed.
"""

from __future__ import annotations

import hashlib
import json
import os
import platform
import re
import tempfile
import urllib.error
import urllib.request

from ._version import VERSION

DEFAULT_GITHUB_REPO = "Open-MBEE/OpenSysML"
GITHUB_REPO_ENV = "OPENSYSML_GITHUB_REPO"

#: The kernel binary, as every release publishes and installs it.
BINARY_NAME = "sysml-jupyter-kernel"
ASSET_PREFIX = BINARY_NAME + "-"
SIDECAR_SUFFIX = ".sha256"

#: The moving alias of the newest nightly snapshot; per-night releases are tagged
#: `nightly-<yyyymmdd>-<commit>`, and a snapshot of this package pins its own.
SNAPSHOT_TAG = "nightly"

NETWORK_TIMEOUT = 30
#: Bounds on what is read from the network, so a misbehaving origin cannot fill memory.
MAX_BINARY_BYTES = 512 * 1024 * 1024
MAX_SIDECAR_BYTES = 4096

PINNED_DIGESTS_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "release-digests.json")

_SHA256 = re.compile(r"[0-9a-f]{64}\Z")
_VERSION_FORM = re.compile(
    r"^(?P<release>\d+\.\d+\.\d+)(?:(?P<phase>a|b|rc)(?P<number>\d+))?(?:\.dev(?P<snapshot>\d+))?$"
)
_SEMVER_PHASE = {"a": "alpha", "b": "beta", "rc": "rc"}


class KernelBinaryError(Exception):
    """The kernel binary could not be obtained."""


class UnsupportedPlatformError(KernelBinaryError):
    """No release publishes a kernel for this operating system or architecture."""


class UnpinnedReleaseError(KernelBinaryError):
    """This package pins no digest for the asset, so it cannot be verified."""


class ChecksumMismatchError(KernelBinaryError):
    """What was downloaded does not hash to the pinned digest."""


class DownloadError(KernelBinaryError):
    """The release could not be reached, or served something malformed."""


def _load_pinned_digests() -> dict[str, dict[str, dict[str, str]]]:
    with open(PINNED_DIGESTS_FILE, encoding="utf-8") as f:
        table = json.load(f)
    if not isinstance(table, dict):
        raise KernelBinaryError(f"{PINNED_DIGESTS_FILE} is not a digest table")
    return table


#: repo -> release tag -> asset name -> SHA-256 hex digest.
PINNED_SHA256 = _load_pinned_digests()


def default_github_repo() -> str:
    """The repository releases are downloaded from: $OPENSYSML_GITHUB_REPO or the default."""
    return os.environ.get(GITHUB_REPO_ENV) or DEFAULT_GITHUB_REPO


def detect_platform() -> tuple[str, str]:
    """This machine as the (GOOS, GOARCH) pair a release names its builds by.

    Raises:
        UnsupportedPlatformError: If no release is published for the machine
    """
    system = platform.system().lower()
    goos = {"linux": "linux", "darwin": "darwin", "windows": "windows"}.get(system)
    if goos is None:
        raise UnsupportedPlatformError(
            f"unsupported operating system {platform.system()!r}: releases are built for "
            "Linux, macOS and Windows; `go build ./cmd/sysml-jupyter-kernel` builds one "
            "here, to install with --binary"
        )
    machine = platform.machine().lower()
    goarch = {"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(machine)
    if goarch is None or (goos == "windows" and goarch != "amd64"):
        raise UnsupportedPlatformError(
            f"unsupported architecture {platform.machine()!r} on {goos}: releases are built "
            "for amd64 and arm64 (amd64 alone on Windows); `go build ./cmd/sysml-jupyter-kernel` "
            "builds one here, to install with --binary"
        )
    return goos, goarch


def binary_name(goos: str | None = None) -> str:
    """The file the kernel is installed as: `sysml-jupyter-kernel`, `.exe` on Windows."""
    if goos is None:
        goos = platform.system().lower()
    return BINARY_NAME + ".exe" if goos == "windows" else BINARY_NAME


def release_asset_name(goos: str | None = None, goarch: str | None = None) -> str:
    """The asset a release publishes the kernel for a platform under."""
    if goos is None or goarch is None:
        detected_os, detected_arch = detect_platform()
        goos = goos or detected_os
        goarch = goarch or detected_arch
    name = f"{ASSET_PREFIX}{goos}-{goarch}"
    return name + ".exe" if goos == "windows" else name


def built_against_releases(version: str | None = None, github_repo: str | None = None) -> tuple[str, ...]:
    """The release tags a version of this package (by default, this one) was built against.

    A release version names its tag; a pre-release both spellings the tag may
    use; a snapshot (`.dev<yyyymmdd>`) the per-night releases its table pins for
    that date, else the moving alias.
    """
    if version is None:
        version = VERSION
    match = _VERSION_FORM.fullmatch(version)
    if match is None:
        return (f"v{version}",)
    if match["snapshot"] is not None:
        repo = github_repo or default_github_repo()
        prefix = f"{SNAPSHOT_TAG}-{match['snapshot']}-"
        pinned = tuple(sorted(tag for tag in PINNED_SHA256.get(repo, {}) if tag.startswith(prefix)))
        return pinned or (SNAPSHOT_TAG,)
    if match["phase"] is None:
        return (f"v{match['release']}",)
    suffix = _SEMVER_PHASE[match["phase"]]
    return (
        f"v{match['release']}-{suffix}{match['number']}",
        f"v{match['release']}-{suffix}.{match['number']}",
    )


def pinned_digest(version: str, asset: str, github_repo: str | None = None) -> str | None:
    """The digest this package pins for an asset of a release, or None."""
    repo = github_repo or default_github_repo()
    return PINNED_SHA256.get(repo, {}).get(version, {}).get(asset)


def pinned_release(asset: str | None = None, github_repo: str | None = None) -> str:
    """The release this package installs by default: the one it was built
    against that pins the asset for this machine.

    Raises:
        UnpinnedReleaseError: If none of them pins it
    """
    asset = asset or release_asset_name()
    candidates = built_against_releases(github_repo=github_repo)
    for tag in candidates:
        if pinned_digest(tag, asset, github_repo) is not None:
            return tag
    raise UnpinnedReleaseError(
        f"jupyter-opensysml-kernel {VERSION} pins no digest for {asset} of "
        f"{' or '.join(candidates)}, so it cannot verify a download of it. "
        "Pass --version with a release this package pins, or --binary with a "
        "sysml-jupyter-kernel built or downloaded and verified by hand."
    )


def release_download_url(version: str, asset: str, github_repo: str | None = None) -> str:
    """The URL a release publishes an asset at."""
    repo = github_repo or default_github_repo()
    return f"https://github.com/{repo}/releases/download/{version}/{asset}"


def fetch(url: str, limit: int) -> bytes:
    """What a URL serves, up to a size bound.

    Raises:
        DownloadError: If the URL cannot be read or serves more than the bound
    """
    try:
        with urllib.request.urlopen(url, timeout=NETWORK_TIMEOUT) as response:
            data: bytes = response.read(limit + 1)
    except (urllib.error.URLError, OSError, ValueError) as e:
        raise DownloadError(f"cannot download {url}: {e}") from e
    if len(data) > limit:
        raise DownloadError(f"{url} serves more than the {limit} bytes allowed for it")
    return data


def sidecar_digest(url: str) -> str:
    """The digest a `.sha256` sidecar serves, in `sha256sum` form or bare.

    Raises:
        DownloadError: If the sidecar is missing or malformed
    """
    text = fetch(url, MAX_SIDECAR_BYTES).decode("ascii", errors="replace")
    fields = text.split()
    if not fields or _SHA256.fullmatch(fields[0]) is None:
        raise DownloadError(f"{url} does not serve a SHA-256 digest")
    return fields[0]


def download_binary(
    dest_dir: str,
    version: str | None = None,
    github_repo: str | None = None,
) -> str:
    """Download the kernel for this machine into a directory, verified.

    The download is hashed and compared with the digest this package pins for
    the release before anything is written; the served sidecar must agree too,
    so an inconsistent release is refused rather than trusted on one side.

    Args:
        dest_dir: Directory the binary is written into, under its installed name
        version: Release tag; the package's own pinned release when omitted
        github_repo: GitHub repository (owner/repo)

    Returns:
        The path of the verified binary

    Raises:
        UnpinnedReleaseError: If this package pins no digest for the asset
        ChecksumMismatchError: If the download or the sidecar disagrees with the pin
        DownloadError: If the release cannot be reached
    """
    asset = release_asset_name()
    if version is None:
        version = pinned_release(asset, github_repo)
    expected = pinned_digest(version, asset, github_repo)
    if expected is None:
        raise UnpinnedReleaseError(
            f"jupyter-opensysml-kernel {VERSION} pins no digest for {asset} of {version}, "
            "so it cannot verify a download of it. Install the package released with "
            f"{version}, or pass --binary with a sysml-jupyter-kernel verified by hand."
        )
    url = release_download_url(version, asset, github_repo)
    served = sidecar_digest(url + SIDECAR_SUFFIX)
    if served != expected:
        raise ChecksumMismatchError(
            f"{asset}{SIDECAR_SUFFIX} of {version} serves {served}, but this package pins "
            f"{expected}; the release is not the one this package was built against"
        )
    data = fetch(url, MAX_BINARY_BYTES)
    actual = hashlib.sha256(data).hexdigest()
    if actual != expected:
        raise ChecksumMismatchError(
            f"{asset} of {version} hashes to {actual}, not the pinned {expected}; "
            "the download is corrupt or tampered with, and was not installed"
        )
    return write_binary(data, os.path.join(dest_dir, binary_name()))


def write_binary(data: bytes, path: str) -> str:
    """Write verified bytes to a path atomically, executable."""
    os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
    fd, tmp = tempfile.mkstemp(prefix=".kernel-", dir=os.path.dirname(path) or ".")
    try:
        with os.fdopen(fd, "wb") as f:
            f.write(data)
        os.chmod(tmp, 0o755)
        os.replace(tmp, path)
    except BaseException:
        try:
            os.unlink(tmp)
        except OSError:
            pass
        raise
    return path
