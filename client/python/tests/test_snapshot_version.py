"""Tests for scripts/snapshot_version.py, which names the nightly client snapshots."""

import importlib.util
import json
import pathlib
import subprocess

import pytest
from packaging.version import Version

SCRIPT = pathlib.Path(__file__).resolve().parents[1] / "scripts" / "snapshot_version.py"
spec = importlib.util.spec_from_file_location("snapshot_version", SCRIPT)
snapshot_version = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(snapshot_version)

VersionError = snapshot_version.VersionError

DATE = "20261006"
COMMIT = "abc1234"


def released(*tags):
    """A tag lookup that knows exactly these releases."""
    return lambda tag: tag in tags


def test_an_unreleased_declared_version_is_the_snapshot_core():
    assert snapshot_version.snapshot_core("0.9.3", released("v0.9.2")) == "0.9.3"


def test_a_released_declared_version_leads_to_the_next_patch():
    assert snapshot_version.snapshot_core("0.9.2", released("v0.9.2")) == "0.9.3"


def test_a_pre_release_leads_to_its_release():
    """develop is heading for 0.10.0; a snapshot ranks below its release candidates."""
    assert snapshot_version.snapshot_core("0.10.0rc1", released("v0.10.0-rc.1")) == "0.10.0"


@pytest.mark.parametrize("declared", ["0.9.3.dev1", "0.9.3.post1", "0.9.3+local"])
def test_a_development_post_or_local_version_is_not_a_snapshot_base(declared):
    with pytest.raises(VersionError, match="already a development, post or local version"):
        snapshot_version.snapshot_core(declared, released())


def test_a_version_without_three_release_segments_is_refused():
    with pytest.raises(VersionError, match="not a <major>.<minor>.<patch> release"):
        snapshot_version.snapshot_core("1.0", released())


def test_the_versions_name_one_night():
    versions = snapshot_version.snapshot_versions(
        DATE, COMMIT, declared="0.9.2", node="0.9.2", released=released("v0.9.2")
    )
    assert versions == {
        "core": "0.9.3",
        "pypi": "0.9.3.dev20261006",
        "npm": "0.9.3-nightly.20261006.gabc1234",
        "tag": "nightly-20261006-abc1234",
    }


def test_the_pypi_version_is_canonical_pep_440_and_ranks_below_the_release():
    """pip names the built files by the canonical form, and skips the snapshot by default."""
    versions = snapshot_version.snapshot_versions(
        DATE, COMMIT, declared="0.9.2", node="0.9.2", released=released("v0.9.2")
    )
    parsed = Version(versions["pypi"])
    assert str(parsed) == versions["pypi"]
    assert parsed.is_devrelease and parsed.is_prerelease
    assert Version("0.9.2") < parsed < Version("0.9.3rc1") < Version("0.9.3")


def test_the_npm_version_is_a_semver_pre_release_of_the_core():
    versions = snapshot_version.snapshot_versions(
        DATE, "0123456", declared="0.9.2", node="0.9.2", released=released("v0.9.2")
    )
    # A commit of digits only would be a numeric identifier with a leading zero, which
    # SemVer forbids; the git-describe prefix keeps the identifier alphanumeric.
    assert versions["npm"] == "0.9.3-nightly.20261006.g0123456"
    assert versions["tag"] == "nightly-20261006-0123456"


def test_the_declared_versions_must_agree():
    with pytest.raises(VersionError, match="client/node/package.json"):
        snapshot_version.snapshot_versions(
            DATE, COMMIT, declared="0.9.2", node="0.9.1", released=released()
        )


def test_the_node_manifest_may_spell_a_pre_release_its_own_way():
    versions = snapshot_version.snapshot_versions(
        DATE, COMMIT, declared="0.10.0rc1", node="0.10.0-rc.1", released=released()
    )
    assert versions["pypi"] == "0.10.0.dev20261006"
    assert versions["npm"] == "0.10.0-nightly.20261006.gabc1234"


@pytest.mark.parametrize("date", ["2026-10-06", "202610", "20261306", "20260230"])
def test_a_malformed_date_is_refused(date):
    with pytest.raises(VersionError, match="Date"):
        snapshot_version.snapshot_versions(date, COMMIT, declared="0.9.2", node="0.9.2", released=released())


@pytest.mark.parametrize("commit", ["abc", "ABC1234", "g" * 7, "0" * 41])
def test_a_malformed_commit_is_refused(commit):
    with pytest.raises(VersionError, match="Commit"):
        snapshot_version.snapshot_versions(DATE, commit, declared="0.9.2", node="0.9.2", released=released())


def test_the_checkout_declares_versions_a_snapshot_can_follow():
    """The committed tree is what the nightly stamps, so it must be stampable as it stands."""
    versions = snapshot_version.snapshot_versions(DATE, COMMIT, released=released())
    declared = snapshot_version.check_version.declared_version()
    assert versions["core"] == Version(declared).base_version


