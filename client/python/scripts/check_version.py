#!/usr/bin/env python3
"""Gate a release on the core tag and the declared opensysml version agreeing.

The package is published from the same `v<version>` tag as the binaries, so the
version it declares must be the one the tag names. Used by the CircleCI release
workflow before anything is built or uploaded: a PyPI version can be yanked but
never re-uploaded, so a tag that does not name the version the package would
publish must fail here rather than after the fact.

    python scripts/check_version.py --tag v0.9.0

Prints the version the tag names on success. With `--pre-release` it prints
`yes`/`no` instead, which the job uses to route a pre-release tag to TestPyPI.
With `--node`, `--java` or `--rust` the version client/node/package.json,
client/java/pom.xml or client/rust/opensysml/Cargo.toml declares is checked
the same way and printed instead — all three clients are published from the
same tag, at the SemVer spelling of the same version.

The core tags are SemVer and the package version is PEP 440, so the tag is
translated before the comparison: `v0.9.0-rc1` names `0.9.0rc1`. Only the SemVer
pre-release forms with one PEP 440 meaning are accepted (`-alpha.N`, `-beta.N`,
`-rc.N`, dot optional); `-1` would be a PEP 440 post-release, so it is refused.
What is printed is the declared version, which the built artifacts are named by.
"""

import argparse
import ast
import json
import os
import re
import sys
import xml.etree.ElementTree as ET

from packaging.version import InvalidVersion, Version

TAG_PREFIX = "v"

# ASCII digits without leading zeros, as SemVer numeric identifiers are.
_NUMBER = r"(?:0|[1-9][0-9]*)"
TAG_VERSION = re.compile(
    rf"^(?P<release>{_NUMBER}\.{_NUMBER}\.{_NUMBER})"
    rf"(?:-(?P<phase>alpha|beta|rc)\.?(?P<number>{_NUMBER}))?$",
    re.ASCII,
)
TAG_FORM = f"{TAG_PREFIX}<major>.<minor>.<patch>[-(alpha|beta|rc)[.]<n>]"
PEP_440_PHASE = {"alpha": "a", "beta": "b", "rc": "rc"}

VERSION_FILE = os.path.join(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "opensysml", "_version.py"
)
REPO_ROOT = os.path.dirname(
    os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
)
NODE_PACKAGE = os.path.join(REPO_ROOT, "client", "node", "package.json")
JAVA_POM = os.path.join(REPO_ROOT, "client", "java", "pom.xml")
RUST_CARGO = os.path.join(REPO_ROOT, "client", "rust", "opensysml", "Cargo.toml")


class VersionError(Exception):
    """A tag that does not name a publishable opensysml version."""


def declared_version(version_file=VERSION_FILE):
    """The version declared in opensysml/_version.py.

    Read rather than imported, so the check needs neither the package's
    dependencies nor an installed distribution.

    Args:
        version_file (str): Path to opensysml/_version.py

    Returns:
        str: The declared version

    Raises:
        VersionError: If the file declares no VERSION string
    """
    with open(version_file, encoding="utf-8") as f:
        module = ast.parse(f.read(), filename=version_file)
    for node in module.body:
        if isinstance(node, ast.Assign) and any(
            isinstance(t, ast.Name) and t.id == "VERSION" for t in node.targets
        ):
            if isinstance(node.value, ast.Constant) and isinstance(node.value.value, str):
                return node.value.value
    raise VersionError(f"{version_file} declares no VERSION string")


def version_from_tag(tag, version=None):
    """The version a release tag names, checked against the declared version.

    Args:
        tag (str): Core release tag, e.g. 'v0.9.0'
        version (str, optional): Declared version; read from
            opensysml/_version.py when omitted

    Returns:
        str: The version to publish, as declared

    Raises:
        VersionError: If the tag is empty, is not a core release tag, or names
            a version other than the declared one; or if the declared version is
            not PEP 440 in canonical form
    """
    declared = version if version is not None else declared_version()
    canonical = str(parse_version(declared, "client/python/opensysml/_version.py declares"))
    if canonical != declared:
        raise VersionError(
            f"client/python/opensysml/_version.py declares {declared!r}, whose "
            f"canonical PEP 440 form is {canonical!r}. The built artifacts are named "
            f"by the canonical form, so declare VERSION = {canonical!r}."
        )
    if not tag:
        raise VersionError(
            "No tag given. The release workflow runs on a "
            f"{TAG_PREFIX}<version> tag and reads CIRCLE_TAG."
        )
    if not tag.startswith(TAG_PREFIX):
        raise VersionError(
            f"Tag {tag!r} does not start with {TAG_PREFIX!r}. The package is "
            f"released with the binaries, by the core {TAG_PREFIX}<version> tag; "
            "no other tag publishes it."
        )
    tag_version = pep440_from_semver(tag[len(TAG_PREFIX):], tag)
    if tag_version != declared:
        raise VersionError(
            f"Tag {tag!r} names version {tag_version!r}, but "
            f"client/python/opensysml/_version.py declares {declared!r}. "
            "The package is released in lockstep with the core, so its declared "
            "version must be the core version being tagged. Set VERSION to "
            f"{tag_version!r} on the release branch and tag again."
        )
    return declared


