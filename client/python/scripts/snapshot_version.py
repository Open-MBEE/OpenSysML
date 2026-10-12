#!/usr/bin/env python3
"""Snapshot versions for the nightly build of the clients.

A snapshot of develop is the next release in the making, so it is versioned as a
development build of that release: the release segment of
client/python/opensysml/_version.py, bumped to the next patch once that release
is tagged, with the build date appended. ``.github/workflows/nightly.yml`` runs
this on the commit it builds, with the date and the commit it names the night by:

    core  0.9.3                            the release the snapshot leads to
    pypi  0.9.3.dev20261006                PEP 440 development release
    npm   0.9.3-nightly.20261006.gabc1234  SemVer pre-release
    tag   nightly-20261006-abc1234         the GitHub release the clients pin

Both package versions rank below the release and its pre-releases. pip skips a
development release unless asked for it by exact version or with ``--pre``, and
npm publishes a snapshot under the ``nightly`` dist-tag, so neither registry's
default install moves. With ``--stamp`` the versions are written into
client/python/opensysml/_version.py, client/node/package.json and
client/jupyter-kernel/jupyter_opensysml_kernel/_version.py of the checkout
being built, which is never committed; jupyter-opensysml-kernel is published
at the PyPI version too.

Usage:
    python client/python/scripts/snapshot_version.py --date 20261006 --commit abc1234 [--stamp]

Prints one ``name=value`` line per version above, in a form ``$GITHUB_OUTPUT``
accepts. Exits 1 with a message when the declared versions disagree or are not
a release the snapshot can lead to.
"""

import argparse
import datetime
import importlib.util
import json
import os
import re
import subprocess
import sys


