"""Tests for scripts/check_version.py, the release gate the release workflow runs."""

import importlib.util
import pathlib

import pytest
from packaging.version import Version

SCRIPT = pathlib.Path(__file__).resolve().parents[1] / "scripts" / "check_version.py"
spec = importlib.util.spec_from_file_location("check_version", SCRIPT)
check_version = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(check_version)


def _core_tag(version):
    """The SemVer core tag naming a PEP 440 version: 0.5.0rc1 is tagged v0.5.0-rc1."""
    parsed = Version(version)
    tag = f"v{parsed.base_version}"
    if parsed.pre is not None:
        phase, number = parsed.pre
        tag += f"-{ {'a': 'alpha', 'b': 'beta', 'rc': 'rc'}[phase] }.{number}"
    return tag


def test_declared_version_reads_the_shipped_version_file():
    from opensysml import __version__

    assert check_version.declared_version() == __version__


def test_declared_version_rejects_a_file_without_a_version(tmp_path):
    version_file = tmp_path / "_version.py"
    version_file.write_text("OTHER = '1.0'\n", encoding="utf-8")
    with pytest.raises(check_version.VersionError, match="declares no VERSION"):
        check_version.declared_version(str(version_file))


def test_version_from_tag_accepts_the_declared_version():
    assert check_version.version_from_tag("v0.4.0", version="0.4.0") == "0.4.0"


@pytest.mark.parametrize(
    "tag, declared",
    [
        ("v0.4.0-rc1", "0.4.0rc1"),
        ("v0.4.0-rc.1", "0.4.0rc1"),
        ("v0.4.0-alpha.2", "0.4.0a2"),
        ("v0.4.0-beta1", "0.4.0b1"),
        ("v0.4.0-rc.10", "0.4.0rc10"),
        ("v10.0.0-rc0", "10.0.0rc0"),
    ],
)
def test_version_from_tag_translates_a_semver_pre_release_to_pep_440(tag, declared):
    """A core pre-release tag is SemVer; the package's version is its PEP 440 form."""
    assert check_version.version_from_tag(tag, version=declared) == declared
    assert check_version.is_pre_release(declared)


@pytest.mark.parametrize(
    "tag",
    [
        "v0.4.0-1",  # PEP 440 would read a post-release, SemVer a pre-release
        "v0.4.0-rc",
        "v0.4.0-pre.1",
        "v0.4.0-rc1+build.5",
        "v0.4.0rc1",
        "v0.4.0.post1",
        "v0.4",
        "v0.4.0.1",
        "v0.4.0-not-a-version",
        "v0.4.0-rc.010",  # leading zero: not a SemVer numeric identifier
        "v0.4.0-rc010",
        "v0.04.0",
        "v0.4.0-rc\u0661",  # Arabic-Indic one: a digit to \\d, not to SemVer
        "v\u0660.4.0",
    ],
)
def test_version_from_tag_rejects_a_suffix_without_one_pep_440_meaning(tag):
    with pytest.raises(check_version.VersionError, match="is not a core release tag of the form"):
        check_version.version_from_tag(tag, version="0.4.0.post1")


@pytest.mark.parametrize(
    "tag, message",
    [
        ("", "No tag given"),
        ("opensysml-v0.4.0", "does not start with 'v'"),
        ("0.4.0", "does not start with 'v'"),
        ("v0.4.1", "names version '0.4.1', but"),
        ("v0.4.0-rc1", "names version '0.4.0rc1', but"),
    ],
)
def test_version_from_tag_rejects_a_tag_that_names_another_version(tag, message):
    with pytest.raises(check_version.VersionError, match=message):
        check_version.version_from_tag(tag, version="0.4.0")


@pytest.mark.parametrize("declared", ["0.4.0-rc1", "0.4.0.RC1", "v0.4.0"])
def test_version_from_tag_requires_the_declared_version_to_be_canonical(declared):
    """The artifacts are named by the canonical form, so the declaration must be it."""
    with pytest.raises(check_version.VersionError, match="canonical PEP 440 form"):
        check_version.version_from_tag("v0.4.0-rc1", version=declared)


def test_version_from_tag_rejects_a_declared_non_version():
    with pytest.raises(check_version.VersionError, match="not a PEP 440 version"):
        check_version.version_from_tag("v0.4.0", version="latest")


@pytest.mark.parametrize(
    "version, pre_release",
    [("0.4.0", False), ("0.4.0rc1", True), ("1.0.0a2", True), ("1.0.0.post1", False)],
)
def test_is_pre_release_follows_pep_440(version, pre_release):
    assert check_version.is_pre_release(version) is pre_release


def test_is_pre_release_rejects_a_non_pep_440_version():
    with pytest.raises(check_version.VersionError, match="not a PEP 440 version"):
        check_version.is_pre_release("latest")


def test_main_prints_the_version_the_tag_names(capsys):
    declared = check_version.declared_version()
    assert check_version.main(["--tag", _core_tag(declared)]) == 0
    assert capsys.readouterr().out.strip() == declared


def test_main_routes_a_pre_release_by_printing_yes_or_no(capsys):
    declared = check_version.declared_version()
    assert check_version.main(["--tag", _core_tag(declared), "--pre-release"]) == 0
    expected = "yes" if check_version.is_pre_release(declared) else "no"
    assert capsys.readouterr().out.strip() == expected


def test_main_fails_on_a_tag_for_another_version(capsys):
    assert check_version.main(["--tag", "v0.0.0"]) == 1
    captured = capsys.readouterr()
    assert captured.out == ""
    assert "error:" in captured.err
    assert "declares" in captured.err


def test_main_fails_on_a_tag_that_names_no_version(capsys):
    assert check_version.main(["--tag", "v0.0.0-not-declared"]) == 1
    captured = capsys.readouterr()
    assert captured.out == ""
    assert "is not a core release tag of the form" in captured.err


def test_main_reads_the_tag_from_circle_tag(monkeypatch, capsys):
    declared = check_version.declared_version()
    monkeypatch.setenv("CIRCLE_TAG", _core_tag(declared))
    assert check_version.main([]) == 0
    assert capsys.readouterr().out.strip() == declared