def test_release_tags_are_read_from_the_checkout(tmp_path):
    subprocess.run(["git", "init", "-q", str(tmp_path)], check=True)
    subprocess.run(
        ["git", "-C", str(tmp_path), "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "x"],
        check=True,
    )
    subprocess.run(["git", "-C", str(tmp_path), "tag", "v0.9.2"], check=True)
    assert snapshot_version.release_is_tagged("v0.9.2", str(tmp_path))
    assert not snapshot_version.release_is_tagged("v0.9.3", str(tmp_path))


def test_release_tags_cannot_be_read_outside_a_checkout(tmp_path):
    with pytest.raises(VersionError, match="Cannot list the tags"):
        snapshot_version.release_is_tagged("v0.9.2", str(tmp_path))


@pytest.fixture
def manifests(tmp_path):
    """Copies of both client manifests as the checkout has them."""
    version_file = tmp_path / "_version.py"
    version_file.write_text(pathlib.Path(snapshot_version.VERSION_FILE).read_text(encoding="utf-8"), encoding="utf-8")
    package_json = tmp_path / "package.json"
    package_json.write_text(pathlib.Path(snapshot_version.NODE_PACKAGE).read_text(encoding="utf-8"), encoding="utf-8")
    return version_file, package_json


def test_stamping_the_declared_versions_changes_nothing(manifests):
    """So a stamp rewrites only the versions, in the layout the files already have."""
    version_file, package_json = manifests
    before = version_file.read_text(encoding="utf-8"), package_json.read_text(encoding="utf-8")
    declared = snapshot_version.check_version.declared_version()
    node = snapshot_version.check_version.node_declared_version()
    snapshot_version.stamp({"pypi": declared, "npm": node}, str(version_file), str(package_json))
    assert (version_file.read_text(encoding="utf-8"), package_json.read_text(encoding="utf-8")) == before


def test_stamping_writes_the_snapshot_into_both_manifests(manifests):
    version_file, package_json = manifests
    versions = snapshot_version.snapshot_versions(
        DATE, COMMIT, declared="0.9.2", node="0.9.2", released=released("v0.9.2")
    )
    snapshot_version.stamp(versions, str(version_file), str(package_json))

    assert snapshot_version.check_version.declared_version(str(version_file)) == "0.9.3.dev20261006"
    assert "The one place the opensysml version is written" in version_file.read_text(encoding="utf-8")
    manifest = json.loads(package_json.read_text(encoding="utf-8"))
    npm = "0.9.3-nightly.20261006.gabc1234"
    assert manifest["version"] == npm
    platforms = {
        name: version
        for name, version in manifest["optionalDependencies"].items()
        if name.startswith(f"{manifest['name']}-sysml-grpc-")
    }
    assert len(platforms) == 5 and set(platforms.values()) == {npm}
    assert manifest["peerDependencies"][f"{manifest['name']}-wasm"] == npm
    # The stamped manifest is what the release gate reads, so it must still read as one.
    assert snapshot_version.check_version.node_declared_version(str(package_json)) == npm


def test_stamping_a_version_file_without_a_version_fails(tmp_path):
    version_file = tmp_path / "_version.py"
    version_file.write_text("OTHER = '1.0'\n", encoding="utf-8")
    with pytest.raises(VersionError, match="declares no VERSION"):
        snapshot_version.stamp_version_file("0.9.3.dev20261006", str(version_file))


def test_stamping_a_manifest_without_platform_packages_fails(tmp_path):
    package_json = tmp_path / "package.json"
    package_json.write_text(json.dumps({"name": "@openmbee/opensysml", "version": "0.9.2"}), encoding="utf-8")
    with pytest.raises(VersionError, match="pins no @openmbee/opensysml-sysml-grpc-"):
        snapshot_version.stamp_node_manifest("0.9.3-nightly.20261006.gabc1234", str(package_json))


def test_the_command_prints_github_output_lines(capsys, monkeypatch):
    monkeypatch.setattr(snapshot_version, "release_is_tagged", released("v0.9.2"))
    monkeypatch.setattr(snapshot_version.check_version, "declared_version", lambda *a: "0.9.2")
    monkeypatch.setattr(snapshot_version.check_version, "node_declared_version", lambda *a: "0.9.2")
    assert snapshot_version.main(["--date", DATE, "--commit", COMMIT]) == 0
    assert capsys.readouterr().out == (
        "core=0.9.3\n"
        "pypi=0.9.3.dev20261006\n"
        "npm=0.9.3-nightly.20261006.gabc1234\n"
        "tag=nightly-20261006-abc1234\n"
    )


def test_the_command_reports_a_bad_date_and_stamps_nothing(capsys, tmp_path, monkeypatch):
    monkeypatch.setattr(snapshot_version, "VERSION_FILE", str(tmp_path / "absent.py"))
    assert snapshot_version.main(["--date", "2026", "--commit", COMMIT, "--stamp"]) == 1
    captured = capsys.readouterr()
    assert captured.out == ""
    assert "Error: Date '2026' is not of the form YYYYMMDD." in captured.err
    assert not (tmp_path / "absent.py").exists()
