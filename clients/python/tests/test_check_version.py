"""Tests for scripts/check_version.py, the release gate the publish job runs."""

import importlib.util
import pathlib

import pytest

SCRIPT = pathlib.Path(__file__).resolve().parents[1] / "scripts" / "check_version.py"
spec = importlib.util.spec_from_file_location("check_version", SCRIPT)
check_version = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(check_version)


def test_declared_version_reads_the_shipped_version_file():
    from opensysml import __version__

    assert check_version.declared_version() == __version__


def test_declared_version_rejects_a_file_without_a_version(tmp_path):
    version_file = tmp_path / "_version.py"
    version_file.write_text("OTHER = '1.0'\n", encoding="utf-8")
    with pytest.raises(check_version.VersionError, match="declares no VERSION"):
        check_version.declared_version(str(version_file))


def test_version_from_tag_accepts_the_declared_version():
    assert check_version.version_from_tag("opensysml-v0.4.0", version="0.4.0") == "0.4.0"


@pytest.mark.parametrize(
    "tag, message",
    [
        ("", "No tag given"),
        ("v0.4.0", "does not start with 'opensysml-v'"),
        ("opensysml-v0.4.1", "names version '0.4.1', but"),
    ],
)
def test_version_from_tag_rejects_a_tag_that_names_another_version(tag, message):
    with pytest.raises(check_version.VersionError, match=message):
        check_version.version_from_tag(tag, version="0.4.0")


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
    assert check_version.main(["--tag", f"opensysml-v{declared}"]) == 0
    assert capsys.readouterr().out.strip() == declared


def test_main_routes_a_pre_release_by_printing_yes_or_no(capsys):
    declared = check_version.declared_version()
    assert check_version.main(["--tag", f"opensysml-v{declared}", "--pre-release"]) == 0
    expected = "yes" if check_version.is_pre_release(declared) else "no"
    assert capsys.readouterr().out.strip() == expected


def test_main_fails_on_a_tag_for_another_version(capsys):
    assert check_version.main(["--tag", "opensysml-v0.0.0-not-declared"]) == 1
    captured = capsys.readouterr()
    assert captured.out == ""
    assert "error:" in captured.err
    assert "declares" in captured.err


def test_main_reads_the_tag_from_circle_tag(monkeypatch, capsys):
    declared = check_version.declared_version()
    monkeypatch.setenv("CIRCLE_TAG", f"opensysml-v{declared}")
    assert check_version.main([]) == 0
    assert capsys.readouterr().out.strip() == declared