def pep440_from_semver(semver, tag, what=None):
    """The canonical PEP 440 version a core tag's SemVer version denotes.

    Args:
        semver (str): The tag without its prefix, e.g. '0.9.0-rc1'
        tag (str): The tag, for the error message
        what (str, optional): What the version came from, for the error
            message; the tag is blamed when omitted

    Returns:
        str: The PEP 440 version, e.g. '0.9.0rc1'

    Raises:
        VersionError: If the version is not a release or an alpha/beta/rc
            pre-release of one; other SemVer suffixes have no single PEP 440
            meaning ('-1' would be a post-release, build metadata a local version)
    """
    match = TAG_VERSION.match(semver)
    if match is None:
        if what is None:
            raise VersionError(
                f"Tag {tag!r} is not a core release tag of the form {TAG_FORM}; only "
                "those pre-release forms have one PEP 440 meaning for the package."
            )
        raise VersionError(
            f"{what} {semver!r} is not of the form {TAG_FORM[len(TAG_PREFIX):]}; only "
            "those pre-release forms have one PEP 440 meaning for the package."
        )
    if match["phase"] is None:
        return match["release"]
    return f"{match['release']}{PEP_440_PHASE[match['phase']]}{match['number']}"


def node_declared_version(package_json=NODE_PACKAGE):
    """The version declared in client/node/package.json.

    Args:
        package_json (str): Path to client/node/package.json

    Returns:
        str: The declared version

    Raises:
        VersionError: If the file declares no version string
    """
    with open(package_json, encoding="utf-8") as f:
        version = json.load(f).get("version")
    if not isinstance(version, str):
        raise VersionError(f"{package_json} declares no version string")
    return version


def node_version(declared=None, node=None, tag=None):
    """client/node/package.json's version, checked against _version.py and, when given, the tag.

    Args:
        declared (str, optional): Version opensysml/_version.py declares; read
            when omitted
        node (str, optional): Version client/node/package.json declares; read
            when omitted
        tag (str, optional): Core release tag the npm publish runs from

    Returns:
        str: The npm version to publish, as package.json declares

    Raises:
        VersionError: If the two files disagree, or the tag does not spell the
            SemVer version package.json declares
    """
    node = node_declared_version() if node is None else node
    return _client_version(
        "client/node/package.json",
        "package.json",
        "npm",
        "The Node client",
        node,
        declared,
        tag,
        'set "version" and every platform package in optionalDependencies to the '
        "SemVer spelling of that version.",
    )


def java_declared_version(pom=JAVA_POM):
    """The version client/java/pom.xml declares.

    Args:
        pom (str): Path to client/java/pom.xml

    Returns:
        str: The declared version

    Raises:
        VersionError: If the pom declares no version of its own
    """
    project = ET.parse(pom).getroot()
    version = project.findtext("{http://maven.apache.org/POM/4.0.0}version")
    if version is None:
        raise VersionError(f"{pom} declares no version")
    return version


def java_version(declared=None, java=None, tag=None):
    """client/java/pom.xml's version, checked against _version.py and, when given, the tag.

    Args:
        declared (str, optional): Version opensysml/_version.py declares; read
            when omitted
        java (str, optional): Version client/java/pom.xml declares; read when
            omitted
        tag (str, optional): Core release tag the Maven publish runs from

    Returns:
        str: The Maven version to publish, as the pom declares

    Raises:
        VersionError: If the pom and _version.py disagree, or the tag does not
            spell the Maven version the pom declares
    """
    java = java_declared_version() if java is None else java
    return _client_version(
        "client/java/pom.xml",
        "the pom",
        "Maven Central",
        "The Java client",
        java,
        declared,
        tag,
        "set the parent pom's <version>, both modules' <parent><version>, and the "
        "editors' references to the Maven spelling of that version.",
    )


def rust_declared_version(cargo_toml=RUST_CARGO):
    """The version client/rust/opensysml/Cargo.toml's [package] table declares.

    A minimal line scan rather than a TOML parse, so the check needs no
    tomllib (Python 3.10 is still supported).

    Args:
        cargo_toml (str): Path to client/rust/opensysml/Cargo.toml

    Returns:
        str: The declared version

    Raises:
        VersionError: If the [package] table declares no version
    """
    with open(cargo_toml, encoding="utf-8") as f:
        lines = f.readlines()
    try:
        start = next(
            i for i, line in enumerate(lines) if line.strip() == "[package]"
        )
    except StopIteration:
        raise VersionError(f"{cargo_toml} declares no [package] version")
    for line in lines[start + 1 :]:
        if line.startswith("["):
            break
        match = re.match(
            r"""^\s*version\s*=\s*(?:"([^"]+)"|'([^']+)')\s*(#.*)?$""", line
        )
        if match:
            return match[1] or match[2]
    raise VersionError(f"{cargo_toml} declares no [package] version")


