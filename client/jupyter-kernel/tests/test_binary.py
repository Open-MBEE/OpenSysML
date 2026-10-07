"""The kernel download: named by platform, verified against the pin, never unpinned."""

import hashlib
import os

import pytest

from jupyter_opensysml_kernel import binary

from .conftest import KERNEL_BYTES, RELEASE, REPO


def test_assets_are_named_by_platform_with_exe_on_windows():
    assert binary.release_asset_name("linux", "amd64") == "sysml-jupyter-kernel-linux-amd64"
    assert binary.release_asset_name("darwin", "arm64") == "sysml-jupyter-kernel-darwin-arm64"
    assert binary.release_asset_name("windows", "amd64") == "sysml-jupyter-kernel-windows-amd64.exe"
    assert binary.binary_name("windows") == "sysml-jupyter-kernel.exe"
    assert binary.binary_name("linux") == "sysml-jupyter-kernel"


def test_the_detected_platform_is_one_a_release_publishes():
    goos, goarch = binary.detect_platform()
    assert goos in {"linux", "darwin", "windows"}
    assert goarch in {"amd64", "arm64"}


def test_unsupported_platforms_point_at_building_from_source(monkeypatch):
    monkeypatch.setattr(binary.platform, "system", lambda: "FreeBSD")
    with pytest.raises(binary.UnsupportedPlatformError, match="go build"):
        binary.detect_platform()
    monkeypatch.setattr(binary.platform, "system", lambda: "Linux")
    monkeypatch.setattr(binary.platform, "machine", lambda: "riscv64")
    with pytest.raises(binary.UnsupportedPlatformError, match="riscv64"):
        binary.detect_platform()
    monkeypatch.setattr(binary.platform, "system", lambda: "Windows")
    monkeypatch.setattr(binary.platform, "machine", lambda: "ARM64")
    with pytest.raises(binary.UnsupportedPlatformError, match="amd64 alone on Windows"):
        binary.detect_platform()


def test_the_version_names_the_release_it_was_built_against(monkeypatch):
    assert binary.built_against_releases("0.9.2") == ["v0.9.2"]
    assert binary.built_against_releases("1.0.0rc2") == ["v1.0.0-rc2", "v1.0.0-rc.2"]
    assert binary.built_against_releases("1.0.0a1") == ["v1.0.0-alpha1", "v1.0.0-alpha.1"]
    assert binary.built_against_releases("odd") == ["vodd"]
    table = {REPO: {"nightly-20261006-abc1234": {}, "nightly-20261005-def5678": {}}}
    monkeypatch.setattr(binary, "PINNED_SHA256", table)
    assert binary.built_against_releases("0.9.3.dev20261006") == ["nightly-20261006-abc1234"]
    assert binary.built_against_releases("0.9.3.dev20261001") == ["nightly"]


def test_the_shipped_table_pins_the_repository_releases():
    assert REPO in binary.PINNED_SHA256
    for version, assets in binary.PINNED_SHA256[REPO].items():
        assert version.startswith(("v", "nightly-")), version
        for asset, digest in assets.items():
            assert binary._SHA256.fullmatch(digest), f"{version} {asset}"


def test_a_pinned_release_downloads_and_verifies(release, tmp_path):
    path = binary.download_binary(str(tmp_path))
    assert os.path.basename(path) == binary.binary_name()
    with open(path, "rb") as f:
        assert f.read() == KERNEL_BYTES
    assert os.access(path, os.X_OK)
    asset = binary.release_asset_name()
    assert release.requests == [
        binary.release_download_url(RELEASE, asset + ".sha256", REPO),
        binary.release_download_url(RELEASE, asset, REPO),
    ]


def test_the_default_release_is_the_one_the_package_pins(release):
    assert binary.pinned_release() == RELEASE


def test_an_unpinned_release_is_refused_before_anything_is_downloaded(release, tmp_path):
    with pytest.raises(binary.UnpinnedReleaseError, match="--binary"):
        binary.download_binary(str(tmp_path), version="v0.0.1")
    assert release.requests == []
    assert list(tmp_path.iterdir()) == []


def test_a_package_pinning_nothing_for_its_release_says_so(release, monkeypatch):
    monkeypatch.setattr(binary, "PINNED_SHA256", {REPO: {}})
    with pytest.raises(binary.UnpinnedReleaseError, match="v9.9.9"):
        binary.pinned_release()


def test_a_tampered_download_is_refused_and_nothing_is_written(release, tmp_path):
    asset = binary.release_asset_name()
    release.assets[asset] = KERNEL_BYTES + b"\n# extra\n"
    with pytest.raises(binary.ChecksumMismatchError, match="corrupt or tampered"):
        binary.download_binary(str(tmp_path))
    assert list(tmp_path.iterdir()) == []


def test_a_sidecar_disagreeing_with_the_pin_is_refused_before_the_binary_is_fetched(release, tmp_path):
    asset = binary.release_asset_name()
    other = hashlib.sha256(b"other").hexdigest()
    release.assets[asset + ".sha256"] = f"{other}  {asset}\n".encode()
    with pytest.raises(binary.ChecksumMismatchError, match="not the one this package was built against"):
        binary.download_binary(str(tmp_path))
    assert release.requests == [binary.release_download_url(RELEASE, asset + ".sha256", REPO)]


def test_a_malformed_sidecar_is_a_download_error(release, tmp_path):
    asset = binary.release_asset_name()
    release.assets[asset + ".sha256"] = b"<html>not found</html>"
    with pytest.raises(binary.DownloadError, match="SHA-256"):
        binary.download_binary(str(tmp_path))


def test_an_unreachable_release_is_a_download_error(release, tmp_path):
    with pytest.raises(binary.DownloadError, match="404"):
        binary.sidecar_digest(binary.release_download_url(RELEASE, "missing.sha256", REPO))


def test_the_repository_can_be_pointed_at_a_fork(monkeypatch):
    monkeypatch.setenv(binary.GITHUB_REPO_ENV, "someone/fork")
    assert binary.default_github_repo() == "someone/fork"
    assert binary.release_download_url("v1.0.0", "x").startswith("https://github.com/someone/fork/releases/download/v1.0.0/x")