def _load_check_version():
    """The release gate beside this script, whose readers and checks this one reuses."""
    path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "check_version.py")
    spec = importlib.util.spec_from_file_location("check_version", path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


check_version = _load_check_version()
VersionError = check_version.VersionError
VERSION_FILE = check_version.VERSION_FILE
NODE_PACKAGE = check_version.NODE_PACKAGE
KERNEL_VERSION_FILE = check_version.KERNEL_VERSION_FILE
REPO_ROOT = check_version.REPO_ROOT
TAG_PREFIX = check_version.TAG_PREFIX

#: The dist-tag npm snapshots are published under, and the moving release alias.
CHANNEL = "nightly"

DATE_FORM = re.compile(r"[0-9]{8}\Z")
COMMIT_FORM = re.compile(r"[0-9a-f]{7,40}\Z")


def release_is_tagged(tag, repo_root=REPO_ROOT):
    """Whether the checkout has a release tag, which only a fetch of the tags can tell.

    Raises:
        VersionError: If git cannot list the tags
    """
    try:
        listed = subprocess.run(
            ["git", "-C", repo_root, "tag", "--list", tag],
            check=True,
            capture_output=True,
            text=True,
        ).stdout
    except (OSError, subprocess.CalledProcessError) as e:
        raise VersionError(f"Cannot list the tags of {repo_root} to see whether {tag} exists: {e}")
    return tag in listed.split()


def snapshot_core(declared, released):
    """The release a snapshot of the declared version leads to.

    Args:
        declared (str): Version client/python/opensysml/_version.py declares
        released (callable): Whether a release tag exists, given the tag

    Returns:
        str: ``<major>.<minor>.<patch>``, the declared release or, once that is
            tagged, the next patch release

    Raises:
        VersionError: If the declared version is not a release or pre-release of
            one, or is itself a development version
    """
    parsed = check_version.parse_version(declared, "client/python/opensysml/_version.py declares")
    if parsed.dev is not None or parsed.post is not None or parsed.local is not None:
        raise VersionError(
            f"client/python/opensysml/_version.py declares {declared!r}, already a "
            "development, post or local version; a snapshot is derived from the "
            "release or pre-release the tree declares."
        )
    if len(parsed.release) != 3:
        raise VersionError(
            f"client/python/opensysml/_version.py declares {declared!r}, not a "
            "<major>.<minor>.<patch> release; the snapshot cannot name the release it leads to."
        )
    major, minor, patch = parsed.release
    core = f"{major}.{minor}.{patch}"
    if released(f"{TAG_PREFIX}{core}"):
        core = f"{major}.{minor}.{patch + 1}"
    return core


def pypi_version(core, date):
    """The PEP 440 development release of a core version built on a date."""
    return f"{core}.dev{date}"


def npm_version(core, date, commit):
    """The SemVer pre-release of a core version built on a date from a commit.

    The commit is prefixed as ``git describe`` does, so the identifier is
    alphanumeric whatever hex digits the commit happens to start with.
    """
    return f"{core}-{CHANNEL}.{date}.g{commit}"


def release_tag(date, commit):
    """The tag of the per-night GitHub release built on a date from a commit."""
    return f"{CHANNEL}-{date}-{commit}"


def snapshot_versions(date, commit, declared=None, node=None, released=None, kernel=None):
    """Every version a snapshot built on a date from a commit is published under.

    Args:
        date (str): The build date as ``YYYYMMDD``
        commit (str): The abbreviated commit, lower-case hex
        declared (str, optional): Version _version.py declares; read when omitted
        node (str, optional): Version client/node/package.json declares; read when omitted
        kernel (str, optional): Version jupyter_opensysml_kernel/_version.py
            declares; read when omitted
        released (callable, optional): Whether a release tag exists, given the
            tag; the checkout's tags are listed when omitted

    Returns:
        dict: ``core``, ``pypi``, ``npm`` and ``tag`` (see the module docstring)

    Raises:
        VersionError: If the date or commit is malformed, the declared versions
            disagree, or no release follows from the declared version
    """
    if DATE_FORM.match(date) is None:
        raise VersionError(f"Date {date!r} is not of the form YYYYMMDD.")
    try:
        datetime.date(int(date[:4]), int(date[4:6]), int(date[6:]))
    except ValueError as e:
        raise VersionError(f"Date {date!r} is not a calendar date: {e}.")
    if COMMIT_FORM.match(commit) is None:
        raise VersionError(
            f"Commit {commit!r} is not an abbreviated commit of 7 to 40 lower-case hex digits."
        )
    declared = check_version.declared_version() if declared is None else declared
    node = check_version.node_declared_version() if node is None else node
    kernel = check_version.declared_version(KERNEL_VERSION_FILE) if kernel is None else kernel
    # The packages are published at one version, so stamping starts from one too.
    check_version.node_version(declared=declared, node=node)
    check_version.kernel_version(declared=declared, kernel=kernel)
    core = snapshot_core(declared, release_is_tagged if released is None else released)
    return {
        "core": core,
        "pypi": pypi_version(core, date),
        "npm": npm_version(core, date, commit),
        "tag": release_tag(date, commit),
    }


def stamp_version_file(version, version_file=VERSION_FILE):
    """Write a version as the one client/python/opensysml/_version.py declares.

    Raises:
        VersionError: If the file declares no VERSION to replace
    """
    with open(version_file, encoding="utf-8") as f:
        text = f.read()
    stamped, count = re.subn(
        r'^VERSION = ["\'].*["\']$', f'VERSION = "{version}"', text, count=1, flags=re.MULTILINE
    )
    if count != 1:
        raise VersionError(f"{version_file} declares no VERSION to stamp.")
    with open(version_file, "w", encoding="utf-8") as f:
        f.write(stamped)
    if check_version.declared_version(version_file) != version:
        raise VersionError(f"{version_file} does not declare {version!r} after stamping it.")


def stamp_node_manifest(version, package_json=NODE_PACKAGE):
    """Write a version as the one client/node/package.json declares and pins.

    The platform packages in optionalDependencies and the WASM peer are pinned at
    the same version, as a release pins them; the file keeps npm's own layout.

    Raises:
        VersionError: If the manifest names no package, or pins no platform package
    """
    with open(package_json, encoding="utf-8") as f:
        manifest = json.load(f)
    name = manifest.get("name")
    if not isinstance(name, str) or not name:
        raise VersionError(f"{package_json} declares no package name.")
    manifest["version"] = version
    platforms = [
        dependency
        for dependency in manifest.get("optionalDependencies", {})
        if dependency.startswith(f"{name}-sysml-grpc-")
    ]
    if not platforms:
        raise VersionError(f"{package_json} pins no {name}-sysml-grpc-* platform package.")
    for dependency in platforms:
        manifest["optionalDependencies"][dependency] = version
    wasm = f"{name}-wasm"
    if wasm in manifest.get("peerDependencies", {}):
        manifest["peerDependencies"][wasm] = version
    with open(package_json, "w", encoding="utf-8") as f:
        f.write(json.dumps(manifest, indent=2) + "\n")


def stamp(
    versions,
    version_file=VERSION_FILE,
    package_json=NODE_PACKAGE,
    kernel_version_file=KERNEL_VERSION_FILE,
):
    """Write a snapshot's versions into every manifest the night publishes from."""
    stamp_version_file(versions["pypi"], version_file)
    stamp_node_manifest(versions["npm"], package_json)
    stamp_version_file(versions["pypi"], kernel_version_file)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument("--date", required=True, help="build date, YYYYMMDD (UTC)")
    parser.add_argument("--commit", required=True, help="abbreviated commit built, lower-case hex")
    parser.add_argument(
        "--stamp",
        action="store_true",
        help="write the versions into both _version.py files and client/node/package.json",
    )
    args = parser.parse_args(argv)
    try:
        versions = snapshot_versions(args.date, args.commit)
        if args.stamp:
            stamp(versions)
    except VersionError as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1
    for name, value in versions.items():
        print(f"{name}={value}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
