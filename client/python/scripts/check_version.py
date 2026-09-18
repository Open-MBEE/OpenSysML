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

The core tags are SemVer and the package version is PEP 440, so the tag is
translated before the comparison: `v0.9.0-rc1` names `0.9.0rc1`. Only the SemVer
pre-release forms with one PEP 440 meaning are accepted (`-alpha.N`, `-beta.N`,
`-rc.N`, dot optional); `-1` would be a PEP 440 post-release, so it is refused.
What is printed is the declared version, which the built artifacts are named by.
"""

import argparse
import ast
import os
import re
import sys

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
    canonical = str(parse_version(declared, "clients/python/opensysml/_version.py declares"))
    if canonical != declared:
        raise VersionError(
            f"clients/python/opensysml/_version.py declares {declared!r}, whose "
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
            f"clients/python/opensysml/_version.py declares {declared!r}. "
            "The package is released in lockstep with the core, so its declared "
            "version must be the core version being tagged. Set VERSION to "
            f"{tag_version!r} on the release branch and tag again."
        )
    return declared


def pep440_from_semver(semver, tag):
    """The canonical PEP 440 version a core tag's SemVer version denotes.

    Args:
        semver (str): The tag without its prefix, e.g. '0.9.0-rc1'
        tag (str): The tag, for the error message

    Returns:
        str: The PEP 440 version, e.g. '0.9.0rc1'

    Raises:
        VersionError: If the version is not a release or an alpha/beta/rc
            pre-release of one; other SemVer suffixes have no single PEP 440
            meaning ('-1' would be a post-release, build metadata a local version)
    """
    match = TAG_VERSION.match(semver)
    if match is None:
        raise VersionError(
            f"Tag {tag!r} is not a core release tag of the form {TAG_FORM}; only "
            "those pre-release forms have one PEP 440 meaning for the package."
        )
    if match["phase"] is None:
        return match["release"]
    return f"{match['release']}{PEP_440_PHASE[match['phase']]}{match['number']}"


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
    parser.add_argument(
        "--pre-release",
        action="store_true",
        help="print yes/no for whether the tag names a pre-release",
    )
    args = parser.parse_args(argv)

    try:
        version = version_from_tag(args.tag)
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