def rust_version(declared=None, rust=None, tag=None):
    """client/rust/opensysml/Cargo.toml's version, checked against _version.py and, when given, the tag.

    Args:
        declared (str, optional): Version opensysml/_version.py declares; read
            when omitted
        rust (str, optional): Version client/rust/opensysml/Cargo.toml
            declares; read when omitted
        tag (str, optional): Core release tag the crates.io publish runs from

    Returns:
        str: The crates.io version to publish, as Cargo.toml declares

    Raises:
        VersionError: If the Cargo.toml and _version.py disagree, or the tag
            does not spell the version Cargo.toml declares
    """
    rust = rust_declared_version() if rust is None else rust
    return _client_version(
        "client/rust/opensysml/Cargo.toml",
        "Cargo.toml",
        "crates.io",
        "The Rust client",
        rust,
        declared,
        tag,
        "set [package] version in client/rust/opensysml/Cargo.toml to the "
        "SemVer spelling of that version and run `cargo update -p opensysml` in "
        "client/rust.",
    )


def _client_version(what, file, registry, client_name, client, declared, tag, remedy):
    """The lockstep check the published client manifests share.

    Args:
        what (str): Path naming the client manifest, for the messages
        file (str): The manifest in short form, for the tag message
        registry (str): The registry the client publishes to, for the tag message
        client_name (str): The client, for the mismatch message
        client (str): The version the manifest declares
        declared (str, optional): Version opensysml/_version.py declares; read
            when omitted
        tag (str, optional): Core release tag the publish runs from
        remedy (str): How to bring the manifest back in step, for the message

    Returns:
        str: The version the manifest declares

    Raises:
        VersionError: If the manifest and _version.py disagree, or the tag does
            not spell the version the manifest declares
    """
    declared = declared_version() if declared is None else declared
    translated = pep440_from_semver(client, tag, what=f"{what} version")
    if translated != declared:
        raise VersionError(
            f"{what} declares {client!r} (PEP 440 {translated!r}), but "
            f"client/python/opensysml/_version.py declares {declared!r}. {client_name} is "
            "released in lockstep with the core and the Python client: " + remedy
        )
    if tag and tag[len(TAG_PREFIX):] != client:
        raise VersionError(
            f"Tag {tag!r} names {registry} version {tag[len(TAG_PREFIX):]!r}, but {what} "
            f"declares {client!r}. {registry} publishes the version {file} declares, so the "
            "tag must spell it exactly."
        )
    return client


def parse_version(version, what):
    """A version string as a PEP 440 version.

    Args:
        version (str): Version string
        what (str): What names the version, for the error message

    Returns:
        packaging.version.Version: The parsed version; str() of it is canonical

    Raises:
        VersionError: If the version is not a valid PEP 440 version
    """
    try:
        return Version(version)
    except InvalidVersion as e:
        raise VersionError(f"{what} {version!r}, which is not a PEP 440 version: {e}")


def is_pre_release(version):
    """Whether a version is a PEP 440 pre-release (alpha/beta/rc).

    Args:
        version (str): Version string

    Returns:
        bool: True for a pre-release, which is published to TestPyPI

    Raises:
        VersionError: If the version is not a valid PEP 440 version
    """
    return parse_version(version, "Version").is_prerelease


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--tag",
        default=os.environ.get("CIRCLE_TAG", ""),
        help="release tag (default: $CIRCLE_TAG)",
    )
    clients = parser.add_mutually_exclusive_group()
    clients.add_argument(
        "--node",
        action="store_true",
        help="check and print client/node/package.json's version instead",
    )
    clients.add_argument(
        "--java",
        action="store_true",
        help="check and print client/java/pom.xml's version instead",
    )
    clients.add_argument(
        "--rust",
        action="store_true",
        help="check and print client/rust/opensysml/Cargo.toml's version instead",
    )
    parser.add_argument(
        "--pre-release",
        action="store_true",
        help="print yes/no for whether the tag names a pre-release",
    )
    args = parser.parse_args(argv)

    try:
        version = version_from_tag(args.tag)
        if args.node:
            version = node_version(declared=version, tag=args.tag)
        elif args.java:
            version = java_version(declared=version, tag=args.tag)
        elif args.rust:
            version = rust_version(declared=version, tag=args.tag)
        pre_release = is_pre_release(version)
    except VersionError as e:
        print(f"error: {e}", file=sys.stderr)
        return 1

    if args.pre_release:
        print("yes" if pre_release else "no")
    else:
        print(version)
    return 0


if __name__ == "__main__":
    sys.exit(main())
